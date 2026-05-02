package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// newRedirectChainServer returns an httptest.Server that responds to a fixed
// chain of paths. Each entry in chain is a {path, status, location} triple.
// The last entry may set status to 200 to terminate the chain.
//
//	chain example: [{"/a", 301, "/b"}, {"/b", 302, "/c"}, {"/c", 200, ""}]
func newRedirectChainServer(t *testing.T, chain []struct {
	path, location string
	status         int
}) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for _, hop := range chain {
		hop := hop // capture for closure
		mux.HandleFunc(hop.path, func(w http.ResponseWriter, r *http.Request) {
			if hop.location != "" {
				w.Header().Set("Location", hop.location)
			}
			w.WriteHeader(hop.status)
		})
	}
	return httptest.NewServer(mux)
}

func newRunnerForTest(t *testing.T, redirectChain bool) *SimpleRunner {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	conf := ffuf.NewConfig(ctx, cancel)
	conf.RedirectChain = redirectChain
	if redirectChain {
		// --redirect-chain implies follow-redirects
		conf.FollowRedirects = true
	}
	conf.Timeout = 5
	r, ok := NewSimpleRunner(&conf, false).(*SimpleRunner)
	if !ok {
		t.Fatalf("NewSimpleRunner returned non-*SimpleRunner")
	}
	return r
}

func TestRedirectChain_CapturedAcrossThreeHops(t *testing.T) {
	srv := newRedirectChainServer(t, []struct {
		path, location string
		status         int
	}{
		{"/a", "", 301},
		{"/b", "", 302},
		{"/c", "", 200},
	})
	defer srv.Close()
	// Wire up Location headers using the test server's actual base URL.
	mux := http.NewServeMux()
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", srv.URL+"/b")
		w.WriteHeader(301)
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", srv.URL+"/c")
		w.WriteHeader(302)
	})
	mux.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	srv.Config.Handler = mux

	r := newRunnerForTest(t, true /* redirectChain */)
	req := &ffuf.Request{
		Method:  "GET",
		Url:     srv.URL + "/a",
		Headers: map[string]string{},
	}
	resp, err := r.Execute(req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("final status = %d, want 200", resp.StatusCode)
	}
	if got, want := len(resp.Redirects), 2; got != want {
		t.Fatalf("captured %d hops, want %d (chain=%+v)", got, want, resp.Redirects)
	}
	// hop 0: /a returned 301 with Location /b
	if resp.Redirects[0].StatusCode != 301 {
		t.Errorf("hop 0 status = %d, want 301", resp.Redirects[0].StatusCode)
	}
	if !strings.HasSuffix(resp.Redirects[0].URL, "/a") {
		t.Errorf("hop 0 URL = %q, want suffix /a", resp.Redirects[0].URL)
	}
	if !strings.HasSuffix(resp.Redirects[0].Location, "/b") {
		t.Errorf("hop 0 Location = %q, want suffix /b", resp.Redirects[0].Location)
	}
	// hop 1: /b returned 302 with Location /c
	if resp.Redirects[1].StatusCode != 302 {
		t.Errorf("hop 1 status = %d, want 302", resp.Redirects[1].StatusCode)
	}
	if !strings.HasSuffix(resp.Redirects[1].URL, "/b") {
		t.Errorf("hop 1 URL = %q, want suffix /b", resp.Redirects[1].URL)
	}
	if !strings.HasSuffix(resp.Redirects[1].Location, "/c") {
		t.Errorf("hop 1 Location = %q, want suffix /c", resp.Redirects[1].Location)
	}
}

func TestRedirectChain_DisabledByDefault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			w.Header().Set("Location", "/b")
			w.WriteHeader(301)
		case "/b":
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	// Default ffuf behavior: no follow, no chain.
	r := newRunnerForTest(t, false /* redirectChain */)
	req := &ffuf.Request{
		Method:  "GET",
		Url:     srv.URL + "/a",
		Headers: map[string]string{},
	}
	resp, err := r.Execute(req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.StatusCode != 301 {
		t.Errorf("status = %d, want 301 (default ffuf does not follow)", resp.StatusCode)
	}
	if len(resp.Redirects) != 0 {
		t.Errorf("Redirects should be empty when --redirect-chain is off, got %+v", resp.Redirects)
	}
}

func TestRedirectChain_FollowRedirectsWithoutChainStillFollows(t *testing.T) {
	// Pre-existing behavior: with -r but no --redirect-chain we follow but
	// don't record. Make sure adding the new flag didn't accidentally regress
	// that path.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/a":
			w.Header().Set("Location", "/b")
			w.WriteHeader(301)
		case "/b":
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	conf := ffuf.NewConfig(ctx, cancel)
	conf.FollowRedirects = true
	conf.RedirectChain = false
	conf.Timeout = 5
	r := NewSimpleRunner(&conf, false).(*SimpleRunner)
	req := &ffuf.Request{Method: "GET", Url: srv.URL + "/a", Headers: map[string]string{}}
	resp, err := r.Execute(req)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("with -r alone, status should be 200 after following, got %d", resp.StatusCode)
	}
	if len(resp.Redirects) != 0 {
		t.Errorf("Redirects must stay empty when --redirect-chain is off (got %+v)", resp.Redirects)
	}
}

func TestRedirectChain_StopsAfterTenHops(t *testing.T) {
	// Build a 30-hop loop; CheckRedirect should bail at 10 to avoid runaway.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /N -> /N+1
		var n int
		_, err := fmt.Sscanf(r.URL.Path, "/%d", &n)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Location", fmt.Sprintf("/%d", n+1))
		w.WriteHeader(301)
	}))
	defer srv.Close()

	r := newRunnerForTest(t, true)
	req := &ffuf.Request{Method: "GET", Url: srv.URL + "/0", Headers: map[string]string{}}
	_, err := r.Execute(req)
	if err == nil {
		t.Fatalf("expected an error after exceeding the 10-hop cap, got nil")
	}
	uerr, ok := err.(*url.Error)
	if !ok {
		t.Fatalf("expected *url.Error, got %T (%v)", err, err)
	}
	if !strings.Contains(uerr.Err.Error(), "10 redirects") {
		t.Errorf("expected error mentioning 10-redirect cap, got: %v", err)
	}
}
