// Package tgutil holds small helpers over the go-telegram/bot API that every
// service needs: sending text, answering callbacks, clearing inline buttons.
package tgutil

import (
	"context"
	"log"
	"strconv"

	tgbot "github.com/go-telegram/bot"
	tg "github.com/go-telegram/bot/models"
)

// ID renders a Telegram user id the way it is stored in MongoDB.
func ID(id int64) string { return strconv.FormatInt(id, 10) }

// CallbackMessage returns the message a callback button was attached to, or
// nil when Telegram reports it as inaccessible.
func CallbackMessage(q *tg.CallbackQuery) *tg.Message {
	if q == nil {
		return nil
	}
	return q.Message.Message
}

// ForwardSenderID returns the original sender of a forwarded message. Only a
// "user" origin carries the id; hidden users, chats and channels yield 0.
func ForwardSenderID(m *tg.Message) int64 {
	if m == nil || m.ForwardOrigin == nil || m.ForwardOrigin.MessageOriginUser == nil {
		return 0
	}
	return m.ForwardOrigin.MessageOriginUser.SenderUser.ID
}

// Btn makes a callback button.
func Btn(text, data string) tg.InlineKeyboardButton {
	return tg.InlineKeyboardButton{Text: text, CallbackData: data}
}

// Keyboard builds an inline keyboard, dropping empty rows. It returns nil when
// nothing is left so callers can omit reply_markup entirely.
func Keyboard(rows ...[]tg.InlineKeyboardButton) *tg.InlineKeyboardMarkup {
	var kept [][]tg.InlineKeyboardButton
	for _, r := range rows {
		if len(r) > 0 {
			kept = append(kept, r)
		}
	}
	if len(kept) == 0 {
		return nil
	}
	return &tg.InlineKeyboardMarkup{InlineKeyboard: kept}
}

// EmptyKeyboard is the reply markup that removes all inline buttons.
func EmptyKeyboard() *tg.InlineKeyboardMarkup {
	return &tg.InlineKeyboardMarkup{InlineKeyboard: [][]tg.InlineKeyboardButton{}}
}

// ClearButtons removes the inline keyboard from a message, logging failures.
func ClearButtons(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int) {
	if _, err := b.EditMessageReplyMarkup(ctx, &tgbot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   messageID,
		ReplyMarkup: EmptyKeyboard(),
	}); err != nil {
		log.Printf("[WARN - tgutil.ClearButtons] %v", err)
	}
}

// Answer acknowledges a callback query, optionally with a toast/alert.
func Answer(ctx context.Context, b *tgbot.Bot, queryID, text string, alert bool) {
	if _, err := b.AnswerCallbackQuery(ctx, &tgbot.AnswerCallbackQueryParams{
		CallbackQueryID: queryID,
		Text:            text,
		ShowAlert:       alert,
	}); err != nil {
		log.Printf("[WARN - tgutil.Answer] %v", err)
	}
}

// SendOpts are the optional parts of a text message.
type SendOpts struct {
	HTML   bool
	Thread int
	Markup *tg.InlineKeyboardMarkup
}

// Send sends a text message.
func Send(ctx context.Context, b *tgbot.Bot, chatID int64, text string, o SendOpts) (*tg.Message, error) {
	params := &tgbot.SendMessageParams{
		ChatID:          chatID,
		Text:            text,
		MessageThreadID: o.Thread,
	}
	if o.HTML {
		params.ParseMode = tg.ParseModeHTML
	}
	if o.Markup != nil {
		params.ReplyMarkup = o.Markup
	}
	return b.SendMessage(ctx, params)
}

// SendLog sends a text message and only logs a failure (fire-and-forget).
func SendLog(ctx context.Context, b *tgbot.Bot, chatID int64, text string, o SendOpts) {
	if _, err := Send(ctx, b, chatID, text, o); err != nil {
		log.Printf("[WARN - tgutil.Send] chat %d: %v", chatID, err)
	}
}

// SendRich sends a rich message with optional thread and buttons.
func SendRich(ctx context.Context, b *tgbot.Bot, chatID int64, msg tg.InputRichMessage, thread int, markup *tg.InlineKeyboardMarkup) (*tg.Message, error) {
	params := &tgbot.SendRichMessageParams{
		ChatID:          chatID,
		MessageThreadID: thread,
		RichMessage:     msg,
	}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	return b.SendRichMessage(ctx, params)
}
