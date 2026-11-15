package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// Regression test for https://github.com/ffuf/ffuf/issues/568
//
// The HTML report renders Duration as a human readable string
// (e.g. "92.517834ms") and the table is sorted client side by
// DataTables. With no numeric sort key, DataTables sorts the column
// lexicographically, so "92.517834ms" is considered greater than
// "896.832625ms" even though 92ms < 896ms. The Duration cell must
// therefore carry the duration in nanoseconds as a data-order
// attribute, which DataTables uses as the sort key while still
// displaying the human readable string.
func TestHTMLDurationSortKey(t *testing.T) {
	results := []ffuf.Result{
		{
			Input:         map[string][]byte{"FUZZ": []byte("a")},
			Position:      0,
			StatusCode:    200,
			ContentLength: 3,
			ContentWords:  4,
			ContentLines:  5,
			ContentType:   "text/html",
			Url:           "http://example.com/a",
			Duration:      92517834 * time.Nanosecond, // 92.517834ms
		},
		{
			Input:         map[string][]byte{"FUZZ": []byte("b")},
			Position:      1,
			StatusCode:    200,
			ContentLength: 3,
			ContentWords:  4,
			ContentLines:  5,
			ContentType:   "text/html",
			Url:           "http://example.com/b",
			Duration:      896832625 * time.Nanosecond, // 896.832625ms
		},
	}

	config := &ffuf.Config{
		CommandLine: "ffuf -u http://example.com/FUZZ",
		InputProviders: []ffuf.InputProviderConfig{
			{Keyword: "FUZZ"},
		},
	}

	outfile := filepath.Join(t.TempDir(), "report.html")
	if err := writeHTML(outfile, config, results); err != nil {
		t.Fatalf("writeHTML failed: %v", err)
	}

	raw, err := os.ReadFile(outfile)
	if err != nil {
		t.Fatalf("could not read generated HTML: %v", err)
	}
	html := string(raw)

	// Both durations must be rendered, and each Duration cell must
	// carry the numeric (nanosecond) sort key DataTables sorts on.
	for _, want := range []string{
		`<td data-order="92517834">92.517834ms</td>`,
		`<td data-order="896832625">896.832625ms</td>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("generated HTML is missing numeric Duration sort key %q", want)
		}
	}
}
