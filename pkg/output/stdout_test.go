package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// captureStdout redirects os.Stdout to an in-memory pipe for the duration of
// fn and returns everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %s", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestResultOutputOmitsControlCharsWhenStdoutIsNotATerminal(t *testing.T) {
	conf := &ffuf.Config{}
	outp := NewStdoutput(conf)
	// A pipe (as used by captureStdout, and as ffuf sees when its stdout is
	// redirected to a file with `>`) is never a terminal.
	outp.stdoutIsTerminal = false

	res := ffuf.Result{StatusCode: 200, ContentLength: 42, ContentWords: 3, ContentLines: 1}

	out := captureStdout(t, func() {
		outp.PrintResult(res)
	})

	if strings.Contains(out, TERMINAL_CLEAR_LINE) {
		t.Errorf("result line written to a non-terminal stdout still contains the terminal clear-line sequence: %q", out)
	}
}

func TestResultOutputKeepsControlCharsOnATerminal(t *testing.T) {
	conf := &ffuf.Config{}
	outp := NewStdoutput(conf)
	// Simulate stdout being an interactive terminal, which is when the
	// progress-bar-clearing sequence is actually needed.
	outp.stdoutIsTerminal = true

	res := ffuf.Result{StatusCode: 200, ContentLength: 42, ContentWords: 3, ContentLines: 1}

	out := captureStdout(t, func() {
		outp.PrintResult(res)
	})

	if !strings.Contains(out, TERMINAL_CLEAR_LINE) {
		t.Errorf("result line written to a terminal stdout is missing the terminal clear-line sequence: %q", out)
	}
}
