package services

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/listen"
	"jsts-salebot/internal/locale"
	"jsts-salebot/internal/models"
	"jsts-salebot/internal/tgutil"
)

// WizardStep is the position within the /newPost wizard, appended to prompts.
type WizardStep struct {
	Locale string
	Index  int
	Total  int
}

// InputService collects wizard input: text, price, media and confirmation.
type InputService struct {
	d *Deps
}

// NewInputService wires the service.
func NewInputService(d *Deps) *InputService { return &InputService{d: d} }

func sameAuthor(reply, msg *tg.Message) bool {
	return reply.Chat.ID == msg.Chat.ID && reply.From != nil && msg.From != nil && reply.From.ID == msg.From.ID
}

func (s *InputService) sendPrompt(ctx context.Context, msg *tg.Message, prompt string, step *WizardStep, markup *tg.InlineKeyboardMarkup) error {
	text := prompt
	if step != nil {
		text = prompt + "\n\n" + s.d.T(step.Locale, "wizardStep", locale.Params{"step": step.Index, "total": step.Total})
	}
	_, err := tgutil.Send(ctx, s.d.Bot, msg.Chat.ID, text, tgutil.SendOpts{Thread: msg.MessageThreadID, Markup: markup})
	return err
}

// Input waits for the next text message from the same user in the same chat.
// Commands are ignored, and non-text messages (photo, sticker...) get an
// "expected text" nudge instead of silently advancing the wizard.
func (s *InputService) Input(ctx context.Context, msg *tg.Message, loc string) (string, error) {
	if loc == "" {
		loc = s.d.Lang()
	}
	reply, err := s.d.Listen.WaitMessage(ctx, func(reply *tg.Message) bool {
		if !sameAuthor(reply, msg) {
			return false
		}
		if strings.HasPrefix(reply.Text, "/") {
			return false
		}
		if strings.TrimSpace(reply.Text) == "" {
			tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "expectedText"), tgutil.SendOpts{Thread: msg.MessageThreadID})
			return false
		}
		return true
	})
	if err != nil {
		return "", err
	}
	return reply.Text, nil
}

// InputWithPrompt sends prompt, then waits for a text answer.
func (s *InputService) InputWithPrompt(ctx context.Context, msg *tg.Message, prompt string, step *WizardStep) (string, error) {
	if err := s.sendPrompt(ctx, msg, prompt, step, nil); err != nil {
		return "", err
	}
	loc := ""
	if step != nil {
		loc = step.Locale
	}
	return s.Input(ctx, msg, loc)
}

var separators = regexp.MustCompile(`[,\s]`)

// ValidatePriceValue accepts any text when validation is off; otherwise the
// input must be a positive number, thousands separators allowed ("2,000").
func (s *InputService) ValidatePriceValue(priceInput string) bool {
	if !s.d.Config.Get().ValidatePrice {
		return true
	}
	price, err := strconv.ParseFloat(separators.ReplaceAllString(priceInput, ""), 64)
	return err == nil && price > 0
}

// InputPrice prompts for a price and re-asks until it validates.
func (s *InputService) InputPrice(ctx context.Context, msg *tg.Message, step WizardStep) (string, error) {
	if err := s.sendPrompt(ctx, msg, s.d.T(step.Locale, "enterPrice"), &step, nil); err != nil {
		return "", err
	}
	for {
		priceInput, err := s.Input(ctx, msg, step.Locale)
		if err != nil {
			return "", err
		}
		if s.ValidatePriceValue(priceInput) {
			return priceInput, nil
		}
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(step.Locale, "invalidPrice"), tgutil.SendOpts{Thread: msg.MessageThreadID})
	}
}

// PromptMedia asks for photos/videos and collects them until the user presses
// the Done button.
func (s *InputService) PromptMedia(ctx context.Context, msg *tg.Message, step WizardStep) ([]models.MediaItem, error) {
	userID := msg.From.ID
	doneData := fmt.Sprintf("done_media_%d", userID)

	var mu sync.Mutex
	items := []models.MediaItem{}
	done := make(chan struct{})
	var once sync.Once

	// Register before prompting so a photo sent immediately is not lost.
	remove := s.d.Listen.Add(&listen.Listener{
		OnMessage: func(reply *tg.Message) bool {
			if !sameAuthor(reply, msg) {
				return false
			}
			mu.Lock()
			defer mu.Unlock()
			if len(reply.Photo) > 0 {
				items = append(items, models.MediaItem{FileID: reply.Photo[len(reply.Photo)-1].FileID, Type: models.MediaPhoto})
			} else if reply.Video != nil {
				items = append(items, models.MediaItem{FileID: reply.Video.FileID, Type: models.MediaVideo})
			}
			return false
		},
		OnCallback: func(q *tg.CallbackQuery) bool {
			if q.Data != doneData || q.From.ID != userID {
				return false
			}
			tgutil.Answer(ctx, s.d.Bot, q.ID, "", false)
			if m := tgutil.CallbackMessage(q); m != nil {
				tgutil.ClearButtons(ctx, s.d.Bot, m.Chat.ID, m.ID)
			}
			once.Do(func() { close(done) })
			return true
		},
	})
	defer remove()

	markup := tgutil.Keyboard([]tg.InlineKeyboardButton{tgutil.Btn(s.d.T(step.Locale, "doneMediaButton"), doneData)})
	if err := s.sendPrompt(ctx, msg, s.d.T(step.Locale, "enterMedia"), &step, markup); err != nil {
		return nil, err
	}

	select {
	case <-done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	mu.Lock()
	defer mu.Unlock()
	return append([]models.MediaItem(nil), items...), nil
}

// ConfirmAction shows the preview with Confirm/Cancel buttons attached (one
// message, so there is no visible gap) and reports the user's choice.
func (s *InputService) ConfirmAction(ctx context.Context, msg *tg.Message, loc string, preview *tg.InputRichMessage) (bool, error) {
	userID := msg.From.ID
	now := time.Now().UnixMilli()
	confirmID := fmt.Sprintf("confirm_%d_%d", userID, now)
	cancelID := fmt.Sprintf("cancel_%d_%d", userID, now)

	markup := tgutil.Keyboard([]tg.InlineKeyboardButton{
		tgutil.Btn(s.d.T(loc, "confirmButton"), confirmID),
		tgutil.Btn(s.d.T(loc, "cancelButton"), cancelID),
	})

	var sent *tg.Message
	var err error
	if preview != nil {
		sent, err = tgutil.SendRich(ctx, s.d.Bot, msg.Chat.ID, *preview, msg.MessageThreadID, markup)
	} else {
		sent, err = tgutil.Send(ctx, s.d.Bot, msg.Chat.ID, "👆", tgutil.SendOpts{Thread: msg.MessageThreadID, Markup: markup})
	}
	if err != nil {
		return false, err
	}

	q, err := s.d.Listen.WaitCallback(ctx, func(q *tg.CallbackQuery) bool {
		return q.From.ID == userID && (q.Data == confirmID || q.Data == cancelID)
	})
	if err != nil {
		return false, err
	}
	tgutil.Answer(ctx, s.d.Bot, q.ID, "", false)
	tgutil.ClearButtons(ctx, s.d.Bot, msg.Chat.ID, sent.ID)
	return q.Data == confirmID, nil
}
