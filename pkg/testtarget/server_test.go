package testtarget

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestBodyHelpers grounds the size/word/line helpers against independently
// computed values, NOT against ffuf's own counters, so a symmetric off-by-one in
// both the mock and ffuf can't cancel out and hide a counting regression.
func TestBodyHelpers(t *testing.T) {
	for _, n := range []int{0, 1, 37, 100} {
		if got := len(filled(n)); got != n {
			t.Errorf("filled(%d) length = %d, want %d", n, got, n)
		}
	}
	// words(n) is n tokens, i.e. n-1 single-space separators.
	for _, n := range []int{1, 5, 10} {
		if got := strings.Count(words(n), " "); got != n-1 {
			t.Errorf("words(%d) has %d spaces, want %d", n, got, n-1)
		}
	}
	// lines(n) is n lines, i.e. n-1 newline separators (no trailing newline).
	for _, n := range []int{1, 3, 5} {
		if got := strings.Count(lines(n), "\n"); got != n-1 {
			t.Errorf("lines(%d) has %d newlines, want %d", n, got, n-1)
		}
	}
}

// TestEndpoints checks the gate/status behavior the integration suite relies on,
// so a mock bug is caught here rather than misattributed to ffuf.
func TestEndpoints(t *testing.T) {
	tt := New()
	defer tt.Close()

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	cases := []struct {
		path   string
		status int
	}{
		{"/status/418", 418},
		{"/size/50", 200},
		{"/map/ok", 200},
		{"/map/bad", 500},
		{"/needs-header", 403}, // gated: no header
		{"/rdir", 301},         // redirect-based directory
		{"/nope", 404},
	}
	for _, c := range cases {
		resp, err := client.Get(tt.URL + c.path)
		if err != nil {
			t.Fatalf("GET %s: %v", c.path, err)
		}
		if resp.StatusCode != c.status {
			t.Errorf("GET %s: status %d, want %d", c.path, resp.StatusCode, c.status)
		}
		resp.Body.Close()
	}

	// /size/50 body is exactly 50 bytes.
	resp, err := client.Get(tt.URL + "/size/50")
	if err != nil {
		t.Fatalf("GET /size/50: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if len(b) != 50 {
		t.Errorf("/size/50 body = %d bytes, want 50", len(b))
	}

	// The gated header endpoint opens with the right header.
	req, _ := http.NewRequest("GET", tt.URL+"/needs-header", nil)
	req.Header.Set("X-Test", "yes")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("GET /needs-header (with header): %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("/needs-header with X-Test: got %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	// Requests were recorded.
	if tt.Count() == 0 {
		t.Error("recorder captured no requests")
	}
}
