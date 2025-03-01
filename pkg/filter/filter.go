package filter

import (
	"fmt"
	"sync"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// MatcherManager handles both filters and matchers.
type MatcherManager struct {
	IsCalibrated     bool
	Mutex            sync.Mutex
	Matchers         map[string]ffuf.FilterProvider
	Filters          map[string]ffuf.FilterProvider
	PerDomainFilters map[string]*PerDomainFilter
}

type PerDomainFilter struct {
	IsCalibrated bool
	Filters      map[string]ffuf.FilterProvider
}

func NewPerDomainFilter(globfilters map[string]ffuf.FilterProvider) *PerDomainFilter {
	return &PerDomainFilter{IsCalibrated: false, Filters: globfilters}
}

func (p *PerDomainFilter) SetCalibrated(value bool) {
	p.IsCalibrated = value
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
	f.IsCalibrated = value
}

func (f *MatcherManager) SetCalibratedForHost(host string, value bool) {
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
	f.Mutex.Lock()
	defer f.Mutex.Unlock()
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
	f.Mutex.Lock()
	defer f.Mutex.Unlock()
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
	f.Mutex.Lock()
	defer f.Mutex.Unlock()
	delete(f.Filters, name)
}

// AddMatcher adds a new matcher to Config
func (f *MatcherManager) AddMatcher(name string, option string) error {
	f.Mutex.Lock()
	defer f.Mutex.Unlock()
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
	return f.Filters
}

func (f *MatcherManager) GetMatchers() map[string]ffuf.FilterProvider {
	return f.Matchers
}

func (f *MatcherManager) FiltersForDomain(domain string) map[string]ffuf.FilterProvider {
	if f.PerDomainFilters[domain] == nil {
		return f.Filters
	}
	return f.PerDomainFilters[domain].Filters
}

func (f *MatcherManager) CalibratedForDomain(domain string) bool {
	if f.PerDomainFilters[domain] != nil {
		return f.PerDomainFilters[domain].IsCalibrated
	}
	return false
}

func (f *MatcherManager) Calibrated() bool {
	return f.IsCalibrated
}
