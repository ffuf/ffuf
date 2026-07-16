//go:build timing

// These tests make wall-clock assertions (response-time matching, rate limiting),
// which are inherently noisy on shared CI runners. They live behind the `timing`
// build tag so they never gate the main suite; run them with:
//
//	go test -tags=timing ./test/integration/...
package integration

import (
	"fmt"
	"testing"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/ffuf/ffuf/v2/pkg/testtarget"
)

// TestTimeMatcher: /sleep/1000 responds ~1s, /sleep/1 near-instant, so a
// "time > 500" matcher keeps only the slow one. The 500ms gap is wide enough to
// stay stable even on a loaded CI runner.
func TestTimeMatcher(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	got := runScan(t, target.URL+"/sleep/FUZZ",
		[]string{"1", "1000"},
		nil,
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "time", ">500") },
	)
	assertSet(t, got, []string{"1000"})
}

// TestRateLimit: N requests at -rate R should take at least ~N/R seconds. A wide
// lower bound absorbs runner noise while still catching a throttle that stops
// throttling entirely.
func TestRateLimit(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	words := make([]string, 30)
	for i := range words {
		words[i] = fmt.Sprintf("w%d", i)
	}

	start := time.Now()
	runScan(t, target.URL+"/status/FUZZ",
		words,
		func(o *ffuf.ConfigOptions) { o.General.Rate = 20 }, // 20 req/s
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "all") },
	)
	elapsed := time.Since(start)

	// 30 requests at 20/s is ~1.5s; require at least 800ms to allow for noise.
	if elapsed < 800*time.Millisecond {
		t.Errorf("rate limiting too fast: 30 requests at -rate 20 took %v, want >= ~800ms", elapsed)
	}
}
