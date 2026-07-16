package integration

import (
	"reflect"
	"sort"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/ffuf/ffuf/v2/pkg/testtarget"
)

// assertSet fails unless got equals want as a set. Both sides are sorted here, so
// callers don't have to pre-sort want and result order (nondeterministic under
// concurrency) never matters.
func assertSet(t *testing.T, got, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("matched set = %v, want %v", got, want)
	}
}

// TestStatusMatcherDecoupled uses /map, where the HTTP status is NOT present in
// the payload or body ("ok" -> 200 "mapped alpha", "bad" -> 500 "mapped gamma").
// So a status matcher can be distinguished from a matcher that keyed on the
// payload or body by mistake.
func TestStatusMatcherDecoupled(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	got := runScan(t, target.URL+"/map/FUZZ",
		[]string{"ok", "bad"},
		nil,
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200") },
	)
	assertSet(t, got, []string{"ok"})
}

func TestStatusMatcher(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// Non-redirect codes only: a bare 3xx interacts with the http client's
	// redirect handling in Go-version-dependent ways (a 301 matches on recent Go
	// but not on the 1.18 floor). Status matching is exercised just as well with
	// 200/403/404/500, and the test then holds on every supported Go version.
	got := runScan(t, target.URL+"/status/FUZZ",
		[]string{"200", "403", "404", "500"},
		nil,
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200,403") },
	)
	assertSet(t, got, []string{"200", "403"})
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
			// Custom calibration strings so calibration is hermetic: this path in
			// autoCalibrationStrings() returns them directly and never reads the
			// strategy JSON files from AUTOCALIBDIR, which a fresh CI checkout does
			// not have (without them, calibration silently installs no filter).
			// Both probe /ac/<string> and return the junk baseline.
			o.General.AutoCalibrationStrings = []string{"calibrate-one", "calibrate-two"}
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

// TestRecursionDefaultStrategy covers the default (redirect-based) recursion
// strategy, which is a different code path (handleDefaultRecursionJob) from the
// greedy one: /rdir 301-redirects to /rdir/, which ffuf detects as a directory
// and descends into, finding /rdir/found. Asserting "found" matched is the
// version-robust core signal (it is a 404 at the root, so it can only appear via
// recursion); the /rdir 301 itself is not asserted because a bare 3xx is handled
// differently across Go versions.
func TestRecursionDefaultStrategy(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	got := runScan(t, target.URL+"/FUZZ",
		[]string{"rdir", "found"},
		func(o *ffuf.ConfigOptions) {
			o.HTTP.Recursion = true
			o.HTTP.RecursionStrategy = "default"
			o.HTTP.RecursionDepth = 1
		},
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200,301") },
	)
	if !contains(got, "found") {
		t.Errorf("default recursion did not descend into /rdir/ to match 'found'; got %v", got)
	}
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
