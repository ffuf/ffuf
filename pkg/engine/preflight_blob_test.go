package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// TestPreflightBlobIncludesFiles guards the engine-side half of keyword support:
// preflightBlob concatenates the preflight/postflight file contents so keyword
// activation can see a keyword used only there.
func TestPreflightBlobIncludesFiles(t *testing.T) {
	dir := t.TempDir()
	pre := filepath.Join(dir, "pre.txt")
	post := filepath.Join(dir, "post.txt")
	if err := os.WriteFile(pre, []byte("GET /pre/FUZZ HTTP/1.1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(post, []byte("GET /post/HASHKW HTTP/1.1"), 0644); err != nil {
		t.Fatal(err)
	}

	conf := &ffuf.Config{
		Preflights:  []ffuf.PreflightConfig{{RequestFile: pre}},
		Postflights: []ffuf.PreflightConfig{{RequestFile: post}},
	}
	blob := preflightBlob(conf)

	if !strings.Contains(blob, "FUZZ") {
		t.Error("blob missing preflight keyword FUZZ")
	}
	if !strings.Contains(blob, "HASHKW") {
		t.Error("blob missing postflight keyword HASHKW")
	}
}
