package markov

import (
	"math"
	"sort"
	"time"
)

// InputProvider interface to avoid circular import
type InputProvider interface {
	Value() map[string][]byte
	Position() int
	Next() bool
	Total() int
	Reset() 
	ActivateKeywords(keywords []string)
	Keywords() []string
}

// Response struct to avoid circular import
type Response struct {
	StatusCode     int64
	ContentLength  int64
	ContentWords   int64
	ContentLines   int64
	Duration       time.Duration
}

// FeedbackController manages the Markov Chain feedback mechanism for fuzzing
type FeedbackController struct {
	chain           *MarkovChain
	matchedInputs   []map[string][]byte  // Store inputs that led to matches
	responseHistory []State              // Store recent states for analysis
	maxHistory      int                  // Maximum number of states to keep in history
}

// NewFeedbackController creates a new feedback controller with Markov Chain
func NewFeedbackController() *FeedbackController {
	return &FeedbackController{
		chain:         NewMarkovChain(),
		matchedInputs: make([]map[string][]byte, 0),
		responseHistory: make([]State, 0),
		maxHistory:    100, // Keep last 100 responses in history
	}
}

// UpdateWithResponse updates the Markov chain with a new response
func (fc *FeedbackController) UpdateWithResponse(resp Response, isMatch bool) {
	// Create state from response
	state := State{
		StatusCode:    int64(resp.StatusCode),
		ContentLength: resp.ContentLength,
		ContentWords:  resp.ContentWords,
		ContentLines:  resp.ContentLines,
		Duration:      resp.Duration,
		IsMatch:       isMatch,
	}
	
	// Update the Markov chain
	fc.chain.UpdateChain(state)
	
	// Add to response history
	fc.responseHistory = append(fc.responseHistory, state)
	if len(fc.responseHistory) > fc.maxHistory {
		fc.responseHistory = fc.responseHistory[1:]
	}
}

// UpdateWithMatchedInput stores inputs that led to matches
func (fc *FeedbackController) UpdateWithMatchedInput(input map[string][]byte) {
	fc.matchedInputs = append(fc.matchedInputs, input)
	if len(fc.matchedInputs) > 100 { // Limit to 100 matched inputs
		fc.matchedInputs = fc.matchedInputs[1:]
	}
}

// GetNextInput generates the next input based on Markov Chain feedback
func (fc *FeedbackController) GetNextInput(baseInputProvider ffuf.InputProvider) (map[string][]byte, bool) {
	// If we have matched inputs and they are more valuable based on Markov analysis,
	// use them with higher probability
	if len(fc.matchedInputs) > 0 && fc.shouldUseMatchedInput() {
		// Get a base input from the provider to use as reference
		originalInput := fc.getBaseInput(baseInputProvider)
		modifiedInput := fc.selectAndModifyInput(originalInput)
		return modifiedInput, true
	}
	
	// Otherwise, continue with normal input provider but adjust based on Markov feedback
	return nil, false
}

// getBaseInput gets a base input from the provider without advancing it
func (fc *FeedbackController) getBaseInput(baseInputProvider ffuf.InputProvider) map[string][]byte {
	// Create a temporary copy that doesn't affect the original position
	// In a more sophisticated implementation, we would need to handle this differently
	// For now, we'll use the current value
	return baseInputProvider.Value()
}

// selectAndModifyInput selects an input from matched inputs and potentially modifies it
func (fc *FeedbackController) selectAndModifyInput(baseInput map[string][]byte) map[string][]byte {
	selectedInput := fc.selectFromMatchedInputs()
	if selectedInput == nil {
		return baseInput
	}
	
	// In a more advanced implementation, we might blend the selected input with the base
	// For now, we'll just return the selected input
	return selectedInput
}

// shouldUseMatchedInput determines if we should use a previously matched input
func (fc *FeedbackController) shouldUseMatchedInput() bool {
	// If we have a high probability transition to a match state, 
	// increase the chance of using matched inputs
	if len(fc.responseHistory) < 2 {
		return len(fc.matchedInputs) > 0
	}
	
	// Look at the probability of transitioning to a match state
	previousState := fc.responseHistory[len(fc.responseHistory)-1]
	matchProb := fc.getProbabilityOfMatchTransition(previousState)
	
	// Use matched inputs more frequently when Markov chain suggests a high probability of matches
	return fc.chain.rand.Float64() < (matchProb * 0.8) // Scale probability by 0.8 to make it more conservative
}

// getProbabilityOfMatchTransition calculates the probability of transitioning to a matching state
func (fc *FeedbackController) getProbabilityOfMatchTransition(fromState State) float64 {
	prob := 0.0
	
	// Find all possible next states that are matches
	for _, historyState := range fc.responseHistory {
		if historyState.IsMatch {
			transitionProb := fc.chain.GetTransitionProbability(fromState, historyState)
			prob += transitionProb
		}
	}
	
	return prob
}

// selectFromMatchedInputs selects an input from previously matched inputs
// based on some heuristic (e.g., recency, similarity to current context)
func (fc *FeedbackController) selectFromMatchedInputs() map[string][]byte {
	if len(fc.matchedInputs) == 0 {
		return nil
	}
	
	// For now, select randomly with bias toward more recent matches
	// In a more advanced implementation, we could use similarity measures
	weightedIndex := fc.getWeightedIndex(len(fc.matchedInputs))
	
	return fc.matchedInputs[weightedIndex]
}

// getWeightedIndex returns an index with bias toward the end of the slice
func (fc *FeedbackController) getWeightedIndex(length int) int {
	if length <= 0 {
		return 0
	}
	
	// Use a power function to bias toward more recent items (higher indices)
	randVal := math.Pow(fc.chain.rand.Float64(), 2) // Square to bias toward higher values
	return int(randVal * float64(length-1))
}

// AnalyzeResponsePatterns analyzes patterns in responses to identify interesting behaviors
func (fc *FeedbackController) AnalyzeResponsePatterns() map[string]interface{} {
	analysis := make(map[string]interface{})
	
	if len(fc.responseHistory) == 0 {
		return analysis
	}
	
	// Calculate statistics
	var totalStatusCodes, totalContentLength, totalContentWords, totalContentLines int64
	var totalDuration time.Duration
	var matchCount int
	
	statusCodes := make(map[int64]int)
	
	for _, state := range fc.responseHistory {
		totalStatusCodes += state.StatusCode
		totalContentLength += state.ContentLength
		totalContentWords += state.ContentWords
		totalContentLines += state.ContentLines
		totalDuration += state.Duration
		
		statusCodes[state.StatusCode]++
		
		if state.IsMatch {
			matchCount++
		}
	}
	
	avgStatusCode := float64(totalStatusCodes) / float64(len(fc.responseHistory))
	avgContentLength := float64(totalContentLength) / float64(len(fc.responseHistory))
	avgContentWords := float64(totalContentWords) / float64(len(fc.responseHistory))
	avgContentLines := float64(totalContentLines) / float64(len(fc.responseHistory))
	avgDuration := float64(totalDuration.Nanoseconds()) / float64(len(fc.responseHistory))
	
	// Get most common status codes
	type statusCodeCount struct {
		Code  int64
		Count int
	}
	
	statusCodeList := make([]statusCodeCount, 0)
	for code, count := range statusCodes {
		statusCodeList = append(statusCodeList, statusCodeCount{Code: code, Count: count})
	}
	
	// Sort by count descending
	sort.Slice(statusCodeList, func(i, j int) bool {
		return statusCodeList[i].Count > statusCodeList[j].Count
	})
	
	analysis["avg_status_code"] = avgStatusCode
	analysis["avg_content_length"] = avgContentLength
	analysis["avg_content_words"] = avgContentWords
	analysis["avg_content_lines"] = avgContentLines
	analysis["avg_duration_ns"] = avgDuration
	analysis["match_rate"] = float64(matchCount) / float64(len(fc.responseHistory))
	analysis["total_responses"] = len(fc.responseHistory)
	analysis["total_matches"] = matchCount
	analysis["top_status_codes"] = statusCodeList[:int(math.Min(5, float64(len(statusCodeList))))]
	
	return analysis
}