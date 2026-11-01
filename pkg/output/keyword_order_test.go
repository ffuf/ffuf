package output

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// The keyword order on the command line (DIR, SUB, FILE) is deliberately not
// alphabetical, so that neither random map iteration nor the sorted map
// iteration of text/template can line the values up by accident.
func keywordOrderFixture() (*ffuf.Config, []ffuf.Result) {
	config := &ffuf.Config{
		CommandLine: "ffuf -u http://example.com/DIR/SUB/FILE",
		InputProviders: []ffuf.InputProviderConfig{
			{Name: "wordlist", Keyword: "DIR"},
			{Name: "wordlist", Keyword: "SUB"},
			{Name: "wordlist", Keyword: "FILE"},
		},
	}
	results := []ffuf.Result{
		{
			Input: map[string][]byte{
				"DIR":      []byte("dir1"),
				"SUB":      []byte("subdir1"),
				"FILE":     []byte("file1"),
				"FFUFHASH": []byte("hash1"),
			},
			Url:        "http://example.com/dir1/subdir1/file1",
			StatusCode: 404,
		},
	}
	return config, results
}

func TestWriteCSVKeywordOrder(t *testing.T) {
	config, results := keywordOrderFixture()
	filename := filepath.Join(t.TempDir(), "report.csv")

	// Map iteration order is random, so repeat to make a regression visible.
	for i := 0; i < 50; i++ {
		if err := writeCSV(filename, config, results, false); err != nil {
			t.Fatalf("writeCSV failed: %v", err)
		}
		f, err := os.Open(filename)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := csv.NewReader(f).ReadAll()
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 2 {
			t.Fatalf("expected header and one row, got %d rows", len(rows))
		}
		if got := rows[0][:3]; !reflect.DeepEqual(got, []string{"DIR", "SUB", "FILE"}) {
			t.Fatalf("unexpected header keywords: %v", got)
		}
		if got := rows[1][:3]; !reflect.DeepEqual(got, []string{"dir1", "subdir1", "file1"}) {
			t.Fatalf("input values not under their keyword columns: %v", got)
		}
		if got := rows[1][len(rows[1])-1]; got != "hash1" {
			t.Fatalf("expected FFUFHASH in the last column, got %q", got)
		}
	}
}

func TestWriteHTMLKeywordOrder(t *testing.T) {
	config, results := keywordOrderFixture()
	filename := filepath.Join(t.TempDir(), "report.html")
	if err := writeHTML(filename, config, results); err != nil {
		t.Fatalf("writeHTML failed: %v", err)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if !strings.Contains(out, "|result_raw|404|dir1|subdir1|file1|") {
		t.Errorf("raw result row has input values in the wrong order")
	}
	dir, sub, file := strings.Index(out, "<td>dir1</td>"), strings.Index(out, "<td>subdir1</td>"), strings.Index(out, "<td>file1</td>")
	if dir == -1 || sub == -1 || file == -1 || !(dir < sub && sub < file) {
		t.Errorf("table cells are not in keyword order (DIR=%d SUB=%d FILE=%d)", dir, sub, file)
	}
}

func TestWriteMarkdownKeywordOrder(t *testing.T) {
	config, results := keywordOrderFixture()
	filename := filepath.Join(t.TempDir(), "report.md")
	if err := writeMarkdown(filename, config, results); err != nil {
		t.Fatalf("writeMarkdown failed: %v", err)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "| dir1 | subdir1 | file1 | http://example.com/dir1/subdir1/file1 |") {
		t.Errorf("markdown row has input values in the wrong order:\n%s", data)
	}
}
