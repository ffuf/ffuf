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
	} else {
		r.rateCounter = ring.New(conf.Threads * 5)
	}
	if conf.Rate > 0 {
		ratemicros := 1000000 / conf.Rate
		r.RateLimiter = time.NewTicker(time.Microsecond * time.Duration(ratemicros))
	} else {
		//Million rps is probably a decent hardcoded upper speedlimit
		r.RateLimiter = time.NewTicker(time.Microsecond * 1)
	}
	return r
}

// CurrentRate calculates requests/second value from circular list of rate
func (r *RateThrottle) CurrentRate() int64 {
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

func (r *RateThrottle) ChangeRate(rate int) {
	ratemicros := 0 // set default to 0, avoids integer divide by 0 error

	if rate != 0 {
		ratemicros = 1000000 / rate
	}

	r.RateLimiter.Stop()
	r.RateLimiter = time.NewTicker(time.Microsecond * time.Duration(ratemicros))
	r.Config.Rate = int64(rate)
	// reset the rate counter
	r.rateCounter = ring.New(rate * 5)
}

// rateTick adds a new duration measurement tick to rate counter
func (r *RateThrottle) Tick(start, end time.Time) {
	r.RateMutex.Lock()
	defer r.RateMutex.Unlock()
	r.rateCounter = r.rateCounter.Next()
	r.rateCounter.Value = end.UnixMicro()
}
