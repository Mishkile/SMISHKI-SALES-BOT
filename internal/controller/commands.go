package controller

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/locale"
	"jsts-salebot/internal/models"
	"jsts-salebot/internal/repository"
	"jsts-salebot/internal/rich"
	"jsts-salebot/internal/services"
	"jsts-salebot/internal/testcases"
	"jsts-salebot/internal/tgutil"
)

const wizardSteps = 5

func (c *Controller) handleStart(ctx context.Context, msg *tg.Message) {
	if err := c.user.EnsureUser(ctx, msg.From); err != nil {
		log.Printf("[ERROR - HandleStart] %v", err)
	}
	loc := c.d.LocaleFor(ctx, msg.From.ID)
	tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(loc, "greeting"), tgutil.SendOpts{})
}

// handleNewPost runs the five-step wizard and hands the post to moderation.
func (c *Controller) handleNewPost(ctx context.Context, msg *tg.Message) {
	log.Printf("[INFO - HandleNewPost] session active userId=%d chatId=%d", msg.From.ID, msg.Chat.ID)
	wctx, end := c.beginWizard(ctx, msg.From.ID)
	defer end()

	if err := c.user.EnsureUser(wctx, msg.From); err != nil {
		log.Printf("[ERROR - HandleNewPost] %v", err)
	}
	user, loc := c.d.UserAndLocale(wctx, msg.From.ID)
	log.Printf("[DEBUG - HandleNewPost] Resolving locale for user: %d (lang_code: %s, pref: %s)", msg.From.ID, msg.From.LanguageCode, models.Str(prefOf(user)))

	step := func(i int) *services.WizardStep {
		return &services.WizardStep{Locale: loc, Index: i, Total: wizardSteps}
	}

	err := func() error {
		title, err := c.input.InputWithPrompt(wctx, msg, c.d.T(loc, "welcome"), step(1))
		if err != nil {
			return err
		}
		description, err := c.input.InputWithPrompt(wctx, msg, c.d.T(loc, "enterDescription"), step(2))
		if err != nil {
			return err
		}
		price, err := c.input.InputPrice(wctx, msg, *step(3))
		if err != nil {
			return err
		}
		location, err := c.input.InputWithPrompt(wctx, msg, c.d.T(loc, "enterLocation"), step(4))
		if err != nil {
			return err
		}
		media, err := c.input.PromptMedia(wctx, msg, *step(5))
		if err != nil {
			return err
		}

		if int64(len(media)) < c.d.Config.Get().MinimumPhotos {
			tgutil.SendLog(wctx, c.d.Bot, msg.Chat.ID, c.d.T(loc, "notEnoughMedia"), tgutil.SendOpts{})
			log.Printf("[INFO - HandleNewPost] session idle (insufficient media) userId=%d", msg.From.ID)
			return nil
		}

		data := services.PostData{
			Title:       title,
			Description: description,
			Price:       price,
			Location:    location,
			Media:       media,
			UserID:      msg.From.ID,
			Username:    msg.From.Username,
			FirstName:   msg.From.FirstName,
		}

		// Preview and confirm as one message (buttons attached to the preview).
		preview := c.post.FormatPostRichMessage(data, services.FormatOpts{PreviewLabel: c.d.T(loc, "preview")})
		confirmed, err := c.input.ConfirmAction(wctx, msg, loc, &preview)
		if err != nil {
			return err
		}
		if !confirmed {
			tgutil.SendLog(wctx, c.d.Bot, msg.Chat.ID, c.d.T(loc, "postCancelled"), tgutil.SendOpts{})
			log.Printf("[INFO - HandleNewPost] session idle (user cancelled) userId=%d", msg.From.ID)
			return nil
		}

		// Save and send to moderation.
		post, err := c.d.Posts.Create(wctx, repository.NewPost{
			UserID:      tgutil.ID(msg.From.ID),
			Title:       title,
			Description: description,
			Price:       price,
			Media:       media,
			Location:    location,
			CreatedAt:   time.Now(),
		})
		if err != nil {
			return err
		}
		modMsgID, err := c.post.SendToModeration(wctx, post.ID.Hex(), data)
		if err != nil {
			return err
		}
		if modMsgID != 0 {
			if err := c.d.Posts.SetModerationMessageID(wctx, post.ID.Hex(), &modMsgID); err != nil {
				log.Printf("[ERROR - HandleNewPost] %v", err)
			}
		}

		tgutil.SendLog(wctx, c.d.Bot, msg.Chat.ID, c.d.T(loc, "postCreated"), tgutil.SendOpts{})
		log.Printf("[INFO - HandleNewPost] session idle (success) userId=%d", msg.From.ID)
		return nil
	}()

	if err != nil {
		if errors.Is(err, context.Canceled) {
			log.Printf("[INFO - HandleNewPost] session idle (wizard cancelled) userId=%d", msg.From.ID)
			return
		}
		log.Printf("[ERROR - HandleNewPost] %v", err)
		tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(c.d.Lang(), "generalError"), tgutil.SendOpts{})
		log.Printf("[INFO - HandleNewPost] session idle (error) userId=%d", msg.From.ID)
	}
}

func prefOf(u *models.User) *string {
	if u == nil {
		return nil
	}
	return u.PreferredLocale
}

// showHelp renders the command list as a Rich Message, with moderator and
// admin sections for users holding those roles.
func (c *Controller) showHelp(ctx context.Context, msg *tg.Message) {
	if err := c.user.EnsureUser(ctx, msg.From); err != nil {
		log.Printf("[ERROR - showHelp] %v", err)
	}
	user, loc := c.d.UserAndLocale(ctx, msg.From.ID)
	t := func(key string) string { return c.d.T(loc, key) }

	// Section titles carry <b> tags for the old HTML path; strip them since
	// rich headings style themselves. Command lines are "/cmd - desc": bold the
	// command part so the list is scannable.
	stripBold := strings.NewReplacer("<b>", "", "</b>", "")
	heading := func(key string, size int) tg.InputRichBlock {
		return rich.Heading(rich.Text(stripBold.Replace(t(key))), size)
	}
	cmd := func(key string) tg.InputRichBlockListItem {
		line := t(key)
		i := strings.Index(line, " - ")
		if i < 0 {
			return rich.Item(rich.Paragraph(rich.Text(line)))
		}
		return rich.Item(rich.Paragraph(rich.Seq(rich.Bold(rich.Text(line[:i])), rich.Text(line[i:]))))
	}
	list := func(keys ...string) tg.InputRichBlock {
		items := make([]tg.InputRichBlockListItem, 0, len(keys))
		for _, k := range keys {
			items = append(items, cmd(k))
		}
		return rich.List(items...)
	}

	cfg := c.d.Config.Get()
	general := []string{"helpStart", "helpNewPost", "helpMyPosts", "helpLang", "helpHelp"}
	if cfg.FaqOn() {
		general = append(general, "helpFaq")
	}
	if cfg.DonationsOn() {
		general = append(general, "helpDonate")
	}

	blocks := []tg.InputRichBlock{heading("helpTitle", 2), list(general...)}

	level := models.Level(user)
	if level >= models.AuthMod {
		blocks = append(blocks, rich.Divider(), heading("helpModSection", 3), list("helpPending", "helpClearPending", "helpAuth"))
	}
	if level >= models.AuthAdmin {
		blocks = append(blocks, rich.Divider(), heading("helpAdminSection", 3),
			list("helpConfig", "helpActiveUsers", "helpPromote", "helpDemote", "helpBroadcastTopic", "helpBroadcastUsers", "helpTest"))
	}

	if _, err := tgutil.SendRich(ctx, c.d.Bot, msg.Chat.ID, rich.Message(blocks...), msg.MessageThreadID, nil); err != nil {
		log.Printf("[ERROR - showHelp] %v", err)
	}
}

// handleActiveUsers lists users currently inside the wizard. Access: ADMIN.
func (c *Controller) handleActiveUsers(ctx context.Context, msg *tg.Message) {
	log.Printf("[DEBUG - handleActiveUsers] Admin command triggered adminId=%d", msg.From.ID)
	user, loc := c.d.UserAndLocale(ctx, msg.From.ID)
	if models.Level(user) < models.AuthAdmin {
		log.Printf("[WARN - handleActiveUsers] Unauthorized access attempt detected userId=%d", msg.From.ID)
		tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(c.d.Lang(), "notAdmin"), tgutil.SendOpts{})
		return
	}

	users, err := c.d.Users.FindManyByIDs(ctx, c.activeUserIDs())
	if err != nil {
		log.Printf("[CRITICAL - handleActiveUsers] System failed to generate active users list %v", err)
		tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(c.d.Lang(), "generalError"), tgutil.SendOpts{})
		return
	}

	var lines []string
	for _, u := range users {
		username := "N/A"
		if models.Str(u.UserName) != "" {
			username = "@" + *u.UserName
		}
		fullName := strings.TrimSpace(models.StrOr(u.FirstName, "N/A") + " " + models.Str(u.LastName))
		lines = append(lines, fmt.Sprintf("• <code>%s</code> | %s | %s", u.UserID, html.EscapeString(username), html.EscapeString(fullName)))
	}

	if len(lines) == 0 {
		log.Printf("[INFO - handleActiveUsers] Command executed, but no active users found.")
		tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(loc, "adminActiveUsersEmpty"), tgutil.SendOpts{})
		return
	}

	log.Printf("[INFO - handleActiveUsers] Reporting %d active user(s) to admin %d", len(lines), msg.From.ID)
	text := c.d.T(loc, "adminActiveUsersTitle") + "\n\n" + strings.Join(lines, "\n")
	tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, text, tgutil.SendOpts{HTML: true})
}

// handleBroadcastUsers DMs active users and pending/approved authors. Access: ADMIN.
func (c *Controller) handleBroadcastUsers(ctx context.Context, msg *tg.Message, args string) {
	adminID := tgutil.ID(msg.From.ID)
	_, loc := c.d.UserAndLocale(ctx, msg.From.ID)

	if !c.user.HasAuthLevel(ctx, adminID, models.AuthAdmin) {
		log.Printf("[WARN - handleBroadcastUsers] Unauthorized access attempt detected userId=%d", msg.From.ID)
		tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(loc, "notAdmin"), tgutil.SendOpts{})
		return
	}

	var htmlMessage string
	switch {
	case msg.ReplyToMessage != nil:
		if msg.ReplyToMessage.Text == "" {
			tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(loc, "broadcastUsersTextOnly"), tgutil.SendOpts{})
			return
		}
		htmlMessage = msg.ReplyToMessage.Text
	case strings.TrimSpace(args) != "":
		htmlMessage = strings.TrimSpace(args)
	}
	if htmlMessage == "" {
		tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(loc, "broadcastUsersUsage"), tgutil.SendOpts{})
		return
	}

	audience, err := c.broadcastUsers.ResolveAudience(ctx, c.activeUserIDs(), adminID)
	if err != nil {
		log.Printf("[ERROR - handleBroadcastUsers] %v", err)
		tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(loc, "generalError"), tgutil.SendOpts{})
		return
	}
	if len(audience) == 0 {
		log.Printf("[INFO - handleBroadcastUsers] No recipients found adminId=%s", adminID)
		tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(loc, "broadcastUsersNoRecipients"), tgutil.SendOpts{})
		return
	}

	report := c.broadcastUsers.SendToMany(ctx, audience, htmlMessage)
	log.Printf("[INFO - handleBroadcastUsers] Broadcast complete adminId=%s total=%d sent=%d failed=%d", adminID, report.Total, report.Sent, len(report.Failures))
	if len(report.Failures) > 0 {
		log.Printf("[INFO - handleBroadcastUsers] Failures: %+v", report.Failures)
	}

	blocks := []tg.InputRichBlock{
		rich.Heading(rich.Text(c.d.T(loc, "broadcastUsersReportTitle")), 2),
		rich.Paragraph(rich.Text(c.d.T(loc, "broadcastUsersReport", locale.Params{"sent": report.Sent, "failed": len(report.Failures), "total": report.Total}))),
	}

	if len(report.Failures) > 0 {
		ids := make([]string, 0, len(report.Failures))
		for _, f := range report.Failures {
			ids = append(ids, f.UserID)
		}
		mention := map[string]string{}
		if users, err := c.d.Users.FindManyByIDs(ctx, ids); err == nil {
			for _, u := range users {
				if models.Str(u.UserName) != "" {
					mention[u.UserID] = "@" + *u.UserName
				}
			}
		}

		shown, remainder := services.TruncateFailures(report.Failures, 30)
		items := make([]tg.InputRichBlockListItem, 0, len(shown))
		for _, f := range shown {
			m, ok := mention[f.UserID]
			if !ok {
				m = "N/A"
			}
			items = append(items, rich.Item(rich.Paragraph(rich.Text(fmt.Sprintf("• %s (%s) — %s", m, f.UserID, f.Reason)))))
		}
		blocks = append(blocks, rich.Divider(), rich.List(items...))
		if remainder > 0 {
			blocks = append(blocks, rich.Paragraph(rich.Text(c.d.T(loc, "broadcastUsersMore", locale.Params{"n": remainder}))))
		}
	}

	if _, err := tgutil.SendRich(ctx, c.d.Bot, msg.Chat.ID, rich.Message(blocks...), msg.MessageThreadID, nil); err != nil {
		log.Printf("[ERROR - handleBroadcastUsers] report: %v", err)
	}
}

// handleLang offers the available locales as buttons.
func (c *Controller) handleLang(ctx context.Context, msg *tg.Message) {
	log.Printf("[INFO - handleLang] user=%s", userIdentifier(msg.From))
	if err := c.user.EnsureUser(ctx, msg.From); err != nil {
		log.Printf("[ERROR - handleLang] %v", err)
	}
	_, current := c.d.UserAndLocale(ctx, msg.From.ID)

	var row []tg.InlineKeyboardButton
	for _, lang := range c.d.Locale.AvailableLocales() {
		row = append(row, tgutil.Btn(strings.ToUpper(lang), "lang_"+lang))
	}
	text := c.d.T(current, "langMenu", locale.Params{"lang": strings.ToUpper(current)})
	tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, text, tgutil.SendOpts{Thread: msg.MessageThreadID, Markup: tgutil.Keyboard(row)})
}

func userIdentifier(u *tg.User) string {
	switch {
	case u == nil:
		return "unknown"
	case u.Username != "":
		return u.Username
	default:
		return tgutil.ID(u.ID)
	}
}

// handleDonate shows the Stars amount picker.
func (c *Controller) handleDonate(ctx context.Context, msg *tg.Message) {
	if !c.d.Config.Get().DonationsOn() {
		tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(c.d.Lang(), "donationDisabled"), tgutil.SendOpts{})
		return
	}
	if err := c.user.EnsureUser(ctx, msg.From); err != nil {
		log.Printf("[ERROR - handleDonate] %v", err)
	}
	loc := c.d.LocaleFor(ctx, msg.From.ID)
	text := c.d.T(loc, "donateTitle") + "\n" + c.d.T(loc, "donateChooseAmount")
	markup := tgutil.Keyboard([]tg.InlineKeyboardButton{
		tgutil.Btn("⭐ 50", "donate_50"),
		tgutil.Btn("⭐ 150", "donate_150"),
		tgutil.Btn(c.d.T(loc, "donateOther"), "donate_other"),
	})
	tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, text, tgutil.SendOpts{HTML: true, Thread: msg.MessageThreadID, Markup: markup})
}

// handleTest offers the in-bot test cases. Access: ADMIN.
func (c *Controller) handleTest(ctx context.Context, msg *tg.Message) {
	user, _ := c.d.UserAndLocale(ctx, msg.From.ID)
	if models.Level(user) < models.AuthAdmin {
		tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, c.d.T(c.d.Lang(), "notAdmin"), tgutil.SendOpts{})
		return
	}
	var rows [][]tg.InlineKeyboardButton
	for _, tc := range testcases.Cases {
		rows = append(rows, []tg.InlineKeyboardButton{tgutil.Btn(tc.Label, "test_"+tc.Key)})
	}
	rows = append(rows, []tg.InlineKeyboardButton{tgutil.Btn("🚀 Run All", "test_all")})
	tgutil.SendLog(ctx, c.d.Bot, msg.Chat.ID, "Select a test case:", tgutil.SendOpts{Thread: msg.MessageThreadID, Markup: tgutil.Keyboard(rows...)})
}
