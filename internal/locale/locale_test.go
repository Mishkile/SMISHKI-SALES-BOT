package locale

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"jsts-salebot/internal/models"
)

// --- unit tests -------------------------------------------------------------

func newTestService(t *testing.T) *Service {
	t.Helper()
	fsys := fstest.MapFS{
		"en/common.json": {Data: []byte(`{"greeting":"Hi","report":"{sent} sent of {total}, ok={ok}"}`)},
		"en/faq.json":    {Data: []byte(`{"meta":{"locale":"en"},"nodes":{"1":"A","1.1":"A1","1.1.1":"A11","2":"B","10":"J","2.1":"B1"}}`)},
		"he/common.json": {Data: []byte(`{"greeting":"shalom"}`)},
		"he/faq.json":    {Data: []byte(`{"meta":{"locale":"he"},"nodes":{}}`)},
	}
	return New(fsys, func() string { return "en" })
}

func TestDiscoverAndTranslate(t *testing.T) {
	s := newTestService(t)
	if got := s.AvailableLocales(); strings.Join(got, ",") != "en,he" {
		t.Fatalf("locales = %v", got)
	}
	if got := s.T("he", "greeting"); got != "shalom" {
		t.Fatalf("he greeting = %q", got)
	}
	if got := s.T("en", "report", Params{"sent": 3, "total": 5, "ok": true}); got != "3 sent of 5, ok=true" {
		t.Fatalf("placeholders = %q", got)
	}
	if got := s.T("en", "nope"); got != "nope" {
		t.Fatalf("missing key should fall back to key, got %q", got)
	}
}

func TestResolveUserLocale(t *testing.T) {
	s := newTestService(t)
	str := func(v string) *string { return &v }
	cases := []struct {
		name string
		user *models.User
		want string
	}{
		{"nil user", nil, "en"},
		{"preferred", &models.User{PreferredLocale: str("he"), LanguageCode: str("en")}, "he"},
		{"unsupported preferred falls to language code", &models.User{PreferredLocale: str("fr"), LanguageCode: str("he-IL")}, "he"},
		{"language code region stripped", &models.User{LanguageCode: str("en-US")}, "en"},
		{"nothing supported", &models.User{LanguageCode: str("de")}, "en"},
	}
	for _, c := range cases {
		if got := s.ResolveUserLocale(c.user); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestFaqKeyOrderMatchesJavaScript(t *testing.T) {
	s := newTestService(t)
	f := s.GetFaqs("en")
	want := []string{"1", "2", "10", "1.1", "1.1.1", "2.1"}
	if strings.Join(f.Keys, " ") != strings.Join(want, " ") {
		t.Fatalf("keys = %v want %v", f.Keys, want)
	}
	if f.Nodes["1.1.1"] != "A11" {
		t.Fatalf("nodes = %v", f.Nodes)
	}
	if e := s.GetFaqs("he"); len(e.Keys) != 0 || len(e.Nodes) != 0 {
		t.Fatalf("empty faq = %+v", e)
	}
	if e := s.GetFaqs("zz"); len(e.Nodes) != 0 {
		t.Fatalf("missing faq should be empty")
	}
}

// --- locale file integrity (port of src/tests/checkLocals.ts) --------------

const localesDir = "../../locales"

func TestLocaleFilesAreConsistent(t *testing.T) {
	entries, err := os.ReadDir(localesDir)
	if err != nil {
		t.Fatalf("locales directory not found: %v", err)
	}
	var languages []string
	for _, e := range entries {
		if e.IsDir() {
			languages = append(languages, e.Name())
		}
	}
	t.Logf("Locales found: %s", strings.Join(languages, ", "))

	placeholder := regexp.MustCompile(`\{([^}]+)\}`)
	validPlaceholder := regexp.MustCompile(`^\{[a-zA-Z_][a-zA-Z0-9_]*\}$`)

	allKeys := map[string]bool{}
	localeData := map[string]map[string]string{}
	allFaqKeys := map[string]bool{}
	faqData := map[string]map[string]string{}

	// 1. Validate individual common.json files (syntax, values, placeholders).
	for _, lang := range languages {
		p := filepath.Join(localesDir, lang, "common.json")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("common.json not found for language %s", lang)
			continue
		}
		var data map[string]string
		if err := json.Unmarshal(raw, &data); err != nil {
			t.Errorf("Invalid JSON in %s/common.json: %v", lang, err)
			continue
		}
		var empty, invalid []string
		for k, v := range data {
			if strings.TrimSpace(v) == "" {
				empty = append(empty, k)
			}
			for _, m := range placeholder.FindAllString(v, -1) {
				if !validPlaceholder.MatchString(m) {
					invalid = append(invalid, k)
					break
				}
			}
			allKeys[k] = true
		}
		if len(empty) > 0 {
			sort.Strings(empty)
			t.Errorf("Empty values in %s/common.json for keys: %s", lang, strings.Join(empty, ", "))
		}
		if len(invalid) > 0 {
			sort.Strings(invalid)
			t.Errorf("Invalid placeholder syntax in %s/common.json for keys: %s", lang, strings.Join(invalid, ", "))
		}
		localeData[lang] = data
	}

	// 2. Validate faq.json files.
	for _, lang := range languages {
		p := filepath.Join(localesDir, lang, "faq.json")
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("faq.json not found for language %s", lang)
			continue
		}
		var file struct {
			Nodes map[string]string `json:"nodes"`
		}
		if err := json.Unmarshal(raw, &file); err != nil {
			t.Errorf("Invalid JSON in %s/faq.json: %v", lang, err)
			continue
		}
		if file.Nodes == nil {
			t.Errorf("Invalid FAQ structure in %s/faq.json - missing 'nodes' object", lang)
			continue
		}
		var empty []string
		for k, v := range file.Nodes {
			if strings.TrimSpace(v) == "" {
				empty = append(empty, k)
			}
			allFaqKeys[k] = true
		}
		if len(empty) > 0 {
			t.Errorf("Empty values in %s/faq.json for keys: %s", lang, strings.Join(empty, ", "))
		}
		faqData[lang] = file.Nodes
	}

	// 3. common.json bidirectional check: every language has every key.
	sortedKeys := sortedSet(allKeys)
	for _, lang := range languages {
		var missing []string
		for _, k := range sortedKeys {
			if _, ok := localeData[lang][k]; !ok {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			t.Errorf("Language '%s' is missing keys: %s", lang, strings.Join(missing, ", "))
		}
	}

	// 4. faq.json coverage is aspirational content, not part of the locale
	// contract: warn but don't fail.
	for _, lang := range languages {
		var missing []string
		for _, k := range sortedSet(allFaqKeys) {
			if _, ok := faqData[lang][k]; !ok {
				missing = append(missing, k)
			}
		}
		if len(missing) > 0 {
			t.Logf("[WARN] FAQ Language '%s' is missing IDs: %s", lang, strings.Join(missing, ", "))
		}
	}

	// 5. Scan Go sources for key usage (warn only).
	var sources []string
	root := "../.."
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
			b, err := os.ReadFile(p)
			if err == nil {
				sources = append(sources, string(b))
			}
		}
		return nil
	})
	all := strings.Join(sources, "\n")
	var unused []string
	for _, k := range sortedKeys {
		re := regexp.MustCompile(`["']` + regexp.QuoteMeta(k) + `["']`)
		if !re.MatchString(all) {
			unused = append(unused, k)
		}
	}
	if len(unused) > 0 {
		t.Logf("[WARN] Potentially unused keys (%d):\n     - %s", len(unused), strings.Join(unused, "\n     - "))
	} else {
		t.Logf("All keys appear to be used.")
	}
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
