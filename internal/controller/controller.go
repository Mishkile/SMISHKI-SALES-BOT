// Package controller routes Telegram updates to the services, owns the
// per-user sessions and the startup sync, mirroring botController.ts.
package controller

import (
	"context"
	"log"
	"strconv"
	"strings"
	"sync"

	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/models"
	"jsts-salebot/internal/services"
	"jsts-salebot/internal/tgutil"
)

// Session is the in-memory conversational state of one user.
type Session struct {
	IsIdle           bool
	AwaitingDonation bool

	cancel context.CancelFunc
	gen    uint64
}

// Controller wires every service and dispatches updates.
type Controller struct {
	d *services.Deps

	mu       sync.Mutex
	sessions map[int64]*Session

	input          *services.InputService
	post           *services.PostService
	moderation     *services.ModerationService
	user           *services.UserService
	myPosts        *services.MyPostsService
	admin          *services.AdminService
	pending        *services.PendingService
	payment        *services.PaymentService
	faq            *services.FaqService
	broadcastUsers *services.BroadcastUsersService
}

// New builds the controller and its services.
func New(d *services.Deps) *Controller {
	c := &Controller{d: d, sessions: map[int64]*Session{}}
	c.input = services.NewInputService(d)
	c.post = services.NewPostService(d)
	c.user = services.NewUserService(d)
	c.moderation = services.NewModerationService(d, c.post, c.user)
	c.myPosts = services.NewMyPostsService(d, c.post)
	c.admin = services.NewAdminService(d, c.user)
	c.pending = services.NewPendingService(d, c.post)
	c.payment = services.NewPaymentService(d)
	c.faq = services.NewFaqService(d, c.user)
	c.broadcastUsers = services.NewBroadcastUsersService(d)
	return c
}

// --- sessions ---------------------------------------------------------------

func (c *Controller) sessionLocked(userID int64) *Session {
	s, ok := c.sessions[userID]
	if !ok {
		s = &Session{IsIdle: true}
		c.sessions[userID] = s
	}
	return s
}

func (c *Controller) withSession(userID int64, fn func(*Session)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn(c.sessionLocked(userID))
}

// activeUserIDs lists users currently inside a wizard.
func (c *Controller) activeUserIDs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	var ids []string
	for id, s := range c.sessions {
		if !s.IsIdle {
			ids = append(ids, strconv.FormatInt(id, 10))
		}
	}
	return ids
}

// beginWizard marks the user busy and returns a context for the wizard. A
// wizard already running for the same user is cancelled so the two cannot
// compete for the user's replies. The returned end func restores the idle
// state (only if this wizard is still the active one) and releases the context.
func (c *Controller) beginWizard(ctx context.Context, userID int64) (context.Context, func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sessionLocked(userID)
	if s.cancel != nil {
		s.cancel()
	}
	wctx, cancel := context.WithCancel(ctx)
	s.gen++
	gen := s.gen
	s.cancel = cancel
	s.IsIdle = false
	end := func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if s.gen == gen {
			s.cancel = nil
			s.IsIdle = true
		}
		cancel()
	}
	return wctx, end
}

// --- startup ----------------------------------------------------------------

// SyncSoldPosts re-applies the sold rendering to every sold post's published
// message, clearing references to messages that no longer exist.
func (c *Controller) SyncSoldPosts(ctx context.Context) {
	sold, err := c.d.Posts.GetSold(ctx)
	if err != nil {
		log.Printf("[ERROR - syncSoldPosts] %v", err)
		return
	}
	synced := 0
	for i := range sold {
		post := &sold[i]
		author, err := c.d.Users.FindByUserID(ctx, post.UserID)
		if err != nil {
			log.Printf("[ERROR - syncSoldPosts] %v", err)
			continue
		}
		msg := c.post.FormatPostRichMessage(services.DataFromPost(post, author, "User"), services.FormatOpts{Sold: true})
		if post.ApprovedMessageID != nil && c.post.MarkSoldInGroup(ctx, *post.ApprovedMessageID, msg) {
			synced++
			continue
		}
		log.Printf("[WARN - syncSoldPosts] Failed to sync sold status for post %s. Clearing approvedMessageId.", post.ID.Hex())
		if err := c.d.Posts.SetApprovedMessageID(ctx, post.ID.Hex(), nil); err != nil {
			log.Printf("[ERROR - syncSoldPosts] %v", err)
		}
	}
	log.Printf("Synced %d/%d sold post(s) in approved group.", synced, len(sold))
}

// --- dispatch ---------------------------------------------------------------

// HandleUpdate is the single entry point for every Telegram update.
func (c *Controller) HandleUpdate(ctx context.Context, u *tg.Update) {
	// Wizard steps waiting for input get first look, like the dynamic
	// listeners of the original implementation.
	c.d.Listen.Dispatch(u)

	switch {
	case u.Message != nil:
		c.onMessage(ctx, u.Message)
	case u.CallbackQuery != nil:
		c.onCallback(ctx, u.CallbackQuery)
	case u.PreCheckoutQuery != nil:
		c.payment.HandlePreCheckout(ctx, u.PreCheckoutQuery)
	}
}

// parseCommand splits "/Cmd@bot rest of line" into ("cmd", "rest of line").
func parseCommand(text string) (cmd, args string, ok bool) {
	if !strings.HasPrefix(text, "/") {
		return "", "", false
	}
	rest := text[1:]
	name := rest
	if i := strings.IndexAny(rest, " \t\r\n"); i >= 0 {
		name = rest[:i]
		args = rest[i+1:]
	}
	if at := strings.Index(name, "@"); at >= 0 {
		name = name[:at]
	}
	if name == "" {
		return "", "", false
	}
	return strings.ToLower(name), args, true
}

func (c *Controller) onMessage(ctx context.Context, msg *tg.Message) {
	// 1. Successful payment.
	if msg.SuccessfulPayment != nil {
		c.payment.HandleSuccessfulPayment(ctx, msg)
		return
	}

	// 2. Replies in the approved group (buyer to seller).
	if msg.Chat.ID == c.d.Config.Get().ApprovedGroupID && msg.ReplyToMessage != nil {
		c.post.HandlePublicReply(ctx, msg)
	}

	if msg.From == nil {
		return
	}

	// 3. Custom donation amount.
	if c.handleDonationAmount(ctx, msg) {
		return
	}

	cmd, args, ok := parseCommand(msg.Text)
	if !ok {
		return
	}
	c.route(ctx, msg, cmd, args)
}

// leadingInt mimics JavaScript's parseInt: an optional sign and leading digits.
func leadingInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	start := i
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == start {
		return 0, false
	}
	n, err := strconv.Atoi(s[:i])
	return n, err == nil
}

func (c *Controller) handleDonationAmount(ctx context.Context, msg *tg.Message) bool {
	var awaiting bool
	c.withSession(msg.From.ID, func(s *Session) { awaiting = s.AwaitingDonation })
	if !awaiting || msg.Text == "" {
		return false
	}
	// Ignore commands so we don't block /cancel or /start.
	if strings.HasPrefix(msg.Text, "/") {
		c.withSession(msg.From.ID, func(s *Session) { s.AwaitingDonation = false })
		return false
	}
	amount, ok := leadingInt(msg.Text)
	if ok && amount > 0 {
		c.withSession(msg.From.ID, func(s *Session) { s.AwaitingDonation = false })
		c.payment.SendDonationInvoice(ctx, msg.Chat.ID, amount)
	} else {
		tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(c.d.Lang(), "donateInvalidAmount"), tgutil.SendOpts{})
	}
	return true
}

func (c *Controller) route(ctx context.Context, msg *tg.Message, cmd, args string) {
	isPrivate := msg.Chat.Type == tg.ChatTypePrivate
	cfg := c.d.Config.Get()
	userID := tgutil.ID(msg.From.ID)

	switch cmd {
	case "start":
		if isPrivate {
			c.handleStart(ctx, msg)
		}
	case "newpost":
		if isPrivate {
			c.handleNewPost(ctx, msg)
		}
	case "myposts":
		if isPrivate {
			c.myPosts.ShowPosts(ctx, msg)
		}
	case "help":
		if isPrivate {
			c.showHelp(ctx, msg)
		}
	case "lang":
		if isPrivate {
			c.handleLang(ctx, msg)
		}
	case "faq":
		if isPrivate && cfg.FaqOn() {
			c.faq.HandleFaq(ctx, msg)
		}
	case "config":
		if isPrivate {
			c.admin.HandleConfig(ctx, msg, args)
		}
	case "activeusers":
		if isPrivate {
			c.handleActiveUsers(ctx, msg)
		}
	case "pending":
		if !c.user.HasAuthLevel(ctx, userID, models.AuthMod) {
			tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(cfg.Lang, "notAdmin"), tgutil.SendOpts{})
			return
		}
		c.pending.HandlePending(ctx, msg)
	case "clearpending":
		if !isPrivate {
			return
		}
		if !c.user.HasAuthLevel(ctx, userID, models.AuthMod) {
			tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(cfg.Lang, "notAdmin"), tgutil.SendOpts{})
			return
		}
		c.pending.HandleClearPending(ctx, msg)
	case "broadcast":
		if isPrivate {
			c.admin.HandleBroadcast(ctx, msg, args)
		}
	case "broadcastusers":
		if isPrivate {
			c.handleBroadcastUsers(ctx, msg, args)
		}
	case "promote":
		if isPrivate {
			c.admin.HandlePromote(ctx, msg, args)
		}
	case "demote":
		if isPrivate {
			c.admin.HandleDemote(ctx, msg, args)
		}
	case "auth":
		if isPrivate {
			c.admin.HandleAuth(ctx, msg, args)
		}
	case "test":
		if isPrivate {
			c.handleTest(ctx, msg)
		}
	case "donate":
		if isPrivate {
			c.handleDonate(ctx, msg)
		}
	}
}
