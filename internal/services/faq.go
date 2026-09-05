package services

import (
	"context"
	"html"
	"log"
	"strings"

	tgbot "github.com/go-telegram/bot"
	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/tgutil"
)

// FaqService renders the hierarchical FAQ from <lang>/faq.json.
type FaqService struct {
	d     *Deps
	users *UserService
}

// NewFaqService wires the service.
func NewFaqService(d *Deps, users *UserService) *FaqService {
	return &FaqService{d: d, users: users}
}

// HandleFaq shows the top-level menu.
func (s *FaqService) HandleFaq(ctx context.Context, msg *tg.Message) {
	if err := s.users.EnsureUser(ctx, msg.From); err != nil {
		log.Printf("[ERROR - FaqService.handleFaq] %v", err)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(s.d.Lang(), "generalError"), tgutil.SendOpts{})
		return
	}
	loc := s.d.LocaleFor(ctx, msg.From.ID)
	s.renderNode(ctx, msg.Chat.ID, loc, "", 0)
}

// HandleCallback navigates to faq_<node> (faq_root = top level).
func (s *FaqService) HandleCallback(ctx context.Context, q *tg.CallbackQuery) {
	nodeID := strings.TrimPrefix(q.Data, "faq_")
	m := tgutil.CallbackMessage(q)
	if nodeID == "" || m == nil {
		return
	}
	loc := s.d.LocaleFor(ctx, q.From.ID)
	if nodeID == "root" {
		nodeID = ""
	}
	s.renderNode(ctx, m.Chat.ID, loc, nodeID, m.ID)
	tgutil.Answer(ctx, s.d.Bot, q.ID, "", false)
}

func truncateLabel(s string) string {
	r := []rune(s)
	if len(r) > 30 {
		return string(r[:27]) + "..."
	}
	return s
}

// renderNode sends (messageID == 0) or edits the view of one FAQ node: branch
// children become buttons, leaf children are appended to the body.
func (s *FaqService) renderNode(ctx context.Context, chatID int64, loc, nodeID string, messageID int) {
	faqs := s.d.Locale.GetFaqs(loc)
	if faqs == nil || len(faqs.Nodes) == 0 {
		tgutil.SendLog(ctx, s.d.Bot, chatID, s.d.T(loc, "faqNotAvailable"), tgutil.SendOpts{})
		return
	}

	depth := 0
	if nodeID != "" {
		depth = strings.Count(nodeID, ".") + 1
	}
	var children []string
	for _, k := range faqs.Keys {
		if nodeID == "" {
			if !strings.Contains(k, ".") {
				children = append(children, k)
			}
			continue
		}
		if strings.HasPrefix(k, nodeID+".") && strings.Count(k, ".")+1 == depth+1 {
			children = append(children, k)
		}
	}

	baseText := "<b>" + html.EscapeString(s.d.T(loc, "helpFaq")) + "</b>"
	if nodeID != "" {
		baseText = "<b>" + html.EscapeString(faqs.Nodes[nodeID]) + "</b>"
	}
	parts := []string{baseText}
	var rows [][]tg.InlineKeyboardButton

	for _, child := range children {
		isBranch := false
		for _, k := range faqs.Keys {
			if strings.HasPrefix(k, child+".") {
				isBranch = true
				break
			}
		}
		if isBranch {
			rows = append(rows, []tg.InlineKeyboardButton{tgutil.Btn(truncateLabel(faqs.Nodes[child]), "faq_"+child)})
		} else {
			parts = append(parts, html.EscapeString(faqs.Nodes[child]))
		}
	}

	text := strings.Join(parts, "\n\n")

	if nodeID != "" {
		parent := "root"
		if i := strings.LastIndex(nodeID, "."); i >= 0 {
			parent = nodeID[:i]
		}
		rows = append(rows, []tg.InlineKeyboardButton{tgutil.Btn(s.d.T(loc, "backButton"), "faq_"+parent)})
	}

	markup := tgutil.Keyboard(rows...)
	if messageID != 0 {
		params := &tgbot.EditMessageTextParams{ChatID: chatID, MessageID: messageID, Text: text, ParseMode: tg.ParseModeHTML}
		if markup != nil {
			params.ReplyMarkup = markup
		} else {
			params.ReplyMarkup = tgutil.EmptyKeyboard()
		}
		if _, err := s.d.Bot.EditMessageText(ctx, params); err != nil {
			log.Printf("[WARN - FaqService.renderNode] edit: %v", err)
		}
		return
	}
	tgutil.SendLog(ctx, s.d.Bot, chatID, text, tgutil.SendOpts{HTML: true, Markup: markup})
}
