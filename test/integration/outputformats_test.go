package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/ffuf/ffuf/v2/pkg/filter"
	"github.com/ffuf/ffuf/v2/pkg/input"
	"github.com/ffuf/ffuf/v2/pkg/output"
	"github.com/ffuf/ffuf/v2/pkg/runner"
	"github.com/ffuf/ffuf/v2/pkg/testtarget"
)

// runScanStdout runs a scan with the real stdout OutputProvider (which
// accumulates Results and can write them to files), and returns it so a test can
// SaveFile each format and inspect the output. It mirrors runScan; only the
// output sink differs.
func runScanStdout(t *testing.T, url string, wordlist []string, configure func(*ffuf.ConfigOptions), setup func(ffuf.MatcherManager)) ffuf.OutputProvider {
	t.Helper()

	opts := ffuf.NewConfigOptions()
	opts.HTTP.URL = url
	opts.HTTP.Method = "GET"
	opts.Input.Wordlists = []string{writeWordlist(t, wordlist)}
	opts.General.Quiet = true
	if configure != nil {
		configure(opts)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	conf, err := ffuf.ConfigFromOptions(opts, ctx, cancel)
	if err != nil {
		t.Fatalf("ConfigFromOptions: %v", err)
	}
	conf.MatcherManager = filter.NewMatcherManager()
	if setup != nil {
		setup(conf.MatcherManager)
	}

	job := ffuf.NewJob(conf)
	in, ierr := input.NewInputProvider(conf)
	if ierr.ErrorOrNil() != nil {
		t.Fatalf("NewInputProvider: %v", ierr.ErrorOrNil())
	}
	job.Input = in
	job.Runner = runner.NewRunnerByName("http", conf, false)
	out := output.NewOutputProviderByName("stdout", conf)
	job.Output = out

	job.Start()
	return out
}

// TestOutputFormats runs a scan with two known matches and writes each supported
// file format, checking the writer succeeds and the output actually carries the
// results. It is the regression net for the pkg/output file writers.
func TestOutputFormats(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	out := runScanStdout(t, target.URL+"/status/FUZZ",
		[]string{"200", "201"},
		nil,
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200,201") },
	)

	dir := t.TempDir()
	for _, format := range []string{"json", "ejson", "csv", "ecsv", "md", "html"} {
		path := filepath.Join(dir, "out."+format)
		if err := out.SaveFile(path, format); err != nil {
			t.Errorf("SaveFile(%s): %v", format, err)
			continue
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", format, err)
			continue
		}
		if len(b) == 0 {
			t.Errorf("%s: wrote an empty file", format)
			continue
		}

		switch format {
		case "json", "ejson":
			// Both are JSON documents with a results array; both matches present.
			var doc struct {
				Results []map[string]interface{} `json:"results"`
			}
			if err := json.Unmarshal(b, &doc); err != nil {
				t.Errorf("%s: not valid JSON: %v", format, err)
			} else if len(doc.Results) != 2 {
				t.Errorf("%s: %d results, want 2", format, len(doc.Results))
			}
		case "csv", "ecsv":
			// Header line plus one row per result.
			if lines := strings.Count(strings.TrimRight(string(b), "\n"), "\n") + 1; lines < 3 {
				t.Errorf("%s: %d lines, want >= 3 (header + 2 rows)", format, lines)
			}
		case "md", "html":
			// Raw (non-encoded) formats: the input values appear verbatim.
			for _, in := range []string{"200", "201"} {
				if !strings.Contains(string(b), in) {
					t.Errorf("%s: output missing input %q", format, in)
				}
			}
		}
	}
}
