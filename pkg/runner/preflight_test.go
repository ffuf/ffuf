package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// newTestConfig returns a minimal Config wired up for tests.
func newTestConfig(targetURL string) *ffuf.Config {
	ctx, cancel := context.WithCancel(context.Background())
	conf := ffuf.NewConfig(ctx, cancel)
	conf.Url = targetURL
	conf.Threads = 1
	conf.Timeout = 5
	conf.Headers = make(map[string]string)
	conf.PreflightMode = "per-request"
	conf.PreflightError = "abort"
	return &conf
}

// newTestRunner builds a SimpleRunner directly (bypassing NewSimpleRunner) so the
// config can be injected after construction. It initializes the lane pool that
// per-thread preflight mode borrows from.
func newTestRunner(conf *ffuf.Config) *SimpleRunner {
	return &SimpleRunner{config: conf, client: &http.Client{Timeout: 0}, lanes: &lanePool{}}
}

// writeTempRequest writes a Burp-style raw HTTP request file and returns its path.
func writeTempRequest(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "preflight-*.txt")
	if err != nil {
		t.Fatalf("could not create temp request file: %s", err)
	}
	_, _ = f.WriteString(content)
	f.Close()
	return f.Name()
}

// TestPreflightVarsSubstitutedIntoMainRequest checks the core mechanic: a value
// extracted from a preflight response is substituted into the main request's URL,
// headers and body wherever its keyword appears.
func TestPreflightVarsSubstitutedIntoMainRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<input name="csrf" value="tok123">`)
	}))
	defer srv.Close()

	reqFile := writeTempRequest(t, fmt.Sprintf("GET / HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String()))
	conf := newTestConfig(srv.URL)
	conf.Preflights = []ffuf.PreflightConfig{{
		RequestFile: reqFile,
		Vars:        []ffuf.VarExtract{{Name: "CSRFTOKEN", Regex: `value="([^"]+)"`}},
	}}

	r := newTestRunner(conf)
	vars, err := r.runPreflightChain(conf.Preflights, nil)
	if err != nil {
		t.Fatalf("runPreflightChain: %s", err)
	}
	if vars["CSRFTOKEN"] != "tok123" {
		t.Fatalf("CSRFTOKEN = %q, want tok123", vars["CSRFTOKEN"])
	}

	req := &ffuf.Request{
		Url:     "http://x/submit?token=CSRFTOKEN",
		Headers: map[string]string{"X-CSRF": "CSRFTOKEN"},
		Data:    []byte("csrf=CSRFTOKEN"),
	}
	r.applyVars(req, vars)
	if req.Url != "http://x/submit?token=tok123" {
		t.Errorf("URL substitution: got %q", req.Url)
	}
	if req.Headers["X-CSRF"] != "tok123" {
		t.Errorf("header substitution: got %q", req.Headers["X-CSRF"])
	}
	if string(req.Data) != "csrf=tok123" {
		t.Errorf("body substitution: got %q", string(req.Data))
	}
}

// preflightServer serves a unique token per preflight hit at /preflight (so
// amortization and freshness are observable) and records the X-Token header of
// every main request at /main. It counts preflight hits.
type preflightServer struct {
	srv           *httptest.Server
	preflightHits int64
	mu            sync.Mutex
	mainTokens    []string
}

func newPreflightServer() *preflightServer {
	ps := &preflightServer{}
	mux := http.NewServeMux()
	mux.HandleFunc("/preflight", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&ps.preflightHits, 1)
		fmt.Fprintf(w, "token=TOK%d;", n)
	})
	mux.HandleFunc("/main", func(w http.ResponseWriter, r *http.Request) {
		ps.mu.Lock()
		ps.mainTokens = append(ps.mainTokens, r.Header.Get("X-Token"))
		ps.mu.Unlock()
		w.WriteHeader(200)
	})
	ps.srv = httptest.NewServer(mux)
	return ps
}

func (ps *preflightServer) close() { ps.srv.Close() }

func (ps *preflightServer) config(t *testing.T, mode string) *ffuf.Config {
	reqFile := writeTempRequest(t, fmt.Sprintf("GET /preflight HTTP/1.1\nHost: %s\n\n", ps.srv.Listener.Addr().String()))
	conf := newTestConfig(ps.srv.URL)
	conf.PreflightMode = mode
	conf.Preflights = []ffuf.PreflightConfig{{
		RequestFile: reqFile,
		Vars:        []ffuf.VarExtract{{Name: "TOKENKW", Regex: `token=(\w+)`}},
	}}
	return conf
}

// mainReq builds a fresh main request carrying the TOKENKW keyword in a header.
func mainReq(baseURL string) *ffuf.Request {
	return &ffuf.Request{
		Method:  "GET",
		Url:     baseURL + "/main",
		Headers: map[string]string{"X-Token": "TOKENKW"},
	}
}

// TestPerRequestRunsPreflightEveryTime confirms per-request mode runs the
// preflight chain once per Execute, so each main request gets a fresh token.
func TestPerRequestRunsPreflightEveryTime(t *testing.T) {
	ps := newPreflightServer()
	defer ps.close()
	r := newTestRunner(ps.config(t, "per-request"))

	const n = 5
	for i := 0; i < n; i++ {
		if _, err := r.Execute(mainReq(ps.srv.URL)); err != nil {
			t.Fatalf("Execute %d: %s", i, err)
		}
	}
	if got := atomic.LoadInt64(&ps.preflightHits); got != n {
		t.Errorf("preflight hits = %d, want %d (per-request should run every time)", got, n)
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	seen := map[string]bool{}
	for i, tok := range ps.mainTokens {
		if !strings.HasPrefix(tok, "TOK") {
			t.Errorf("main request %d got token %q, want a fresh TOK*", i, tok)
		}
		if seen[tok] {
			t.Errorf("token %q reused; per-request tokens must be fresh", tok)
		}
		seen[tok] = true
	}
}

// TestPerThreadAmortizesPreflight confirms per-thread mode runs the preflight
// chain once per lane and reuses the token across subsequent requests on that
// lane, rather than re-running it every request.
func TestPerThreadAmortizesPreflight(t *testing.T) {
	ps := newPreflightServer()
	defer ps.close()
	r := newTestRunner(ps.config(t, "per-thread"))

	const n = 10
	for i := 0; i < n; i++ {
		if _, err := r.Execute(mainReq(ps.srv.URL)); err != nil {
			t.Fatalf("Execute %d: %s", i, err)
		}
	}
	// Sequential calls reuse the single lane, so the preflight runs exactly once.
	if got := atomic.LoadInt64(&ps.preflightHits); got != 1 {
		t.Errorf("preflight hits = %d, want 1 (per-thread should amortize to one per lane)", got)
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for i, tok := range ps.mainTokens {
		if tok != "TOK1" {
			t.Errorf("main request %d got token %q, want the amortized TOK1", i, tok)
		}
	}
}

// TestConcurrentPerThreadRaceSafe is the reconciliation guard: many goroutines
// share one runner (as the engine does) in per-thread mode. It must be race-free
// (run with -race), amortize the preflight to roughly the worker count rather than
// the request count, and give every main request a valid token.
func TestConcurrentPerThreadRaceSafe(t *testing.T) {
	ps := newPreflightServer()
	defer ps.close()
	r := newTestRunner(ps.config(t, "per-thread"))

	const workers = 4
	const perWorker = 25
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				if _, err := r.Execute(mainReq(ps.srv.URL)); err != nil {
					t.Errorf("Execute: %s", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	total := workers * perWorker
	hits := atomic.LoadInt64(&ps.preflightHits)
	if hits < 1 || hits > int64(2*workers) {
		t.Errorf("preflight hits = %d, want between 1 and %d (amortized per lane, not %d per request)", hits, 2*workers, total)
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if len(ps.mainTokens) != total {
		t.Fatalf("recorded %d main requests, want %d", len(ps.mainTokens), total)
	}
	valid := regexp.MustCompile(`^TOK\d+$`)
	for i, tok := range ps.mainTokens {
		if !valid.MatchString(tok) {
			t.Errorf("main request %d got invalid token %q", i, tok)
		}
	}
}

// TestPreflightErrorAbort makes a non-matching extraction fatal in abort mode.
func TestPreflightErrorAbort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "no token here")
	}))
	defer srv.Close()
	reqFile := writeTempRequest(t, fmt.Sprintf("GET / HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String()))
	conf := newTestConfig(srv.URL)
	conf.PreflightError = "abort"
	conf.Preflights = []ffuf.PreflightConfig{{
		RequestFile: reqFile,
		Vars:        []ffuf.VarExtract{{Name: "X", Regex: `token=(\w+)`}},
	}}
	r := newTestRunner(conf)
	if _, err := r.runPreflightChain(conf.Preflights, nil); err == nil {
		t.Error("expected an error when the extraction regex does not match in abort mode")
	}
}

// TestPreflightErrorIgnore makes the same failure non-fatal in ignore mode.
func TestPreflightErrorIgnore(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "no token here")
	}))
	defer srv.Close()
	reqFile := writeTempRequest(t, fmt.Sprintf("GET / HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String()))
	conf := newTestConfig(srv.URL)
	conf.PreflightError = "ignore"
	conf.Preflights = []ffuf.PreflightConfig{{
		RequestFile: reqFile,
		Vars:        []ffuf.VarExtract{{Name: "X", Regex: `token=(\w+)`}},
	}}
	r := newTestRunner(conf)
	vars, err := r.runPreflightChain(conf.Preflights, nil)
	if err != nil {
		t.Fatalf("ignore mode should not error, got: %s", err)
	}
	if _, ok := vars["X"]; ok {
		t.Error("no variable should have been extracted from a non-matching response")
	}
}
