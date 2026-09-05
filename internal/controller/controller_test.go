package controller

import "testing"

func TestParseCommand(t *testing.T) {
	cases := []struct {
		text     string
		cmd      string
		args     string
		ok       bool
		scenario string
	}{
		{"/start", "start", "", true, "bare command"},
		{"/NewPost", "newpost", "", true, "case-insensitive"},
		{"/config dailyBumpLimit 5", "config", "dailyBumpLimit 5", true, "arguments"},
		{"/broadcast@JSTSBot hello\nsecond line", "broadcast", "hello\nsecond line", true, "bot suffix and multi-line args"},
		{"/broadcastUsers hi", "broadcastusers", "hi", true, "distinct from /broadcast"},
		{"/clearpending", "clearpending", "", true, "not confused with /pending"},
		{"please /start", "", "", false, "command must lead the message"},
		{"hello", "", "", false, "plain text"},
		{"/", "", "", false, "lone slash"},
	}
	for _, c := range cases {
		cmd, args, ok := parseCommand(c.text)
		if cmd != c.cmd || args != c.args || ok != c.ok {
			t.Errorf("%s: parseCommand(%q) = (%q, %q, %v), want (%q, %q, %v)", c.scenario, c.text, cmd, args, ok, c.cmd, c.args, c.ok)
		}
	}
}

func TestLeadingInt(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"50", 50, true},
		{" 150 stars", 150, true},
		{"-3", -3, true},
		{"abc", 0, false},
		{"", 0, false},
		{"+7", 7, true},
	}
	for _, c := range cases {
		got, ok := leadingInt(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("leadingInt(%q) = (%d, %v), want (%d, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
