package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Kind is the value type a config key accepts.
type Kind int

const (
	KindBoolean Kind = iota
	KindNumber
	KindEnum
)

// FieldSpec is the per-key validation rule applied by /config.
//
// Without this, /config would infer the type from the *current* value, so any
// string key (lang, mediaLayout) would accept anything and null-valued keys
// would be treated as strings, which is how broadcastTopicId could end up as
// the string "null" instead of null.
type FieldSpec struct {
	Kind     Kind
	Nullable bool
	Min      *int64
	Values   []string // enum values
	// LocaleEnum marks an enum whose allowed values are the available locales.
	LocaleEnum bool
}

func minOf(n int64) *int64 { return &n }

// Schema lists every editable key.
var Schema = map[string]FieldSpec{
	"lang":        {Kind: KindEnum, LocaleEnum: true},
	"mediaLayout": {Kind: KindEnum, Values: []string{LayoutSlideshow, LayoutCollage}},

	"moderationGroupId": {Kind: KindNumber},
	"approvedGroupId":   {Kind: KindNumber},
	"moderationTopicId": {Kind: KindNumber, Nullable: true},
	"approvedTopicId":   {Kind: KindNumber, Nullable: true},
	"broadcastTopicId":  {Kind: KindNumber, Nullable: true},

	"timeOut":        {Kind: KindNumber, Min: minOf(0)},
	"minimumPhotos":  {Kind: KindNumber, Min: minOf(0)},
	"dailyBumpLimit": {Kind: KindNumber, Min: minOf(0)},

	"validatePrice":    {Kind: KindBoolean},
	"donationsEnabled": {Kind: KindBoolean},
	"enableFaq":        {Kind: KindBoolean},

	// These three live in config.json but nothing reads them yet; typed here so
	// /config still validates rather than accepting anything.
	"faqAllowInGroups":    {Kind: KindBoolean},
	"faqMaxButtonsPerRow": {Kind: KindNumber, Min: minOf(1)},
	"faqMaxDepth":         {Kind: KindNumber, Nullable: true, Min: minOf(1)},
}

// ParseResult is the outcome of validating a raw /config value.
// On success Value is one of: bool, int64, string, or nil (explicit null).
type ParseResult struct {
	OK       bool
	Value    any
	Expected string
}

// ParseValue validates and coerces a raw /config value against the key's spec.
// availableLocales feeds the "lang" enum.
func ParseValue(key, raw string, availableLocales []string) ParseResult {
	spec, ok := Schema[key]
	if !ok {
		return ParseResult{Expected: "a known config key"}
	}

	value := strings.TrimSpace(raw)

	switch spec.Kind {
	case KindBoolean:
		switch value {
		case "true":
			return ParseResult{OK: true, Value: true}
		case "false":
			return ParseResult{OK: true, Value: false}
		}
		return ParseResult{Expected: "true or false"}

	case KindNumber:
		if spec.Nullable && value == "null" {
			return ParseResult{OK: true, Value: nil}
		}
		num, err := strconv.ParseInt(value, 10, 64)
		if value == "" || err != nil {
			if spec.Nullable {
				return ParseResult{Expected: "a number or null"}
			}
			return ParseResult{Expected: "a number"}
		}
		if spec.Min != nil && num < *spec.Min {
			return ParseResult{Expected: fmt.Sprintf("a number >= %d", *spec.Min)}
		}
		return ParseResult{OK: true, Value: num}

	default: // KindEnum
		allowed := spec.Values
		if spec.LocaleEnum {
			allowed = availableLocales
		}
		for _, a := range allowed {
			if a == value {
				return ParseResult{OK: true, Value: value}
			}
		}
		return ParseResult{Expected: strings.Join(allowed, " | ")}
	}
}
