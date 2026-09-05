package services

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/models"
	"jsts-salebot/internal/tgutil"
)

// MyPostsService implements /myposts and its sold/bump/clear buttons.
type MyPostsService struct {
	d     *Deps
	posts *PostService
}

// NewMyPostsService wires the service.
func NewMyPostsService(d *Deps, posts *PostService) *MyPostsService {
	return &MyPostsService{d: d, posts: posts}
}

func (s *MyPostsService) statusLabel(status models.PostStatus, loc string) string {
	switch status {
	case models.StatusPending:
		return s.d.T(loc, "myPostsStatusPending")
	case models.StatusApproved:
		return s.d.T(loc, "myPostsStatusApproved")
	case models.StatusRejected:
		return s.d.T(loc, "myPostsStatusRejected")
	case models.StatusSold:
		return s.d.T(loc, "myPostsStatusSold")
	}
	return string(status)
}

func (s *MyPostsService) buildSummary(posts []models.Post, loc string) (string, *tg.InlineKeyboardMarkup) {
	var b strings.Builder
	b.WriteString(s.d.T(loc, "myPostsTitle"))
	b.WriteString("\n\n")

	var hasRejected, hasSold bool
	for _, p := range posts {
		hasRejected = hasRejected || p.Status == models.StatusRejected
		hasSold = hasSold || p.Status == models.StatusSold
	}
	var row []tg.InlineKeyboardButton
	if hasRejected {
		row = append(row, tgutil.Btn(s.d.T(loc, "clearRejectedButton"), "clear_rejected"))
	}
	if hasSold {
		row = append(row, tgutil.Btn(s.d.T(loc, "clearSoldButton"), "clear_sold"))
	}

	limit := s.d.Config.Get().DailyBumpLimit
	for i, p := range posts {
		latest := ""
		if i == 0 {
			latest = fmt.Sprintf("  [%s]", s.d.T(loc, "latestPostTag"))
		}
		fmt.Fprintf(&b, "- <b>%s</b>  |  %s  |  %s%s", html.EscapeString(p.Title), html.EscapeString(p.Price), s.statusLabel(p.Status, loc), latest)
		if p.Status == models.StatusApproved {
			fmt.Fprintf(&b, "  |  %s: %d/%d", s.d.T(loc, "bumpsUsed"), p.DailyBumpCount, limit)
		}
		b.WriteString("\n")
	}
	return b.String(), tgutil.Keyboard(row)
}

// ShowPosts sends the summary plus a card for every approved post.
func (s *MyPostsService) ShowPosts(ctx context.Context, msg *tg.Message) {
	userID := tgutil.ID(msg.From.ID)
	user, loc := s.d.UserAndLocale(ctx, msg.From.ID)
	posts, err := s.d.Posts.FindByUserID(ctx, userID)
	if err != nil {
		log.Printf("[ERROR - MyPostsService.showPosts] %v", err)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "generalError"), tgutil.SendOpts{})
		return
	}
	if len(posts) == 0 {
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "myPostsEmpty"), tgutil.SendOpts{})
		return
	}

	text, markup := s.buildSummary(posts, loc)
	tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, text, tgutil.SendOpts{HTML: true, Markup: markup})

	for i := range posts {
		if posts[i].Status == models.StatusApproved {
			s.sendApprovedPostDetail(ctx, msg.Chat.ID, &posts[i], user, loc)
		}
	}
}

// sendApprovedPostDetail sends the post card with the sold/bump buttons attached.
func (s *MyPostsService) sendApprovedPostDetail(ctx context.Context, chatID int64, post *models.Post, user *models.User, loc string) {
	msg := s.posts.FormatPostRichMessage(DataFromPost(post, user, "User"), FormatOpts{})
	id := post.ID.Hex()
	markup := tgutil.Keyboard([]tg.InlineKeyboardButton{
		tgutil.Btn(s.d.T(loc, "markSoldButton"), "sold_"+id),
		tgutil.Btn(s.d.T(loc, "bumpButton"), "bump_"+id),
	})
	if _, err := tgutil.SendRich(ctx, s.d.Bot, chatID, msg, 0, markup); err != nil {
		log.Printf("[ERROR - MyPostsService.sendApprovedPostDetail] %v", err)
	}
}

func (s *MyPostsService) refreshMessage(ctx context.Context, q *tg.CallbackQuery) {
	m := tgutil.CallbackMessage(q)
	if m == nil {
		return
	}
	_, loc := s.d.UserAndLocale(ctx, q.From.ID)
	posts, err := s.d.Posts.FindByUserID(ctx, tgutil.ID(q.From.ID))
	if err != nil {
		log.Printf("[ERROR - MyPostsService.refreshMessage] %v", err)
		return
	}
	text, markup := s.buildSummary(posts, loc)
	params := &tgbot.EditMessageTextParams{ChatID: m.Chat.ID, MessageID: m.ID, Text: text, ParseMode: tg.ParseModeHTML}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	if _, err := s.d.Bot.EditMessageText(ctx, params); err != nil {
		log.Printf("[WARN - MyPostsService.refreshMessage] %v", err)
	}
}

// HandleClearStatus deletes all of the user's posts in the given status.
func (s *MyPostsService) HandleClearStatus(ctx context.Context, q *tg.CallbackQuery, status models.PostStatus) {
	userID := tgutil.ID(q.From.ID)
	_, loc := s.d.UserAndLocale(ctx, q.From.ID)

	posts, err := s.d.Posts.FindByUserID(ctx, userID)
	if err != nil {
		log.Printf("[ERROR - MyPostsService.handleClearStatus] %v", err)
		return
	}
	for _, p := range posts {
		if p.Status == status {
			if err := s.d.Posts.DeleteByID(ctx, p.ID.Hex()); err != nil {
				log.Printf("[ERROR - MyPostsService.handleClearStatus] delete %s: %v", p.ID.Hex(), err)
			}
		}
	}

	successKey := "clearSoldSuccess"
	if status == models.StatusRejected {
		successKey = "clearRejectedSuccess"
	}
	tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, successKey), false)
	s.refreshMessage(ctx, q)
}

// ownedPost loads the post behind a callback and checks it belongs to the user.
func (s *MyPostsService) ownedPost(ctx context.Context, q *tg.CallbackQuery, prefix string) (*models.Post, string, bool) {
	postID := strings.TrimPrefix(q.Data, prefix)
	post, err := s.d.Posts.FindByID(ctx, postID)
	if err != nil {
		log.Printf("[ERROR - MyPostsService] %v", err)
	}
	_, loc := s.d.UserAndLocale(ctx, q.From.ID)
	if post == nil || post.UserID != tgutil.ID(q.From.ID) {
		tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, "postNotFound"), false)
		return nil, loc, false
	}
	return post, loc, true
}

// HandleSoldCallback marks a post sold and rewrites the published message.
func (s *MyPostsService) HandleSoldCallback(ctx context.Context, q *tg.CallbackQuery) {
	post, loc, ok := s.ownedPost(ctx, q, "sold_")
	if !ok {
		return
	}
	postID := post.ID.Hex()
	if err := s.d.Posts.UpdateStatus(ctx, postID, models.StatusSold); err != nil {
		log.Printf("[ERROR - MyPostsService.handleSoldCallback] %v", err)
		tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, "generalError"), false)
		return
	}
	tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, "postMarkedSold"), false)

	// Edit the message in the approved group.
	if post.ApprovedMessageID != nil {
		author, err := s.d.Users.FindByUserID(ctx, post.UserID)
		if err != nil {
			log.Printf("[ERROR - MyPostsService.handleSoldCallback] %v", err)
		}
		msg := s.posts.FormatPostRichMessage(DataFromPost(post, author, "User"), FormatOpts{Sold: true})
		s.posts.MarkSoldInGroup(ctx, *post.ApprovedMessageID, msg)
	}

	s.refreshMessage(ctx, q)
}

func sameDay(a, b time.Time) bool {
	a, b = a.Local(), b.Local()
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

// HandleBumpCallback re-posts an approved post, subject to the daily limit.
func (s *MyPostsService) HandleBumpCallback(ctx context.Context, q *tg.CallbackQuery) {
	post, loc, ok := s.ownedPost(ctx, q, "bump_")
	if !ok {
		return
	}
	if post.Status != models.StatusApproved {
		tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, "bumpNotApproved"), false)
		return
	}

	// Reset the counter if the last bump was on a different day.
	now := time.Now()
	bumpCount := post.DailyBumpCount
	if post.LastBumpAt != nil && sameDay(*post.LastBumpAt, now) {
		if int64(bumpCount) >= s.d.Config.Get().DailyBumpLimit {
			tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, "bumpLimitReached"), true)
			return
		}
	} else {
		bumpCount = 0
	}

	// Answer immediately to avoid the callback timing out.
	tgutil.Answer(ctx, s.d.Bot, q.ID, s.d.T(loc, "bumpSuccess"), false)

	author, err := s.d.Users.FindByUserID(ctx, post.UserID)
	if err != nil {
		log.Printf("[ERROR - MyPostsService.handleBumpCallback] %v", err)
	}
	msg := s.posts.FormatPostRichMessage(DataFromPost(post, author, "User"), FormatOpts{ShowCta: true})

	postID := post.ID.Hex()
	newMessageID, err := s.posts.SendToApproved(ctx, msg)
	if err != nil {
		log.Printf("[ERROR - MyPostsService.handleBumpCallback] publish: %v", err)
	}

	if err := s.d.Posts.UpdateBump(ctx, postID, bumpCount+1); err != nil {
		log.Printf("[ERROR - MyPostsService.handleBumpCallback] %v", err)
	}
	if newMessageID != 0 {
		if err := s.d.Posts.SetApprovedMessageID(ctx, postID, &newMessageID); err != nil {
			log.Printf("[ERROR - MyPostsService.handleBumpCallback] %v", err)
		}
	}

	s.refreshMessage(ctx, q)
}
