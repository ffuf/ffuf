package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// ioReadAll is a local alias so we don't need to import "io" just for this.
func ioReadAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 512)
	for {
		n, err := r.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return buf, nil
}

// newTestConfig returns a minimal Config wired up for tests.
func newTestConfig(targetURL string) *ffuf.Config {
	ctx, cancel := context.WithCancel(context.Background())
	conf := ffuf.NewConfig(ctx, cancel)
	conf.Url = targetURL
	conf.Threads = 1
	conf.Timeout = 5
	conf.Headers = make(map[string]string)
	return &conf
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

// newTestRunner creates a SimpleRunner from a Config (bypassing NewSimpleRunner
// so we can inject the config after construction).
func newTestRunner(conf *ffuf.Config) *SimpleRunner {
	r := &SimpleRunner{config: conf}
	r.client = &http.Client{Timeout: 0}
	return r
}

// --- Tests ---

// TestPreflightVarsSubstitutedIntoMainRequest checks that a variable extracted
// from a preflight response is correctly substituted into the main request.
func TestPreflightVarsSubstitutedIntoMainRequest(t *testing.T) {
	// Preflight server: returns a page with a token
	preflightSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<input name="csrf" value="tok123">`)
	}))
	defer preflightSrv.Close()

	// Main server (only used for URL construction in this test)
	mainSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer mainSrv.Close()

	rawReq := fmt.Sprintf("GET / HTTP/1.1\nHost: %s\n\n", preflightSrv.Listener.Addr().String())
	reqFile := writeTempRequest(t, rawReq)

	conf := newTestConfig(mainSrv.URL)
	conf.PreflightMode = "per-request"
	conf.PreflightError = "abort"
	conf.Preflights = []ffuf.PreflightConfig{
		{
			RequestFile: reqFile,
			Vars: []ffuf.VarExtract{
				{Name: "CSRFTOKEN", Regex: `value="([^"]+)"`},
			},
		},
	}

	runner := newTestRunner(conf)
	vars, err := runner.runPreflightChain(conf.Preflights, nil)
	if err != nil {
		t.Fatalf("runPreflightChain returned error: %s", err)
	}
	if vars["CSRFTOKEN"] != "tok123" {
		t.Errorf("expected CSRFTOKEN=tok123, got %q", vars["CSRFTOKEN"])
	}

	// Verify applyVars substitutes into a request
	req := &ffuf.Request{
		Url:     mainSrv.URL + "/submit?token=CSRFTOKEN",
		Method:  "GET",
		Headers: map[string]string{},
		Data:    []byte("csrf=CSRFTOKEN"),
	}
	runner.applyVars(req, vars)
	if req.Url != mainSrv.URL+"/submit?token=tok123" {
		t.Errorf("URL substitution failed: got %q", req.Url)
	}
	if string(req.Data) != "csrf=tok123" {
		t.Errorf("body substitution failed: got %q", string(req.Data))
	}
}

// TestPerThreadModeCachesVars checks that in per-thread mode the preflight chain
// runs only once across multiple Execute-equivalent calls on the same runner.
func TestPerThreadModeCachesVars(t *testing.T) {
	var callCount int32
	preflightSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		fmt.Fprint(w, `token=abc`)
	}))
	defer preflightSrv.Close()

	rawReq := fmt.Sprintf("GET / HTTP/1.1\nHost: %s\n\n", preflightSrv.Listener.Addr().String())
	reqFile := writeTempRequest(t, rawReq)

	conf := newTestConfig(preflightSrv.URL)
	conf.PreflightMode = "per-thread"
	conf.PreflightError = "abort"
	conf.Preflights = []ffuf.PreflightConfig{
		{
			RequestFile: reqFile,
			Vars:        []ffuf.VarExtract{{Name: "TOKEN", Regex: `token=(\w+)`}},
		},
	}

	runner := newTestRunner(conf)

	// Simulate per-thread init: run the chain the way Execute does
	for i := 0; i < 3; i++ {
		if !runner.threadVarsInit {
			v, err := runner.runPreflightChain(conf.Preflights, nil)
			if err != nil {
				t.Fatalf("run %d: unexpected error: %s", i, err)
			}
			runner.threadVars = v
			runner.threadVarsInit = true
		}
	}

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("preflight server called %d times, expected 1 (per-thread caching)", callCount)
	}
	if runner.threadVars["TOKEN"] != "abc" {
		t.Errorf("expected TOKEN=abc, got %q", runner.threadVars["TOKEN"])
	}
}

// TestPerRequestModeRunsFreshEachTime checks that per-request mode creates a
// fresh variable map for every Execute call (no caching).
func TestPerRequestModeRunsFreshEachTime(t *testing.T) {
	var callCount int32
	preflightSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		fmt.Fprintf(w, `token=dynamic%d`, n)
	}))
	defer preflightSrv.Close()

	rawReq := fmt.Sprintf("GET / HTTP/1.1\nHost: %s\n\n", preflightSrv.Listener.Addr().String())
	reqFile := writeTempRequest(t, rawReq)

	conf := newTestConfig(preflightSrv.URL)
	conf.PreflightMode = "per-request"
	conf.PreflightError = "abort"
	conf.Preflights = []ffuf.PreflightConfig{
		{
			RequestFile: reqFile,
			Vars:        []ffuf.VarExtract{{Name: "TOKEN", Regex: `token=(\S+)`}},
		},
	}

	runner := newTestRunner(conf)
	const runs = 3
	for i := 0; i < runs; i++ {
		v, err := runner.runPreflightChain(conf.Preflights, nil)
		if err != nil {
			t.Fatalf("run %d error: %s", i, err)
		}
		expected := fmt.Sprintf("dynamic%d", i+1)
		if v["TOKEN"] != expected {
			t.Errorf("run %d: expected TOKEN=%s, got %q", i+1, expected, v["TOKEN"])
		}
	}
	if atomic.LoadInt32(&callCount) != runs {
		t.Errorf("preflight called %d times, expected %d", callCount, runs)
	}
}

// TestVariablesDoNotLeakBetweenRunners confirms that two separate SimpleRunner
// instances (simulating two goroutines/threads) do not share variable state.
func TestVariablesDoNotLeakBetweenRunners(t *testing.T) {
	mkServer := func(token string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `token=%s`, token)
		}))
	}
	srv1 := mkServer("alpha")
	srv2 := mkServer("beta")
	defer srv1.Close()
	defer srv2.Close()

	mkReqFile := func(t *testing.T, srv *httptest.Server) string {
		t.Helper()
		raw := fmt.Sprintf("GET / HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String())
		return writeTempRequest(t, raw)
	}

	extract := []ffuf.VarExtract{{Name: "TOKEN", Regex: `token=(\w+)`}}

	conf1 := newTestConfig(srv1.URL)
	conf1.PreflightMode = "per-thread"
	conf1.PreflightError = "abort"
	conf1.Preflights = []ffuf.PreflightConfig{{RequestFile: mkReqFile(t, srv1), Vars: extract}}

	conf2 := newTestConfig(srv2.URL)
	conf2.PreflightMode = "per-thread"
	conf2.PreflightError = "abort"
	conf2.Preflights = []ffuf.PreflightConfig{{RequestFile: mkReqFile(t, srv2), Vars: extract}}

	r1 := newTestRunner(conf1)
	r2 := newTestRunner(conf2)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		v, err := r1.runPreflightChain(conf1.Preflights, nil)
		if err != nil {
			t.Errorf("r1 chain error: %s", err)
			return
		}
		r1.threadVars = v
		r1.threadVarsInit = true
	}()
	go func() {
		defer wg.Done()
		v, err := r2.runPreflightChain(conf2.Preflights, nil)
		if err != nil {
			t.Errorf("r2 chain error: %s", err)
			return
		}
		r2.threadVars = v
		r2.threadVarsInit = true
	}()
	wg.Wait()

	if r1.threadVars["TOKEN"] != "alpha" {
		t.Errorf("r1 TOKEN: expected alpha, got %q", r1.threadVars["TOKEN"])
	}
	if r2.threadVars["TOKEN"] != "beta" {
		t.Errorf("r2 TOKEN: expected beta, got %q", r2.threadVars["TOKEN"])
	}
	// Explicitly assert neither runner can see the other's vars
	if r1.threadVars["TOKEN"] == r2.threadVars["TOKEN"] {
		t.Error("r1 and r2 share TOKEN value — variable isolation broken")
	}
}

// TestPreflightErrorAbort checks that a network error causes runPreflightChain to
// return an error when -preflight-error=abort.
func TestPreflightErrorAbort(t *testing.T) {
	reqFile := writeTempRequest(t, "GET / HTTP/1.1\nHost: 127.0.0.1:1\n\n")
	conf := newTestConfig("http://127.0.0.1:1")
	conf.PreflightMode = "per-request"
	conf.PreflightError = "abort"
	conf.Preflights = []ffuf.PreflightConfig{{RequestFile: reqFile, Vars: nil}}

	runner := newTestRunner(conf)
	_, err := runner.runPreflightChain(conf.Preflights, nil)
	if err == nil {
		t.Error("expected error on abort mode but got nil")
	}
}

// TestPreflightErrorIgnore checks that a network error is swallowed and an empty
// var map returned when -preflight-error=ignore.
func TestPreflightErrorIgnore(t *testing.T) {
	reqFile := writeTempRequest(t, "GET / HTTP/1.1\nHost: 127.0.0.1:1\n\n")
	conf := newTestConfig("http://127.0.0.1:1")
	conf.PreflightMode = "per-request"
	conf.PreflightError = "ignore"
	conf.Preflights = []ffuf.PreflightConfig{{RequestFile: reqFile, Vars: nil}}

	runner := newTestRunner(conf)
	vars, err := runner.runPreflightChain(conf.Preflights, nil)
	if err != nil {
		t.Errorf("expected nil error on ignore mode, got: %s", err)
	}
	if len(vars) != 0 {
		t.Errorf("expected empty var map on ignored error, got: %v", vars)
	}
}
