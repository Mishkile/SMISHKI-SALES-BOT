// Package services holds the bot's business logic, one service per concern,
// mirroring the original src/services/ layout.
package services

import (
	"context"
	"log"

	tgbot "github.com/go-telegram/bot"

	"jsts-salebot/internal/config"
	"jsts-salebot/internal/listen"
	"jsts-salebot/internal/locale"
	"jsts-salebot/internal/models"
	"jsts-salebot/internal/repository"
	"jsts-salebot/internal/tgutil"
)

// Deps are the shared collaborators every service receives.
type Deps struct {
	Bot    *tgbot.Bot
	Config *config.Store
	Locale *locale.Service
	Posts  *repository.PostRepository
	Users  *repository.UserRepository
	Listen *listen.Registry
}

// Lang is the configured default locale.
func (d *Deps) Lang() string { return d.Config.Lang() }

// T translates a key.
func (d *Deps) T(locale, key string, params ...locale.Params) string {
	return d.Locale.T(locale, key, params...)
}

// UserAndLocale loads a user by Telegram id and resolves their locale. Lookup
// failures are logged and treated as an unknown user.
func (d *Deps) UserAndLocale(ctx context.Context, userID int64) (*models.User, string) {
	u, err := d.Users.FindByUserID(ctx, tgutil.ID(userID))
	if err != nil {
		log.Printf("[ERROR - UserAndLocale] %v", err)
	}
	return u, d.Locale.ResolveUserLocale(u)
}

// LocaleFor resolves the locale for a Telegram user id.
func (d *Deps) LocaleFor(ctx context.Context, userID int64) string {
	_, loc := d.UserAndLocale(ctx, userID)
	return loc
}
