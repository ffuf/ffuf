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

// captureStream drains the read end while fn runs. Reading only after fn returns
// deadlocks once fn writes more than the pipe buffer holds.
func captureStream(t *testing.T, stream **os.File, fn func()) string {
	t.Helper()
	orig := *stream
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %s", err)
	}
	*stream = w
	defer func() { *stream = orig }()

	type capture struct {
		out string
		err error
	}
	done := make(chan capture, 1)
	go func() {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, r)
		done <- capture{buf.String(), err}
	}()

	fn()

	w.Close()
	c := <-done
	r.Close()
	if c.err != nil {
		t.Fatalf("failed to read captured output: %s", c.err)
	}
	return c.out
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
	if ANSI_CLEAR == "" {
		t.Skip("this build emits no ANSI escapes, so there is no reset code to look for")
	}
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
	if ANSI_CLEAR == "" {
		t.Skip("this build emits no ANSI escapes, so there is no reset code to look for")
	}
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

func TestFormatNormalResultAlignsMetricsWithoutTruncatingInput(t *testing.T) {
	cases := []struct {
		name  string
		input string
		res   ffuf.Result
	}{
		{"short input", "admin", ffuf.Result{StatusCode: 200, ContentLength: 42, ContentWords: 3, ContentLines: 1, Duration: 5 * time.Millisecond}},
		{"wide numeric fields", "login", ffuf.Result{StatusCode: 404, ContentLength: 12345678, ContentWords: 123456, ContentLines: 12345, Duration: 1200 * time.Millisecond}},
		{"long input remains complete", "really-long-path-which-exceeds-the-normal-column-width", ffuf.Result{StatusCode: 301, ContentLength: 128, ContentWords: 20, ContentLines: 7, Duration: 44 * time.Millisecond}},
		{"unicode input", "küche", ffuf.Result{StatusCode: 200, ContentLength: 50, ContentWords: 5, ContentLines: 2, Duration: 4 * time.Millisecond}},
	}
	var metricsColumn map[string]int
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatNormalResult(tc.input, tc.res)
			if !strings.Contains(got, tc.input) {
				t.Fatalf("result lost input %q: %q", tc.input, got)
			}
			lines := strings.Split(got, "\n")
			long := len([]rune(tc.input)) > normalResultInputWidth
			if long && (len(lines) != 2 || lines[0] != tc.input) {
				t.Fatalf("long input must be preserved on its own line: %q", got)
			}
			if !long && len(lines) != 1 {
				t.Fatalf("unexpected extra line for short input: %q", got)
			}
			last := lines[len(lines)-1]
			labels := []string{"[Status:", ", Size:", ", Words:", ", Lines:", ", Duration:"}
			if metricsColumn == nil {
				metricsColumn = make(map[string]int)
				for _, label := range labels {
					metricsColumn[label] = strings.Index(last, label)
				}
			}
			for _, label := range labels {
				if pos := strings.Index(last, label); pos != metricsColumn[label] {
					t.Errorf("%s starts at column %d, want %d in %q", label, pos, metricsColumn[label], got)
				}
			}
			if idx := strings.Index(last, "[Status:"); idx != normalResultInputWidth+1 {
				t.Errorf("metrics start at column %d, want %d", idx, normalResultInputWidth+1)
			}
		})
	}
}
