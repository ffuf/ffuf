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
