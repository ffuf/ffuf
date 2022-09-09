package filter

import (
	"github.com/ffuf/ffuf/pkg/http"
)

// MatcherManager provides functions for managing matchers and filters
type MatcherManagerIface interface {
	SetCalibrated(calibrated bool)
	SetCalibratedForHost(host string, calibrated bool)
	AddFilter(name string, option string, replace bool) error
	AddPerDomainFilter(domain string, name string, option string) error
	RemoveFilter(name string)
	AddMatcher(name string, option string) error
	GetFilters() map[string]FilterProvider
	GetMatchers() map[string]FilterProvider
	FiltersForDomain(domain string) map[string]FilterProvider
	CalibratedForDomain(domain string) bool
	Calibrated() bool
}

// FilterProvider is a generic interface for both Matchers and Filters
type FilterProvider interface {
	Filter(response *http.Response) (bool, error)
	Repr() string
	ReprVerbose() string
}
