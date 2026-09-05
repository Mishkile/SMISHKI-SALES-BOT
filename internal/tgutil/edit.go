package tgutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	tgbot "github.com/go-telegram/bot"
	tg "github.com/go-telegram/bot/models"
)

// APIBaseURL is the Telegram Bot API endpoint used for direct calls.
var APIBaseURL = "https://api.telegram.org"

var editClient = &http.Client{Timeout: 30 * time.Second}

// EditRichMessage replaces a message's content with a rich message.
//
// go-telegram/bot's EditMessageText always serialises its text field, even
// when empty, and Telegram treats an empty text as a request in its own
// right. The call is therefore issued directly with only chat_id, message_id
// and rich_message. API failures are returned with Telegram's description so
// callers can match on it ("message to edit not found", "message is not
// modified").
func EditRichMessage(ctx context.Context, b *tgbot.Bot, chatID int64, messageID int, msg tg.InputRichMessage) error {
	body, err := json.Marshal(map[string]any{
		"chat_id":      chatID,
		"message_id":   messageID,
		"rich_message": msg,
	})
	if err != nil {
		return fmt.Errorf("editMessageText: encode: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, APIBaseURL+"/bot"+b.Token()+"/editMessageText", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := editClient.Do(req)
	if err != nil {
		return fmt.Errorf("editMessageText: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("editMessageText: read: %w", err)
	}
	var r struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return fmt.Errorf("editMessageText: decode (%d): %w", resp.StatusCode, err)
	}
	if !r.OK {
		return fmt.Errorf("editMessageText: %d %s", r.ErrorCode, r.Description)
	}
	return nil
}
