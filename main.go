// JSTS-SaleBot: a Telegram marketplace bot. Users create sale posts through a
// guided wizard, moderators approve them, and approved posts are published to
// a public group.
package main

import (
	"context"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

	tgbot "github.com/go-telegram/bot"
	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/config"
	"jsts-salebot/internal/controller"
	"jsts-salebot/internal/db"
	"jsts-salebot/internal/listen"
	"jsts-salebot/internal/locale"
	"jsts-salebot/internal/repository"
	"jsts-salebot/internal/services"
	"jsts-salebot/locales"
)

// version is shown at startup; keep in sync with docs/VERSION.md
// (or override at build time: -ldflags "-X main.version=x.y.z").
var version = "2.0.0"

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmsgprefix)
	loadDotEnv(".env")

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN is missing. Set it in your .env file.")
	}
	mongoURI := envOr("MONGO_URI", db.DefaultURI)
	configPath := envOr("CONFIG_PATH", "config.json")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	var localeFS fs.FS = locales.FS
	if dir := os.Getenv("LOCALES_DIR"); dir != "" {
		localeFS = os.DirFS(dir)
	}
	loc := locale.New(localeFS, store.Lang)

	client, err := db.Connect(ctx, mongoURI)
	if err != nil {
		log.Printf("Shutdown before MongoDB became available: %v", err)
		return
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Printf("Shutdown error: %v", err)
		}
	}()
	database := client.Database(db.DatabaseName(mongoURI))

	var ctrl *controller.Controller
	b, err := tgbot.New(token,
		tgbot.WithDefaultHandler(func(ctx context.Context, _ *tgbot.Bot, u *tg.Update) { ctrl.HandleUpdate(ctx, u) }),
		tgbot.WithErrorsHandler(func(err error) { log.Printf("[ERROR - telegram] %v", err) }),
		tgbot.WithAllowedUpdates(tgbot.AllowedUpdates{"message", "callback_query", "pre_checkout_query"}),
	)
	if err != nil {
		log.Fatalf("telegram: %v", err)
	}

	deps := &services.Deps{
		Bot:    b,
		Config: store,
		Locale: loc,
		Posts:  repository.NewPostRepository(database),
		Users:  repository.NewUserRepository(database),
		Listen: listen.NewRegistry(),
	}
	ctrl = controller.New(deps)
	ctrl.SyncSoldPosts(ctx)

	log.Printf("Bot v%s is running...", version)
	b.Start(ctx) // blocks until ctx is cancelled (SIGINT/SIGTERM)

	log.Println("Shutting down bot...")
}
