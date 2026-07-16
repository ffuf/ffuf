package integration

import (
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/ffuf/ffuf/v2/pkg/testtarget"
)

func TestInputModeSniper(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// Sniper fuzzes one marked position at a time, holding the others at their
	// literal text: /reflect/§a§/§b§ with [x,y] hits /reflect/{x,y}/b (first
	// position) and /reflect/a/{x,y} (second position).
	runScan(t, target.URL+"/reflect/§a§/§b§",
		[]string{"x", "y"},
		func(o *ffuf.ConfigOptions) { o.Input.InputMode = "sniper" },
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "all") },
	)

	assertContainsAll(t, recordedPaths(target.Requests()),
		[]string{"/reflect/x/b", "/reflect/y/b", "/reflect/a/x", "/reflect/a/y"})
}

func TestMatcherModeAnd(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// Two matchers in "and" mode: a response must be 200 AND contain "keep".
	// Every /reflect is 200, so the regexp is the discriminator.
	got := runScan(t, target.URL+"/reflect/FUZZ",
		[]string{"keep", "drop"},
		func(o *ffuf.ConfigOptions) { o.Matcher.Mode = "and" },
		func(mm ffuf.MatcherManager) {
			mustMatch(t, mm, "status", "200")
			mustMatch(t, mm, "regexp", "keep")
		},
	)
	assertSet(t, got, []string{"keep"})
}

func TestFilterModeAnd(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	// Two filters in "and" mode: a response is dropped only if it is 200 AND
	// contains "secret". Every /reflect is 200, so only /reflect/secret is
	// dropped; with the default "or" mode the 200 filter would drop everything.
	got := runScan(t, target.URL+"/reflect/FUZZ",
		[]string{"secret", "public"},
		func(o *ffuf.ConfigOptions) { o.Filter.Mode = "and" },
		func(mm ffuf.MatcherManager) {
			mustMatch(t, mm, "status", "all")
			mustFilter(t, mm, "status", "200")
			mustFilter(t, mm, "regexp", "secret")
		},
	)
	assertSet(t, got, []string{"public"})
}
