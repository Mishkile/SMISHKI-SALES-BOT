package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	content := "# comment\nJSTS_TEST_A=hello\nexport JSTS_TEST_B=\"quoted value\"\nJSTS_TEST_C='single'\nJSTS_TEST_EXISTING=from-file\nnot a pair\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JSTS_TEST_EXISTING", "from-env")
	for _, k := range []string{"JSTS_TEST_A", "JSTS_TEST_B", "JSTS_TEST_C"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}

	loadDotEnv(path)

	if got := os.Getenv("JSTS_TEST_A"); got != "hello" {
		t.Errorf("A = %q", got)
	}
	if got := os.Getenv("JSTS_TEST_B"); got != "quoted value" {
		t.Errorf("B = %q", got)
	}
	if got := os.Getenv("JSTS_TEST_C"); got != "single" {
		t.Errorf("C = %q", got)
	}
	if got := os.Getenv("JSTS_TEST_EXISTING"); got != "from-env" {
		t.Errorf("existing env must win, got %q", got)
	}
	loadDotEnv(filepath.Join(t.TempDir(), "missing")) // must not panic
}
