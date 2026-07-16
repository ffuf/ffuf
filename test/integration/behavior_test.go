package integration

import (
	"reflect"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/ffuf/ffuf/v2/pkg/testtarget"
)

// assertSet fails unless got equals want as a set (both are pre-sorted by
// runScan / the literal). Result order is nondeterministic, so this is the only
// correct comparison.
func assertSet(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("matched set = %v, want %v", got, want)
	}
}

func TestStatusMatcher(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	got := runScan(t, target.URL+"/status/FUZZ",
		[]string{"200", "301", "404", "500"},
		nil,
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200,301") },
	)
	assertSet(t, got, []string{"200", "301"})
}

func TestSizeFilter(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	got := runScan(t, target.URL+"/size/FUZZ",
		[]string{"10", "20", "30"},
		nil,
		func(mm ffuf.MatcherManager) {
			mustMatch(t, mm, "status", "all")
			mustFilter(t, mm, "size", "20")
		},
	)
	assertSet(t, got, []string{"10", "30"})
}

func TestWordsFilter(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// /words/5 -> "w w w w w" -> 5 words; filtering words=5 drops it.
	got := runScan(t, target.URL+"/words/FUZZ",
		[]string{"1", "5", "10"},
		nil,
		func(mm ffuf.MatcherManager) {
			mustMatch(t, mm, "status", "all")
			mustFilter(t, mm, "word", "5")
		},
	)
	assertSet(t, got, []string{"1", "10"})
}

func TestLinesFilter(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// /lines/3 -> "L\nL\nL\n" -> 3 lines; filtering lines=3 drops it.
	got := runScan(t, target.URL+"/lines/FUZZ",
		[]string{"1", "3", "5"},
		nil,
		func(mm ffuf.MatcherManager) {
			mustMatch(t, mm, "status", "all")
			mustFilter(t, mm, "line", "3")
		},
	)
	assertSet(t, got, []string{"1", "5"})
}

func TestRegexpMatcher(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// Body is "reflected: <val>", so only the alpha request's body contains "alpha".
	got := runScan(t, target.URL+"/reflect/FUZZ",
		[]string{"alpha", "beta"},
		nil,
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "regexp", "alpha") },
	)
	assertSet(t, got, []string{"alpha"})
}

func TestAutocalibration(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// The /ac/ tree returns a constant soft-404 body for everything except
	// /ac/real. Autocalibration should learn the junk baseline and filter it,
	// leaving only the genuinely different "real" response.
	got := runScan(t, target.URL+"/ac/FUZZ",
		[]string{"junk1", "junk2", "real"},
		func(o *ffuf.ConfigOptions) {
			o.General.AutoCalibration = true
			// Single-threaded so calibration installs the junk-size filter before
			// any wordlist request is matched. With concurrency, requests race the
			// calibration and the result is nondeterministic.
			o.General.Threads = 1
		},
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "all") },
	)
	assertSet(t, got, []string{"real"})
}

func TestRecursion(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// /admin matches (200); greedy recursion queues /admin/FUZZ, where /admin/secret
	// matches. Without recursion, "secret" (i.e. /secret) is a 404 and never matches,
	// so this test fails the moment recursion breaks.
	got := runScan(t, target.URL+"/FUZZ",
		[]string{"admin", "secret", "missing"},
		func(o *ffuf.ConfigOptions) {
			o.HTTP.Recursion = true
			o.HTTP.RecursionStrategy = "greedy"
			o.HTTP.RecursionDepth = 1
		},
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200") },
	)
	assertSet(t, got, []string{"admin", "secret"})
}

func TestRequestRecorder(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	_ = runScan(t, target.URL+"/status/FUZZ",
		[]string{"200"},
		nil,
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200") },
	)

	reqs := target.Requests()
	if len(reqs) == 0 {
		t.Fatal("recorder captured no requests")
	}
	if reqs[0].Method != "GET" {
		t.Errorf("recorded method = %q, want GET", reqs[0].Method)
	}
}

func mustMatch(t *testing.T, mm ffuf.MatcherManager, name, value string) {
	t.Helper()
	if err := mm.AddMatcher(name, value); err != nil {
		t.Fatalf("AddMatcher(%q,%q): %v", name, value, err)
	}
}

func mustFilter(t *testing.T, mm ffuf.MatcherManager, name, value string) {
	t.Helper()
	if err := mm.AddFilter(name, value, false); err != nil {
		t.Fatalf("AddFilter(%q,%q): %v", name, value, err)
	}
}
