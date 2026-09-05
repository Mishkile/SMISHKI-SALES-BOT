package controller

import (
	"context"
	"log"
	"strconv"
	"strings"

	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/locale"
	"jsts-salebot/internal/models"
	"jsts-salebot/internal/repository"
	"jsts-salebot/internal/services"
	"jsts-salebot/internal/testcases"
	"jsts-salebot/internal/tgutil"
)

func (c *Controller) onCallback(ctx context.Context, q *tg.CallbackQuery) {
	data := q.Data
	if data == "" {
		return
	}

	switch {
	case strings.HasPrefix(data, "test_"):
		c.handleTestCallback(ctx, q, strings.TrimPrefix(data, "test_"))
	case strings.HasPrefix(data, "faq_"):
		c.faq.HandleCallback(ctx, q)
	case strings.HasPrefix(data, "approve_"), strings.HasPrefix(data, "reject_"):
		c.moderation.HandleCallback(ctx, q)
	case data == "clear_rejected":
		c.myPosts.HandleClearStatus(ctx, q, models.StatusRejected)
	case data == "clear_sold":
		c.myPosts.HandleClearStatus(ctx, q, models.StatusSold)
	case strings.HasPrefix(data, "sold_"):
		c.myPosts.HandleSoldCallback(ctx, q)
	case strings.HasPrefix(data, "bump_"):
		c.myPosts.HandleBumpCallback(ctx, q)
	case strings.HasPrefix(data, "lang_"):
		c.handleLangCallback(ctx, q, strings.TrimPrefix(data, "lang_"))
	case strings.HasPrefix(data, "donate_"):
		c.handleDonateCallback(ctx, q, strings.TrimPrefix(data, "donate_"))
	}
}

func (c *Controller) handleLangCallback(ctx context.Context, q *tg.CallbackQuery, lang string) {
	if _, err := c.d.Users.UpdateUser(ctx, tgutil.ID(q.From.ID), repository.Fields{"preferredLocale": lang}); err != nil {
		log.Printf("[ERROR - callback_query lang] %v", err)
	}
	tgutil.Answer(ctx, c.d.Bot, q.ID, "", false)
	if m := tgutil.CallbackMessage(q); m != nil {
		// Confirm in the language the user just picked.
		tgutil.SendLog(ctx, c.d.Bot, m.Chat.ID, c.d.T(lang, "langUpdated", locale.Params{"lang": strings.ToUpper(lang)}), tgutil.SendOpts{})
	}
}

func (c *Controller) handleDonateCallback(ctx context.Context, q *tg.CallbackQuery, action string) {
	m := tgutil.CallbackMessage(q)
	if m == nil {
		return
	}
	chatID := m.Chat.ID

	tgutil.ClearButtons(ctx, c.d.Bot, chatID, m.ID)
	tgutil.Answer(ctx, c.d.Bot, q.ID, "", false)

	if action == "other" {
		c.withSession(q.From.ID, func(s *Session) { s.AwaitingDonation = true })
		tgutil.SendLog(ctx, c.d.Bot, chatID, c.d.T(c.d.Lang(), "donateEnterAmount"), tgutil.SendOpts{})
		return
	}
	if amount, err := strconv.Atoi(action); err == nil {
		c.payment.SendDonationInvoice(ctx, chatID, amount)
	}
}

func (c *Controller) handleTestCallback(ctx context.Context, q *tg.CallbackQuery, key string) {
	m := tgutil.CallbackMessage(q)
	if m == nil {
		return
	}
	tgutil.Answer(ctx, c.d.Bot, q.ID, "", false)
	tgutil.ClearButtons(ctx, c.d.Bot, m.Chat.ID, m.ID)

	fake := *m
	from := q.From
	fake.From = &from

	env := &testcases.Env{
		D:              c.d,
		Post:           c.post,
		User:           c.user,
		Payment:        c.payment,
		Input:          c.input,
		BroadcastUsers: services.NewBroadcastUsersService(c.d),
	}

	run := func(tc *testcases.Case) {
		if err := tc.Run(ctx, env, &fake); err != nil {
			log.Printf("[ERROR - test %s] %v", tc.Key, err)
			tgutil.SendLog(ctx, c.d.Bot, fake.Chat.ID, "❌ Test "+tc.Key+" failed: "+err.Error(), tgutil.SendOpts{})
		}
	}

	if key == "all" {
		for i := range testcases.Cases {
			run(&testcases.Cases[i])
		}
		return
	}
	if tc := testcases.Find(key); tc != nil {
		run(tc)
	}
}
