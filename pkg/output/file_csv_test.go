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

	csv := toCSV(result, []string{"x"})

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
		"resultfile",
		"A"}) {
		t.Errorf("CSV was not generated in expected format")
	}
}

func TestToCSV_MultipleKeywords(t *testing.T) {
	result := ffuf.Result{
		Input:            map[string][]byte{"FUZ2": []byte("bbb"), "FUZZ": []byte("aaa"), "FFUFHASH": []byte("A")},
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

	csv := toCSV(result, []string{"FUZZ", "FUZ2"})
	expected := []string{"aaa", "bbb", "http://as.df", "http://no.pe", "1", "200", "3", "4", "5", "application/json", "123ns", "resultfile", "A"}
	if !reflect.DeepEqual(csv, expected) {
		t.Errorf("CSV was not generated in expected format. Expected: %v, Got: %v", expected, csv)
	}

	// Test reversed keyword order
	csvReversed := toCSV(result, []string{"FUZ2", "FUZZ"})
	expectedReversed := []string{"bbb", "aaa", "http://as.df", "http://no.pe", "1", "200", "3", "4", "5", "application/json", "123ns", "resultfile", "A"}
	if !reflect.DeepEqual(csvReversed, expectedReversed) {
		t.Errorf("CSV was not generated in expected format. Expected: %v, Got: %v", expectedReversed, csvReversed)
	}
}
