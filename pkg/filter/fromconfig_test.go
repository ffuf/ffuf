package filter

import (
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// TestFromConfig_DefaultStatusMatcher covers the one behavior main used to drive
// via flag.Visit: whether the default status matcher is installed. It is now
// testable without the flag package.
func TestFromConfig_DefaultStatusMatcher(t *testing.T) {
	opts := ffuf.NewConfigOptions()

	mm, err := FromConfig(opts, true)
	if err != nil {
		t.Fatalf("FromConfig: %v", err)
	}
	if _, ok := mm.GetMatchers()["status"]; !ok {
		t.Error("default status matcher should be installed when addDefaultStatusMatcher=true")
	}

	mm2, err := FromConfig(opts, false)
	if err != nil {
		t.Fatalf("FromConfig: %v", err)
	}
	if _, ok := mm2.GetMatchers()["status"]; ok {
		t.Error("no status matcher should be installed when addDefaultStatusMatcher=false and none configured")
	}
}

// TestFromConfig_FiltersAndMatchers checks that configured filters/matchers are
// installed from the ConfigOptions values.
func TestFromConfig_FiltersAndMatchers(t *testing.T) {
	opts := ffuf.NewConfigOptions()
	opts.Filter.Size = "100"
	opts.Matcher.Regexp = "admin"

	mm, err := FromConfig(opts, false)
	if err != nil {
		t.Fatalf("FromConfig: %v", err)
	}
	if _, ok := mm.GetFilters()["size"]; !ok {
		t.Error("size filter should be installed from opts.Filter.Size")
	}
	if _, ok := mm.GetMatchers()["regexp"]; !ok {
		t.Error("regexp matcher should be installed from opts.Matcher.Regexp")
	}
}
