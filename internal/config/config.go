// Package config loads config.json, exposes it to the rest of the bot, and
// persists the edits made through /config.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
)

// Media layouts for posts carrying more than one photo/video.
const (
	LayoutSlideshow = "slideshow"
	LayoutCollage   = "collage"
)

// Config mirrors config.json. Field order is the on-disk key order.
type Config struct {
	Lang                string `json:"lang"`
	ModerationGroupID   int64  `json:"moderationGroupId"`
	ApprovedGroupID     int64  `json:"approvedGroupId"`
	ModerationTopicID   *int64 `json:"moderationTopicId"`
	ApprovedTopicID     *int64 `json:"approvedTopicId"`
	TimeOut             int64  `json:"timeOut"`
	ValidatePrice       bool   `json:"validatePrice"`
	MinimumPhotos       int64  `json:"minimumPhotos"`
	DailyBumpLimit      int64  `json:"dailyBumpLimit"`
	DonationsEnabled    *bool  `json:"donationsEnabled,omitempty"`
	EnableFaq           *bool  `json:"enableFaq,omitempty"`
	MediaLayout         string `json:"mediaLayout,omitempty"`
	FaqAllowInGroups    *bool  `json:"faqAllowInGroups,omitempty"`
	FaqMaxButtonsPerRow *int64 `json:"faqMaxButtonsPerRow,omitempty"`
	FaqMaxDepth         *int64 `json:"faqMaxDepth,omitempty"`
	BroadcastTopicID    *int64 `json:"broadcastTopicId"`
}

// KnownKeys is the display order used by /config.
var KnownKeys = []string{
	"lang", "moderationGroupId", "approvedGroupId", "moderationTopicId", "approvedTopicId",
	"timeOut", "validatePrice", "minimumPhotos", "dailyBumpLimit", "donationsEnabled",
	"enableFaq", "mediaLayout", "faqAllowInGroups", "faqMaxButtonsPerRow", "faqMaxDepth",
	"broadcastTopicId",
}

// DonationsOn reports whether /donate is enabled (default true when unset).
func (c Config) DonationsOn() bool { return c.DonationsEnabled == nil || *c.DonationsEnabled }

// FaqOn reports whether /faq is enabled (default true when unset).
func (c Config) FaqOn() bool { return c.EnableFaq == nil || *c.EnableFaq }

// Layout returns the media layout, defaulting to slideshow.
func (c Config) Layout() string {
	if c.MediaLayout == "" {
		return LayoutSlideshow
	}
	return c.MediaLayout
}

// Thread converts a forum topic id into the message_thread_id to send with.
// It returns 0 (omit) when the topic is unset, or is the General topic (1),
// which Telegram rejects as an explicit thread id.
func Thread(id *int64) int {
	if id == nil || *id == 0 || *id == 1 {
		return 0
	}
	return int(*id)
}

// Store is the live, mutex-guarded configuration shared by every service.
type Store struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

// Load reads config.json from path.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &Store{path: path, cfg: c}, nil
}

// NewStore wraps an in-memory config (tests); Save is a no-op when path is "".
func NewStore(c Config, path string) *Store { return &Store{path: path, cfg: c} }

// Get returns a snapshot of the current configuration.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// Lang returns the default locale.
func (s *Store) Lang() string { return s.Get().Lang }

// Path returns the backing file.
func (s *Store) Path() string { return s.path }

// Entry is one key/value pair for display.
type Entry struct {
	Key   string
	Value string
}

// Entries renders every known key with its current value, in display order.
func (s *Store) Entries() []Entry {
	c := s.Get()
	out := make([]Entry, 0, len(KnownKeys))
	for _, k := range KnownKeys {
		out = append(out, Entry{Key: k, Value: display(c.value(k))})
	}
	return out
}

func display(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case *int64:
		if x == nil {
			return "null"
		}
		return strconv.FormatInt(*x, 10)
	case *bool:
		if x == nil {
			return "null"
		}
		return strconv.FormatBool(*x)
	case string:
		if x == "" {
			return "null"
		}
		return x
	default:
		return fmt.Sprint(x)
	}
}

func (c Config) value(key string) any {
	switch key {
	case "lang":
		return c.Lang
	case "moderationGroupId":
		return c.ModerationGroupID
	case "approvedGroupId":
		return c.ApprovedGroupID
	case "moderationTopicId":
		return c.ModerationTopicID
	case "approvedTopicId":
		return c.ApprovedTopicID
	case "timeOut":
		return c.TimeOut
	case "validatePrice":
		return c.ValidatePrice
	case "minimumPhotos":
		return c.MinimumPhotos
	case "dailyBumpLimit":
		return c.DailyBumpLimit
	case "donationsEnabled":
		return c.DonationsEnabled
	case "enableFaq":
		return c.EnableFaq
	case "mediaLayout":
		return c.MediaLayout
	case "faqAllowInGroups":
		return c.FaqAllowInGroups
	case "faqMaxButtonsPerRow":
		return c.FaqMaxButtonsPerRow
	case "faqMaxDepth":
		return c.FaqMaxDepth
	case "broadcastTopicId":
		return c.BroadcastTopicID
	}
	return nil
}

// Update validates raw against the key's schema, applies it, and persists the
// file. The returned ParseResult reports validation failures; err reports I/O.
func (s *Store) Update(key, raw string, availableLocales []string) (ParseResult, error) {
	res := ParseValue(key, raw, availableLocales)
	if !res.OK {
		return res, nil
	}
	s.mu.Lock()
	s.cfg.set(key, res.Value)
	s.mu.Unlock()
	return res, s.Save()
}

func (c *Config) set(key string, v any) {
	num := func() *int64 {
		if v == nil {
			return nil
		}
		n := v.(int64)
		return &n
	}
	boolean := func() *bool {
		b := v.(bool)
		return &b
	}
	switch key {
	case "lang":
		c.Lang = v.(string)
	case "mediaLayout":
		c.MediaLayout = v.(string)
	case "moderationGroupId":
		c.ModerationGroupID = v.(int64)
	case "approvedGroupId":
		c.ApprovedGroupID = v.(int64)
	case "moderationTopicId":
		c.ModerationTopicID = num()
	case "approvedTopicId":
		c.ApprovedTopicID = num()
	case "broadcastTopicId":
		c.BroadcastTopicID = num()
	case "timeOut":
		c.TimeOut = v.(int64)
	case "minimumPhotos":
		c.MinimumPhotos = v.(int64)
	case "dailyBumpLimit":
		c.DailyBumpLimit = v.(int64)
	case "validatePrice":
		c.ValidatePrice = v.(bool)
	case "donationsEnabled":
		c.DonationsEnabled = boolean()
	case "enableFaq":
		c.EnableFaq = boolean()
	case "faqAllowInGroups":
		c.FaqAllowInGroups = boolean()
	case "faqMaxButtonsPerRow":
		c.FaqMaxButtonsPerRow = num()
	case "faqMaxDepth":
		c.FaqMaxDepth = num()
	}
}

// Save writes the configuration back to disk with 4-space indentation.
func (s *Store) Save() error {
	if s.path == "" {
		return nil
	}
	c := s.Get()
	data, err := json.MarshalIndent(c, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, append(data, '\n'), 0o644)
}

// Display formats a parsed value the way /config echoes it back.
func Display(v any) string {
	if v == nil {
		return "null"
	}
	return fmt.Sprint(v)
}
