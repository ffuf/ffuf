package output

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// captureStdout redirects os.Stdout to an in-memory pipe for the duration of
// fn and returns everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return captureStream(t, &os.Stdout, fn)
}

// captureStderr is the os.Stderr equivalent of captureStdout.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return captureStream(t, &os.Stderr, fn)
}

func captureStream(t *testing.T, stream **os.File, fn func()) string {
	t.Helper()
	orig := *stream
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %s", err)
	}
	*stream = w
	defer func() { *stream = orig }()

	fn()

	w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured output: %s", err)
	}
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

func TestErrorOutputOmitsControlCharsWhenStderrIsNotATerminal(t *testing.T) {
	conf := &ffuf.Config{}
	outp := NewStdoutput(conf)
	outp.stderrIsTerminal = false

	out := captureStderr(t, func() {
		outp.Error("something broke")
	})

	if strings.Contains(out, TERMINAL_CLEAR_LINE) {
		t.Errorf("Error() written to a non-terminal stderr still contains the terminal clear-line sequence: %q", out)
	}
}

func TestErrorOutputKeepsControlCharsOnATerminal(t *testing.T) {
	conf := &ffuf.Config{}
	outp := NewStdoutput(conf)
	outp.stderrIsTerminal = true

	out := captureStderr(t, func() {
		outp.Error("something broke")
	})

	if !strings.Contains(out, TERMINAL_CLEAR_LINE) {
		t.Errorf("Error() written to a terminal stderr is missing the terminal clear-line sequence: %q", out)
	}
}

func TestProgressOutputOmitsControlCharsWhenStderrIsNotATerminal(t *testing.T) {
	conf := &ffuf.Config{}
	outp := NewStdoutput(conf)
	outp.stderrIsTerminal = false

	out := captureStderr(t, func() {
		outp.Progress(ffuf.Progress{StartedAt: time.Now()})
	})

	if strings.Contains(out, TERMINAL_CLEAR_LINE) {
		t.Errorf("Progress() written to a non-terminal stderr still contains the terminal clear-line sequence: %q", out)
	}
}

func TestResultOutputOmitsColorResetWhenColorsAreDisabled(t *testing.T) {
	conf := &ffuf.Config{Colors: false}
	outp := NewStdoutput(conf)
	outp.stdoutIsTerminal = true // isolate this from the clear-line behavior above

	res := ffuf.Result{StatusCode: 200, ContentLength: 42, ContentWords: 3, ContentLines: 1}

	out := captureStdout(t, func() {
		outp.PrintResult(res)
	})

	if strings.Contains(out, ANSI_CLEAR) {
		t.Errorf("result line printed without -c still carries an ANSI reset code: %q", out)
	}
}

func TestResultOutputKeepsColorResetWhenColorsAreEnabled(t *testing.T) {
	conf := &ffuf.Config{Colors: true}
	outp := NewStdoutput(conf)
	outp.stdoutIsTerminal = false // -c output is opt-in and shouldn't depend on terminal detection

	res := ffuf.Result{StatusCode: 200, ContentLength: 42, ContentWords: 3, ContentLines: 1}

	out := captureStdout(t, func() {
		outp.PrintResult(res)
	})

	if !strings.Contains(out, ANSI_CLEAR) {
		t.Errorf("result line printed with -c is missing its ANSI reset code: %q", out)
	}
}
