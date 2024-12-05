package filter

import (
	"sync"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

type UniqueSizeFilter struct {
	seenSizes map[int64]string // maps size to first URL with that size
	mutex     sync.Mutex
}

func NewUniqueSizeFilter() ffuf.FilterProvider {
	return &UniqueSizeFilter{
		seenSizes: make(map[int64]string),
	}
}

func (f *UniqueSizeFilter) Filter(response *ffuf.Response) (bool, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	size := response.ContentLength
	
	if firstURL, seen := f.seenSizes[size]; !seen {
		// First time seeing this size
		f.seenSizes[size] = response.Request.Url
		return false, nil
	} else if firstURL == response.Request.Url {
		// This is the first URL we saw with this size, keep it
		return false, nil
	}
	
	// Not the first URL with this size, filter it out
	return true, nil
}

func (f *UniqueSizeFilter) Repr() string {
	return "Unique response sizes only"
}

func (f *UniqueSizeFilter) ReprVerbose() string {
	return "Unique response sizes only"
}

func (f *UniqueSizeFilter) MarshalJSON() ([]byte, error) {
	return []byte(`{"type":"uniquesize"}`), nil
}
