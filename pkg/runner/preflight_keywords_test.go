package runner

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// recordingServer records the path of each request to /pre/... (the preflight
// target) so a test can assert what keyword substitution produced.
func recordingServer(t *testing.T) (*httptest.Server, func() string) {
	t.Helper()
	var mu sync.Mutex
	var last string
	mux := http.NewServeMux()
	mux.HandleFunc("/pre/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		last = r.URL.Path
		mu.Unlock()
		fmt.Fprint(w, "ok")
	})
	mux.HandleFunc("/main", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	srv := httptest.NewServer(mux)
	return srv, func() string { mu.Lock(); defer mu.Unlock(); return last }
}

// TestKeywordSubstitutedIntoPreflight is the feature: a fuzzing input keyword in a
// preflight request is substituted with the current payload, just like the main
// request.
func TestKeywordSubstitutedIntoPreflight(t *testing.T) {
	srv, prePath := recordingServer(t)
	defer srv.Close()

	reqFile := writeTempRequest(t, fmt.Sprintf("GET /pre/FUZZ HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String()))
	conf := newTestConfig(srv.URL)
	conf.Preflights = []ffuf.PreflightConfig{{RequestFile: reqFile}}
	r := newTestRunner(conf)

	req := &ffuf.Request{
		Method:  "GET",
		Url:     srv.URL + "/main",
		Headers: map[string]string{},
		Input:   map[string][]byte{"FUZZ": []byte("admin")},
	}
	if _, err := r.Execute(req); err != nil {
		t.Fatalf("Execute: %s", err)
	}
	if got := prePath(); got != "/pre/admin" {
		t.Errorf("preflight path = %q, want /pre/admin (FUZZ not substituted into the preflight)", got)
	}
}

// TestFFUFHASHSubstitutedIntoPreflight confirms it isn't special-cased to FUZZ:
// any input keyword, including FFUFHASH, is substituted.
func TestFFUFHASHSubstitutedIntoPreflight(t *testing.T) {
	srv, prePath := recordingServer(t)
	defer srv.Close()

	reqFile := writeTempRequest(t, fmt.Sprintf("GET /pre/FFUFHASH HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String()))
	conf := newTestConfig(srv.URL)
	conf.Preflights = []ffuf.PreflightConfig{{RequestFile: reqFile}}
	r := newTestRunner(conf)

	req := &ffuf.Request{
		Method:  "GET",
		Url:     srv.URL + "/main",
		Headers: map[string]string{},
		Input:   map[string][]byte{"FFUFHASH": []byte("deadbeef")},
	}
	if _, err := r.Execute(req); err != nil {
		t.Fatalf("Execute: %s", err)
	}
	if got := prePath(); got != "/pre/deadbeef" {
		t.Errorf("preflight path = %q, want /pre/deadbeef", got)
	}
}

// TestPreflightVhostHostResolved confirms the vhost case now works: a relative-path
// preflight whose host is derived from a keyworded target (-u FUZZ.example.com)
// resolves the keyword instead of erroring.
func TestPreflightVhostHostResolved(t *testing.T) {
	r := newTestRunner(newTestConfig("http://FUZZ.example.com/"))
	f := writeTempRequest(t, "GET /login HTTP/1.1\n\n") // relative path, no Host header
	req, err := r.parsePreflightRequest(f, map[string][]byte{"FUZZ": []byte("admin")}, nil)
	if err != nil {
		t.Fatalf("parsePreflightRequest: %s", err)
	}
	if req.Host != "admin.example.com" {
		t.Errorf("derived host = %q, want admin.example.com (keyword not resolved)", req.Host)
	}
}

// TestPerThreadPreflightAmortizesKeyword documents the per-thread semantics: the
// preflight runs once per lane, so its keyword reflects the payload of the request
// that initialized the lane and is reused for the rest.
func TestPerThreadPreflightAmortizesKeyword(t *testing.T) {
	srv, prePath := recordingServer(t)
	defer srv.Close()

	reqFile := writeTempRequest(t, fmt.Sprintf("GET /pre/FUZZ HTTP/1.1\nHost: %s\n\n", srv.Listener.Addr().String()))
	conf := newTestConfig(srv.URL)
	conf.PreflightMode = "per-thread"
	conf.Preflights = []ffuf.PreflightConfig{{RequestFile: reqFile}}
	r := newTestRunner(conf)

	for _, payload := range []string{"first", "second"} {
		req := &ffuf.Request{
			Method:  "GET",
			Url:     srv.URL + "/main",
			Headers: map[string]string{},
			Input:   map[string][]byte{"FUZZ": []byte(payload)},
		}
		if _, err := r.Execute(req); err != nil {
			t.Fatalf("Execute: %s", err)
		}
	}
	// The lane initialized on the first request, so the preflight ran once with
	// "first" and was not re-run for "second".
	if got := prePath(); got != "/pre/first" {
		t.Errorf("preflight path = %q, want /pre/first (per-thread amortizes to the first payload)", got)
	}
}
