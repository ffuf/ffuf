package scraper

import (
	"path/filepath"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// TestFromDir_UsesDirnameNotGlobal proves FromDir enumerates the directory it is
// given rather than the package-global ffuf.SCRAPERDIR. Before the fix it read the
// global for the directory listing while opening the entries relative to dirname,
// so a valid dirname combined with a missing global still errored.
func TestFromDir_UsesDirnameNotGlobal(t *testing.T) {
	orig := ffuf.SCRAPERDIR
	defer func() { ffuf.SCRAPERDIR = orig }()

	valid := t.TempDir() // exists, empty
	ffuf.SCRAPERDIR = filepath.Join(t.TempDir(), "does-not-exist")

	_, errs := FromDir(valid, "all")
	if errs.ErrorOrNil() != nil {
		t.Errorf("FromDir should read its dirname argument (a valid dir), not the global (missing): %v", errs.ErrorOrNil())
	}
}
