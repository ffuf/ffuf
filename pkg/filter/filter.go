package filter

import (
	"fmt"
	"sync"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// MatcherManager handles both filters and matchers.
//
// Every access to the Matchers/Filters/PerDomainFilters maps goes through the
// methods below under mu (an RWMutex). Readers RLock and return a *copy* of the
// map: the request path (Job.isMatch) ranges the returned map after the method
// has returned, so handing back the live map would let a worker iterate it while
// autocalibration or an interactive filter command mutates it on another
// goroutine, which the Go runtime aborts with "concurrent map read and map
// write". Copying the map header (the FilterProvider values are immutable once
// created) keeps the caller's iteration off the shared map.
type MatcherManager struct {
	mu               sync.RWMutex
	IsCalibrated     bool
	Matchers         map[string]ffuf.FilterProvider
	Filters          map[string]ffuf.FilterProvider
	PerDomainFilters map[string]*PerDomainFilter
}

type PerDomainFilter struct {
	IsCalibrated bool
	Filters      map[string]ffuf.FilterProvider
}

// NewPerDomainFilter copies the supplied (global) filter map. Storing the map by
// reference aliased the global Filters, so a per-domain filter write mutated the
// global filter set and leaked into every host.
func NewPerDomainFilter(globfilters map[string]ffuf.FilterProvider) *PerDomainFilter {
	return &PerDomainFilter{IsCalibrated: false, Filters: copyFilterMap(globfilters)}
}

func (p *PerDomainFilter) SetCalibrated(value bool) {
	p.IsCalibrated = value
}

// copyFilterMap returns a shallow copy of a filter map. The FilterProvider values
// are safe to share: they are replaced, never mutated, after creation.
func copyFilterMap(in map[string]ffuf.FilterProvider) map[string]ffuf.FilterProvider {
	out := make(map[string]ffuf.FilterProvider, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func NewMatcherManager() ffuf.MatcherManager {
	return &MatcherManager{
		IsCalibrated:     false,
		Matchers:         make(map[string]ffuf.FilterProvider),
		Filters:          make(map[string]ffuf.FilterProvider),
		PerDomainFilters: make(map[string]*PerDomainFilter),
	}
}

func (f *MatcherManager) SetCalibrated(value bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.IsCalibrated = value
}

func (f *MatcherManager) SetCalibratedForHost(host string, value bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.PerDomainFilters[host] != nil {
		f.PerDomainFilters[host].IsCalibrated = value
	} else {
		newFilter := NewPerDomainFilter(f.Filters)
		newFilter.IsCalibrated = true
		f.PerDomainFilters[host] = newFilter
	}
}

func NewFilterByName(name string, value string) (ffuf.FilterProvider, error) {
	if name == "status" {
		return NewStatusFilter(value)
	}
	if name == "size" {
		return NewSizeFilter(value)
	}
	if name == "word" {
		return NewWordFilter(value)
	}
	if name == "line" {
		return NewLineFilter(value)
	}
	if name == "regexp" {
		return NewRegexpFilter(value)
	}
	if name == "time" {
		return NewTimeFilter(value)
	}
	return nil, fmt.Errorf("Could not create filter with name %s", name)
}

// AddFilter adds a new filter to MatcherManager
func (f *MatcherManager) AddFilter(name string, option string, replace bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	newf, err := NewFilterByName(name, option)
	if err == nil {
		// valid filter create or append
		if f.Filters[name] == nil || replace {
			f.Filters[name] = newf
		} else {
			newoption := f.Filters[name].Repr() + "," + option
			newerf, err := NewFilterByName(name, newoption)
			if err == nil {
				f.Filters[name] = newerf
			}
		}
	}
	return err
}

// AddPerDomainFilter adds a new filter to PerDomainFilter configuration
func (f *MatcherManager) AddPerDomainFilter(domain string, name string, option string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	var pdFilters *PerDomainFilter
	if filter, ok := f.PerDomainFilters[domain]; ok {
		pdFilters = filter
	} else {
		pdFilters = NewPerDomainFilter(f.Filters)
	}
	newf, err := NewFilterByName(name, option)
	if err == nil {
		// valid filter create or append
		if pdFilters.Filters[name] == nil {
			pdFilters.Filters[name] = newf
		} else {
			newoption := pdFilters.Filters[name].Repr() + "," + option
			newerf, err := NewFilterByName(name, newoption)
			if err == nil {
				pdFilters.Filters[name] = newerf
			}
		}
	}
	f.PerDomainFilters[domain] = pdFilters
	return err
}

// RemoveFilter removes a filter of a given type
func (f *MatcherManager) RemoveFilter(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Filters, name)
}

// AddMatcher adds a new matcher to Config
func (f *MatcherManager) AddMatcher(name string, option string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	newf, err := NewFilterByName(name, option)
	if err == nil {
		// valid filter create or append
		if f.Matchers[name] == nil {
			f.Matchers[name] = newf
		} else {
			newoption := f.Matchers[name].Repr() + "," + option
			newerf, err := NewFilterByName(name, newoption)
			if err == nil {
				f.Matchers[name] = newerf
			}
		}
	}
	return err
}

func (f *MatcherManager) GetFilters() map[string]ffuf.FilterProvider {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return copyFilterMap(f.Filters)
}

func (f *MatcherManager) GetMatchers() map[string]ffuf.FilterProvider {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return copyFilterMap(f.Matchers)
}

func (f *MatcherManager) FiltersForDomain(domain string) map[string]ffuf.FilterProvider {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.PerDomainFilters[domain] == nil {
		return copyFilterMap(f.Filters)
	}
	return copyFilterMap(f.PerDomainFilters[domain].Filters)
}

func (f *MatcherManager) CalibratedForDomain(domain string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.PerDomainFilters[domain] != nil {
		return f.PerDomainFilters[domain].IsCalibrated
	}
	return false
}

func (f *MatcherManager) Calibrated() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.IsCalibrated
}
