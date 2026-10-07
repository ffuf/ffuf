package input

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

func TestStripCommentsIgnoresCommentLines(t *testing.T) {
	text, _ := stripComments("# text")

	if text != "" {
		t.Errorf("Returned text was not a blank string")
	}
}

func TestStripCommentsStripsCommentAfterText(t *testing.T) {
	text, _ := stripComments("text # comment")

	if text != "text" {
		t.Errorf("Comment was not stripped or pre-comment text was not returned")
	}
}

func TestWordlistAcceptsLinesLargerThanScannerDefault(t *testing.T) {
	// Issue #567 reports a real 448 KiB single-entry wordlist.
	payload := strings.Repeat("a", 448*1024)
	wordlist := filepath.Join(t.TempDir(), "large-wordlist.txt")
	if err := os.WriteFile(wordlist, []byte(payload+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	config := ffuf.NewConfig(ctx, cancel)
	input, err := NewWordlistInput("FUZZ", wordlist, &config)
	if err != nil {
		t.Fatalf("reading large wordlist entry: %v", err)
	}
	if input.Total() != 1 {
		t.Fatalf("got %d entries, want 1", input.Total())
	}
	if got := string(input.Value()); got != payload {
		t.Fatalf("payload length = %d, want %d", len(got), len(payload))
	}
}

// A wordlist line that contains the "%ext%" placeholder used to skip comment
// stripping entirely: a fully commented-out "%ext%" line leaked straight into
// the fuzzed candidates, and a trailing "# comment" on a "%ext%" line was
// never trimmed off. Both must behave the same as any other line.
func TestReadFileStripsCommentsOnExtLines(t *testing.T) {
	f, err := os.CreateTemp("", "ffuf-wordlist-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	if _, err := f.WriteString("admin.%ext% # internal, do not scan\n# %ext% fully commented out\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()

	conf := &ffuf.Config{
		DirSearchCompat:        true,
		IgnoreWordlistComments: true,
		Extensions:             []string{".php"},
	}

	wl, err := NewWordlistInput("FUZZ", f.Name(), conf)
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for wl.Next() {
		got = append(got, string(wl.Value()))
		wl.IncrementPosition()
	}

	want := []string{"admin..php"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("expected %q, got %q", want[i], got[i])
		}
	}
}
