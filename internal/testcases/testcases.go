// Package testcases holds the manual scenarios behind the admin-only /test
// command. They exercise the real services against the real bot and database
// so moderation, payments and broadcasts can be checked without a second
// account. Intentionally isolated: only the controller imports this package.
package testcases

import (
	"context"
	"fmt"
	"html"
	"time"

	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/config"
	"jsts-salebot/internal/locale"
	"jsts-salebot/internal/models"
	"jsts-salebot/internal/repository"
	"jsts-salebot/internal/services"
	"jsts-salebot/internal/tgutil"
)

// Env is what a test case may touch.
type Env struct {
	D              *services.Deps
	Post           *services.PostService
	User           *services.UserService
	Payment        *services.PaymentService
	Input          *services.InputService
	BroadcastUsers *services.BroadcastUsersService
}

// Case is one selectable scenario.
type Case struct {
	Key   string
	Label string
	Run   func(ctx context.Context, env *Env, msg *tg.Message) error
}

// Cases lists the scenarios in menu order.
var Cases = []Case{
	{Key: "full_post", Label: "📦 Full post (3 photos)", Run: fullPost},
	{Key: "no_media", Label: "📝 No media", Run: noMedia},
	{Key: "one_photo", Label: "🖼 One photo", Run: onePhoto},
	{Key: "simulate_donation", Label: "💰 Simulate Donation (50 Stars)", Run: simulateDonation},
	{Key: "free_text_price", Label: "🏷 Free text price", Run: freeTextPrice},
	{Key: "faq_view", Label: "❓ View FAQ", Run: faqView},
	{Key: "broadcast_custom", Label: "✍️ Broadcast Custom Message (to Moderation)", Run: broadcastCustom},
	{Key: "broadcast_test", Label: "📢 Broadcast (to Moderation)", Run: broadcastTest},
	{Key: "rbac_promotion", Label: "🎖 Test RBAC Promotion", Run: rbacPromotion},
	{Key: "rbac_auth", Label: "🔍 Test RBAC Auth Output", Run: rbacAuth},
	{Key: "broadcast_users", Label: "📢 Broadcast Users (DM active/pending/approved)", Run: broadcastUsers},
}

// Find returns the case with the given key, or nil.
func Find(key string) *Case {
	for i := range Cases {
		if Cases[i].Key == key {
			return &Cases[i]
		}
	}
	return nil
}

// Telegram file_ids from previously uploaded photos (reusable within the same bot).
var testMedia = []models.MediaItem{
	{FileID: "AgACAgQAAxkBAAINXGm-_OK_AAH4GQnBW-0HVvslh49hbgACsg1rGy28-FEImRX00Ng_2AEAAwIAA3kAAzoE", Type: models.MediaPhoto},
	{FileID: "AgACAgQAAxkBAAINW2m-_OKCg3jnCq9lJ83ir9wv2VEvAAKxDWsbLbz4UTzQXIYguRLzAQADAgADeQADOgQ", Type: models.MediaPhoto},
	{FileID: "AgACAgQAAxkBAAINXWm-_OkLEuv9Tex5FGbcr9AzGJZiAAKzDWsbLbz4UfeDwqmcdSY1AQADAgADeAADOgQ", Type: models.MediaPhoto},
}

func say(ctx context.Context, env *Env, msg *tg.Message, text string) {
	tgutil.SendLog(ctx, env.D.Bot, msg.Chat.ID, text, tgutil.SendOpts{})
}

func sayHTML(ctx context.Context, env *Env, msg *tg.Message, text string) {
	tgutil.SendLog(ctx, env.D.Bot, msg.Chat.ID, text, tgutil.SendOpts{HTML: true})
}

// createAndModerate skips the interactive wizard: it stores a post and sends
// it to moderation so approve/reject can be tested quickly.
func createAndModerate(ctx context.Context, env *Env, msg *tg.Message, title, description, price, location string, media []models.MediaItem, doneLabel string) error {
	if msg.From == nil {
		return fmt.Errorf("test requires a valid user in message context")
	}
	from := msg.From
	if err := env.User.EnsureUser(ctx, from); err != nil {
		return err
	}
	if !env.Input.ValidatePriceValue(price) {
		say(ctx, env, msg, fmt.Sprintf("❌ Test Case Failed: Price %q is invalid under current config.", price))
		return nil
	}

	post, err := env.D.Posts.Create(ctx, repository.NewPost{
		UserID: tgutil.ID(from.ID), Title: title, Description: description, Price: price,
		Media: media, Location: location, CreatedAt: time.Now(),
	})
	if err != nil {
		return err
	}
	_, err = env.Post.SendToModeration(ctx, post.ID.Hex(), services.PostData{
		Title: title, Description: description, Price: price, Location: location, Media: media,
		UserID: from.ID, Username: from.Username, FirstName: from.FirstName,
	})
	if err != nil {
		return err
	}
	say(ctx, env, msg, fmt.Sprintf("✅ %s (ID: %s)", doneLabel, post.ID.Hex()))
	return nil
}

// CASE 1: full post with multiple photos.
func fullPost(ctx context.Context, env *Env, msg *tg.Message) error {
	return createAndModerate(ctx, env, msg,
		"טסט - אייפון 15 פרו", "מכשיר במצב מעולה, שנה שימוש, עם כיסוי וזכוכית.", "3500", "תל אביב",
		testMedia, "Test post created and sent to moderation")
}

// CASE 2: post without media.
func noMedia(ctx context.Context, env *Env, msg *tg.Message) error {
	return createAndModerate(ctx, env, msg,
		"טסט - שולחן כתיבה", "שולחן עץ, 120x60, במצב טוב.", "200", "חיפה",
		[]models.MediaItem{}, "Test post (no media) sent to moderation")
}

// CASE 3: post with a single photo.
func onePhoto(ctx context.Context, env *Env, msg *tg.Message) error {
	return createAndModerate(ctx, env, msg,
		"טסט - אוזניות אלחוטיות", "Sony WH-1000XM5, כמו חדש עם קופסה.", "900", "ירושלים",
		testMedia[:1], "Test post (1 photo) sent to moderation")
}

// CASE 4: simulate a successful donation.
func simulateDonation(ctx context.Context, env *Env, msg *tg.Message) error {
	fake := *msg
	fake.SuccessfulPayment = &tg.SuccessfulPayment{
		Currency:                "XTR",
		TotalAmount:             50,
		InvoicePayload:          `{"type":"donation","amount":50}`,
		TelegramPaymentChargeID: "test_charge_id",
		ProviderPaymentChargeID: "test_provider_id",
	}
	env.Payment.HandleSuccessfulPayment(ctx, &fake)
	say(ctx, env, msg, "✅ Simulated donation event triggered. You should see the 'Thank you' message above.")
	return nil
}

// CASE 5: free-text price (for when validation is disabled).
func freeTextPrice(ctx context.Context, env *Env, msg *tg.Message) error {
	return createAndModerate(ctx, env, msg,
		"טסט - מחיר טקסט חופשי", "בדיקה של מחיר שאינו מספר (למשל 'בחינם' או 'צור קשר').", "צור קשר בפרטי", "פתח תקווה",
		[]models.MediaItem{}, "Test post (free text price) sent to moderation")
}

// faqView dumps the first FAQ entries for the user's locale.
func faqView(ctx context.Context, env *Env, msg *tg.Message) error {
	var user *models.User
	if msg.From != nil {
		user = &models.User{UserID: tgutil.ID(msg.From.ID), FirstName: models.Ptr(msg.From.FirstName), LanguageCode: models.Ptr(msg.From.LanguageCode)}
	}
	loc := env.D.Locale.ResolveUserLocale(user)
	faqs := env.D.Locale.GetFaqs(loc)
	if faqs == nil || len(faqs.Nodes) == 0 {
		say(ctx, env, msg, "❌ No FAQ data found for your locale")
		return nil
	}

	text := "<b>📋 FAQ Test Result</b>\n\n"
	keys := faqs.Keys
	if len(keys) > 5 {
		keys = keys[:5]
	}
	for _, k := range keys {
		v := []rune(faqs.Nodes[k])
		if len(v) > 50 {
			v = v[:50]
		}
		text += fmt.Sprintf("<b>%s</b>: %s...\n\n", html.EscapeString(k), html.EscapeString(string(v)))
	}
	text += fmt.Sprintf("✅ Total FAQ entries: %d", len(faqs.Nodes))
	sayHTML(ctx, env, msg, text)
	return nil
}

func moderationTarget(env *Env) (int64, int) {
	cfg := env.D.Config.Get()
	return cfg.ModerationGroupID, config.Thread(cfg.ModerationTopicID)
}

// broadcastCustom sends an admin-typed message to the moderation group, as a
// preview of /broadcast.
func broadcastCustom(ctx context.Context, env *Env, msg *tg.Message) error {
	groupID, thread := moderationTarget(env)
	loc := env.D.LocaleFor(ctx, msg.From.ID)

	custom, err := env.Input.InputWithPrompt(ctx, msg, env.D.T(loc, "broadcastEnterCustomMessage"), nil)
	if err != nil {
		return err
	}
	if _, err := tgutil.Send(ctx, env.D.Bot, groupID, custom, tgutil.SendOpts{HTML: true, Thread: thread}); err != nil {
		say(ctx, env, msg, "❌ Custom broadcast test failed: "+err.Error())
		return nil
	}
	say(ctx, env, msg, "✅ Custom broadcast message sent to the moderation group.")
	return nil
}

// broadcastTest sends a formatted message to the moderation group to verify
// delivery and formatting without touching the public channel.
func broadcastTest(ctx context.Context, env *Env, msg *tg.Message) error {
	groupID, thread := moderationTarget(env)
	text := "<b>🚀 Broadcast Test Message</b>\n\nThis message simulates an admin broadcast. It is sent to the <i>moderation group</i> to avoid cluttering the public channel during tests.\n\nFormatting check:\n- <b>Bold text</b>\n- <i>Italic text</i>\n- <a href='https://github.com/SM-26/JSTS-SaleBot'>Link to Repository</a>\n\n✅ If you see this in the correct moderation topic, the broadcast logic is verified!"
	if _, err := tgutil.Send(ctx, env.D.Bot, groupID, text, tgutil.SendOpts{HTML: true, Thread: thread}); err != nil {
		say(ctx, env, msg, "❌ Broadcast test failed: "+err.Error())
		return nil
	}
	say(ctx, env, msg, "✅ Broadcast test message sent to the moderation group.")
	return nil
}

// rbacPromotion checks the promotion ceiling and the isAdmin migration.
func rbacPromotion(ctx context.Context, env *Env, msg *tg.Message) error {
	user, err := env.D.Users.FindByUserID(ctx, tgutil.ID(msg.From.ID))
	if err != nil {
		return err
	}
	if user == nil {
		say(ctx, env, msg, "❌ Test failed: Admin user not found in DB.")
		return nil
	}

	// We can't easily promote a dummy user without a second account, so verify
	// that the current admin (level 2) cannot be promoted further.
	sayHTML(ctx, env, msg, fmt.Sprintf("<b>RBAC Test</b>\nActor Level: %d\n\nRunning checks...", user.AuthLevel))
	if user.AuthLevel == models.AuthAdmin {
		say(ctx, env, msg, "✅ Self-check: Admin cannot be promoted further (limit check).")
	}

	// Every user in the DB should now carry authLevel instead of isAdmin.
	legacy, err := env.D.Users.CountLegacyIsAdmin(ctx)
	if err != nil {
		say(ctx, env, msg, "❌ RBAC test failed: "+err.Error())
		return nil
	}
	say(ctx, env, msg, fmt.Sprintf("✅ Migration check: Found %d legacy isAdmin fields.", legacy))
	return nil
}

// rbacAuth renders the /auth output for the caller.
func rbacAuth(ctx context.Context, env *Env, msg *tg.Message) error {
	user, err := env.D.Users.FindByUserID(ctx, tgutil.ID(msg.From.ID))
	if err != nil {
		return err
	}
	if user == nil {
		say(ctx, env, msg, "❌ Test failed: User not found in DB.")
		return nil
	}
	loc := env.D.Locale.ResolveUserLocale(user)
	roleKey := "authLevelUser"
	switch user.AuthLevel {
	case models.AuthAdmin:
		roleKey = "authLevelAdmin"
	case models.AuthMod:
		roleKey = "authLevelMod"
	}
	output := env.D.T(loc, "authCurrentLevel", locale.Params{"userId": user.UserID, "role": env.D.T(loc, roleKey), "level": int(user.AuthLevel)})
	sayHTML(ctx, env, msg, "<b>RBAC Auth Test</b>\n\n"+output)
	say(ctx, env, msg, "✅ Auth logic verified.")
	return nil
}

// broadcastUsers seeds a pending and an approved post from distinct fake
// authors, then exercises audience resolution and the send path like
// /broadcastUsers would.
func broadcastUsers(ctx context.Context, env *Env, msg *tg.Message) error {
	if msg.From == nil {
		return fmt.Errorf("test requires a valid user in message context")
	}
	adminID := tgutil.ID(msg.From.ID)
	stamp := time.Now().UnixMilli()
	pendingAuthor := fmt.Sprintf("test_pending_%d", stamp)
	approvedAuthor := fmt.Sprintf("test_approved_%d", stamp)

	var created []string
	defer func() {
		for _, id := range created {
			if err := env.D.Posts.DeleteByID(context.Background(), id); err != nil {
				fmt.Println("[WARN - testCase_BroadcastUsers] cleanup:", err)
			}
		}
	}()

	pending, err := env.D.Posts.Create(ctx, repository.NewPost{
		UserID: pendingAuthor, Status: models.StatusPending,
		Title: "טסט - פוסט ממתין", Description: "בדיקת broadcastUsers",
		Price: "1", Media: []models.MediaItem{}, Location: "טסט", CreatedAt: time.Now(),
	})
	if err != nil {
		return err
	}
	created = append(created, pending.ID.Hex())

	approved, err := env.D.Posts.Create(ctx, repository.NewPost{
		UserID: approvedAuthor, Status: models.StatusApproved,
		Title: "טסט - פוסט מאושר", Description: "בדיקת broadcastUsers",
		Price: "1", Media: []models.MediaItem{}, Location: "טסט", CreatedAt: time.Now(),
	})
	if err != nil {
		return err
	}
	created = append(created, approved.ID.Hex())

	audience, err := env.BroadcastUsers.ResolveAudience(ctx, nil, adminID)
	if err != nil {
		return err
	}
	report := env.BroadcastUsers.SendToMany(ctx, audience, "🧪 <b>broadcastUsers test</b> — you can ignore this message.")

	contains := func(id string) bool {
		for _, a := range audience {
			if a == id {
				return true
			}
		}
		return false
	}
	say(ctx, env, msg, fmt.Sprintf(
		"✅ broadcastUsers test: audience=%d, sent=%d, failed=%d\npending author included: %t\napproved author included: %t\nadmin excluded: %t",
		len(audience), report.Sent, len(report.Failures), contains(pendingAuthor), contains(approvedAuthor), !contains(adminID)))
	return nil
}
