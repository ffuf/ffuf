//go:build timing

package engine

import (
	"context"
	"testing"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// TestRunTask_RetryHonorsDelay verifies that the configured -p delay is applied
// between a failed request attempt and its retry. It is timing-tagged because it
// makes a wall-clock assertion.
func TestRunTask_RetryHonorsDelay(t *testing.T) {
	opts := ffuf.NewConfigOptions()
	opts.HTTP.URL = "http://example.test/FUZZ"
	opts.HTTP.Retries = 1
	opts.Input.Wordlists = []string{"/tmp/words.txt"}
	opts.General.Delay = "0.1"
	conf, err := ffuf.ConfigFromOptions(opts, context.Background(), func() {})
	if err != nil {
		t.Fatalf("ConfigFromOptions: %v", err)
	}
	conf.MatcherManager = &fakeMatcherManager{}
	conf.RateLimitFunc = func() {}
	runner := &retryRunner{failures: 1}
	job := &Job{
		Config: conf,
		Runner: runner,
		Output: NewNullOutput(),
	}

	start := time.Now()
	job.runTask(jobContext{basereq: ffuf.Request{Url: opts.HTTP.URL}}, map[string][]byte{"FUZZ": []byte("value")}, 1)
	elapsed := time.Since(start)

	_, execute := runner.calls()
	if execute != 2 {
		t.Fatalf("execute calls = %d, want 2", execute)
	}
	if elapsed < 80*time.Millisecond {
		t.Fatalf("retry delay too short: got %v, want at least 80ms", elapsed)
	}
}
