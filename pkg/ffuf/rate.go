package ffuf

import (
	"container/ring"
	"sync"
	"time"
)

type RateThrottle struct {
	rateCounter       *ring.Ring
	RateAdjustment    float64
	RateAdjustmentPos int
	Config            *Config
	RateMutex         sync.Mutex
	lastAdjustment    time.Time
}

func NewRateThrottle(conf *Config) *RateThrottle {
	return &RateThrottle{
		rateCounter:       ring.New(conf.Threads),
		RateAdjustment:    0,
		RateAdjustmentPos: 0,
		Config:            conf,
		lastAdjustment:    time.Now(),
	}
}

//CurrentRate calculates requests/second value from circular list of rate
func (r *RateThrottle) CurrentRate() int64 {
	n := r.rateCounter.Len()
	var total int64
	total = 0
	r.rateCounter.Do(func(r interface{}) {
		switch val := r.(type) {
		case int64:
			total += val
		default:
			// circular list entry was nil, happens when < number_of_threads responses have been recorded.
			// the total number of entries is less than length of the list
			n -= 1
		}
	})
	if total > 0 {
		avg := total / int64(n)
		return time.Second.Nanoseconds() * int64(r.Config.Threads) / avg
	}

	return 0
}

//rateTick adds a new duration measurement tick to rate counter
func (r *RateThrottle) Tick(start, end time.Time) {
	if start.Before(r.lastAdjustment) {
		// We don't want to store data for threads started pre-adjustment
		return
	}
	r.RateMutex.Lock()
	defer r.RateMutex.Unlock()
	dur := end.Sub(start).Nanoseconds()
	r.rateCounter = r.rateCounter.Next()
	r.RateAdjustmentPos += 1
	r.rateCounter.Value = dur
}

func (r *RateThrottle) Throttle() {
	if r.Config.Rate == 0 {
		// No throttling
		return
	}
	if r.RateAdjustment > 0.0 {
		delayNS := float64(time.Second.Nanoseconds()) * r.RateAdjustment
		time.Sleep(time.Nanosecond * time.Duration(delayNS))
	}
}

//Adjust changes the RateAdjustment value, which is multiplier of second to pause between requests in a thread
func (r *RateThrottle) Adjust() {
	if r.RateAdjustmentPos < r.Config.Threads {
		// Do not adjust if we don't have enough data yet
		return
	}
	r.RateMutex.Lock()
	defer r.RateMutex.Unlock()
	currentRate := r.CurrentRate()

	if r.RateAdjustment == 0.0 {
		if currentRate > r.Config.Rate {
			// If we're adjusting the rate for the first time, start at a safe point (0.2sec)
			r.RateAdjustment = 0.2
			return
		} else {
			// NOOP
			return
		}
	}
	difference := float64(currentRate) / float64(r.Config.Rate)
	if r.RateAdjustment < 0.00001 && difference < 0.9 {
		// Reset the rate adjustment as throttling is not relevant at current speed
		r.RateAdjustment = 0.0
	} else {
		r.RateAdjustment = r.RateAdjustment * difference
	}
	// Reset the counters
	r.lastAdjustment = time.Now()
	r.RateAdjustmentPos = 0
}
