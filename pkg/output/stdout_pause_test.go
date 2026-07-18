package output

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

func resp(i int) ffuf.Response {
	return ffuf.Response{
		StatusCode: 200,
		Request: &ffuf.Request{
			Input: map[string][]byte{"FUZZ": []byte(fmt.Sprintf("w%d", i))},
			Url:   fmt.Sprintf("http://example/%d", i),
		},
	}
}

// TestResultSuppressedWhilePaused locks the core fix: while the interactive
// console is open (SetPaused(true)), Result must record the match but write
// nothing to the terminal, so inflight requests completing cannot scroll the
// console banner off screen. The recorded results must still be retrievable
// (show / savejson / -o output stay complete).
func TestResultSuppressedWhilePaused(t *testing.T) {
	s := NewStdoutput(&ffuf.Config{})
	s.stdoutIsTerminal = false
	s.SetPaused(true)

	const n = 5
	out := captureStdout(t, func() {
		for i := 0; i < n; i++ {
			s.Result(resp(i))
		}
	})

	if out != "" {
		t.Errorf("Result printed to stdout while paused: %q", out)
	}
	if got := len(s.GetCurrentResults()); got != n {
		t.Errorf("paused Result did not record: got %d results, want %d", got, n)
	}
	if got := s.PendingResults(); got != n {
		t.Errorf("PendingResults = %d, want %d", got, n)
	}
}

// TestResultPrintsWhenNotPaused is the negative control: with no pause in
// effect, Result streams to the terminal and PendingResults stays zero.
func TestResultPrintsWhenNotPaused(t *testing.T) {
	s := NewStdoutput(&ffuf.Config{})
	s.stdoutIsTerminal = false

	out := captureStdout(t, func() {
		s.Result(resp(1))
	})

	if !strings.Contains(out, "http://example/1") && !strings.Contains(out, "Status: 200") {
		t.Errorf("Result did not print while unpaused: %q", out)
	}
	if got := s.PendingResults(); got != 0 {
		t.Errorf("PendingResults = %d while unpaused, want 0", got)
	}
}

// TestPendingReconcilesWithFilter is the fix for the correctness gap: a held
// result that a mid-pause filter change prunes from CurrentResults must also
// drop out of the pending count, so the "N new matches" report can never
// contradict what "show" displays.
func TestPendingReconcilesWithFilter(t *testing.T) {
	s := NewStdoutput(&ffuf.Config{})
	s.stdoutIsTerminal = false

	s.SetPaused(true)
	s.Result(resp(1))
	s.Result(resp(2))
	if got := s.PendingResults(); got != 2 {
		t.Fatalf("PendingResults = %d, want 2", got)
	}

	// Simulate an interactive filter change that keeps only w1, as
	// refreshResults does via FilterCurrentResults.
	s.FilterCurrentResults(func(r ffuf.Result) bool {
		return string(r.Input["FUZZ"]) == "w1"
	})

	if got := s.PendingResults(); got != 1 {
		t.Errorf("PendingResults = %d after filtering out a held result, want 1 (count must track CurrentResults)", got)
	}
	if got := len(s.GetCurrentResults()); got != 1 {
		t.Errorf("CurrentResults = %d, want 1", got)
	}
}

// TestUnpauseMarksHeldSeen proves resume acknowledges held results: after
// leaving a pause a subsequent pause with no new matches reports zero, rather
// than re-counting results the user was already told about.
func TestUnpauseMarksHeldSeen(t *testing.T) {
	s := NewStdoutput(&ffuf.Config{})
	s.stdoutIsTerminal = false

	s.SetPaused(true)
	s.Result(resp(1))
	s.Result(resp(2))
	if got := s.PendingResults(); got != 2 {
		t.Fatalf("PendingResults = %d, want 2", got)
	}

	s.SetPaused(false) // resume acknowledges the held results
	if got := s.PendingResults(); got != 0 {
		t.Errorf("PendingResults = %d after resume, want 0 (held results not marked seen)", got)
	}

	s.SetPaused(true) // pause again, no new matches arrive
	if got := s.PendingResults(); got != 0 {
		t.Errorf("PendingResults = %d on re-pause, want 0 (old results re-counted as new)", got)
	}
}

// TestConcurrentResultWhilePaused runs Result from many goroutines while paused
// (as the engine's worker pool does) and asserts nothing is lost and nothing is
// printed. Run with -race to catch a data race on paused/Printed directly.
func TestConcurrentResultWhilePaused(t *testing.T) {
	s := NewStdoutput(&ffuf.Config{})
	s.stdoutIsTerminal = false
	s.SetPaused(true)

	const n = 200
	out := captureStdout(t, func() {
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				s.Result(resp(i))
			}(i)
		}
		wg.Wait()
	})

	if out != "" {
		t.Errorf("Result printed to stdout while paused under concurrency: %q", out)
	}
	if got := len(s.GetCurrentResults()); got != n {
		t.Errorf("got %d results, want %d (results lost while paused)", got, n)
	}
	if got := s.PendingResults(); got != n {
		t.Errorf("PendingResults = %d, want %d", got, n)
	}
}
