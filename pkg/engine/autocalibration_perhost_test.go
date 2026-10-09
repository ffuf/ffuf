package engine

import (
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/ffuf/ffuf/v2/pkg/filter"
)

// responses whose sizes all differ but whose word counts agree, so the
// documented "work down from the most specific common denominator" order
// should settle on a word filter rather than a size filter.
func calibrationResponses() []ffuf.Response {
	req := &ffuf.Request{Url: "http://host.example/path/FUZZ", Host: "host.example"}
	out := make([]ffuf.Response, 0, 3)
	for i, size := range []int64{100, 200, 300} {
		out = append(out, ffuf.Response{
			StatusCode:    200,
			ContentLength: size,
			ContentWords:  42,
			ContentLines:  int64(7 + i),
			Request:       req,
		})
	}
	return out
}

func newCalibrationJob() *Job {
	return &Job{
		Config: &ffuf.Config{MatcherManager: filter.NewMatcherManager()},
		Output: NewNullOutput(),
	}
}

func filterNames(fs map[string]ffuf.FilterProvider) []string {
	names := make([]string, 0, len(fs))
	for name := range fs {
		names = append(names, name)
	}
	return names
}

// The global -ac path collects every probe first and calls calibrateFilters
// once, so it compares the responses against each other.
func TestCalibrateFiltersGlobalUsesWords(t *testing.T) {
	j := newCalibrationJob()

	if err := j.calibrateFilters(calibrationResponses(), false); err != nil {
		t.Fatalf("calibrateFilters: %v", err)
	}

	got := filterNames(j.Config.MatcherManager.GetFilters())
	if len(got) != 1 || got[0] != "word" {
		t.Errorf("global filters = %v, want [word]", got)
	}
}

// The per-host -ach path must reach the same conclusion from the same probes.
func TestCalibrateFiltersPerHostUsesWords(t *testing.T) {
	responses := calibrationResponses()
	host := ffuf.HostURLFromRequest(*responses[0].Request)
	j := newCalibrationJob()

	if err := j.calibrateFilters(responses, true); err != nil {
		t.Fatalf("calibrateFilters: %v", err)
	}

	got := filterNames(j.Config.MatcherManager.FiltersForDomain(host))
	if len(got) != 1 || got[0] != "word" {
		t.Errorf("per-host filters = %v, want [word]", got)
	}
}

// probeRunner hands CalibrateForHost a fixed series of responses, standing in
// for the probe requests a real host would answer. Sizes all differ, word
// counts agree.
type probeRunner struct {
	calls int
	host  string
}

func (r *probeRunner) Prepare(input map[string][]byte, basereq *ffuf.Request) (ffuf.Request, error) {
	req := *basereq
	req.Url = "http://" + r.host + "/path/" + string(input["FUZZ"])
	req.Host = r.host
	req.Input = input
	return req, nil
}

func (r *probeRunner) Execute(req *ffuf.Request) (ffuf.Response, error) {
	r.calls++
	return ffuf.Response{
		StatusCode:    200,
		ContentLength: int64(100 * r.calls),
		ContentWords:  42,
		ContentLines:  int64(7 + r.calls),
		Request:       req,
	}, nil
}

func (r *probeRunner) Dump(req *ffuf.Request) ([]byte, error) { return nil, nil }

// CalibrateForHost has to compare its probes against each other, the way the
// global path does. Calling calibrateFilters once per probe handed it a single
// response at a time, so it only ever installed a size filter.
func TestCalibrateForHostComparesProbesAgainstEachOther(t *testing.T) {
	const host = "host.example"
	mm := filter.NewMatcherManager()
	if err := mm.AddMatcher("status", "200"); err != nil {
		t.Fatalf("AddMatcher: %v", err)
	}
	j := &Job{
		Config: &ffuf.Config{
			Url:                    "http://" + host + "/path/FUZZ",
			MatcherManager:         mm,
			AutoCalibrationKeyword: "FUZZ",
			AutoCalibrationStrings: []string{"aaa", "bbb", "ccc"},
		},
		Output: NewNullOutput(),
		Runner: &probeRunner{host: host},
	}

	if err := j.CalibrateForHost(host, map[string][]byte{"FUZZ": []byte("x")}); err != nil {
		t.Fatalf("CalibrateForHost: %v", err)
	}

	var domain string
	for d := range map[string]bool{host + "/path": true} {
		domain = d
	}
	got := filterNames(j.Config.MatcherManager.FiltersForDomain(domain))
	if len(got) != 1 || got[0] != "word" {
		t.Errorf("filters after CalibrateForHost = %v, want [word]", got)
	}
}
