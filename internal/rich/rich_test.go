package rich

import (
	"encoding/json"
	"strings"
	"testing"
)

// The block/text "type" discriminators must match the Bot API wire format the
// TypeScript version emitted, otherwise Telegram rejects the message.
func TestMarshalMatchesBotAPIShapes(t *testing.T) {
	msg := Message(
		Paragraph(Text("Post preview:")),
		Heading(Strike(Text("Title")), 2),
		Blockquote(Paragraph(Text("desc"))),
		Divider(),
		List(
			Item(Paragraph(Seq(Text("💰 "), Bold(Text("100"))))),
			Item(Paragraph(Seq(Text("👤 "), TextMention("Bob", 42)))),
		),
		Slideshow([]tg_blocks{Photo("PHOTO_ID"), Video("VIDEO_ID")}),
		Footer(URL("Review", "https://t.me/c/1/2")),
		Footer(Text("🔴 SOLD")),
	)
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{
		`"type":"paragraph"`,
		`"type":"heading"`, `"size":2`,
		`"type":"strikethrough"`,
		`"type":"blockquote"`,
		`"type":"divider"`,
		`"type":"list"`,
		`"type":"bold"`,
		`"type":"text_mention"`, `"id":42`,
		`"type":"slideshow"`,
		`"type":"photo","photo":{"type":"photo","media":"PHOTO_ID"}`,
		`"type":"video","video":{"type":"video","media":"VIDEO_ID"}`,
		`"type":"footer"`, `"type":"url"`, `"url":"https://t.me/c/1/2"`,
		`"text":["💰 ",{"type":"bold","text":"100"}]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in\n%s", want, got)
		}
	}
}
