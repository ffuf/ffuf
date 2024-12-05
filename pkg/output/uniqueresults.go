package output

import (
	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// FilterUniqueResults filters out results with duplicate sizes, keeping only the first occurrence
func FilterUniqueResults(results []ffuf.Result) []ffuf.Result {
	seenSizes := make(map[int64]bool)
	uniqueResults := make([]ffuf.Result, 0)

	for _, result := range results {
		if !seenSizes[result.ContentLength] {
			seenSizes[result.ContentLength] = true
			uniqueResults = append(uniqueResults, result)
		}
	}

	return uniqueResults
}
