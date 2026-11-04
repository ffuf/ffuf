package output

import (
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// These tests guard the deterministic ordering of result output (issue #307):
// with multiple wordlists, every result line must list its input parameters in
// a stable order that is the same on every run, never in Go map iteration
// order. Map iteration order is randomized per range statement, so asserting an
// exact expected line over repeated calls reliably catches a regression to
// iterating res.Input.

// multiKeywordResult returns a result whose input map is deliberately filled in
// non-sorted order so that map iteration order cannot accidentally match the
// expected output order.
func multiKeywordResult() ffuf.Result {
	return ffuf.Result{
		Position: 7,
		Input: map[string][]byte{
			"PATH": []byte("p"),
			"HOST": []byte("h"),
			"AAA":  []byte("a"),
		},
	}
}

func multiKeywordConfig() *ffuf.Config {
	return &ffuf.Config{
		InputProviders: []ffuf.InputProviderConfig{
			{Name: "wordlist", Keyword: "PATH"},
			{Name: "wordlist", Keyword: "HOST"},
			{Name: "wordlist", Keyword: "AAA"},
		},
	}
}

func TestPrepareInputsOneLineStableParameterOrder(t *testing.T) {
	outp := NewStdoutput(multiKeywordConfig())
	res := multiKeywordResult()
	// Parameters in sorted keyword order, "KEY : value " pairs as produced by
	// the single-line output modes.
	want := "AAA : a HOST : h PATH : p "
	for i := 0; i < 50; i++ {
		if got := outp.prepareInputsOneLine(res); got != want {
			t.Fatalf("iteration %d: prepareInputsOneLine() = %q, want %q", i, got, want)
		}
	}
}

func TestResultQuietOutputStableParameterOrder(t *testing.T) {
	conf := multiKeywordConfig()
	conf.Quiet = true
	outp := NewStdoutput(conf)
	outp.stdoutIsTerminal = false
	res := multiKeywordResult()

	want := "AAA : a HOST : h PATH : p \n"
	for i := 0; i < 20; i++ {
		out := captureStdout(t, func() {
			outp.PrintResult(res)
		})
		if out != want {
			t.Fatalf("iteration %d: silent result line = %q, want %q", i, out, want)
		}
	}
}

func TestResultMultilineStableParameterAndScraperOrder(t *testing.T) {
	outp := NewStdoutput(multiKeywordConfig())
	outp.stdoutIsTerminal = false
	res := multiKeywordResult()
	res.ScraperData = map[string][]string{
		"zeta":  {"z1"},
		"alpha": {"a1"},
		"mid":   {"m1"},
	}

	// Multiline result lines, in order: input parameters in sorted keyword
	// order, then scraper fields in sorted key order.
	wantSeq := []string{
		"* AAA:", "* HOST:", "* PATH:",
		"* alpha:", "* mid:", "* zeta:",
	}
	for i := 0; i < 20; i++ {
		out := captureStdout(t, func() {
			outp.PrintResult(res)
		})
		last := -1
		for _, want := range wantSeq {
			idx := strings.Index(out, want)
			if idx < 0 {
				t.Fatalf("iteration %d: multiline output missing %q:\n%s", i, want, out)
			}
			if idx < last {
				t.Fatalf("iteration %d: %q printed out of order:\n%s", i, want, out)
			}
			last = idx
		}
	}
}
