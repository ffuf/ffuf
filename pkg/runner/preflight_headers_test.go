package runner

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// headersServer sends a body with a token and a set of headers, some of which
// carry values a regex could also match.
func headersServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Token", "token=hdr1")
		h.Set("Location", "/cb?code=abc123&state=xyz")
		h.Add("Set-Cookie", "sessionid=sess-1; Path=/")
		h.Add("Set-Cookie", "XSRF-TOKEN=xsrf-2; Path=/")
		fmt.Fprint(w, `<p>token=body1</p>`)
	}))
}

func regexVar(t *testing.T, r *SimpleRunner, srv *httptest.Server, regex string) (string, error) {
	t.Helper()
	return runVar(t, r, srv, "/", ffuf.VarExtract{Name: "V", Regex: regex})
}

// A regex that matches the body keeps capturing from the body, even when a
// header would match it too: specs written before headers were searched still
// capture the same text.
func TestPreflightVarBodyWinsOverHeaders(t *testing.T) {
	srv := headersServer()
	defer srv.Close()
	got, err := regexVar(t, newTestRunner(newTestConfig(srv.URL)), srv, `token=(\w+)`)
	if err != nil || got != "body1" {
		t.Fatalf("got %q, %v; want body1", got, err)
	}
}

// A regex that finds nothing in the body is run against the headers.
func TestPreflightVarFallsBackToHeaders(t *testing.T) {
	srv := headersServer()
	defer srv.Close()
	cases := map[string]string{
		// Part of a header value, the case the by-name [header] source can't do.
		`(?im)^location:.*[?&]code=([^&\r\n]+)`: "abc123",
		// A cookie other than the first Set-Cookie.
		`XSRF-TOKEN=([^;\r\n]+)`: "xsrf-2",
		// Names are canonical, so an exact-case regex written that way works.
		`X-Token: token=(\w+)`: "hdr1",
	}
	for regex, want := range cases {
		got, err := regexVar(t, newTestRunner(newTestConfig(srv.URL)), srv, regex)
		if err != nil || got != want {
			t.Errorf("%s: got %q, %v; want %q", regex, got, err, want)
		}
	}
}

func TestPreflightVarNoMatchMentionsHeaders(t *testing.T) {
	srv := headersServer()
	defer srv.Close()
	_, err := regexVar(t, newTestRunner(newTestConfig(srv.URL)), srv, `nothere=(\w+)`)
	if err == nil || !strings.Contains(err.Error(), "body or headers") {
		t.Fatalf("error = %v, want it to say the body and headers were searched", err)
	}
}

// A regex that runs across header lines captures a CRLF, which the existing
// control-character check refuses.
func TestPreflightVarHeaderSpanRefused(t *testing.T) {
	srv := headersServer()
	defer srv.Close()
	_, err := regexVar(t, newTestRunner(newTestConfig(srv.URL)), srv, `(?s)Location: (.*)Set-Cookie`)
	if err == nil || !strings.Contains(err.Error(), "control character") {
		t.Fatalf("error = %v, want a control character refusal", err)
	}
}
