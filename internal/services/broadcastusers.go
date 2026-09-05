package services

import (
	"context"
	"errors"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"

	"jsts-salebot/internal/models"
	"jsts-salebot/internal/tgutil"
)

// BroadcastFailure records one recipient the DM could not reach.
type BroadcastFailure struct {
	UserID string
	Reason string
}

// BroadcastReport summarises a /broadcastUsers run.
type BroadcastReport struct {
	Total    int
	Sent     int
	Failures []BroadcastFailure
}

const sendThrottle = 50 * time.Millisecond

var chatNotFound = regexp.MustCompile(`(?i)chat not found`)

// MapSendError maps a failed sendMessage error to a short admin-facing reason.
// go-telegram/bot wraps API failures as "<sentinel>, <Telegram description>";
// anything else falls back to the error text.
func MapSendError(err error) string {
	if err == nil {
		return "unknown error"
	}
	description := err.Error()
	for _, sentinel := range []error{tgbot.ErrorForbidden, tgbot.ErrorBadRequest, tgbot.ErrorUnauthorized, tgbot.ErrorNotFound, tgbot.ErrorConflict, tgbot.ErrorTooManyRequests} {
		if errors.Is(err, sentinel) {
			description = strings.TrimPrefix(description, sentinel.Error()+", ")
			break
		}
	}

	switch {
	case errors.Is(err, tgbot.ErrorForbidden):
		return "blocked the bot"
	case errors.Is(err, tgbot.ErrorBadRequest) && chatNotFound.MatchString(description):
		return "hasn't started the bot"
	case tgbot.IsTooManyRequestsError(err) || errors.Is(err, tgbot.ErrorTooManyRequests):
		return "rate limited"
	}
	return description
}

// TruncateFailures splits a failure list into the itemised head and the
// count of the remainder, for the report's "...and N more" line.
func TruncateFailures(failures []BroadcastFailure, limit int) (shown []BroadcastFailure, remainder int) {
	if limit <= 0 {
		limit = 30
	}
	if len(failures) <= limit {
		return failures, 0
	}
	return failures[:limit], len(failures) - limit
}

// BroadcastUsersService DMs every active user and pending/approved author.
type BroadcastUsersService struct {
	d *Deps
}

// NewBroadcastUsersService wires the service.
func NewBroadcastUsersService(d *Deps) *BroadcastUsersService {
	return &BroadcastUsersService{d: d}
}

// ResolveAudience merges active-session ids with pending/approved post
// authors, de-duplicates, and drops excludeID (the initiating admin). Order is
// preserved: active users first, then pending authors, then approved authors.
func (s *BroadcastUsersService) ResolveAudience(ctx context.Context, activeIDs []string, excludeID string) ([]string, error) {
	pendingIDs, err := s.d.Posts.DistinctUserIDsByStatus(ctx, models.StatusPending)
	if err != nil {
		return nil, err
	}
	approvedIDs, err := s.d.Posts.DistinctUserIDsByStatus(ctx, models.StatusApproved)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{excludeID: true}
	var out []string
	for _, group := range [][]string{activeIDs, pendingIDs, approvedIDs} {
		for _, id := range group {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out, nil
}

// SendToMany fans the HTML message out sequentially with a small throttle.
// It never aborts on a single failure.
func (s *BroadcastUsersService) SendToMany(ctx context.Context, userIDs []string, htmlMessage string) BroadcastReport {
	report := BroadcastReport{Total: len(userIDs)}
	for _, id := range userIDs {
		chatID, err := strconv.ParseInt(id, 10, 64)
		if err == nil {
			_, err = tgutil.Send(ctx, s.d.Bot, chatID, htmlMessage, tgutil.SendOpts{HTML: true})
		}
		if err != nil {
			report.Failures = append(report.Failures, BroadcastFailure{UserID: id, Reason: MapSendError(err)})
		} else {
			report.Sent++
		}
		select {
		case <-ctx.Done():
			log.Printf("[WARN - BroadcastUsersService.sendToMany] cancelled after %d of %d", report.Sent+len(report.Failures), report.Total)
			return report
		case <-time.After(sendThrottle):
		}
	}
	return report
}
