// Package integration runs the real ffuf engine end-to-end against the
// deterministic mock target in pkg/testtarget. It assembles a Job exactly as
// main.go's prepareJob does (real input provider, real http runner, real
// matchers and filters) and swaps in a capturing OutputProvider so a test can
// assert on the exact set of inputs that matched. Nothing here parses argv, so
// the global flag.CommandLine state is never touched; ConfigOptions are built
// directly. CLI/flag wiring is covered separately by the black-box e2e layer.
package integration

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/assembly"
	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/ffuf/ffuf/v2/pkg/filter"
)

// TestMain points ffuf.SCRAPERDIR at an empty temp dir for the whole package.
// BuildJob loads scraper rules from that global dir; a fresh environment (a CI
// runner) has no populated XDG config dir, and unlike main the in-process harness
// never runs ReadDefaultConfig to create it. Redirecting to an empty temp dir
// keeps these tests hermetic, mirroring how the ffuf package redirects AUTOCALIBDIR.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "ffuf-scraperdir-*")
	if err != nil {
		panic(err)
	}
	ffuf.SCRAPERDIR = tmp
	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

// runScan runs a full ffuf job against url with the given wordlist. configure
// tweaks the ConfigOptions (recursion, autocalibration, request options, ...)
// and setup installs the matchers/filters under test on the MatcherManager. It
// returns the sorted, de-duplicated set of FUZZ inputs that matched.
//
// Result order is nondeterministic (the engine is concurrent), so callers must
// compare against an expected SET, never an ordered slice. runScan sorts for
// exactly that reason.
func runScan(t *testing.T, url string, wordlist []string, configure func(*ffuf.ConfigOptions), setup func(ffuf.MatcherManager)) []string {
	t.Helper()

	opts := ffuf.NewConfigOptions()
	opts.HTTP.URL = url
	opts.HTTP.Method = "GET"
	opts.Input.Wordlists = []string{writeWordlist(t, wordlist)}
	opts.General.Quiet = true
	if configure != nil {
		configure(opts)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	conf, err := ffuf.ConfigFromOptions(opts, ctx, cancel)
	if err != nil {
		t.Fatalf("ConfigFromOptions: %v", err)
	}

	// Install matchers/filters directly rather than via main.SetupFilters,
	// which reads the global flag set (flag.Visit) to decide defaults. The test
	// states its matchers explicitly; SetupFilters' CLI-defaulting logic is the
	// e2e layer's concern.
	conf.MatcherManager = filter.NewMatcherManager()
	if setup != nil {
		setup(conf.MatcherManager)
	}

	// Wire the Job through the SAME assembly path main uses, then swap in the
	// capturing output provider. This is what stops the harness from drifting from
	// production wiring.
	job, err := assembly.BuildJob(conf)
	if err != nil {
		t.Fatalf("BuildJob: %v", err)
	}
	out := &capture{}
	job.Output = out

	job.Start()
	return out.inputs()
}

// writeWordlist writes words to a temp file (cleaned up with the test) and
// returns its path.
func writeWordlist(t *testing.T, words []string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "wordlist-*.txt")
	if err != nil {
		t.Fatalf("temp wordlist: %v", err)
	}
	defer f.Close()
	for _, w := range words {
		fmt.Fprintln(f, w)
	}
	return f.Name()
}

// capture is a test OutputProvider that records matched responses. Result is
// called from worker goroutines, so access is mutex-guarded.
type capture struct {
	mu        sync.Mutex
	responses []ffuf.Response
	results   []ffuf.Result
}

func (c *capture) Result(resp ffuf.Response) {
	c.mu.Lock()
	c.responses = append(c.responses, resp)
	c.mu.Unlock()
}

// inputs returns the sorted, de-duplicated FUZZ payloads that matched.
func (c *capture) inputs() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	seen := make(map[string]bool)
	out := make([]string, 0, len(c.responses))
	for _, r := range c.responses {
		v := string(r.Request.Input["FUZZ"])
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}

// --- OutputProvider no-ops (the engine drives these; tests ignore them) ---

func (c *capture) Banner()                       {}
func (c *capture) Finalize() error               { return nil }
func (c *capture) Progress(ffuf.Progress)        {}
func (c *capture) Info(string)                   {}
func (c *capture) Error(string)                  {}
func (c *capture) Raw(string)                    {}
func (c *capture) Warning(string)                {}
func (c *capture) PrintResult(ffuf.Result)       {}
func (c *capture) SaveFile(string, string) error { return nil }
func (c *capture) GetCurrentResults() []ffuf.Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.results
}
func (c *capture) SetCurrentResults(r []ffuf.Result) { c.mu.Lock(); c.results = r; c.mu.Unlock() }
func (c *capture) FilterCurrentResults(keep func(ffuf.Result) bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	filtered := make([]ffuf.Result, 0, len(c.results))
	for _, r := range c.results {
		if keep(r) {
			filtered = append(filtered, r)
		}
	}
	c.results = filtered
}
func (c *capture) SetPaused(bool)      {}
func (c *capture) PendingResults() int { return 0 }
func (c *capture) Reset()              {}
func (c *capture) Cycle()              {}
