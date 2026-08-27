package engine

import (
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/ffuf/ffuf/v2/pkg/filter"
)

// calibJob builds a minimal Job wired to a real MatcherManager, which is
// enough to drive calibrateFilters directly without any HTTP plumbing.
func calibJob() (*Job, ffuf.MatcherManager) {
	mm := filter.NewMatcherManager()
	j := &Job{
		Config: &ffuf.Config{
			AutoCalibrationKeyword: "FUZZ",
			MatcherManager:         mm,
		},
		Output: NewNullOutput(),
	}
	return j, mm
}

func calibResponse(fuzz string, length, words, lines int64) ffuf.Response {
	return ffuf.Response{
		ContentLength: length,
		ContentWords:  words,
		ContentLines:  lines,
		Request: &ffuf.Request{
			Url:   "http://example.com/FUZZ",
			Host:  "example.com",
			Input: map[string][]byte{"FUZZ": []byte(fuzz)},
		},
	}
}

func TestCalibrateFilters_SizeMatch(t *testing.T) {
	j, mm := calibJob()
	responses := []ffuf.Response{
		calibResponse("aaaa", 512, 80, 20),
		calibResponse("bbbbbbbb", 512, 90, 25),
	}
	if err := j.calibrateFilters(responses, false); err != nil {
		t.Fatalf("calibrateFilters: %v", err)
	}
	f := mm.GetFilters()["size"]
	if f == nil {
		t.Fatalf("expected a \"size\" filter to be registered")
	}
	if f.Repr() != "512" {
		t.Errorf("expected size filter value \"512\", got %q", f.Repr())
	}
}

func TestCalibrateFilters_WordsMatchWhenSizeVaries(t *testing.T) {
	j, mm := calibJob()
	responses := []ffuf.Response{
		calibResponse("aaaa", 500, 42, 20),
		calibResponse("bbbbbbbb", 504, 42, 25),
	}
	if err := j.calibrateFilters(responses, false); err != nil {
		t.Fatalf("calibrateFilters: %v", err)
	}
	f := mm.GetFilters()["word"]
	if f == nil {
		t.Fatalf("expected a \"word\" filter to be registered")
	}
	if f.Repr() != "42" {
		t.Errorf("expected word filter value \"42\", got %q", f.Repr())
	}
}

func TestCalibrateFilters_LinesMatchWhenSizeAndWordsVary(t *testing.T) {
	j, mm := calibJob()
	responses := []ffuf.Response{
		calibResponse("aaaa", 500, 40, 9),
		calibResponse("bbbbbbbb", 504, 44, 9),
	}
	if err := j.calibrateFilters(responses, false); err != nil {
		t.Fatalf("calibrateFilters: %v", err)
	}
	f := mm.GetFilters()["line"]
	if f == nil {
		t.Fatalf("expected a \"line\" filter to be registered")
	}
	if f.Repr() != "9" {
		t.Errorf("expected line filter value \"9\", got %q", f.Repr())
	}
}

// TestCalibrateFilters_DynamicSizeWhenReflected covers the motivating case for
// this change: a custom 404 page that echoes the fuzzed value back into the
// body, so raw size, word count and line count all vary between calibration
// samples even though the page is always "not found". Before this change,
// calibrateFilters gave up here with "No common filtering values found" and
// installed no filter at all, so this class of page flooded results with
// false positives.
func TestCalibrateFilters_DynamicSizeWhenReflected(t *testing.T) {
	j, mm := calibJob()
	// Simulated body: "Cannot find /" + fuzzvalue -> ContentLength = 13 + len(fuzzvalue).
	// Word/line counts are made to disagree between samples so the test can't
	// accidentally pass via the word/line branches instead.
	responses := []ffuf.Response{
		calibResponse("aaaa", 17, 3, 1),
		calibResponse("bbbbbbbbbbbbbbbb", 29, 5, 2),
	}
	if err := j.calibrateFilters(responses, false); err != nil {
		t.Fatalf("calibrateFilters: %v", err)
	}
	f := mm.GetFilters()["dynamicsize"]
	if f == nil {
		t.Fatalf("expected a \"dynamicsize\" filter to be registered, got filters: %v", mm.GetFilters())
	}
	if f.Repr() != "13:FUZZ" {
		t.Errorf("expected dynamicsize filter value \"13:FUZZ\", got %q", f.Repr())
	}

	// The registered filter must generalize to a fuzz value that was never
	// part of calibration.
	live := calibResponse("totally-new-guess", 30, 4, 1) // len("totally-new-guess") == 17
	match, err := f.Filter(&live)
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if !match {
		t.Errorf("expected the dynamicsize filter to match a live reflected 404 for an unseen fuzz value")
	}

	// And it must NOT match a response whose size doesn't fit the formula,
	// i.e. an actual hit.
	realHit := calibResponse("admin", 4096, 400, 120)
	match, err = f.Filter(&realHit)
	if err != nil {
		t.Fatalf("Filter: %v", err)
	}
	if match {
		t.Errorf("dynamicsize filter incorrectly matched a response that does not fit the reflected-size formula")
	}
}

func TestCalibrateFilters_NoCommonValue(t *testing.T) {
	j, mm := calibJob()
	// Sizes, words and lines all differ, and the size difference doesn't
	// track the fuzz value's length either (both fuzz values are 4 bytes).
	responses := []ffuf.Response{
		calibResponse("aaaa", 100, 10, 2),
		calibResponse("bbbb", 200, 20, 4),
	}
	err := j.calibrateFilters(responses, false)
	if err == nil {
		t.Fatalf("expected an error when no dimension is consistent, got nil")
	}
	if len(mm.GetFilters()) != 0 {
		t.Errorf("expected no filters to be registered, got %v", mm.GetFilters())
	}
}

func TestCalibrateFilters_PerHost(t *testing.T) {
	j, mm := calibJob()
	responses := []ffuf.Response{
		calibResponse("aaaa", 512, 80, 20),
		calibResponse("bbbbbbbb", 512, 90, 25),
	}
	if err := j.calibrateFilters(responses, true); err != nil {
		t.Fatalf("calibrateFilters: %v", err)
	}
	domainFilters := mm.FiltersForDomain("example.com")
	if domainFilters["size"] == nil {
		t.Fatalf("expected a per-domain \"size\" filter for example.com")
	}
	if len(mm.GetFilters()) != 0 {
		t.Errorf("expected no global filters to be registered in per-host mode, got %v", mm.GetFilters())
	}
}

func TestCalibrateFilters_SkipsWhenAlreadyFiltered(t *testing.T) {
	j, mm := calibJob()
	// Pre-install a size filter that already matches the calibration baseline.
	if err := mm.AddFilter("size", "512", false); err != nil {
		t.Fatalf("AddFilter: %v", err)
	}
	responses := []ffuf.Response{
		calibResponse("aaaa", 512, 80, 20),
		calibResponse("bbbbbbbb", 512, 90, 25),
	}
	if err := j.calibrateFilters(responses, false); err != nil {
		t.Fatalf("calibrateFilters: %v", err)
	}
	if len(mm.GetFilters()) != 1 {
		t.Errorf("expected the pre-existing filter to be left alone, got %v", mm.GetFilters())
	}
}
