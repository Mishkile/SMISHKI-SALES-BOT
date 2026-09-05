package services

import (
	"context"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"

	tgbot "github.com/go-telegram/bot"
	tg "github.com/go-telegram/bot/models"

	"jsts-salebot/internal/config"
	"jsts-salebot/internal/models"
	"jsts-salebot/internal/rich"
	"jsts-salebot/internal/tgutil"
)

// PostData is everything needed to render a post.
type PostData struct {
	Title       string
	Description string
	Price       string
	Location    string
	Media       []models.MediaItem
	UserID      int64
	Username    string
	FirstName   string
}

// DataFromPost builds PostData from a stored post and (optionally) its author.
// fallbackName is shown when the author has no first name on record.
func DataFromPost(p *models.Post, author *models.User, fallbackName string) PostData {
	uid, _ := strconv.ParseInt(p.UserID, 10, 64)
	d := PostData{
		Title:       p.Title,
		Description: p.Description,
		Price:       p.Price,
		Location:    p.Location,
		Media:       p.Media,
		UserID:      uid,
		FirstName:   fallbackName,
	}
	if author != nil {
		d.Username = models.Str(author.UserName)
		d.FirstName = models.StrOr(author.FirstName, fallbackName)
	}
	return d
}

// FormatOpts tweak the rendered post.
type FormatOpts struct {
	Sold         bool
	ShowCta      bool
	PreviewLabel string
	Link         *Link
}

// Link is a footer deep-link (e.g. /pending's "Review" link).
type Link struct {
	Label string
	URL   string
}

// PostService renders posts and publishes them to the groups.
type PostService struct {
	d *Deps
}

// NewPostService wires the service.
func NewPostService(d *Deps) *PostService { return &PostService{d: d} }

// FormatUserMention renders an HTML mention: @username, or a tg://user link.
func (s *PostService) FormatUserMention(userID int64, username, firstName string) string {
	if username != "" {
		return "@" + username
	}
	if firstName == "" {
		firstName = "User"
	}
	return fmt.Sprintf(`<a href="tg://user?id=%d">%s</a>`, userID, html.EscapeString(firstName))
}

// FormatUserMentionRich is the rich-text equivalent of FormatUserMention.
func (s *PostService) FormatUserMentionRich(userID int64, username, firstName string) tg.RichText {
	if username != "" {
		return rich.Text("@" + username)
	}
	if firstName == "" {
		firstName = "User"
	}
	return rich.TextMention(firstName, userID)
}

// FormatPostRichMessage renders a post as a Rich Message: heading, quoted
// description, price/location/seller list, media gallery and footer.
func (s *PostService) FormatPostRichMessage(data PostData, opts FormatOpts) tg.InputRichMessage {
	cfg := s.d.Config.Get()

	title := rich.Text(data.Title)
	if opts.Sold {
		title = rich.Strike(title)
	}

	var blocks []tg.InputRichBlock
	if opts.PreviewLabel != "" {
		blocks = append(blocks, rich.Paragraph(rich.Text(opts.PreviewLabel)))
	}
	blocks = append(blocks,
		rich.Heading(title, 2),
		rich.Blockquote(rich.Paragraph(rich.Text(data.Description))),
		rich.Divider(),
		rich.List(
			rich.Item(rich.Paragraph(rich.Seq(rich.Text("💰 "), rich.Bold(rich.Text(data.Price))))),
			rich.Item(rich.Paragraph(rich.Text("📍 "+data.Location))),
			rich.Item(rich.Paragraph(rich.Seq(rich.Text("👤 "), s.FormatUserMentionRich(data.UserID, data.Username, data.FirstName)))),
		),
	)

	var media []tg.InputRichBlock
	for _, m := range data.Media {
		if m.Type == models.MediaVideo {
			media = append(media, rich.Video(m.FileID))
		} else {
			media = append(media, rich.Photo(m.FileID))
		}
	}
	switch {
	case len(media) == 1:
		blocks = append(blocks, media[0])
	case len(media) > 1:
		// slideshow (swipeable) vs collage (grid), chosen in config.json.
		if cfg.Layout() == config.LayoutCollage {
			blocks = append(blocks, rich.Collage(media))
		} else {
			blocks = append(blocks, rich.Slideshow(media))
		}
	}

	if opts.Link != nil {
		blocks = append(blocks, rich.Footer(rich.URL(opts.Link.Label, opts.Link.URL)))
	}

	// Sold posts show the sold marker; only the public post gets a contact CTA.
	var footer string
	switch {
	case opts.Sold:
		footer = s.d.T(cfg.Lang, "soldTag")
	case opts.ShowCta:
		footer = s.d.T(cfg.Lang, "contactSellerCta")
	}
	if footer != "" {
		blocks = append(blocks, rich.Footer(rich.Text(footer)))
	}

	return rich.Message(blocks...)
}

// SendToModeration posts the card with approve/reject buttons to the
// moderation group and returns the message id.
func (s *PostService) SendToModeration(ctx context.Context, postID string, data PostData) (int, error) {
	cfg := s.d.Config.Get()
	markup := tgutil.Keyboard([]tg.InlineKeyboardButton{
		tgutil.Btn(s.d.T(cfg.Lang, "approveButton"), "approve_"+postID),
		tgutil.Btn(s.d.T(cfg.Lang, "rejectButton"), "reject_"+postID),
	})
	sent, err := tgutil.SendRich(ctx, s.d.Bot, cfg.ModerationGroupID, s.FormatPostRichMessage(data, FormatOpts{}), config.Thread(cfg.ModerationTopicID), markup)
	if err != nil {
		return 0, err
	}
	return sent.ID, nil
}

// SendToApproved publishes a rich message to the approved group.
func (s *PostService) SendToApproved(ctx context.Context, msg tg.InputRichMessage) (int, error) {
	cfg := s.d.Config.Get()
	sent, err := tgutil.SendRich(ctx, s.d.Bot, cfg.ApprovedGroupID, msg, config.Thread(cfg.ApprovedTopicID), nil)
	if err != nil {
		return 0, err
	}
	return sent.ID, nil
}

// SendToApprovedText publishes an HTML text message to the approved group.
func (s *PostService) SendToApprovedText(ctx context.Context, text string) (int, error) {
	cfg := s.d.Config.Get()
	sent, err := tgutil.Send(ctx, s.d.Bot, cfg.ApprovedGroupID, text, tgutil.SendOpts{HTML: true, Thread: config.Thread(cfg.ApprovedTopicID)})
	if err != nil {
		return 0, err
	}
	return sent.ID, nil
}

// MarkSoldInGroup rewrites the published post with the sold rendering. It
// returns false when the message no longer exists (caller clears the id).
func (s *PostService) MarkSoldInGroup(ctx context.Context, approvedMessageID int, msg tg.InputRichMessage) bool {
	cfg := s.d.Config.Get()
	err := tgutil.EditRichMessage(ctx, s.d.Bot, cfg.ApprovedGroupID, approvedMessageID, msg)
	if err == nil {
		return true
	}
	errText := err.Error()
	if strings.Contains(errText, "message to edit not found") {
		log.Printf("[WARN - markSoldInGroup] Message with ID %d not found in group %d. Clearing approvedMessageId reference.", approvedMessageID, cfg.ApprovedGroupID)
		return false
	}
	if strings.Contains(errText, "message is not modified") {
		return true
	}
	log.Printf("[ERROR - markSoldInGroup] %v", err)
	return false
}

// HandlePublicReply relays a reply in the approved group to the post's author.
func (s *PostService) HandlePublicReply(ctx context.Context, msg *tg.Message) {
	cfg := s.d.Config.Get()
	if msg.ReplyToMessage == nil || msg.Chat.ID != cfg.ApprovedGroupID || msg.From == nil {
		return
	}
	post, err := s.d.Posts.FindByApprovedMessageID(ctx, msg.ReplyToMessage.ID)
	if err != nil {
		log.Printf("[ERROR - PostService.handlePublicReply] %v", err)
		return
	}
	if post == nil {
		return
	}
	authorID, _ := strconv.ParseInt(post.UserID, 10, 64)
	loc := s.d.LocaleFor(ctx, authorID)

	// 1. Send the text notification.
	text := fmt.Sprintf("💬 <b>%s</b>\nPost: <i>%s</i>\nFrom: %s",
		s.d.T(loc, "newReplyNotification"),
		html.EscapeString(post.Title),
		s.FormatUserMention(msg.From.ID, msg.From.Username, msg.From.FirstName),
	)
	tgutil.SendLog(ctx, s.d.Bot, authorID, text, tgutil.SendOpts{HTML: true})

	// 2. Forward, falling back to copy (copy works in protected groups).
	if _, err := s.d.Bot.ForwardMessage(ctx, &tgbot.ForwardMessageParams{ChatID: authorID, FromChatID: msg.Chat.ID, MessageID: msg.ID}); err != nil {
		log.Printf("[WARN - PostService.handlePublicReply] Forward failed, attempting copy... %v", err)
		if _, err := s.d.Bot.CopyMessage(ctx, &tgbot.CopyMessageParams{ChatID: authorID, FromChatID: msg.Chat.ID, MessageID: msg.ID}); err != nil {
			log.Printf("[ERROR - PostService.handlePublicReply] Both forward and copy failed. %v", err)
		}
	}

	log.Printf("[INFO - PostService.handlePublicReply] postId=%s authorId=%s buyerId=%d", post.ID.Hex(), post.UserID, msg.From.ID)
}
