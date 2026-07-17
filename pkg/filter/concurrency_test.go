package filter

import (
	"sync"
	"testing"
	"time"
)

// TestMatcherManager_ConcurrentReadWrite regresses C1: the reader methods took
// no lock while the writers did, so the write-side mutex protected writers only
// from each other. With -ac at default threads a worker ranging GetFilters()
// raced autocalibration's AddFilter, which the Go runtime aborts with
// "concurrent map read and map write". Run under -race; before the fix this
// crashes.
func TestMatcherManager_ConcurrentReadWrite(t *testing.T) {
	mm := NewMatcherManager()
	if err := mm.AddMatcher("status", "all"); err != nil {
		t.Fatalf("AddMatcher: %v", err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// writers: churn the maps like autocalibration and the interactive fc/fs
	// commands do.
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_ = mm.AddFilter("size", "1337", false)
					mm.RemoveFilter("size")
					_ = mm.AddPerDomainFilter("host.example", "word", "9")
					mm.SetCalibratedForHost("host.example", true)
					mm.SetCalibrated(true)
				}
			}
		}()
	}
	// readers: exactly what every worker's isMatch does.
	for r := 0; r < 8; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					for range mm.GetFilters() {
					}
					for range mm.GetMatchers() {
					}
					for range mm.FiltersForDomain("host.example") {
					}
					_ = mm.Calibrated()
					_ = mm.CalibratedForDomain("host.example")
				}
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// TestMatcherManager_PerDomainDoesNotAliasGlobal regresses F1: NewPerDomainFilter
// stored the global filter map by reference, so a per-domain filter write leaked
// into the global filter set (and thus applied to every host).
func TestMatcherManager_PerDomainDoesNotAliasGlobal(t *testing.T) {
	mm := NewMatcherManager()
	if err := mm.AddFilter("size", "100", false); err != nil {
		t.Fatalf("AddFilter global: %v", err)
	}
	if err := mm.AddPerDomainFilter("host.example", "word", "5"); err != nil {
		t.Fatalf("AddPerDomainFilter: %v", err)
	}

	if _, leaked := mm.GetFilters()["word"]; leaked {
		t.Error("per-domain 'word' filter leaked into the global filter set (F1 aliasing)")
	}
	perDomain := mm.FiltersForDomain("host.example")
	if _, ok := perDomain["word"]; !ok {
		t.Error("per-domain filter set should contain its own 'word' filter")
	}
	if _, ok := perDomain["size"]; !ok {
		t.Error("per-domain filter set should inherit the global 'size' filter")
	}
}
