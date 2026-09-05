package services

import (
	"context"
	"fmt"
	"log"
	"strings"

	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/config"
	"jsts-salebot/internal/locale"
	"jsts-salebot/internal/models"
	"jsts-salebot/internal/tgutil"
)

const pendingDisplayLimit = 10

// PendingService implements /pending and /clearpending.
type PendingService struct {
	d     *Deps
	posts *PostService
}

// NewPendingService wires the service.
func NewPendingService(d *Deps, posts *PostService) *PendingService {
	return &PendingService{d: d, posts: posts}
}

// HandlePending lists posts awaiting review with approve/reject buttons.
// Access: MOD.
func (s *PendingService) HandlePending(ctx context.Context, msg *tg.Message) {
	log.Printf("[DEBUG - pendingService.handlePending] userId=%d chatId=%d", msg.From.ID, msg.Chat.ID)
	_, loc := s.d.UserAndLocale(ctx, msg.From.ID)
	cfg := s.d.Config.Get()

	// Inside the moderation group, answer in the moderation topic.
	thread := msg.MessageThreadID
	if msg.Chat.ID == cfg.ModerationGroupID {
		thread = config.Thread(cfg.ModerationTopicID)
	}
	if thread == 1 {
		thread = 0
	}
	opts := tgutil.SendOpts{HTML: true, Thread: thread}

	ok, err := s.d.Users.HasAuthLevel(ctx, tgutil.ID(msg.From.ID), models.AuthMod)
	if err != nil {
		log.Printf("[ERROR - PendingService.handlePending] %v", err)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "adminError"), tgutil.SendOpts{Thread: thread})
		return
	}
	if !ok {
		log.Printf("[WARN - PendingService.handlePending] non-admin attempted access userId=%d", msg.From.ID)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "notAdmin"), opts)
		return
	}

	pending, err := s.d.Posts.GetPendingPosts(ctx)
	if err != nil {
		log.Printf("[ERROR - PendingService.handlePending] %v", err)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "adminError"), tgutil.SendOpts{Thread: thread})
		return
	}
	if len(pending) == 0 {
		log.Printf("[INFO - handlePending] no pending posts available")
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "adminPendingEmpty"), opts)
		return
	}

	tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "adminPendingTitle"), opts)

	// Limit display to avoid hitting Telegram message size limits.
	display := pending
	if len(display) > pendingDisplayLimit {
		display = display[:pendingDisplayLimit]
	}
	chatIDStr := strings.TrimPrefix(fmt.Sprint(cfg.ModerationGroupID), "-100")

	for i := range display {
		post := &display[i]
		author, err := s.d.Users.FindByUserID(ctx, post.UserID)
		if err != nil {
			log.Printf("[ERROR - PendingService.handlePending] %v", err)
		}

		fo := FormatOpts{}
		if post.ModerationMessageID != nil {
			fo.Link = &Link{
				Label: s.d.T(loc, "adminPendingLink"),
				URL:   fmt.Sprintf("https://t.me/c/%s/%d", chatIDStr, *post.ModerationMessageID),
			}
		}
		card := s.posts.FormatPostRichMessage(DataFromPost(post, author, "Unknown"), fo)

		// One Rich Message with the approve/reject buttons attached.
		id := post.ID.Hex()
		markup := tgutil.Keyboard([]tg.InlineKeyboardButton{
			tgutil.Btn(s.d.T(loc, "approveButton"), "approve_"+id),
			tgutil.Btn(s.d.T(loc, "rejectButton"), "reject_"+id),
		})
		if _, err := tgutil.SendRich(ctx, s.d.Bot, msg.Chat.ID, card, thread, markup); err != nil {
			log.Printf("[ERROR - PendingService.handlePending] send card %s: %v", id, err)
		}
	}

	if len(pending) > pendingDisplayLimit {
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "pendingMore", locale.Params{"n": len(pending) - pendingDisplayLimit}), opts)
	}
}

// HandleClearPending rejects every pending post. Access: MOD.
func (s *PendingService) HandleClearPending(ctx context.Context, msg *tg.Message) {
	_, loc := s.d.UserAndLocale(ctx, msg.From.ID)

	ok, err := s.d.Users.HasAuthLevel(ctx, tgutil.ID(msg.From.ID), models.AuthMod)
	if err != nil {
		log.Printf("[ERROR - PendingService.handleClearPending] %v", err)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "adminError"), tgutil.SendOpts{})
		return
	}
	if !ok {
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "notAdmin"), tgutil.SendOpts{})
		return
	}

	if _, err := s.d.Posts.ExpireAllPendingPosts(ctx); err != nil {
		log.Printf("[ERROR - PendingService.handleClearPending] %v", err)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "adminError"), tgutil.SendOpts{})
		return
	}
	tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "adminClearPendingSuccess"), tgutil.SendOpts{})
}
