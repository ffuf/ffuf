// Package e2e is the black-box layer: it builds the real ffuf binary and runs it
// as a subprocess against the in-process mock target (reachable over real TCP),
// then asserts on its output. This is the only layer that exercises argv -> flag
// parsing -> engine -> formatted output end to end, which is where CLI/flag wiring
// regressions surface.
package e2e

import (
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

	"github.com/ffuf/ffuf/v2/pkg/testtarget"
)

// buildFfuf compiles the ffuf binary from the repo root into a temp path.
func buildFfuf(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "ffuf")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "../.." // go test runs with cwd = this package's dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ffuf: %v\n%s", err, out)
	}
	return bin
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
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// matchedPayloads parses -json stdout (one marshaled Result per line) and returns
// the sorted set of last-path-segments of the matched URLs (the FUZZ payload).
func matchedPayloads(stdout []byte) []string {
	seen := make(map[string]bool)
	var out []string
	for _, line := range strings.Split(string(stdout), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var r struct {
			Url string `json:"url"`
		}
		if json.Unmarshal([]byte(line), &r) != nil || r.Url == "" {
			continue
		}
		p := path.Base(r.Url)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// TestE2E_JSONOutput runs the built binary against the mock and checks that the
// CLI parsed the flags, ran the scan, and emitted the matched set as JSON. This
// is what protects the declarative flag/config wiring.
func TestE2E_JSONOutput(t *testing.T) {
	bin := buildFfuf(t)

	target := testtarget.New()
	defer target.Close()

	wl := writeWordlist(t, []string{"200", "201", "404"})
	cmd := exec.Command(bin,
		"-u", target.URL+"/status/FUZZ",
		"-w", wl,
		"-mc", "200,201",
		"-json",
		"-t", "1", // deterministic ordering; correctness test, not a concurrency test
		"-noninteractive",
	)
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffuf run failed: %v", err)
	}

	assertSet(t, matchedPayloads(stdout), []string{"200", "201"})
}

// firstHashAndPayload parses the first -json result and returns its FFUFHASH
// (Input unmarshals from base64 straight into []byte) and the payload (last URL
// segment).
func firstHashAndPayload(t *testing.T, stdout []byte) (hash, payload string) {
	t.Helper()
	for _, line := range strings.Split(string(stdout), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var r struct {
			Input map[string][]byte `json:"input"`
			Url   string            `json:"url"`
		}
		if json.Unmarshal([]byte(line), &r) != nil {
			continue
		}
		if h, ok := r.Input["FFUFHASH"]; ok && len(h) > 0 && r.Url != "" {
			return string(h), path.Base(r.Url)
		}
	}
	t.Fatalf("no result with a FFUFHASH found in:\n%s", stdout)
	return "", ""
}

// TestE2E_FFUFHashRoundTrip runs a scan, takes a FFUFHASH from the output, and
// feeds it back to `ffuf -search`, asserting the original request is
// reconstructed. This exercises the retained-options history serialization end to
// end (the mechanism the declarative-config refactor replaced ToOptions with).
func TestE2E_FFUFHashRoundTrip(t *testing.T) {
	bin := buildFfuf(t)

	target := testtarget.New()
	defer target.Close()

	// Isolate the config/history dir for both runs (honored on Linux; on macOS it
	// falls back to the real dir but the two runs still agree, so the round-trip holds).
	cfgHome := t.TempDir()
	env := append(os.Environ(), "XDG_CONFIG_HOME="+cfgHome)

	wl := writeWordlist(t, []string{"200", "201"})
	run1 := exec.Command(bin,
		"-u", target.URL+"/status/FUZZ", "-w", wl,
		"-mc", "200,201", "-json", "-t", "1", "-noninteractive",
	)
	run1.Env = env
	out1, err := run1.Output()
	if err != nil {
		t.Fatalf("scan run failed: %v", err)
	}
	hash, payload := firstHashAndPayload(t, out1)

	run2 := exec.Command(bin, "-search", hash, "-noninteractive")
	run2.Env = env
	out2, err := run2.Output()
	if err != nil {
		t.Fatalf("search run failed: %v", err)
	}

	s := string(out2)
	if !strings.Contains(s, "Request candidate") {
		t.Fatalf("-search found no candidate for %q:\n%s", hash, s)
	}
	if !strings.Contains(s, "/status/"+payload) {
		t.Errorf("-search did not reconstruct the request path /status/%s:\n%s", payload, s)
	}
}
