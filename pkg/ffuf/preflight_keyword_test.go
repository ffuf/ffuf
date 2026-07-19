package ffuf

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestKeywordInPreflightRetainsWordlist guards the config-side half of keyword
// support: a wordlist keyword used ONLY in a preflight file (not the main request)
// must not have its provider dropped by keywordPresent.
func TestKeywordInPreflightRetainsWordlist(t *testing.T) {
	dir := t.TempDir()
	wl := filepath.Join(dir, "wl.txt")
	if err := os.WriteFile(wl, []byte("admin\nroot\n"), 0644); err != nil {
		t.Fatal(err)
	}
	pre := filepath.Join(dir, "pre.txt")
	if err := os.WriteFile(pre, []byte("GET /pre/FUZZ HTTP/1.1\nHost: example.com\n\n"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := NewConfigOptions()
	opts.HTTP.URL = "http://example.com/" // no FUZZ in the main request
	opts.Input.Wordlists = []string{wl}   // default keyword FUZZ
	opts.HTTP.Preflights = []PreflightConfig{{RequestFile: pre}}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf, _ := ConfigFromOptions(opts, ctx, cancel)

	found := false
	for _, ip := range conf.InputProviders {
		if ip.Keyword == "FUZZ" {
			found = true
		}
	}
	if !found {
		t.Error("FUZZ wordlist provider was dropped even though FUZZ is used in the preflight file")
	}
}

// TestKeywordPresentChecksPreflightFiles is the direct unit test for the gate.
func TestKeywordPresentChecksPreflightFiles(t *testing.T) {
	dir := t.TempDir()
	pre := filepath.Join(dir, "pre.txt")
	if err := os.WriteFile(pre, []byte("GET /login?u=FUZZ HTTP/1.1\n\n"), 0644); err != nil {
		t.Fatal(err)
	}
	conf := &Config{Preflights: []PreflightConfig{{RequestFile: pre}}}

	if !keywordPresent("FUZZ", conf) {
		t.Error("keywordPresent should find a keyword used in a preflight file")
	}
	if keywordPresent("NOPE", conf) {
		t.Error("keywordPresent should not find an absent keyword")
	}
}
