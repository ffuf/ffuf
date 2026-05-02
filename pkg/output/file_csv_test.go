package output

import (
	"reflect"
	"testing"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

func TestToCSV(t *testing.T) {
	result := ffuf.Result{
		Input:            map[string][]byte{"x": {66}, "FFUFHASH": {65}},
		Position:         1,
		StatusCode:       200,
		ContentLength:    3,
		ContentWords:     4,
		ContentLines:     5,
		ContentType:      "application/json",
		RedirectLocation: "http://no.pe",
		Url:              "http://as.df",
		Duration:         time.Duration(123),
		ResultFile:       "resultfile",
		Host:             "host",
	}

	csv := toCSV(result)

	// Empty redirects column (no --redirect-chain captured) sits between
	// redirectlocation and position; matches the staticheaders order.
	if !reflect.DeepEqual(csv, []string{
		"B",
		"http://as.df",
		"http://no.pe",
		"",
		"1",
		"200",
		"3",
		"4",
		"5",
		"application/json",
		"123ns",
		"resultfile",
		"A"}) {
		t.Errorf("CSV was not generated in expected format: %#v", csv)
	}
}

func TestToCSV_WithRedirectChain(t *testing.T) {
	result := ffuf.Result{
		Input:            map[string][]byte{"x": {66}, "FFUFHASH": {65}},
		Position:         1,
		StatusCode:       200,
		ContentLength:    3,
		ContentWords:     4,
		ContentLines:     5,
		ContentType:      "application/json",
		RedirectLocation: "",
		Redirects: []ffuf.RedirectHop{
			{URL: "http://a/x", StatusCode: 301, Location: "http://a/y"},
			{URL: "http://a/y", StatusCode: 302, Location: "http://a/z"},
		},
		Url:        "http://a/z",
		Duration:   time.Duration(123),
		ResultFile: "",
		Host:       "host",
	}

	csv := toCSV(result)
	want := "301 http://a/x -> http://a/y | 302 http://a/y -> http://a/z"
	// In staticheaders the order is: url, redirectlocation, redirects, ...
	// With one user input keyword ("x") prepended, the redirects cell sits at
	// csv[3] (csv[0]=B, csv[1]=url, csv[2]=redirectlocation, csv[3]=redirects).
	if csv[3] != want {
		t.Errorf("redirects column: got %q, want %q (full row %#v)", csv[3], want, csv)
	}
}
