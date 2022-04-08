package output

import (
	"reflect"
	"testing"
	"time"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

func TestToCSV(t *testing.T) {
	result := ffuf.Result{
		Input:            map[string][]byte{"x": {66}},
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

	if !reflect.DeepEqual(csv, []string{
		"B",
		"http://as.df",
		"http://no.pe",
		"1",
		"200",
		"3",
		"4",
		"5",
		"application/json",
		"123ns",
		"resultfile"}) {

		t.Errorf("CSV was not generated in expected format")
	}
}
