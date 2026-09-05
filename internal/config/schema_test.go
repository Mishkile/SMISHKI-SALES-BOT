package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var locales = []string{"en", "he", "ru"}

func TestParseValueEnums(t *testing.T) {
	if r := ParseValue("mediaLayout", "slideshow", locales); !r.OK || r.Value != "slideshow" {
		t.Fatalf("slideshow rejected: %+v", r)
	}
	if r := ParseValue("mediaLayout", "collage", locales); !r.OK || r.Value != "collage" {
		t.Fatalf("collage rejected: %+v", r)
	}
	bad := ParseValue("mediaLayout", "banana", locales)
	if bad.OK || !strings.Contains(bad.Expected, "slideshow") {
		t.Fatalf("banana should be rejected listing allowed layouts: %+v", bad)
	}

	// lang is validated against the discovered locales, so a number is rejected.
	if r := ParseValue("lang", "123", locales); r.OK {
		t.Fatalf("lang 123 accepted")
	}
	if r := ParseValue("lang", "en", locales); !r.OK || r.Value != "en" {
		t.Fatalf("lang en rejected: %+v", r)
	}
}

func TestParseValueBooleans(t *testing.T) {
	if r := ParseValue("validatePrice", "true", locales); !r.OK || r.Value != true {
		t.Fatalf("true: %+v", r)
	}
	if r := ParseValue("validatePrice", "false", locales); !r.OK || r.Value != false {
		t.Fatalf("false: %+v", r)
	}
	for _, junk := range []string{"yes", "1"} {
		if r := ParseValue("validatePrice", junk, locales); r.OK {
			t.Fatalf("%q accepted as boolean", junk)
		}
	}
}

func TestParseValueNumbers(t *testing.T) {
	if r := ParseValue("minimumPhotos", "2", locales); !r.OK || r.Value != int64(2) {
		t.Fatalf("2: %+v", r)
	}
	if r := ParseValue("minimumPhotos", "-1", locales); r.OK {
		t.Fatalf("min should be enforced")
	}
	for _, junk := range []string{"abc", "", "null"} {
		if r := ParseValue("minimumPhotos", junk, locales); r.OK {
			t.Fatalf("%q accepted as non-nullable number", junk)
		}
	}
}

func TestParseValueNullableNumbers(t *testing.T) {
	// This is the case that previously stored the *string* "null".
	if r := ParseValue("broadcastTopicId", "null", locales); !r.OK || r.Value != nil {
		t.Fatalf("null: %+v", r)
	}
	if r := ParseValue("broadcastTopicId", "73", locales); !r.OK || r.Value != int64(73) {
		t.Fatalf("73: %+v", r)
	}
	if r := ParseValue("broadcastTopicId", "abc", locales); r.OK {
		t.Fatalf("abc accepted")
	}
}

func TestParseValueUnknownKey(t *testing.T) {
	if r := ParseValue("notARealKey", "x", locales); r.OK {
		t.Fatalf("unknown key accepted")
	}
}

func TestStoreUpdatePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	src := `{
    "lang": "en",
    "moderationGroupId": -1000000000000,
    "approvedGroupId": -1234567890123,
    "moderationTopicId": 10,
    "approvedTopicId": 11,
    "timeOut": 1440,
    "validatePrice": true,
    "minimumPhotos": 1,
    "dailyBumpLimit": 2,
    "donationsEnabled": true,
    "enableFaq": true,
    "mediaLayout": "slideshow",
    "broadcastTopicId": null
}`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Get(); got.BroadcastTopicID != nil || !got.DonationsOn() || got.Layout() != LayoutSlideshow {
		t.Fatalf("unexpected load: %+v", got)
	}

	if r, err := s.Update("broadcastTopicId", "73", locales); err != nil || !r.OK {
		t.Fatalf("update: %+v %v", r, err)
	}
	if r, err := s.Update("donationsEnabled", "false", locales); err != nil || !r.OK {
		t.Fatalf("update: %+v %v", r, err)
	}
	if r, _ := s.Update("lang", "xx", locales); r.OK {
		t.Fatalf("invalid lang accepted")
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	c := reloaded.Get()
	if c.BroadcastTopicID == nil || *c.BroadcastTopicID != 73 {
		t.Fatalf("broadcastTopicId not persisted: %+v", c.BroadcastTopicID)
	}
	if c.DonationsOn() {
		t.Fatalf("donationsEnabled=false not persisted")
	}
	if c.Lang != "en" {
		t.Fatalf("lang changed to %q", c.Lang)
	}
	if Thread(c.ModerationTopicID) != 10 || Thread(nil) != 0 || Thread(c.BroadcastTopicID) != 73 {
		t.Fatalf("Thread() wrong")
	}
	one := int64(1)
	if Thread(&one) != 0 {
		t.Fatalf("General topic (1) must be omitted")
	}
}
