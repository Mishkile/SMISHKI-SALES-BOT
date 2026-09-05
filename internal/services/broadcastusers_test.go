package services

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	tgbot "github.com/go-telegram/bot"
)

// apiError mimics how go-telegram/bot wraps a Telegram API failure.
func apiError(sentinel error, description string) error {
	return fmt.Errorf("%w, %s", sentinel, description)
}

func TestMapSendError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{apiError(tgbot.ErrorForbidden, "Forbidden: bot was blocked by the user"), "blocked the bot"},
		{apiError(tgbot.ErrorBadRequest, "Bad Request: chat not found"), "hasn't started the bot"},
		{&tgbot.TooManyRequestsError{Message: "too many requests", RetryAfter: 5}, "rate limited"},
		{apiError(tgbot.ErrorBadRequest, "Bad Request: message text is empty"), "Bad Request: message text is empty"},
		{errors.New("Something else broke"), "Something else broke"},
	}
	for _, c := range cases {
		if got := MapSendError(c.err); got != c.want {
			t.Errorf("MapSendError(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

func TestTruncateFailures(t *testing.T) {
	failures := make([]BroadcastFailure, 80)
	for i := range failures {
		failures[i] = BroadcastFailure{UserID: strconv.Itoa(i), Reason: "blocked the bot"}
	}
	shown, remainder := TruncateFailures(failures, 30)
	if len(shown) != 30 || remainder != 50 {
		t.Fatalf("got %d shown, %d remainder", len(shown), remainder)
	}
	shown, remainder = TruncateFailures(failures[:5], 30)
	if len(shown) != 5 || remainder != 0 {
		t.Fatalf("small list: got %d shown, %d remainder", len(shown), remainder)
	}
}
