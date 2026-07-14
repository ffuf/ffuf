package ffuf

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// State represents a state in the Markov chain based on HTTP response characteristics
type mState struct {
	StatusCode    int64
	ContentLength int64
	ContentWords  int64
	ContentLines  int64
	Duration      time.Duration
	IsMatch       bool
}

// String returns a string representation of the state for use as map key
func (s mState) String() string {
	return fmt.Sprintf("%d_%d_%d_%d_%d_%t", 
		s.StatusCode, 
		s.ContentLength, 
		s.ContentWords, 
		s.ContentLines, 
		s.Duration.Milliseconds(), 
		s.IsMatch)
}

// mMarkovChain represents a Markov chain for fuzzing feedback
type mMarkovChain struct {
	// Transition probabilities: current state -> next state -> probability
	Transitions map[string]map[string]float64
	
	// State counts for calculating probabilities
	StateCounts map[string]map[string]int
	
	// Current state of the chain
	CurrentState mState
	
	// History of states visited
	History []mState
	
	// Random generator for state selection
	rand *rand.Rand
	
	// Mutex for thread safety
	mutex sync.RWMutex
}

// NewMarkovChain creates a new Markov chain instance
func newMarkovChain() *mMarkovChain {
	return &mMarkovChain{
		Transitions: make(map[string]map[string]float64),
		StateCounts: make(map[string]map[string]int),
		History:     make([]mState, 0),
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// UpdateTransition updates the transition probability from current state to next state
func (mc *mMarkovChain) UpdateTransition(currentState, nextState mState) {
	currentStr := currentState.String()
	nextStr := nextState.String()
	
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	
	// Initialize maps if they don't exist
	if _, exists := mc.StateCounts[currentStr]; !exists {
		mc.StateCounts[currentStr] = make(map[string]int)
	}
	
	// Increment the count for this transition
	mc.StateCounts[currentStr][nextStr]++
	
	// Update probabilities
	mc.updateProbabilities(currentStr)
}

// updateProbabilities recalculates transition probabilities for a given state
func (mc *mMarkovChain) updateProbabilities(currentState string) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	
	mc.updateProbabilitiesNoLock(currentState)
}

// updateProbabilitiesNoLock recalculates transition probabilities without locking (for internal use)
func (mc *mMarkovChain) updateProbabilitiesNoLock(currentState string) {
	total := 0
	for _, count := range mc.StateCounts[currentState] {
		total += count
	}
	
	if total == 0 {
		return
	}
	
	// Initialize transitions map for this state if needed
	if _, exists := mc.Transitions[currentState]; !exists {
		mc.Transitions[currentState] = make(map[string]float64)
	}
	
	// Calculate probabilities
	for nextState, count := range mc.StateCounts[currentState] {
		mc.Transitions[currentState][nextState] = float64(count) / float64(total)
	}
}

// GetNextState returns the next state based on transition probabilities
func (mc *mMarkovChain) GetNextState() mState {
	currentStr := mc.CurrentState.String()
	
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	
	// If no transitions exist for this state, return the current state
	transitions, exists := mc.Transitions[currentStr]
	if !exists || len(transitions) == 0 {
		return mc.CurrentState
	}
	
	// Generate a random value between 0 and 1
	randVal := mc.rand.Float64()
	
	// Find the next state based on cumulative probabilities
	cumulative := 0.0
	for nextStateStr, prob := range transitions {
		cumulative += prob
		if randVal <= cumulative {
			return mc.stringToState(nextStateStr)
		}
	}
	
	// Fallback: return the first state if something went wrong
	for nextStateStr := range transitions {
		return mc.stringToState(nextStateStr)
	}
	
	return mc.CurrentState
}

// stringToState converts a string representation back to a State
func (mc *mMarkovChain) stringToState(stateStr string) mState {
	parts := strings.Split(stateStr, "_")
	if len(parts) < 6 {
		return mState{}
	}
	
	statusCode, _ := strconv.ParseInt(parts[0], 10, 64)
	contentLength, _ := strconv.ParseInt(parts[1], 10, 64)
	contentWords, _ := strconv.ParseInt(parts[2], 10, 64)
	contentLines, _ := strconv.ParseInt(parts[3], 10, 64)
	durationMs, _ := strconv.ParseInt(parts[4], 10, 64)
	isMatch, _ := strconv.ParseBool(parts[5])
	
	return mState{
		StatusCode:    statusCode,
		ContentLength: contentLength,
		ContentWords:  contentWords,
		ContentLines:  contentLines,
		Duration:      time.Duration(durationMs) * time.Millisecond,
		IsMatch:       isMatch,
	}
}

// UpdateChain updates the Markov chain with a new response state
func (mc *mMarkovChain) UpdateChain(newState mState) {
	mc.mutex.Lock()
	defer mc.mutex.Unlock()
	
	if len(mc.History) > 0 {
		// Update transition from the previous state to the new state
		prevState := mc.History[len(mc.History)-1]
		mc.updateTransitionNoLock(prevState, newState) // Use internal method to avoid double locking
	}
	
	// Update current state and history
	mc.CurrentState = newState
	mc.History = append(mc.History, newState)
}

// updateTransitionNoLock is an internal method that updates transition without locking (used when already locked)
func (mc *mMarkovChain) updateTransitionNoLock(currentState, nextState mState) {
	currentStr := currentState.String()
	nextStr := nextState.String()
	
	// Initialize maps if they don't exist
	if _, exists := mc.StateCounts[currentStr]; !exists {
		mc.StateCounts[currentStr] = make(map[string]int)
	}
	
	// Increment the count for this transition
	mc.StateCounts[currentStr][nextStr]++
	
	// Update probabilities
	mc.updateProbabilitiesNoLock(currentStr)
}

// GetTransitionProbability returns the probability of transitioning from one state to another
func (mc *mMarkovChain) GetTransitionProbability(fromState, toState mState) float64 {
	fromStr := fromState.String()
	toStr := toState.String()
	
	mc.mutex.RLock()
	defer mc.mutex.RUnlock()
	
	transitions, exists := mc.Transitions[fromStr]
	if !exists {
		return 0.0
	}
	
	prob, exists := transitions[toStr]
	if !exists {
		return 0.0
	}
	
	return prob
}

// mFeedbackController manages the Markov Chain feedback mechanism for fuzzing
type mFeedbackController struct {
	chain           *mMarkovChain
	matchedInputs   []map[string][]byte  // Store inputs that led to matches
	responseHistory []mState             // Store recent states for analysis
	maxHistory      int                  // Maximum number of states to keep in history
	verbose         bool                 // Enable verbose output for Markov Chain
}

// NewFeedbackController creates a new feedback controller with Markov Chain
func NewFeedbackController() MarkovFeedback {
	return &mFeedbackController{
		chain:         newMarkovChain(),
		matchedInputs: make([]map[string][]byte, 0),
		responseHistory: make([]mState, 0),
		maxHistory:    100, // Keep last 100 responses in history
		verbose:       false, // Default to no verbose output
	}
}

// SetVerbose enables/disables verbose output for Markov Chain
func (fc *mFeedbackController) SetVerbose(verbose bool) {
	fc.verbose = verbose
}

// GetVerbose returns current verbose setting
func (fc *mFeedbackController) GetVerbose() bool {
	return fc.verbose
}

// PrintMarkovInfo prints Markov Chain parameters and statistics
func (fc *mFeedbackController) PrintMarkovInfo() {
	fc.chain.mutex.RLock()
	defer fc.chain.mutex.RUnlock()
	
	fmt.Printf("\n[MARKOV CHAIN FEEDBACK INFO]\n")
	fmt.Printf("Total States Visited: %d\n", len(fc.responseHistory))
	fmt.Printf("Matched Inputs Stored: %d\n", len(fc.matchedInputs))
	fmt.Printf("Chain History Size: %d/%d\n", len(fc.chain.History), fc.maxHistory)
	fmt.Printf("Total Transitions Tracked: %d\n", len(fc.chain.Transitions))
	fmt.Printf("Current Markov State: StatusCode=%d, ContentLength=%d, IsMatch=%t\n", 
		fc.chain.CurrentState.StatusCode, fc.chain.CurrentState.ContentLength, fc.chain.CurrentState.IsMatch)
	fmt.Printf("[END MARKOV CHAIN FEEDBACK INFO]\n")
}

// UpdateWithResponse updates the Markov chain with a new response
func (fc *mFeedbackController) UpdateWithResponse(resp Response, isMatch bool) {
	// Create state from response
	state := mState{
		StatusCode:    resp.StatusCode,
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
func (fc *mFeedbackController) UpdateWithMatchedInput(input map[string][]byte) {
	fc.matchedInputs = append(fc.matchedInputs, input)
	if len(fc.matchedInputs) > 100 { // Limit to 100 matched inputs
		fc.matchedInputs = fc.matchedInputs[1:]
	}
}

// GetNextInput generates the next input based on Markov Chain feedback
func (fc *mFeedbackController) GetNextInput(baseInputProvider InputProvider) (map[string][]byte, bool) {
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
func (fc *mFeedbackController) getBaseInput(baseInputProvider InputProvider) map[string][]byte {
	// Create a temporary copy that doesn't affect the original position
	// In a more sophisticated implementation, we would need to handle this differently
	// For now, we'll use the current value
	return baseInputProvider.Value()
}

// selectAndModifyInput selects an input from matched inputs and potentially modifies it
func (fc *mFeedbackController) selectAndModifyInput(baseInput map[string][]byte) map[string][]byte {
	selectedInput := fc.selectFromMatchedInputs()
	if selectedInput == nil {
		return baseInput
	}
	
	// In a more advanced implementation, we might blend the selected input with the base
	// For now, we'll just return the selected input
	return selectedInput
}

// shouldUseMatchedInput determines if we should use a previously matched input
func (fc *mFeedbackController) shouldUseMatchedInput() bool {
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
func (fc *mFeedbackController) getProbabilityOfMatchTransition(fromState mState) float64 {
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
func (fc *mFeedbackController) selectFromMatchedInputs() map[string][]byte {
	if len(fc.matchedInputs) == 0 {
		return nil
	}
	
	// For now, select randomly with bias toward more recent matches
	// In a more advanced implementation, we could use similarity measures
	weightedIndex := fc.getWeightedIndex(len(fc.matchedInputs))
	
	return fc.matchedInputs[weightedIndex]
}

// getWeightedIndex returns an index with bias toward the end of the slice
func (fc *mFeedbackController) getWeightedIndex(length int) int {
	if length <= 0 {
		return 0
	}
	
	// Use a power function to bias toward more recent items (higher indices)
	randVal := math.Pow(fc.chain.rand.Float64(), 2) // Square to bias toward higher values
	return int(randVal * float64(length-1))
}

// AnalyzeResponsePatterns analyzes patterns in responses to identify interesting behaviors
func (fc *mFeedbackController) AnalyzeResponsePatterns() map[string]interface{} {
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