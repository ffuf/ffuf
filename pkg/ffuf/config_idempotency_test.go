package ffuf

import (
	"context"
	"strings"
	"testing"
)

// TestConfigFromOptions_Idempotent locks that ConfigFromOptions does not mutate its
// input and produces the same result when called twice on the same ConfigOptions.
// Previously it folded -b/-cookie into parseOpts.HTTP.Headers in place, so a second
// call double-appended the Cookie header and corrupted the caller's options.
func TestConfigFromOptions_Idempotent(t *testing.T) {
	opts := NewConfigOptions()
	opts.HTTP.URL = "https://example.org/FUZZ"
	opts.Input.Wordlists = []string{"/tmp/wl.txt"}
	opts.HTTP.Cookies = []string{"SESSION=abc"}

	c1, err := ConfigFromOptions(opts, context.Background(), func() {})
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	c2, err := ConfigFromOptions(opts, context.Background(), func() {})
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	// The caller's options must be untouched (no in-place Cookie fold).
	if len(opts.HTTP.Headers) != 0 {
		t.Errorf("ConfigFromOptions mutated its input: opts.HTTP.Headers = %v", opts.HTTP.Headers)
	}

	// Each call folds exactly one Cookie header, identically.
	for i, c := range []*Config{c1, c2} {
		if got := c.Headers["Cookie"]; got != "SESSION=abc" {
			t.Errorf("call %d: Cookie header = %q, want %q", i+1, got, "SESSION=abc")
		}
		n := 0
		for _, h := range c.Options.HTTP.Headers {
			if strings.HasPrefix(h, "Cookie: ") {
				n++
			}
		}
		if n != 1 {
			t.Errorf("call %d: retained snapshot has %d Cookie headers, want 1 (%v)", i+1, n, c.Options.HTTP.Headers)
		}
	}
}
