// Package testtarget provides a deterministic mock HTTP target for ffuf
// integration tests. It models the response dimensions ffuf matches and filters
// on (status code, body size, word count, line count, latency, reflection,
// soft-404 baselines, redirects, request-gated endpoints, and a nested
// directory tree for recursion) and records every request it receives.
//
// The server is hermetic: it binds to an ephemeral localhost port via
// httptest, uses no randomness, and returns byte-identical responses for
// identical requests. That determinism is what lets integration tests assert on
// exact result sets. Point the real ffuf http runner at Target.URL and only the
// target and the output sink are test doubles; the engine, runner, matchers and
// filters are the real ones.
package testtarget

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Recorded is one request the target received, exposed via Target.Requests so a
// test can assert on what ffuf actually sent (method, headers, body, timing).
type Recorded struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   string
	At     time.Time
}

// Target wraps an httptest.Server with a thread-safe request recorder.
type Target struct {
	*httptest.Server
	mu       sync.Mutex
	recorded []Recorded
}

// New starts a new deterministic target server. The caller must Close it.
func New() *Target {
	t := &Target{}
	t.Server = httptest.NewServer(http.HandlerFunc(t.handle))
	return t
}

// Requests returns a snapshot copy of every request received so far.
func (t *Target) Requests() []Recorded {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Recorded, len(t.recorded))
	copy(out, t.recorded)
	return out
}

// Count returns how many requests the target has received.
func (t *Target) Count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.recorded)
}

func (t *Target) handle(w http.ResponseWriter, r *http.Request) {
	body := drain(r)
	t.mu.Lock()
	t.recorded = append(t.recorded, Recorded{
		Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery,
		Header: r.Header.Clone(), Body: body, At: time.Now(),
	})
	t.mu.Unlock()

	p := r.URL.Path
	switch {
	// --- match/filter dimension endpoints -------------------------------
	case strings.HasPrefix(p, "/status/"):
		if code, ok := tailInt(p, "/status/"); ok {
			w.WriteHeader(code)
			fmt.Fprintf(w, "status %d", code)
			return
		}
	case strings.HasPrefix(p, "/size/"):
		if n, ok := tailInt(p, "/size/"); ok {
			w.Write(filled(n))
			return
		}
	case strings.HasPrefix(p, "/words/"):
		if n, ok := tailInt(p, "/words/"); ok {
			w.Write([]byte(words(n)))
			return
		}
	case strings.HasPrefix(p, "/lines/"):
		if n, ok := tailInt(p, "/lines/"); ok {
			w.Write([]byte(lines(n)))
			return
		}
	case strings.HasPrefix(p, "/sleep/"):
		if ms, ok := tailInt(p, "/sleep/"); ok {
			time.Sleep(time.Duration(ms) * time.Millisecond)
			fmt.Fprintf(w, "slept %d", ms)
			return
		}
	case strings.HasPrefix(p, "/reflect/"):
		val := strings.TrimPrefix(p, "/reflect/")
		fmt.Fprintf(w, "reflected: %s", val)
		return
	case strings.HasPrefix(p, "/redirect/"):
		if n, ok := tailInt(p, "/redirect/"); ok {
			if n <= 0 {
				fmt.Fprint(w, "arrived")
			} else {
				w.Header().Set("Location", fmt.Sprintf("/redirect/%d", n-1))
				w.WriteHeader(http.StatusFound)
			}
			return
		}

	// --- autocalibration: soft-404 baseline with one real outlier -------
	case p == "/ac/real":
		// Distinctly larger/wordier than the soft-404 baseline so a calibrated
		// size or word filter lets it through while the junk is filtered.
		fmt.Fprint(w, "REAL content that is meaningfully larger than the junk baseline response")
		return
	case strings.HasPrefix(p, "/ac/"):
		// Any other /ac/* path (including ffuf's random calibration probes)
		// returns a constant small body: the classic soft-404.
		fmt.Fprint(w, "junk")
		return

	// --- request-gated endpoints ----------------------------------------
	case p == "/needs-header":
		if r.Header.Get("X-Test") == "yes" {
			fmt.Fprint(w, "header ok")
			return
		}
		forbidden(w)
		return
	case p == "/needs-cookie":
		if c, err := r.Cookie("SESSION"); err == nil && c.Value != "" {
			fmt.Fprint(w, "cookie ok")
			return
		}
		forbidden(w)
		return
	case p == "/needs-method":
		if r.Method == http.MethodPost {
			fmt.Fprint(w, "method ok")
			return
		}
		forbidden(w)
		return
	case p == "/needs-body":
		if strings.Contains(body, "token") {
			fmt.Fprint(w, "body ok")
			return
		}
		forbidden(w)
		return

	// --- recursion tree -------------------------------------------------
	case p == "/":
		fmt.Fprint(w, "root")
		return
	case p == "/admin":
		fmt.Fprint(w, "admin directory")
		return
	case p == "/admin/secret":
		fmt.Fprint(w, "the secret")
		return
	}

	notFound(w)
}

func forbidden(w http.ResponseWriter) {
	w.WriteHeader(http.StatusForbidden)
	fmt.Fprint(w, "forbidden")
}
func notFound(w http.ResponseWriter) { w.WriteHeader(http.StatusNotFound); fmt.Fprint(w, "not found") }

// tailInt parses the integer immediately following prefix in path. It requires
// the remainder to be exactly an integer (no further path segments), so
// /size/20 parses and /size/20/extra does not.
func tailInt(path, prefix string) (int, bool) {
	rest := strings.TrimPrefix(path, prefix)
	if rest == "" || strings.Contains(rest, "/") {
		return 0, false
	}
	n, err := strconv.Atoi(rest)
	if err != nil {
		return 0, false
	}
	return n, true
}

// filled returns n bytes of 'A'. net/http sets Content-Length to n for these
// small buffered bodies, which is what ffuf's size matcher reads.
func filled(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'A'
	}
	return b
}

// words returns a body of exactly n whitespace-separated tokens.
func words(n int) string {
	if n <= 0 {
		return ""
	}
	toks := make([]string, n)
	for i := range toks {
		toks[i] = "w"
	}
	return strings.Join(toks, " ")
}

// lines returns a body that ffuf counts as exactly n lines. ffuf's line count
// is len(strings.Split(data, "\n")), so n "L"s joined by newlines (no trailing
// newline) yields exactly n, matching the /size and /words convention.
func lines(n int) string {
	if n <= 0 {
		return ""
	}
	ls := make([]string, n)
	for i := range ls {
		ls[i] = "L"
	}
	return strings.Join(ls, "\n")
}

func drain(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	buf := make([]byte, 0, 512)
	tmp := make([]byte, 512)
	for {
		n, err := r.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return string(buf)
}
