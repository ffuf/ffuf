package filter

import (
	"sync"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

type UniqueSizeFilter struct {
	seenSizes map[int64]bool
	mutex     sync.Mutex
	firstOccurrence map[int64]bool
}

func NewUniqueSizeFilter() ffuf.FilterProvider {
	return &UniqueSizeFilter{
		seenSizes: make(map[int64]bool),
		firstOccurrence: make(map[int64]bool),
	}
}

func (f *UniqueSizeFilter) Filter(response *ffuf.Response) (bool, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	size := response.ContentLength
	
	if !f.seenSizes[size] {
		// First time seeing this size
		f.seenSizes[size] = true
		f.firstOccurrence[size] = true
		return false, nil
	}
	
	// If we've seen this size before, only allow it through if it's the first occurrence
	if f.firstOccurrence[size] {
		f.firstOccurrence[size] = false
		return false, nil
	}
	
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
