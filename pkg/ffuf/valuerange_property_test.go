package ffuf

import (
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"testing/quick"
)

// qcfg fixes the seed so a failure is reproducible (quick.Check with a nil config
// seeds from time.Now, which is unreplayable) and raises the sample count.
var qcfg = &quick.Config{Rand: rand.New(rand.NewSource(1)), MaxCount: 1000}

// The range/value parsers back every numeric matcher and filter (-mc, -fc, -ms,
// -fs, ...). These property tests assert their invariants hold across a wide
// input space via testing/quick (stdlib, no dependency), not just a few examples.

// A "a-b" range with a < b parses to exactly {a, b}; a >= b is an error (the
// regex matches but ValueRangeFromString rejects min >= max).
func TestValueRange_RangeInvariant(t *testing.T) {
	f := func(a, b uint16) bool {
		lo, hi := int64(a), int64(b)
		vr, err := ValueRangeFromString(fmt.Sprintf("%d-%d", lo, hi))
		if lo < hi {
			return err == nil && vr.Min == lo && vr.Max == hi
		}
		return err != nil
	}
	if err := quick.Check(f, qcfg); err != nil {
		t.Error(err)
	}
}

// A single non-negative integer parses to the degenerate range {n, n}.
func TestValueRange_SingleValueInvariant(t *testing.T) {
	f := func(n uint32) bool {
		vr, err := ValueRangeFromString(strconv.FormatUint(uint64(n), 10))
		return err == nil && vr.Min == int64(n) && vr.Max == int64(n)
	}
	if err := quick.Check(f, qcfg); err != nil {
		t.Error(err)
	}
}

// Arbitrary input never panics, and on success the returned range is always
// well-formed (Min <= Max).
func TestValueRange_NeverPanics(t *testing.T) {
	f := func(s string) bool {
		vr, err := ValueRangeFromString(s)
		return err != nil || vr.Min <= vr.Max
	}
	if err := quick.Check(f, qcfg); err != nil {
		t.Error(err)
	}
}

// A single float initializes optRange to a non-range delay whose Min holds the
// parsed value. (Max is intentionally left unset for a single value: sleepIfNeeded
// only reads Min when IsRange is false.)
func TestOptRange_SingleFloatInvariant(t *testing.T) {
	f := func(whole uint16, frac uint8) bool {
		val := fmt.Sprintf("%d.%02d", whole, frac%100)
		want, _ := strconv.ParseFloat(val, 64)
		var o optRange
		if err := o.Initialize(val); err != nil {
			return false
		}
		return o.HasDelay && !o.IsRange && o.Min == want
	}
	if err := quick.Check(f, qcfg); err != nil {
		t.Error(err)
	}
}

// Arbitrary input never panics, and any non-empty value that initializes without
// error must have HasDelay set (the contract of a successfully-parsed delay).
func TestOptRange_NeverPanics(t *testing.T) {
	f := func(s string) bool {
		var o optRange
		err := o.Initialize(s)
		return err != nil || len(s) == 0 || o.HasDelay
	}
	if err := quick.Check(f, qcfg); err != nil {
		t.Error(err)
	}
}
