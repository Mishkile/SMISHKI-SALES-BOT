package services

import (
	"context"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/config"
	"jsts-salebot/internal/locale"
	"jsts-salebot/internal/models"
	"jsts-salebot/internal/repository"
	"jsts-salebot/internal/tgutil"
)

// AdminService implements /config, /broadcast, /promote, /demote and /auth.
type AdminService struct {
	d     *Deps
	users *UserService
}

// NewAdminService wires the service.
func NewAdminService(d *Deps, users *UserService) *AdminService {
	return &AdminService{d: d, users: users}
}

// HandleConfig shows or edits config.json. Access: ADMIN.
func (s *AdminService) HandleConfig(ctx context.Context, msg *tg.Message, args string) {
	user := s.getUser(ctx, msg.From.ID)
	loc := s.d.Locale.ResolveUserLocale(user)

	if !s.users.HasAuthLevel(ctx, tgutil.ID(msg.From.ID), models.AuthAdmin) {
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "notAdmin"), tgutil.SendOpts{})
		return
	}

	args = strings.TrimSpace(args)
	if args == "" {
		s.showConfig(ctx, msg.Chat.ID, loc)
		return
	}

	parts := strings.Fields(args)
	if len(parts) >= 2 {
		s.updateConfig(ctx, msg.Chat.ID, parts[0], strings.Join(parts[1:], " "), loc)
		return
	}

	tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "configUsage"), tgutil.SendOpts{HTML: true})
}

func (s *AdminService) showConfig(ctx context.Context, chatID int64, loc string) {
	var b strings.Builder
	b.WriteString(s.d.T(loc, "configTitle"))
	b.WriteString("\n\n")
	for _, e := range s.d.Config.Entries() {
		fmt.Fprintf(&b, "<b>%s</b>: <code>%s</code>\n", e.Key, html.EscapeString(e.Value))
	}
	tgutil.SendLog(ctx, s.d.Bot, chatID, strings.TrimRight(b.String(), "\n"), tgutil.SendOpts{HTML: true})
}

func (s *AdminService) updateConfig(ctx context.Context, chatID int64, key, raw, loc string) {
	if _, known := config.Schema[key]; !known {
		tgutil.SendLog(ctx, s.d.Bot, chatID, s.d.T(loc, "configKeyNotFound"), tgutil.SendOpts{})
		return
	}

	res, err := s.d.Config.Update(key, raw, s.d.Locale.AvailableLocales())
	if !res.OK {
		text := s.d.T(loc, "configInvalidValue") + "\n" +
			s.d.T(loc, "configExpected", locale.Params{"key": key, "expected": html.EscapeString(res.Expected)})
		tgutil.SendLog(ctx, s.d.Bot, chatID, text, tgutil.SendOpts{HTML: true})
		return
	}
	if err != nil {
		log.Printf("[ERROR - AdminService.updateConfig] persist: %v", err)
		tgutil.SendLog(ctx, s.d.Bot, chatID, s.d.T(loc, "generalError"), tgutil.SendOpts{})
		return
	}

	text := fmt.Sprintf("%s\n<b>%s</b>: <code>%s</code>", s.d.T(loc, "configUpdated"), key, html.EscapeString(config.Display(res.Value)))
	tgutil.SendLog(ctx, s.d.Bot, chatID, text, tgutil.SendOpts{HTML: true})
}

// HandleBroadcast posts to the approved group's broadcast topic. Access: ADMIN.
func (s *AdminService) HandleBroadcast(ctx context.Context, msg *tg.Message, args string) {
	user := s.getUser(ctx, msg.From.ID)
	loc := s.d.Locale.ResolveUserLocale(user)

	if !s.users.HasAuthLevel(ctx, tgutil.ID(msg.From.ID), models.AuthAdmin) {
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "notAdmin"), tgutil.SendOpts{})
		return
	}

	cfg := s.d.Config.Get()
	// A null broadcastTopicId means the General topic.
	thread := config.Thread(cfg.BroadcastTopicID)
	opts := tgutil.SendOpts{HTML: true, Thread: thread}
	args = strings.TrimSpace(args)

	var err error
	kind := "text"
	switch {
	case msg.ReplyToMessage != nil:
		kind = "copy"
		if msg.ReplyToMessage.Text != "" {
			_, err = tgutil.Send(ctx, s.d.Bot, cfg.ApprovedGroupID, msg.ReplyToMessage.Text, opts)
		} else {
			_, err = s.d.Bot.CopyMessage(ctx, &tgbot.CopyMessageParams{
				ChatID:          cfg.ApprovedGroupID,
				FromChatID:      msg.Chat.ID,
				MessageID:       msg.ReplyToMessage.ID,
				MessageThreadID: thread,
				ParseMode:       tg.ParseModeHTML,
			})
		}
	case args != "":
		_, err = tgutil.Send(ctx, s.d.Bot, cfg.ApprovedGroupID, args, opts)
	default:
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "broadcastUsage"), tgutil.SendOpts{})
		return
	}

	if err != nil {
		log.Printf("[ERROR - AdminService.handleBroadcast] %v (approvedGroupId=%d broadcastTopicId=%v thread=%d)", err, cfg.ApprovedGroupID, cfg.BroadcastTopicID, thread)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "generalError"), tgutil.SendOpts{})
		return
	}
	tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "broadcastSuccess"), tgutil.SendOpts{})
	log.Printf("[INFO - AdminService.handleBroadcast] Broadcast successful adminId=%d type=%s", msg.From.ID, kind)
}

// HandlePromote raises a user's role by one level. Access: ADMIN.
func (s *AdminService) HandlePromote(ctx context.Context, msg *tg.Message, args string) {
	actorID := tgutil.ID(msg.From.ID)
	actor := s.getUser(ctx, msg.From.ID)
	loc := s.d.Locale.ResolveUserLocale(actor)

	if !s.users.HasAuthLevel(ctx, actorID, models.AuthAdmin) {
		log.Printf("[WARN - AdminService.handlePromote] Unauthorized attempt by user %s", actorID)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "notAdmin"), tgutil.SendOpts{})
		return
	}

	target := s.resolveTargetUser(ctx, msg, args)
	if target == nil {
		log.Printf("[WARN - AdminService.handlePromote] Target user not found for actor %s with args: %s", actorID, args)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "userNotFound"), tgutil.SendOpts{})
		return
	}

	if target.UserID == actorID {
		log.Printf("[WARN - AdminService.handlePromote] Admin %s attempted to promote themselves.", actorID)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "promoteAlreadyAtLevel"), tgutil.SendOpts{})
		return
	}
	if target.AuthLevel >= models.AuthAdmin {
		log.Printf("[WARN - handlePromote] Target user %s is already ADMIN. Actor: %s", target.UserID, actorID)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "promoteLimitReached"), tgutil.SendOpts{})
		return
	}

	newLevel := target.AuthLevel + 1
	if _, err := s.d.Users.UpdateUser(ctx, target.UserID, repository.Fields{"authLevel": newLevel}); err != nil {
		log.Printf("[ERROR - handlePromote] %v", err)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "generalError"), tgutil.SendOpts{})
		return
	}
	log.Printf("[INFO - handlePromote] Action: PROMOTE | Actor: %s | Target: %s | Old Level: %d | New Level: %d | Time: %s",
		actorID, target.UserID, target.AuthLevel, newLevel, time.Now().UTC().Format(time.RFC3339))
	tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "promoteSuccess", locale.Params{"userId": target.UserID, "level": int(newLevel)}), tgutil.SendOpts{HTML: true})
}

// HandleDemote lowers a user's role by one level. Access: ADMIN.
func (s *AdminService) HandleDemote(ctx context.Context, msg *tg.Message, args string) {
	actorID := tgutil.ID(msg.From.ID)
	actor := s.getUser(ctx, msg.From.ID)
	loc := s.d.Locale.ResolveUserLocale(actor)

	if !s.users.HasAuthLevel(ctx, actorID, models.AuthAdmin) {
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "notAdmin"), tgutil.SendOpts{})
		return
	}

	target := s.resolveTargetUser(ctx, msg, args)
	if target == nil {
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "userNotFound"), tgutil.SendOpts{})
		return
	}
	if target.AuthLevel <= models.AuthUser {
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "demoteAlreadyAtLevel"), tgutil.SendOpts{})
		return
	}

	oldLevel := target.AuthLevel
	newLevel := oldLevel - 1
	if _, err := s.d.Users.UpdateUser(ctx, target.UserID, repository.Fields{"authLevel": newLevel}); err != nil {
		log.Printf("[ERROR - handleDemote] %v", err)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "generalError"), tgutil.SendOpts{})
		return
	}
	log.Printf("[INFO - handleDemote] Action: DEMOTE | Actor: %s | Target: %s | Old Level: %d | New Level: %d | Time: %s",
		actorID, target.UserID, oldLevel, newLevel, time.Now().UTC().Format(time.RFC3339))
	tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "demoteSuccess", locale.Params{"userId": target.UserID, "level": int(newLevel)}), tgutil.SendOpts{HTML: true})

	if oldLevel == models.AuthAdmin {
		count, err := s.d.Users.CountByAuthLevel(ctx, models.AuthAdmin)
		if err != nil {
			log.Printf("[ERROR - handleDemote] count admins: %v", err)
		} else if count == 0 {
			tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "demoteAdminWarning"), tgutil.SendOpts{HTML: true})
		}
	}
}

// HandleAuth shows a user's role. Access: MOD or ADMIN.
func (s *AdminService) HandleAuth(ctx context.Context, msg *tg.Message, args string) {
	actorID := tgutil.ID(msg.From.ID)
	actor := s.getUser(ctx, msg.From.ID)
	loc := s.d.Locale.ResolveUserLocale(actor)

	if !s.users.HasAuthLevel(ctx, actorID, models.AuthMod) {
		log.Printf("[WARN - handleAuth] Unauthorized attempt by user %s", actorID)
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "notAdmin"), tgutil.SendOpts{})
		return
	}

	var target *models.User
	if strings.TrimSpace(args) == "" && msg.ReplyToMessage == nil && tgutil.ForwardSenderID(msg) == 0 {
		target = actor
	} else {
		target = s.resolveTargetUser(ctx, msg, args)
	}
	if target == nil {
		tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, s.d.T(loc, "userNotFound"), tgutil.SendOpts{})
		return
	}

	roleKey := "authLevelUser"
	switch target.AuthLevel {
	case models.AuthAdmin:
		roleKey = "authLevelAdmin"
	case models.AuthMod:
		roleKey = "authLevelMod"
	}
	text := s.d.T(loc, "authCurrentLevel", locale.Params{"userId": target.UserID, "role": s.d.T(loc, roleKey), "level": int(target.AuthLevel)})
	tgutil.SendLog(ctx, s.d.Bot, msg.Chat.ID, text, tgutil.SendOpts{HTML: true})
}

// resolveTargetUser finds the user an admin command refers to: the author of
// the replied-to (or forwarded) message, a numeric id, or an @username.
func (s *AdminService) resolveTargetUser(ctx context.Context, msg *tg.Message, args string) *models.User {
	var targetID string

	switch {
	case msg.ReplyToMessage != nil:
		if fwd := tgutil.ForwardSenderID(msg.ReplyToMessage); fwd != 0 {
			targetID = tgutil.ID(fwd)
		} else if msg.ReplyToMessage.From != nil {
			targetID = tgutil.ID(msg.ReplyToMessage.From.ID)
		}
	case tgutil.ForwardSenderID(msg) != 0:
		targetID = tgutil.ID(tgutil.ForwardSenderID(msg))
	case strings.TrimSpace(args) != "":
		arg := strings.TrimSpace(args)
		if _, err := strconv.ParseInt(arg, 10, 64); err == nil {
			targetID = arg
		} else if strings.HasPrefix(arg, "@") {
			username := arg[1:]
			u, err := s.d.Users.FindByUsername(ctx, username)
			if err != nil {
				log.Printf("[ERROR - resolveTargetUser] %v", err)
			}
			if u == nil {
				log.Printf("[WARN - resolveTargetUser] User not found by username: %s", username)
			}
			return u
		}
	}

	if targetID == "" {
		return nil
	}
	u, err := s.d.Users.FindByUserID(ctx, targetID)
	if err != nil {
		log.Printf("[ERROR - resolveTargetUser] %v", err)
	}
	return u
}

// getUser loads the acting user, registering a placeholder record if missing.
func (s *AdminService) getUser(ctx context.Context, userID int64) *models.User {
	u, err := s.d.Users.FindByUserID(ctx, tgutil.ID(userID))
	if err != nil {
		log.Printf("[ERROR - AdminService.getUser] %v", err)
		return nil
	}
	if u != nil {
		return u
	}
	if err := s.users.EnsureUser(ctx, &tg.User{ID: userID, FirstName: "Unknown", Username: "unknown"}); err != nil {
		log.Printf("[ERROR - AdminService.getUser] %v", err)
		return nil
	}
	u, _ = s.d.Users.FindByUserID(ctx, tgutil.ID(userID))
	return u
}
