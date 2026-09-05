// Package rich builds Telegram Rich Messages (Bot API 10.2) with small
// constructors so the post, help and report layouts read like the original
// block definitions.
package rich

import tg "github.com/go-telegram/bot/models"

// Text is a plain string rich-text node.
func Text(s string) tg.RichText { return tg.RichText{PlainText: s} }

// Seq concatenates rich-text nodes.
func Seq(parts ...tg.RichText) tg.RichText {
	if parts == nil {
		parts = []tg.RichText{}
	}
	return tg.RichText{Array: parts}
}

// Bold wraps t in bold.
func Bold(t tg.RichText) tg.RichText {
	return tg.RichText{Type: tg.RichTextTypeBold, RichTextBold: &tg.RichTextBold{Text: t}}
}

// Strike wraps t in strikethrough.
func Strike(t tg.RichText) tg.RichText {
	return tg.RichText{Type: tg.RichTextTypeStrikethrough, RichTextStrikethrough: &tg.RichTextStrikethrough{Text: t}}
}

// URL makes a link.
func URL(text, url string) tg.RichText {
	return tg.RichText{Type: tg.RichTextTypeURL, RichTextURL: &tg.RichTextURL{Text: Text(text), URL: url}}
}

// TextMention links a display name to a user id (for users without a username).
func TextMention(name string, userID int64) tg.RichText {
	return tg.RichText{Type: tg.RichTextTypeTextMention, RichTextTextMention: &tg.RichTextTextMention{
		Text: Text(name),
		User: &tg.User{ID: userID, IsBot: false, FirstName: name},
	}}
}

// Paragraph is a text block.
func Paragraph(t tg.RichText) tg.InputRichBlock {
	return tg.InputRichBlock{Type: tg.RichBlockTypeParagraph, InputRichBlockParagraph: &tg.InputRichBlockParagraph{Text: t}}
}

// Heading is a section heading of the given size (1 largest).
func Heading(t tg.RichText, size int) tg.InputRichBlock {
	return tg.InputRichBlock{Type: tg.RichBlockTypeSectionHeading, InputRichBlockSectionHeading: &tg.InputRichBlockSectionHeading{Text: t, Size: size}}
}

// Divider is a horizontal rule.
func Divider() tg.InputRichBlock {
	return tg.InputRichBlock{Type: tg.RichBlockTypeDivider, InputRichBlockDivider: &tg.InputRichBlockDivider{}}
}

// Blockquote quotes the given blocks.
func Blockquote(blocks ...tg.InputRichBlock) tg.InputRichBlock {
	return tg.InputRichBlock{Type: tg.RichBlockTypeBlockQuotation, InputRichBlockBlockQuotation: &tg.InputRichBlockBlockQuotation{Blocks: blocks}}
}

// Item is one list entry.
func Item(blocks ...tg.InputRichBlock) tg.InputRichBlockListItem {
	return tg.InputRichBlockListItem{Blocks: blocks}
}

// List is a bulleted list.
func List(items ...tg.InputRichBlockListItem) tg.InputRichBlock {
	if items == nil {
		items = []tg.InputRichBlockListItem{}
	}
	return tg.InputRichBlock{Type: tg.RichBlockTypeList, InputRichBlockList: &tg.InputRichBlockList{Items: items}}
}

// Footer is the small trailing text of a message.
func Footer(t tg.RichText) tg.InputRichBlock {
	return tg.InputRichBlock{Type: tg.RichBlockTypeFooter, InputRichBlockFooter: &tg.InputRichBlockFooter{Text: t}}
}

// Photo embeds an already-uploaded photo by file_id.
func Photo(fileID string) tg.InputRichBlock {
	return tg.InputRichBlock{Type: tg.RichBlockTypePhoto, InputRichBlockPhoto: &tg.InputRichBlockPhoto{Photo: tg.InputMediaPhoto{Media: fileID}}}
}

// Video embeds an already-uploaded video by file_id.
func Video(fileID string) tg.InputRichBlock {
	return tg.InputRichBlock{Type: tg.RichBlockTypeVideo, InputRichBlockVideo: &tg.InputRichBlockVideo{Video: tg.InputMediaVideo{Media: fileID}}}
}

// Slideshow renders media blocks as a swipeable gallery.
func Slideshow(blocks []tg.InputRichBlock) tg.InputRichBlock {
	return tg.InputRichBlock{Type: tg.RichBlockTypeSlideshow, InputRichBlockSlideshow: &tg.InputRichBlockSlideshow{Blocks: blocks}}
}

// Collage renders media blocks as a grid.
func Collage(blocks []tg.InputRichBlock) tg.InputRichBlock {
	return tg.InputRichBlock{Type: tg.RichBlockTypeCollage, InputRichBlockCollage: &tg.InputRichBlockCollage{Blocks: blocks}}
}

// Message assembles blocks into an outgoing rich message.
func Message(blocks ...tg.InputRichBlock) tg.InputRichMessage {
	return tg.InputRichMessage{Blocks: blocks}
}
