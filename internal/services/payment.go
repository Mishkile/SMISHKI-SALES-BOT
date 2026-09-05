package services

import (
	"context"
	"encoding/json"
	"log"

	tgbot "github.com/go-telegram/bot"
	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/locale"
	"jsts-salebot/internal/tgutil"
)

// PaymentService handles Telegram Stars donations.
type PaymentService struct {
	d *Deps
}

// NewPaymentService wires the service.
func NewPaymentService(d *Deps) *PaymentService { return &PaymentService{d: d} }

// SendDonationInvoice sends a Stars invoice for amount to chatID.
func (s *PaymentService) SendDonationInvoice(ctx context.Context, chatID int64, amount int) {
	loc := s.d.LocaleFor(ctx, chatID)
	payload, _ := json.Marshal(map[string]any{"type": "donation", "amount": amount})

	_, err := s.d.Bot.SendInvoice(ctx, &tgbot.SendInvoiceParams{
		ChatID:        chatID,
		Title:         s.d.T(loc, "donateInvoiceTitle"),
		Description:   s.d.T(loc, "donateInvoiceDesc"),
		Payload:       string(payload),
		ProviderToken: "", // empty for Telegram Stars
		Currency:      "XTR",
		Prices:        []tg.LabeledPrice{{Label: "Donation", Amount: amount}},
	})
	if err != nil {
		log.Printf("[PaymentService] Error sending invoice: %v", err)
	}
}

// HandlePreCheckout always approves donation checkouts.
func (s *PaymentService) HandlePreCheckout(ctx context.Context, q *tg.PreCheckoutQuery) {
	if _, err := s.d.Bot.AnswerPreCheckoutQuery(ctx, &tgbot.AnswerPreCheckoutQueryParams{PreCheckoutQueryID: q.ID, OK: true}); err != nil {
		log.Printf("[PaymentService] Error answering pre-checkout: %v", err)
	}
}

// HandleSuccessfulPayment thanks the donor.
func (s *PaymentService) HandleSuccessfulPayment(ctx context.Context, msg *tg.Message) {
	if msg.SuccessfulPayment == nil || msg.From == nil {
		return
	}
	loc := s.d.LocaleFor(ctx, msg.From.ID)
	text := s.d.T(loc, "donationSuccess", locale.Params{"amount": msg.SuccessfulPayment.TotalAmount})
	tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, text, tgutil.SendOpts{})
}
