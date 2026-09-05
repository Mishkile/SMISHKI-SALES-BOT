// Package locale resolves a user's language and serves translated strings and
// FAQ trees from <lang>/common.json and <lang>/faq.json.
package locale

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"jsts-salebot/internal/models"
)

// Params are the {placeholder} substitutions for T.
type Params map[string]any

// Faqs is one locale's FAQ tree. Keys preserves the file's key order with
// JavaScript's object-key semantics (integer-like keys first, ascending, then
// insertion order), so the menu renders exactly as the original bot did.
type Faqs struct {
	Keys  []string
	Nodes map[string]string
}

// Service loads and caches locale files.
type Service struct {
	fsys        fs.FS
	defaultLang func() string
	locales     []string

	mu       sync.Mutex
	messages map[string]map[string]string
	faqs     map[string]*Faqs
}

// New discovers the locale directories in fsys. defaultLang supplies the
// fallback locale (config.lang) and is consulted live so /config lang applies.
func New(fsys fs.FS, defaultLang func() string) *Service {
	s := &Service{
		fsys:        fsys,
		defaultLang: defaultLang,
		messages:    map[string]map[string]string{},
		faqs:        map[string]*Faqs{},
	}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		log.Printf("[ERROR - LocaleService] Failed to discover locales: %v", err)
		return s
	}
	for _, e := range entries {
		if e.IsDir() {
			s.locales = append(s.locales, e.Name())
		}
	}
	sort.Strings(s.locales)
	log.Printf("[INFO - LocaleService] discovered locales %v", s.locales)
	return s
}

// AvailableLocales returns a copy of the discovered locale codes.
func (s *Service) AvailableLocales() []string {
	return append([]string(nil), s.locales...)
}

// Has reports whether locale is available.
func (s *Service) Has(locale string) bool {
	for _, l := range s.locales {
		if l == locale {
			return true
		}
	}
	return false
}

// normalize maps "en-US" to "en" when "en" is available, else "".
func (s *Service) normalize(locale string) string {
	if locale == "" {
		return ""
	}
	base := strings.SplitN(locale, "-", 2)[0]
	if s.Has(base) {
		return base
	}
	return ""
}

// ResolveUserLocale picks user.preferredLocale, then user.languageCode
// (normalized), then the configured default.
func (s *Service) ResolveUserLocale(user *models.User) string {
	userID := "unknown"
	if user != nil {
		userID = user.UserID
		if pref := models.Str(user.PreferredLocale); pref != "" {
			if n := s.normalize(pref); n != "" {
				return n
			}
			log.Printf("[WARN - LocaleService] User %s preferredLocale %q unsupported.", userID, pref)
		}
		if code := models.Str(user.LanguageCode); code != "" {
			if n := s.normalize(code); n != "" {
				return n
			}
			log.Printf("[WARN - LocaleService] User %s languageCode %q unsupported.", userID, code)
		}
	}
	def := s.defaultLang()
	log.Printf("[WARN - LocaleService] Falling back to default locale %q for user %s as no preferredLocale or languageCode provided.", def, userID)
	return def
}

// Messages returns the key-to-text map for locale/namespace (namespace "common"
// by default), cached after first load. Missing files yield an empty map.
func (s *Service) Messages(locale, namespace string) map[string]string {
	if namespace == "" {
		namespace = "common"
	}
	key := locale + "-" + namespace
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.messages[key]; ok {
		return m
	}
	m := map[string]string{}
	data, err := fs.ReadFile(s.fsys, path.Join(locale, namespace+".json"))
	if err == nil {
		err = json.Unmarshal(data, &m)
	}
	if err != nil {
		log.Printf("[ERROR - LocaleService] Failed to load messages for %s/%s: %v", locale, namespace, err)
		m = map[string]string{}
	}
	s.messages[key] = m
	return m
}

// T translates key in locale, substituting {name} placeholders from params.
// Unknown keys fall back to the key itself with a warning.
func (s *Service) T(locale, key string, params ...Params) string {
	text, ok := s.Messages(locale, "common")[key]
	if !ok || text == "" {
		log.Printf("[WARN - LocaleService] missing translation key {locale: %s, key: %s}", locale, key)
		text = key
	}
	for _, p := range params {
		for name, value := range p {
			text = strings.ReplaceAll(text, "{"+name+"}", fmt.Sprint(value))
		}
	}
	return text
}

// GetFaqs returns the FAQ tree for locale (empty when missing/invalid).
func (s *Service) GetFaqs(locale string) *Faqs {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.faqs[locale]; ok {
		return f
	}
	f, err := loadFaqs(s.fsys, locale)
	if err != nil {
		log.Printf("[ERROR - LocaleService] Failed to load FAQ for %s: %v", locale, err)
		f = &Faqs{Nodes: map[string]string{}}
	}
	s.faqs[locale] = f
	return f
}

func loadFaqs(fsys fs.FS, locale string) (*Faqs, error) {
	data, err := fs.ReadFile(fsys, path.Join(locale, "faq.json"))
	if err != nil {
		return nil, err
	}
	var file struct {
		Nodes json.RawMessage `json:"nodes"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if len(file.Nodes) == 0 {
		return &Faqs{Nodes: map[string]string{}}, nil
	}
	return ParseOrderedNodes(file.Nodes)
}

// ParseOrderedNodes decodes a JSON object of string values keeping key order
// as JavaScript would enumerate it.
func ParseOrderedNodes(raw []byte) (*Faqs, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("nodes must be an object")
	}
	f := &Faqs{Nodes: map[string]string{}}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key := keyTok.(string)
		var value string
		if err := dec.Decode(&value); err != nil {
			return nil, fmt.Errorf("node %q: %w", key, err)
		}
		if _, dup := f.Nodes[key]; !dup {
			f.Keys = append(f.Keys, key)
		}
		f.Nodes[key] = value
	}
	if _, err := dec.Token(); err != nil { // closing brace
		return nil, err
	}
	f.Keys = jsKeyOrder(f.Keys)
	return f, nil
}

// jsKeyOrder reorders keys like JavaScript's OrdinaryOwnPropertyKeys: array
// indices (canonical non-negative integers below 2^32-1) first in ascending
// numeric order, then the remaining keys in insertion order.
func jsKeyOrder(keys []string) []string {
	type idx struct {
		n   uint64
		key string
	}
	var indices []idx
	var rest []string
	for _, k := range keys {
		if n, ok := arrayIndex(k); ok {
			indices = append(indices, idx{n, k})
		} else {
			rest = append(rest, k)
		}
	}
	sort.SliceStable(indices, func(i, j int) bool { return indices[i].n < indices[j].n })
	out := make([]string, 0, len(keys))
	for _, i := range indices {
		out = append(out, i.key)
	}
	return append(out, rest...)
}

func arrayIndex(k string) (uint64, bool) {
	if k == "" || (len(k) > 1 && k[0] == '0') {
		return 0, false
	}
	n, err := strconv.ParseUint(k, 10, 64)
	if err != nil || n >= 4294967295 {
		return 0, false
	}
	return n, true
}
