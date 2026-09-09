package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

func destRunner(t *testing.T, targetURL string, allowCaptured bool) *SimpleRunner {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	conf := ffuf.NewConfig(ctx, cancel)
	conf.Url = targetURL
	conf.PreflightAnyHost = allowCaptured
	return NewSimpleRunner(&conf, false).(*SimpleRunner)
}

// Flight requests inherit the operator's auth headers, and captured values come
// from the scanned target. A captured value that moves the request to a host the
// operator did not write is refused, so the target cannot pick where those
// credentials go.
func TestPreflight_CapturedValueCannotChangeDestination(t *testing.T) {
	cases := []struct {
		name string
		file string
		vars map[string]string
	}{
		{
			name: "captured absolute URL in the request line",
			file: "GET NEXT HTTP/1.1\n\n",
			vars: map[string]string{"NEXT": "http://attacker.example.invalid/steal"},
		},
		{
			name: "captured host in the Host header, relative path",
			file: "GET /steal HTTP/1.1\nHost: NEXTHOST\n\n",
			vars: map[string]string{"NEXTHOST": "attacker.example.invalid"},
		},
		{
			name: "captured value changes the port only",
			file: "GET /steal HTTP/1.1\nHost: NEXTHOST\n\n",
			vars: map[string]string{"NEXTHOST": "target.example.com:9999"},
		},
		{
			name: "captured value downgrades the scheme",
			file: "GET NEXT HTTP/1.1\n\n",
			vars: map[string]string{"NEXT": "http://target.example.com/steal"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := destRunner(t, "https://target.example.com/FUZZ", false)
			_, err := runner.parsePreflightRequest(writeTempRequest(t, tc.file), tc.vars)
			if err == nil {
				t.Fatal("expected the request to be refused, it was built")
			}
			if !strings.Contains(err.Error(), "changed the request destination") {
				t.Errorf("unexpected error: %s", err)
			}
		})
	}
}

// The check is scoped to captured values, not to cross-host preflights. Anything
// the operator wrote themselves still works, including authenticating against a
// separate identity provider.
func TestPreflight_AuthoredDestinationsStillWork(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		vars    map[string]string
		wantURL string
	}{
		{
			name:    "operator writes an absolute URL to a different host",
			file:    "POST https://login.example.com/token HTTP/1.1\n\n",
			vars:    map[string]string{"TOKEN": "abc123"},
			wantURL: "https://login.example.com/token",
		},
		{
			name:    "operator writes a different host in the Host header",
			file:    "POST /token HTTP/1.1\nHost: login.example.com\n\n",
			vars:    map[string]string{"TOKEN": "abc123"},
			wantURL: "https://login.example.com/token",
		},
		{
			name:    "captured value lands in the path, destination unchanged",
			file:    "GET /api/TOKEN/profile HTTP/1.1\n\n",
			vars:    map[string]string{"TOKEN": "abc123"},
			wantURL: "https://target.example.com/api/abc123/profile",
		},
		{
			name:    "captured value lands in a header, destination unchanged",
			file:    "GET /api/profile HTTP/1.1\nX-Csrf-Token: TOKEN\n\n",
			vars:    map[string]string{"TOKEN": "abc123"},
			wantURL: "https://target.example.com/api/profile",
		},
		{
			name:    "captured value lands in the body, destination unchanged",
			file:    "POST /api/login HTTP/1.1\n\ncsrf=TOKEN",
			vars:    map[string]string{"TOKEN": "abc123"},
			wantURL: "https://target.example.com/api/login",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := destRunner(t, "https://target.example.com/FUZZ", false)
			req, err := runner.parsePreflightRequest(writeTempRequest(t, tc.file), tc.vars)
			if err != nil {
				t.Fatalf("operator-authored destination was refused: %s", err)
			}
			if got := req.URL.String(); got != tc.wantURL {
				t.Errorf("URL = %q, want %q", got, tc.wantURL)
			}
		})
	}
}

// Discovery-driven flows genuinely need the target to name the next host, e.g.
// reading token_endpoint out of /.well-known/openid-configuration. That is what
// the opt-in is for.
func TestPreflight_AllowCapturedHostOptsBackIn(t *testing.T) {
	file := "POST NEXT HTTP/1.1\n\n"
	vars := map[string]string{"NEXT": "https://login.example.com/oauth2/token"}

	runner := destRunner(t, "https://target.example.com/FUZZ", true)
	req, err := runner.parsePreflightRequest(writeTempRequest(t, file), vars)
	if err != nil {
		t.Fatalf("with -preflight-anyhost the request must be built: %s", err)
	}
	if got := req.URL.String(); got != "https://login.example.com/oauth2/token" {
		t.Errorf("URL = %q, want the captured endpoint", got)
	}
}

// With no captured variables there is nothing to check, and the file is used as
// written.
func TestPreflight_NoVarsIsUnaffected(t *testing.T) {
	runner := destRunner(t, "https://target.example.com/FUZZ", false)
	req, err := runner.parsePreflightRequest(writeTempRequest(t, "GET /login HTTP/1.1\n\n"), nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got := req.URL.String(); got != "https://target.example.com/login" {
		t.Errorf("URL = %q", got)
	}
}
