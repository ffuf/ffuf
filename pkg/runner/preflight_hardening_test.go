package runner

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// TestHostPinBlocksHostChange is the fix for the host-hijack finding: a value
// captured from a preflight response that would change the request's host makes
// Execute refuse to send, so inherited credentials never reach another host.
func TestHostPinBlocksHostChange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "host=evil.example")
	}))
	defer srv.Close()
	reqFile := writeTempRequest(t, fmt.Sprintf("GET / HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String()))
	conf := newTestConfig(srv.URL)
	conf.Preflights = []ffuf.PreflightConfig{{
		RequestFile: reqFile,
		Vars:        []ffuf.VarExtract{{Name: "HOSTVAR", Regex: `host=([\w.]+)`}},
	}}
	r := newTestRunner(conf)

	req := &ffuf.Request{Method: "GET", Url: "http://HOSTVAR/main", Headers: map[string]string{}}
	_, err := r.Execute(req)
	if err == nil {
		t.Fatal("expected Execute to refuse a host-changing preflight variable")
	}
	if !strings.Contains(err.Error(), "changed the request host") {
		t.Errorf("unexpected error: %s", err)
	}
}

// TestSubstitutionDeterministic proves substitution is order-independent (longest
// name first) and single-pass (a replaced value is not rescanned), so overlapping
// names and value-contains-name cascades give one fixed result.
func TestSubstitutionDeterministic(t *testing.T) {
	r := newTestRunner(newTestConfig("http://x"))
	vars := map[string]string{"TOK": "aaa", "TOKEN": "bbb", "AAA": "BBB", "BBB": "zzz"}

	wantURL := "http://x/bbb?a=BBB" // TOKEN wins over TOK; AAA->BBB not rescanned to zzz
	wantData := "v=aaa"
	for i := 0; i < 50; i++ {
		req := &ffuf.Request{Url: "http://x/TOKEN?a=AAA", Headers: map[string]string{"X": "TOK"}, Data: []byte("v=TOK")}
		r.applyVars(req, vars)
		if req.Url != wantURL {
			t.Fatalf("run %d: url=%q want %q (nondeterministic or cascading)", i, req.Url, wantURL)
		}
		if string(req.Data) != wantData {
			t.Fatalf("run %d: data=%q want %q", i, string(req.Data), wantData)
		}
	}
}

// TestPreflightHonorsRateLimit confirms preflight requests consume the rate
// limiter (the main request is metered by the dispatch loop, not here).
func TestPreflightHonorsRateLimit(t *testing.T) {
	ps := newPreflightServer()
	defer ps.close()
	conf := ps.config(t, "per-request")
	var calls int64
	conf.RateLimitFunc = func() { atomic.AddInt64(&calls, 1) }
	r := newTestRunner(conf)

	const n = 3
	for i := 0; i < n; i++ {
		if _, err := r.Execute(mainReq(ps.srv.URL)); err != nil {
			t.Fatalf("Execute %d: %s", i, err)
		}
	}
	if got := atomic.LoadInt64(&calls); got != n {
		t.Errorf("rate limiter consulted %d times, want %d (one per preflight request)", got, n)
	}
}

// TestPostflightRunsOnIgnoredBody confirms postflight runs when the main request
// produced a response even though its body was not downloaded (-ignore-body),
// rather than being skipped on that early return.
func TestPostflightRunsOnIgnoredBody(t *testing.T) {
	var postHits int64
	mux := http.NewServeMux()
	mux.HandleFunc("/main", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "3")
		_, _ = w.Write([]byte("abc"))
	})
	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&postHits, 1)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	postFile := writeTempRequest(t, fmt.Sprintf("GET /post HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String()))
	conf := newTestConfig(srv.URL)
	conf.IgnoreBody = true
	conf.Postflights = []ffuf.PreflightConfig{{RequestFile: postFile}}
	r := newTestRunner(conf)

	if _, err := r.Execute(mainReq(srv.URL)); err != nil {
		t.Fatalf("Execute: %s", err)
	}
	if got := atomic.LoadInt64(&postHits); got != 1 {
		t.Errorf("postflight ran %d times, want 1 (must run for an ignored-body response)", got)
	}
}

// TestPreflightRejectsControlCharValue rejects a captured value containing a
// control character (defense-in-depth against a poisoned token).
func TestPreflightRejectsControlCharValue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "token=aaa\rbbb") // bare CR inside the captured value
	}))
	defer srv.Close()
	reqFile := writeTempRequest(t, fmt.Sprintf("GET / HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String()))

	confAbort := newTestConfig(srv.URL)
	confAbort.PreflightError = "abort"
	confAbort.Preflights = []ffuf.PreflightConfig{{RequestFile: reqFile, Vars: []ffuf.VarExtract{{Name: "T", Regex: `token=(.+)`}}}}
	if _, err := newTestRunner(confAbort).runPreflightChain(confAbort.Preflights, nil); err == nil {
		t.Error("abort mode: expected an error for a control-char value")
	}

	confIgnore := newTestConfig(srv.URL)
	confIgnore.PreflightError = "ignore"
	confIgnore.Preflights = []ffuf.PreflightConfig{{RequestFile: reqFile, Vars: []ffuf.VarExtract{{Name: "T", Regex: `token=(.+)`}}}}
	vars, err := newTestRunner(confIgnore).runPreflightChain(confIgnore.Preflights, nil)
	if err != nil {
		t.Fatalf("ignore mode should not error: %s", err)
	}
	if _, ok := vars["T"]; ok {
		t.Error("ignore mode: control-char value should be skipped, not stored")
	}
}

// TestParsePreflightKeepsLastHeader locks the dropped-last-header fix: a request
// file ending on a header with no trailing newline keeps that header.
func TestParsePreflightKeepsLastHeader(t *testing.T) {
	r := newTestRunner(newTestConfig("http://example.com/"))
	f := writeTempRequest(t, "GET / HTTP/1.1\r\nHost: example.com\r\nX-Last: kept")
	req, err := r.parsePreflightRequest(f, nil)
	if err != nil {
		t.Fatalf("parsePreflightRequest: %s", err)
	}
	if got := req.Header.Get("X-Last"); got != "kept" {
		t.Errorf("last header dropped: X-Last=%q, want %q", got, "kept")
	}
}
