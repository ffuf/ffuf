package filter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// dynamicSizeEntry pairs a fixed byte offset with the fuzz keyword whose
// per-request value length must be added to that offset to reproduce a
// response's expected Content-Length.
type dynamicSizeEntry struct {
	Offset  int64
	Keyword string
}

// DynamicSizeFilter filters responses whose Content-Length equals
// Offset + len(request.Input[Keyword]) for one of its entries.
//
// The existing -fs/-fw/-fl filters compare a response against a single fixed
// number, so they cannot express a page whose size legitimately depends on
// how long the fuzzed value is. That is a common source of false positives:
// a "custom 404" page that echoes the requested path back into the body
// (e.g. "Cannot find /adminXXXXXXXXXXXXXXXX") has a Content-Length that
// changes with every guess, so a static size/word/line filter never
// stabilizes even though every one of those responses is functionally a
// not-found. Auto-calibration derives entries for this filter by observing
// that ContentLength scales linearly with the calibration keyword's length
// (see engine.calibrateFilters); the filter itself just replays that
// relationship per-request.
type DynamicSizeFilter struct {
	Entries []dynamicSizeEntry
}

// NewDynamicSizeFilter parses a comma-separated list of "offset:keyword"
// pairs, e.g. "173:FUZZ" or "173:FUZZ,42:FUZZ2".
func NewDynamicSizeFilter(value string) (ffuf.FilterProvider, error) {
	var entries []dynamicSizeEntry
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.SplitN(part, ":", 2)
		if len(fields) != 2 || fields[1] == "" {
			return &DynamicSizeFilter{}, fmt.Errorf("dynamic size filter: invalid value %q, expected \"offset:keyword\"", part)
		}
		offset, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return &DynamicSizeFilter{}, fmt.Errorf("dynamic size filter: invalid offset in %q: %s", part, err)
		}
		entries = append(entries, dynamicSizeEntry{Offset: offset, Keyword: fields[1]})
	}
	if len(entries) == 0 {
		return &DynamicSizeFilter{}, fmt.Errorf("dynamic size filter: no valid entries in value %q", value)
	}
	return &DynamicSizeFilter{Entries: entries}, nil
}

func (f *DynamicSizeFilter) Filter(response *ffuf.Response) (bool, error) {
	if response.Request == nil {
		return false, nil
	}
	for _, e := range f.Entries {
		fuzzval, ok := response.Request.Input[e.Keyword]
		if !ok {
			continue
		}
		if response.ContentLength == e.Offset+int64(len(fuzzval)) {
			return true, nil
		}
	}
	return false, nil
}

func (f *DynamicSizeFilter) Repr() string {
	parts := make([]string, 0, len(f.Entries))
	for _, e := range f.Entries {
		parts = append(parts, fmt.Sprintf("%d:%s", e.Offset, e.Keyword))
	}
	return strings.Join(parts, ",")
}

func (f *DynamicSizeFilter) ReprVerbose() string {
	return fmt.Sprintf("Dynamic response size (offset:keyword): %s", f.Repr())
}
