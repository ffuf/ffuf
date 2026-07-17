package ffuf

import "testing"

// TestNewRateThrottle_HugeRateNoPanic regresses P3: conf.Rate > 1000000 makes
// 1000000/rate integer-divide to 0, and a non-positive ticker interval panics.
func TestNewRateThrottle_HugeRateNoPanic(t *testing.T) {
	_ = NewRateThrottle(&Config{Rate: 2000000, Threads: 40})
	_ = NewRateThrottle(&Config{Rate: 1000001, Threads: 40})
}

// TestChangeRate_HugeRateNoPanic regresses the same on the interactive `rate`
// command path, plus the zero/negative branches.
func TestChangeRate_HugeRateNoPanic(t *testing.T) {
	r := NewRateThrottle(&Config{Rate: 0, Threads: 40})
	r.ChangeRate(2000000) // 1000000/2000000 == 0
	r.ChangeRate(1000001)
	r.ChangeRate(0)
	r.ChangeRate(-5)
	r.ChangeRate(500) // and a normal value still works
}
