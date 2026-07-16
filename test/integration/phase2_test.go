package integration

import (
	"sort"
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/ffuf/ffuf/v2/pkg/testtarget"
)

// anyRequest reports whether any recorded request satisfies pred. Used to assert
// what ffuf actually put on the wire (method, headers, body), not just that a
// response matched.
func anyRequest(reqs []testtarget.Recorded, pred func(testtarget.Recorded) bool) bool {
	for _, r := range reqs {
		if pred(r) {
			return true
		}
	}
	return false
}

// recordedPaths returns the sorted, de-duplicated set of request paths the target
// received. Order is nondeterministic (the engine is concurrent), so callers
// compare as a set.
func recordedPaths(reqs []testtarget.Recorded) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(reqs))
	for _, r := range reqs {
		if !seen[r.Path] {
			seen[r.Path] = true
			out = append(out, r.Path)
		}
	}
	sort.Strings(out)
	return out
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}

func assertContainsAll(t *testing.T, got, want []string) {
	t.Helper()
	for _, w := range want {
		if !contains(got, w) {
			t.Errorf("missing %q in %v", w, got)
		}
	}
}

// --- request options, verified through the request recorder ----------------

func TestRequestHeaderSent(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// /needs-header returns 200 only when X-Test: yes is present, else 403.
	got := runScan(t, target.URL+"/needs-header?p=FUZZ",
		[]string{"h"},
		func(o *ffuf.ConfigOptions) { o.HTTP.Headers = []string{"X-Test: yes"} },
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200") },
	)
	assertSet(t, got, []string{"h"})

	sent := false
	for _, r := range target.Requests() {
		if r.Header.Get("X-Test") == "yes" {
			sent = true
		}
	}
	if !sent {
		t.Error("the -H header was not sent on the wire")
	}
}

func TestRequestHeaderMissingDoesNotMatch(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// Without the header, /needs-header returns 403, so nothing matches.
	got := runScan(t, target.URL+"/needs-header?p=FUZZ",
		[]string{"h"},
		nil,
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200") },
	)
	assertSet(t, got, []string{})
	// De-vacuum: the empty result must be because the request was made and got a
	// 403, not because the engine failed to send anything.
	if target.Count() == 0 {
		t.Fatal("engine sent no request; the empty match set is meaningless")
	}
}

func TestRequestCookieSent(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	got := runScan(t, target.URL+"/needs-cookie?p=FUZZ",
		[]string{"c"},
		func(o *ffuf.ConfigOptions) { o.HTTP.Cookies = []string{"SESSION=abc"} },
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200") },
	)
	assertSet(t, got, []string{"c"})
	if !anyRequest(target.Requests(), func(r testtarget.Recorded) bool {
		return strings.Contains(r.Header.Get("Cookie"), "SESSION=abc")
	}) {
		t.Error("the -b cookie was not sent on the wire")
	}
}

func TestRequestMethod(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	got := runScan(t, target.URL+"/needs-method?p=FUZZ",
		[]string{"m"},
		func(o *ffuf.ConfigOptions) { o.HTTP.Method = "POST" },
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200") },
	)
	assertSet(t, got, []string{"m"})
	if !anyRequest(target.Requests(), func(r testtarget.Recorded) bool { return r.Method == "POST" }) {
		t.Error("the -X method was not sent as POST")
	}
}

func TestRequestBody(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// /needs-body returns 200 only when the body contains "token".
	got := runScan(t, target.URL+"/needs-body?p=FUZZ",
		[]string{"b"},
		func(o *ffuf.ConfigOptions) {
			o.HTTP.Method = "POST"
			o.HTTP.Data = "token=FUZZ"
		},
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200") },
	)
	assertSet(t, got, []string{"b"})
	// Verify FUZZ was actually substituted in the body (not just that "token"
	// from the template survived): the wire body must be "token=b".
	if !anyRequest(target.Requests(), func(r testtarget.Recorded) bool { return r.Body == "token=b" }) {
		t.Error("the -d body was not sent with FUZZ substituted (want \"token=b\")")
	}
}

// --- redirects -------------------------------------------------------------

func TestFollowRedirects(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// /redirect/3 -> /redirect/2 -> /redirect/1 -> /redirect/0 (200). With -r,
	// the final 200 matches; the recorder proves the whole chain was walked.
	got := runScan(t, target.URL+"/redirect/FUZZ",
		[]string{"3"},
		func(o *ffuf.ConfigOptions) { o.HTTP.FollowRedirects = true },
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200") },
	)
	assertSet(t, got, []string{"3"})

	assertContainsAll(t, recordedPaths(target.Requests()),
		[]string{"/redirect/3", "/redirect/2", "/redirect/1", "/redirect/0"})
}

// --- matcher/filter combination --------------------------------------------

func TestFilterExcludesStatuses(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// Match everything, then filter out 404 and 500: only 200 survives.
	got := runScan(t, target.URL+"/status/FUZZ",
		[]string{"200", "404", "500"},
		nil,
		func(mm ffuf.MatcherManager) {
			mustMatch(t, mm, "status", "all")
			mustFilter(t, mm, "status", "404,500")
		},
	)
	assertSet(t, got, []string{"200"})
}

// --- input modes (asserted through the recorder) ---------------------------

func TestInputModeClusterbomb(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	runScan(t, target.URL+"/reflect/KEY1-KEY2",
		nil,
		func(o *ffuf.ConfigOptions) {
			o.Input.InputMode = "clusterbomb"
			o.Input.Wordlists = []string{
				writeWordlist(t, []string{"a", "b"}) + ":KEY1",
				writeWordlist(t, []string{"1", "2"}) + ":KEY2",
			}
		},
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "all") },
	)

	// clusterbomb is the cartesian product: exactly every combination, no more.
	// Exact set catches extra paths; the count catches duplicates (recordedPaths
	// de-duplicates, so a doubled request would slip past a set check alone).
	assertSet(t, recordedPaths(target.Requests()),
		[]string{"/reflect/a-1", "/reflect/a-2", "/reflect/b-1", "/reflect/b-2"})
	if n := target.Count(); n != 4 {
		t.Errorf("clusterbomb 2x2 made %d requests, want exactly 4 (over-production?)", n)
	}
}

func TestInputModePitchfork(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	runScan(t, target.URL+"/reflect/KEY1-KEY2",
		nil,
		func(o *ffuf.ConfigOptions) {
			o.Input.InputMode = "pitchfork"
			o.Input.Wordlists = []string{
				writeWordlist(t, []string{"a", "b"}) + ":KEY1",
				writeWordlist(t, []string{"1", "2"}) + ":KEY2",
			}
		},
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "all") },
	)

	// pitchfork reads the wordlists in lockstep: exactly a-1 and b-2, no cross terms.
	assertSet(t, recordedPaths(target.Requests()), []string{"/reflect/a-1", "/reflect/b-2"})
	if n := target.Count(); n != 2 {
		t.Errorf("pitchfork made %d requests, want exactly 2 (over-production?)", n)
	}
}
