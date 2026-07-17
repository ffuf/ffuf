package integration

import (
	"fmt"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/ffuf/ffuf/v2/pkg/testtarget"
)

// TestRecursionConcurrentQueue regresses C2: worker goroutines appended to the
// shared recursion queue with no synchronization. Greedy recursion on
// /reflect/FUZZ makes every one of the (default 40 thread) base matches queue a
// recursion job at nearly the same instant, so many workers append concurrently.
// Run under -race; before the fix the concurrent appends trip the detector and
// can drop a queued job or panic on a torn slice header.
func TestRecursionConcurrentQueue(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	words := make([]string, 25)
	for i := range words {
		words[i] = fmt.Sprintf("d%d", i)
	}

	_ = runScan(t, target.URL+"/reflect/FUZZ",
		words,
		func(o *ffuf.ConfigOptions) {
			o.HTTP.Recursion = true
			o.HTTP.RecursionStrategy = "greedy"
			o.HTTP.RecursionDepth = 1
		},
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "200") },
	)

	// With every base match queuing a depth-1 job that then runs the whole
	// wordlist, the target must see far more than the 25 base requests. This both
	// proves recursion descended and is the workload that exercises the concurrent
	// queue appends under -race.
	if n := target.Count(); n <= len(words) {
		t.Errorf("expected recursion to append and process queued jobs (>%d requests), got %d", len(words), n)
	}
}
