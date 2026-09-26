package ffuf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// parseRawFile writes raw to a temp file and runs it through parseRawRequest.
func parseRawFile(t *testing.T, raw string) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "request.txt")
	if err := os.WriteFile(path, []byte(raw), 0644); err != nil {
		t.Fatal(err)
	}
	opts := NewConfigOptions()
	opts.Input.Request = path
	opts.Input.RequestProto = "https"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf := NewConfig(ctx, cancel)
	return &conf, parseRawRequest(opts, &conf)
}

// TestParseRawRequestKeepsLastHeaderWithoutTrailingNewline is the regression this
// file exists for: the header loop used to check the read error before storing the
// line, so the last header of a file that does not end in a newline was dropped
// without a word. A dropped Cookie or Authorization header turns a whole scan into
// false negatives.
func TestParseRawRequestKeepsLastHeaderWithoutTrailingNewline(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"lf", "POST /a HTTP/1.1\nHost: example.org\nCookie: sid=abc"},
		{"crlf", "POST /a HTTP/1.1\r\nHost: example.org\r\nCookie: sid=abc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf, err := parseRawFile(t, tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := conf.Headers["Cookie"]; got != "sid=abc" {
				t.Errorf("last header was dropped: Cookie=%q, headers=%v", got, conf.Headers)
			}
			if got := conf.Headers["Host"]; got != "example.org" {
				t.Errorf("Host=%q, want example.org", got)
			}
		})
	}
}

// TestParseRawRequestSingleLine covers a request file that is just the request
// line with an absolute URL, with and without a trailing newline. Without one the
// first read returns the line together with io.EOF, which used to abort with
// "could not read request: EOF".
func TestParseRawRequestSingleLine(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
	}{
		{"no trailing newline", "GET http://example.org/FUZZ HTTP/1.1"},
		{"trailing newline", "GET http://example.org/FUZZ HTTP/1.1\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf, err := parseRawFile(t, tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if conf.Method != "GET" {
				t.Errorf("Method=%q, want GET", conf.Method)
			}
			if conf.Url != "http://example.org/FUZZ" {
				t.Errorf("Url=%q, want http://example.org/FUZZ", conf.Url)
			}
			if conf.Headers["Host"] != "example.org" {
				t.Errorf("Host=%q, want example.org", conf.Headers["Host"])
			}
		})
	}
}

// TestParseRawRequestRejections locks the errors, including the CR-only case:
// ReadString('\n') hands back the entire file as one line there, so a request
// built from it would silently target the wrong thing.
func TestParseRawRequestRejections(t *testing.T) {
	for _, tc := range []struct {
		name, raw, wantErr string
	}{
		{"empty file", "", "is empty"},
		{"cr only line endings", "POST /a HTTP/1.1\rHost: example.org\r\rbody", "CR-only line endings"},
		{"request line without version", "GET /a\nHost: example.org\n\n", "malformed request supplied"},
		{"blank first line", "\nPOST /a HTTP/1.1\nHost: example.org\n\n", "malformed request supplied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseRawFile(t, tc.raw)
			if err == nil {
				t.Fatalf("expected an error containing %q, got none", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// TestParseRawRequestWellFormed pins the behavior that was already correct, so the
// line-ending changes above cannot quietly alter it.
func TestParseRawRequestWellFormed(t *testing.T) {
	for _, tc := range []struct {
		name, raw, wantURL, wantData string
		wantHeaders                  map[string]string
	}{
		{
			name:        "lf with body",
			raw:         "POST /a HTTP/1.1\nHost: example.org\nContent-Length: 6\n\nu=FUZZ\n",
			wantURL:     "https://example.org/a",
			wantData:    "u=FUZZ",
			wantHeaders: map[string]string{"Host": "example.org"},
		},
		{
			name:        "crlf with body",
			raw:         "POST /a HTTP/1.1\r\nHost: example.org\r\n\r\nu=FUZZ\r\n",
			wantURL:     "https://example.org/a",
			wantData:    "u=FUZZ",
			wantHeaders: map[string]string{"Host": "example.org"},
		},
		{
			name:        "absolute url wins over host header",
			raw:         "GET http://other.example/x HTTP/1.1\nHost: ignored.example\n\n",
			wantURL:     "http://other.example/x",
			wantData:    "",
			wantHeaders: map[string]string{"Host": "other.example"},
		},
		{
			name:        "header value keeps its colons",
			raw:         "GET /a HTTP/1.1\nHost: example.org\nReferer: http://example.org/b\n\n",
			wantURL:     "https://example.org/a",
			wantData:    "",
			wantHeaders: map[string]string{"Host": "example.org", "Referer": "http://example.org/b"},
		},
		{
			name:        "line without a colon is skipped, not fatal",
			raw:         "GET /a HTTP/1.1\nHost: example.org\ngarbage\n\nbody",
			wantURL:     "https://example.org/a",
			wantData:    "body",
			wantHeaders: map[string]string{"Host": "example.org"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conf, err := parseRawFile(t, tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if conf.Url != tc.wantURL {
				t.Errorf("Url=%q, want %q", conf.Url, tc.wantURL)
			}
			if conf.Data != tc.wantData {
				t.Errorf("Data=%q, want %q", conf.Data, tc.wantData)
			}
			if len(conf.Headers) != len(tc.wantHeaders) {
				t.Errorf("Headers=%v, want %v", conf.Headers, tc.wantHeaders)
			}
			for k, v := range tc.wantHeaders {
				if conf.Headers[k] != v {
					t.Errorf("Headers[%q]=%q, want %q", k, conf.Headers[k], v)
				}
			}
		})
	}
}
