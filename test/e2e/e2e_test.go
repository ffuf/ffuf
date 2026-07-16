// Package e2e is the black-box layer: it builds the real ffuf binary and runs it
// as a subprocess against the in-process mock target (reachable over real TCP),
// then asserts on its output. This is the only layer that exercises argv -> flag
// parsing -> engine -> formatted output end to end, which is where CLI/flag wiring
// regressions surface.
package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/testtarget"
)

// ffufBin is the binary built once for the whole package by TestMain.
var ffufBin string

func TestMain(m *testing.M) { os.Exit(runMain(m)) }

func runMain(m *testing.M) int {
	dir, err := os.MkdirTemp("", "ffuf-e2e")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)

	ffufBin = filepath.Join(dir, "ffuf")
	if runtime.GOOS == "windows" {
		ffufBin += ".exe"
	}
	build := exec.Command("go", "build", "-o", ffufBin, ".")
	build.Dir = "../.." // go test runs with cwd = this package's dir
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build ffuf failed: %v\n%s", err, out)
		return 1
	}
	return m.Run()
}

// runFfuf runs the built binary with a hard timeout so a hung run fails fast
// rather than burning the whole test timeout. env may be nil (inherit).
func runFfuf(t *testing.T, env []string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffufBin, args...)
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.Output()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("ffuf timed out; args: %v", args)
	}
	if err != nil {
		t.Fatalf("ffuf failed: %v; args: %v", err, args)
	}
	return out
}

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

func assertSet(t *testing.T, got, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// jsonResult is the subset of a -json result line the e2e tests read. Input
// unmarshals from base64 straight into []byte.
type jsonResult struct {
	Input map[string][]byte `json:"input"`
	Url   string            `json:"url"`
}

func parseResults(stdout []byte) []jsonResult {
	var out []jsonResult
	for _, line := range strings.Split(string(stdout), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var r jsonResult
		if json.Unmarshal([]byte(line), &r) == nil && r.Url != "" {
			out = append(out, r)
		}
	}
	return out
}

// matchedPayloads returns the sorted set of last-path-segments of matched URLs.
func matchedPayloads(stdout []byte) []string {
	seen := make(map[string]bool)
	var out []string
	for _, r := range parseResults(stdout) {
		p := path.Base(r.Url)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// TestE2E_JSONOutput: the CLI parsed the flags, ran the scan, and emitted the
// matched set as JSON.
func TestE2E_JSONOutput(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	wl := writeWordlist(t, []string{"200", "201", "404"})
	out := runFfuf(t, nil,
		"-u", target.URL+"/status/FUZZ", "-w", wl,
		"-mc", "200,201", "-json", "-t", "1", "-noninteractive",
	)
	assertSet(t, matchedPayloads(out), []string{"200", "201"})
}

// TestE2E_DefaultMatcher: with NO -mc, the default status set
// (200-299,301,302,307,401,403,405,500) applies. This is the only test that
// exercises main.SetupFilters' defaulting; 404 must be filtered out.
func TestE2E_DefaultMatcher(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	wl := writeWordlist(t, []string{"200", "403", "404", "500"})
	out := runFfuf(t, nil,
		"-u", target.URL+"/status/FUZZ", "-w", wl,
		"-json", "-t", "1", "-noninteractive",
	)
	assertSet(t, matchedPayloads(out), []string{"200", "403", "500"})
}

// TestE2E_MatcherSuppressesDefault: setting -mr (any non-status matcher) with no
// -mc suppresses the default status matcher (SetupFilters: statusSet || !matcherSet).
// If the default were NOT suppressed, every 200 /reflect response would match and
// "drop" would leak in.
func TestE2E_MatcherSuppressesDefault(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	wl := writeWordlist(t, []string{"keep", "drop"})
	out := runFfuf(t, nil,
		"-u", target.URL+"/reflect/FUZZ", "-w", wl,
		"-mr", "keep", "-json", "-t", "1", "-noninteractive",
	)
	assertSet(t, matchedPayloads(out), []string{"keep"})
}

// hashFor returns the FFUFHASH of the result whose URL ends in the given payload.
func hashFor(t *testing.T, stdout []byte, payload string) string {
	t.Helper()
	for _, r := range parseResults(stdout) {
		if path.Base(r.Url) == payload {
			if h := r.Input["FFUFHASH"]; len(h) > 0 {
				return string(h)
			}
		}
	}
	t.Fatalf("no result for payload %q with a FFUFHASH in:\n%s", payload, stdout)
	return ""
}

// TestE2E_FFUFHashRoundTrip: take a FFUFHASH from a scan and feed it to -search,
// asserting the original request is reconstructed. Exercises the retained-options
// history serialization end to end. Config dir isolated via XDG_CONFIG_HOME.
func TestE2E_FFUFHashRoundTrip(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	cfgHome := t.TempDir()
	env := append(os.Environ(), "XDG_CONFIG_HOME="+cfgHome)

	wl := writeWordlist(t, []string{"200", "201"})
	out1 := runFfuf(t, env,
		"-u", target.URL+"/status/FUZZ", "-w", wl,
		"-mc", "200,201", "-json", "-t", "1", "-noninteractive",
	)
	hash := hashFor(t, out1, "200")

	// Hermeticity: on Linux XDG_CONFIG_HOME is honored, so the history must have
	// been written under the temp dir. (Skipped on macOS where adrg/xdg ignores it.)
	if runtime.GOOS == "linux" {
		if _, err := os.Stat(filepath.Join(cfgHome, "ffuf", "history")); err != nil {
			t.Errorf("history was not written under the isolated config dir: %v", err)
		}
	}

	out2 := runFfuf(t, env, "-search", hash, "-noninteractive")
	s := string(out2)
	if !strings.Contains(s, "Request candidate") {
		t.Fatalf("-search found no candidate for %q:\n%s", hash, s)
	}
	if !strings.Contains(s, "/status/200") {
		t.Errorf("-search did not reconstruct /status/200:\n%s", s)
	}
}

// TestE2E_FFUFHashRecursion: a recursed result's history must carry the RECURSED
// URL, not the base (historyOptions refreshes HTTP.URL from the live Config). The
// "secret" match comes from the /admin/ recursion job, so -search must reconstruct
// /admin/secret.
func TestE2E_FFUFHashRecursion(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	cfgHome := t.TempDir()
	env := append(os.Environ(), "XDG_CONFIG_HOME="+cfgHome)

	wl := writeWordlist(t, []string{"admin", "secret"})
	out1 := runFfuf(t, env,
		"-u", target.URL+"/FUZZ", "-w", wl,
		"-mc", "200", "-recursion", "-recursion-strategy", "greedy", "-recursion-depth", "1",
		"-json", "-t", "1", "-noninteractive",
	)
	// "secret" only matches at /admin/secret, i.e. inside the recursion job.
	if !strings.Contains(strings.Join(matchedPayloads(out1), ","), "secret") {
		t.Fatalf("recursion did not find 'secret':\n%s", out1)
	}
	hash := hashFor(t, out1, "secret")

	out2 := runFfuf(t, env, "-search", hash, "-noninteractive")
	if !strings.Contains(string(out2), "/admin/secret") {
		t.Errorf("recursed hash did not reconstruct /admin/secret (history kept the base URL?):\n%s", out2)
	}
}

// TestE2E_FFUFHashRequestOptions: headers, method and body must survive the
// serialize -> history file -> deserialize -> reconstruct round-trip. -search
// dumps the raw request, so they appear verbatim.
func TestE2E_FFUFHashRequestOptions(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	cfgHome := t.TempDir()
	env := append(os.Environ(), "XDG_CONFIG_HOME="+cfgHome)

	wl := writeWordlist(t, []string{"200"})
	out1 := runFfuf(t, env,
		"-u", target.URL+"/status/FUZZ", "-w", wl,
		"-X", "POST", "-H", "X-Test: roundtrip", "-d", "body=FUZZ",
		"-mc", "200", "-json", "-t", "1", "-noninteractive",
	)
	hash := hashFor(t, out1, "200")

	out2 := runFfuf(t, env, "-search", hash, "-noninteractive")
	s := string(out2)
	for _, want := range []string{"POST", "X-Test: roundtrip", "body=200"} {
		if !strings.Contains(s, want) {
			t.Errorf("-search reconstruction missing %q:\n%s", want, s)
		}
	}
}
