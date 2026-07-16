package integration

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
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

func indexOf(row []string, col string) int {
	for i, c := range row {
		if c == col {
			return i
		}
	}
	return -1
}

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

	// Single-threaded: the real stdout provider accumulates results without
	// synchronizing, so under concurrency a result can be lost. This test is
	// about the file writers, not concurrency, so run it deterministically.
	out := runScanStdout(t, target.URL+"/status/FUZZ",
		[]string{"200", "201"},
		func(o *ffuf.ConfigOptions) { o.General.Threads = 1 },
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

		// Assert the actual result FIELDS (status + input), not just that a file
		// was written or that "200" appears (which the URL would satisfy).
		switch format {
		case "json":
			var doc struct {
				Results []struct {
					Input  map[string]string `json:"input"`
					Status int64             `json:"status"`
				} `json:"results"`
			}
			if err := json.Unmarshal(b, &doc); err != nil {
				t.Errorf("json: %v", err)
				continue
			}
			var statuses, inputs []string
			for _, r := range doc.Results {
				statuses = append(statuses, fmt.Sprintf("%d", r.Status))
				inputs = append(inputs, r.Input["FUZZ"])
			}
			assertSet(t, statuses, []string{"200", "201"})
			assertSet(t, inputs, []string{"200", "201"})
		case "ejson":
			// ejson base64-encodes the input; unmarshalling into []byte decodes it.
			var doc struct {
				Results []struct {
					Input  map[string][]byte `json:"input"`
					Status int64             `json:"status"`
				} `json:"results"`
			}
			if err := json.Unmarshal(b, &doc); err != nil {
				t.Errorf("ejson: %v", err)
				continue
			}
			var statuses, inputs []string
			for _, r := range doc.Results {
				statuses = append(statuses, fmt.Sprintf("%d", r.Status))
				inputs = append(inputs, string(r.Input["FUZZ"]))
			}
			assertSet(t, statuses, []string{"200", "201"})
			assertSet(t, inputs, []string{"200", "201"})
		case "csv", "ecsv":
			rows, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
			if err != nil {
				t.Errorf("%s: not valid CSV: %v", format, err)
				continue
			}
			if len(rows) != 3 {
				t.Errorf("%s: %d rows, want 3 (header + 2 results)", format, len(rows))
				continue
			}
			col := indexOf(rows[0], "status_code")
			if col < 0 {
				t.Errorf("%s: no StatusCode column in header %v", format, rows[0])
				continue
			}
			assertSet(t, []string{rows[1][col], rows[2][col]}, []string{"200", "201"})
		case "md", "html":
			// Human formats: both status codes must be present in the rendered output.
			for _, in := range []string{"200", "201"} {
				if !strings.Contains(string(b), in) {
					t.Errorf("%s: output missing status %q", format, in)
				}
			}
		}
	}
}
