package ffuf

import (
	"container/ring"
	"sync"
	"time"
)

type RateThrottle struct {
	rateCounter    *ring.Ring
	Config         *Config
	RateMutex      sync.Mutex
	RateLimiter    *time.Ticker
	lastAdjustment time.Time
}

func NewRateThrottle(conf *Config) *RateThrottle {
	r := &RateThrottle{
		Config:         conf,
		lastAdjustment: time.Now(),
	}

	if conf.Rate > 0 {
		r.rateCounter = ring.New(int(conf.Rate * 5))
		ratemicros := 1000000 / conf.Rate
		if ratemicros < 1 {
			// conf.Rate > 1000000 divides to 0; NewTicker panics on a non-positive
			// interval. Clamp to the 1us floor (~1M req/s).
			ratemicros = 1
		}
		r.RateLimiter = time.NewTicker(time.Microsecond * time.Duration(ratemicros))
	} else {
		r.rateCounter = ring.New(conf.Threads * 5)
		//Million rps is probably a decent hardcoded upper speedlimit
		r.RateLimiter = time.NewTicker(time.Microsecond * 1)
	}
	return r
}

// CurrentRate calculates requests/second value from circular list of rate
func (r *RateThrottle) CurrentRate() int64 {
	r.RateMutex.Lock()
	defer r.RateMutex.Unlock()
	n := r.rateCounter.Len()
	lowest := int64(0)
	highest := int64(0)
	r.rateCounter.Do(func(r interface{}) {
		switch val := r.(type) {
		case int64:
			if lowest == 0 || val < lowest {
				lowest = val
			}
			if val > highest {
				highest = val
			}
		default:
			// circular list entry was nil, happens when < number_of_threads * 5 responses have been recorded.
			// the total number of entries is less than length of the list
			n -= 1
		}
	})

	earliest := time.UnixMicro(lowest)
	latest := time.UnixMicro(highest)
	elapsed := latest.Sub(earliest)
	if n > 0 && elapsed.Milliseconds() > 1 {
		return int64(1000 * int64(n) / elapsed.Milliseconds())
	}
	return 0
}

// CurrentConfiguredRate returns the configured rate under the lock, for callers
// (e.g. the interactive handler) that only need to display it.
func (r *RateThrottle) CurrentConfiguredRate() int64 {
	r.RateMutex.Lock()
	defer r.RateMutex.Unlock()
	return r.Config.Rate
}

func (r *RateThrottle) ChangeRate(rate int) {
	r.RateMutex.Lock()
	defer r.RateMutex.Unlock()

	var period time.Duration
	if rate > 0 {
		period = time.Microsecond * time.Duration(1000000/rate)
		// reset the rate counter
		r.rateCounter = ring.New(rate * 5)
	} else {
		period = time.Microsecond * 1
		// reset the rate counter
		r.rateCounter = ring.New(r.Config.Threads * 5)
	}
	if period <= 0 {
		// rate > 1000000 makes 1000000/rate integer-divide to 0; a non-positive
		// ticker interval panics. Clamp to the 1us floor (~1M req/s).
		period = time.Microsecond
	}

	// Reset re-periodizes the existing ticker rather than replacing it, so the
	// RateLimiter pointer is written once (at construction) and never reassigned.
	// That is what lets the execution loop read RateLimiter.C without a lock.
	r.RateLimiter.Reset(period)

	r.Config.Rate = int64(rate)
}

// rateTick adds a new duration measurement tick to rate counter
func (r *RateThrottle) Tick(start, end time.Time) {
	r.RateMutex.Lock()
	defer r.RateMutex.Unlock()
	r.rateCounter = r.rateCounter.Next()
	r.rateCounter.Value = end.UnixMicro()
}
