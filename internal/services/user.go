package services

import (
	"context"
	"log"
	"strings"

	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/models"
	"jsts-salebot/internal/repository"
	"jsts-salebot/internal/tgutil"
)

// UserService registers users on first contact and answers role checks.
type UserService struct {
	d *Deps
}

// NewUserService wires the service.
func NewUserService(d *Deps) *UserService { return &UserService{d: d} }

// EnsureUser upserts the Telegram user. Only non-empty profile fields are
// written, so a partial update never wipes data Telegram happens to omit.
func (s *UserService) EnsureUser(ctx context.Context, from *tg.User) error {
	if from == nil {
		return nil
	}
	set := repository.Fields{}
	if strings.TrimSpace(from.FirstName) != "" {
		set["firstName"] = from.FirstName
	}
	if strings.TrimSpace(from.LastName) != "" {
		set["lastName"] = from.LastName
	}
	if strings.TrimSpace(from.Username) != "" {
		set["userName"] = from.Username
	}
	if strings.TrimSpace(from.LanguageCode) != "" {
		set["languageCode"] = from.LanguageCode
	}
	_, err := s.d.Users.UpsertUserWithInsert(ctx, tgutil.ID(from.ID), set)
	return err
}

// HasAuthLevel reports whether userId holds at least level; errors count as no.
func (s *UserService) HasAuthLevel(ctx context.Context, userID string, level models.AuthLevel) bool {
	ok, err := s.d.Users.HasAuthLevel(ctx, userID, level)
	if err != nil {
		log.Printf("[ERROR - UserService.HasAuthLevel] %v", err)
		return false
	}
	return ok
}

// IsUserAdmin reports whether the user is an admin.
func (s *UserService) IsUserAdmin(ctx context.Context, userID string) bool {
	return s.HasAuthLevel(ctx, userID, models.AuthAdmin)
}
