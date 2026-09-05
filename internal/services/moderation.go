package services

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbot "github.com/go-telegram/bot"
	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/listen"
	"jsts-salebot/internal/models"
	"jsts-salebot/internal/tgutil"
)

// ModerationService handles the approve/reject buttons on moderation cards.
type ModerationService struct {
	d     *Deps
	posts *PostService
	users *UserService
}

// NewModerationService wires the service.
func NewModerationService(d *Deps, posts *PostService, users *UserService) *ModerationService {
	return &ModerationService{d: d, posts: posts, users: users}
}

// HandleCallback processes approve_<id> / reject_<id>.
func (s *ModerationService) HandleCallback(ctx context.Context, q *tg.CallbackQuery) {
	isApprove := strings.HasPrefix(q.Data, "approve_")
	isReject := strings.HasPrefix(q.Data, "reject_")
	if !isApprove && !isReject {
		return
	}
	postID := strings.TrimPrefix(strings.TrimPrefix(q.Data, "approve_"), "reject_")
	adminID := tgutil.ID(q.From.ID)
	_, loc := s.d.UserAndLocale(ctx, q.From.ID)

	log.Printf("[DEBUG - ModerationService.handleCallback] adminId=%d data=%s", q.From.ID, q.Data)

	if !s.users.HasAuthLevel(ctx, adminID, models.AuthMod) {
		tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, "notAdmin"), true)
		return
	}

	post, err := s.d.Posts.FindByID(ctx, postID)
	if err != nil {
		log.Printf("[ERROR - ModerationService.handleCallback] %v", err)
		tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, "adminError"), false)
		return
	}
	if post == nil {
		tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, "adminPostNotFound"), false)
		return
	}
	if post.UserID == adminID {
		log.Printf("[WARN - ModerationService.handleCallback] Admin is moderating their own post adminId=%d postId=%s", q.From.ID, postID)
	}
	if post.Status != models.StatusPending {
		tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, "adminPostHandled"), false)
		return
	}

	author, err := s.d.Users.FindByUserID(ctx, post.UserID)
	if err != nil {
		log.Printf("[ERROR - ModerationService.handleCallback] %v", err)
		tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, "adminError"), false)
		return
	}
	if author == nil {
		tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, "adminUserNotFound"), false)
		return
	}

	if isApprove {
		err = s.handleApproval(ctx, q, postID, post, author, loc)
	} else {
		err = s.handleRejection(ctx, q, postID, post, author, loc)
	}
	if err != nil {
		log.Printf("[ERROR - ModerationService.handleCallback] %v", err)
		tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, "adminError"), false)
		return
	}

	if m := tgutil.CallbackMessage(q); m != nil {
		status := s.d.T(loc, "statusRejected")
		if isApprove {
			status = s.d.T(loc, "statusApproved")
		}
		if _, err := s.d.Bot.EditMessageText(ctx, &tgbot.EditMessageTextParams{ChatID: m.Chat.ID, MessageID: m.ID, Text: status}); err != nil {
			log.Printf("[WARN - ModerationService.handleCallback] edit status: %v", err)
		}
	}
}

func (s *ModerationService) handleApproval(ctx context.Context, q *tg.CallbackQuery, postID string, post *models.Post, author *models.User, adminLocale string) error {
	msg := s.posts.FormatPostRichMessage(DataFromPost(post, author, "User"), FormatOpts{ShowCta: true})

	messageID, err := s.posts.SendToApproved(ctx, msg)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	if err := s.d.Posts.SetApprovedMessageID(ctx, postID, &messageID); err != nil {
		return err
	}
	// Only update status in DB after the Telegram post succeeded.
	if err := s.d.Posts.UpdateStatus(ctx, postID, models.StatusApproved); err != nil {
		return err
	}

	authorID, _ := strconv.ParseInt(post.UserID, 10, 64)
	tgutil.SendLog(ctx, s.d.Bot, authorID, s.d.T(s.d.Locale.ResolveUserLocale(author), "postApproved"), tgutil.SendOpts{})
	tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(adminLocale, "adminApproved"), false)

	log.Printf("[INFO - ModerationService.handleApproval] action=APPROVED postId=%s title=%q admin=%d(%s) author=%s(%s %s)",
		postID, post.Title, q.From.ID, q.From.Username, post.UserID, models.Str(author.UserName), models.Str(author.FirstName))
	return nil
}

func (s *ModerationService) handleRejection(ctx context.Context, q *tg.CallbackQuery, postID string, post *models.Post, author *models.User, adminLocale string) error {
	tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(adminLocale, "adminRejected"), false)

	reason, err := s.askRejectReason(ctx, q)
	if err != nil {
		return err
	}

	// Only update status in DB after the reason is handled.
	if err := s.d.Posts.UpdateStatus(ctx, postID, models.StatusRejected); err != nil {
		return err
	}

	authorID, _ := strconv.ParseInt(post.UserID, 10, 64)
	authorLocale := s.d.Locale.ResolveUserLocale(author)
	if reason != "" {
		tgutil.SendLog(ctx, s.d.Bot, authorID, s.d.T(authorLocale, "postRejectedWithReason")+reason, tgutil.SendOpts{})
	} else {
		tgutil.SendLog(ctx, s.d.Bot, authorID, s.d.T(authorLocale, "postRejected"), tgutil.SendOpts{})
	}

	action := "REJECTED_WITHOUT_REASON"
	logReason := "N/A"
	if reason != "" {
		action = "REJECTED_WITH_REASON"
		logReason = reason
	}
	log.Printf("[INFO - ModerationService.handleRejection] action=%s postId=%s title=%q reason=%q admin=%d(%s) author=%s(%s %s)",
		action, postID, post.Title, logReason, q.From.ID, q.From.Username, post.UserID, models.Str(author.UserName), models.Str(author.FirstName))
	return nil
}

// askRejectReason prompts the moderator for a reason (with a Skip button) and
// waits for either a text reply in the same chat/topic or the Skip press.
func (s *ModerationService) askRejectReason(ctx context.Context, q *tg.CallbackQuery) (string, error) {
	m := tgutil.CallbackMessage(q)
	if m == nil {
		return "", nil
	}
	chatID := m.Chat.ID
	topicID := m.MessageThreadID
	adminID := q.From.ID
	lang := s.d.Lang()

	skipData := fmt.Sprintf("skip_reason_%d_%d", adminID, time.Now().UnixMilli())

	thread := 0
	if topicID != 0 && topicID != 1 {
		thread = topicID
	}
	sent, err := tgutil.Send(ctx, s.d.Bot, chatID, s.d.T(lang, "rejectReasonPrompt"), tgutil.SendOpts{
		Thread: thread,
		Markup: tgutil.Keyboard([]tg.InlineKeyboardButton{tgutil.Btn(s.d.T(lang, "skipReasonButton"), skipData)}),
	})
	if err != nil {
		return "", err
	}

	result := make(chan string, 1)
	var once sync.Once
	remove := s.d.Listen.Add(&listen.Listener{
		OnCallback: func(cb *tg.CallbackQuery) bool {
			if cb.From.ID != adminID || cb.Data != skipData {
				return false
			}
			tgutil.Answer(ctx, s.d.Bot, cb.ID, "", false)
			tgutil.ClearButtons(ctx, s.d.Bot, chatID, sent.ID)
			once.Do(func() { result <- "" })
			return true
		},
		OnMessage: func(reply *tg.Message) bool {
			if reply.Chat.ID != chatID || reply.From == nil || reply.From.ID != adminID {
				return false
			}
			if thread != 0 && reply.MessageThreadID != topicID {
				return false
			}
			if strings.HasPrefix(reply.Text, "/") {
				return false
			}
			tgutil.ClearButtons(ctx, s.d.Bot, chatID, sent.ID)
			once.Do(func() { result <- reply.Text })
			return true
		},
	})
	defer remove()

	select {
	case r := <-result:
		return r, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
