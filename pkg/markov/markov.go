package markov

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

// State represents a state in the Markov chain based on HTTP response characteristics
type State struct {
	StatusCode    int64
	ContentLength int64
	ContentWords  int64
	ContentLines  int64
	Duration      time.Duration
	IsMatch       bool
}

// String returns a string representation of the state for use as map key
func (s State) String() string {
	return fmt.Sprintf("%d_%d_%d_%d_%d_%t", 
		s.StatusCode, 
		s.ContentLength, 
		s.ContentWords, 
		s.ContentLines, 
		s.Duration.Milliseconds(), 
		s.IsMatch)
}

// MarkovChain represents a Markov chain for fuzzing feedback
type MarkovChain struct {
	// Transition probabilities: current state -> next state -> probability
	Transitions map[string]map[string]float64
	
	// State counts for calculating probabilities
	StateCounts map[string]map[string]int
	
	// Current state of the chain
	CurrentState State
	
	// History of states visited
	History []State
	
	// Random generator for state selection
	rand *rand.Rand
}

// NewMarkovChain creates a new Markov chain instance
func NewMarkovChain() *MarkovChain {
	return &MarkovChain{
		Transitions: make(map[string]map[string]float64),
		StateCounts: make(map[string]map[string]int),
		History:     make([]State, 0),
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// UpdateTransition updates the transition probability from current state to next state
func (mc *MarkovChain) UpdateTransition(currentState, nextState State) {
	currentStr := currentState.String()
	nextStr := nextState.String()
	
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
func (mc *MarkovChain) updateProbabilities(currentState string) {
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
func (mc *MarkovChain) GetNextState() State {
	currentStr := mc.CurrentState.String()
	
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
			return mc.StringToState(nextStateStr)
		}
	}
	
	// Fallback: return the first state if something went wrong
	for nextStateStr := range transitions {
		return mc.StringToState(nextStateStr)
	}
	
	return mc.CurrentState
}

// StringToState converts a string representation back to a State
func (mc *MarkovChain) StringToState(stateStr string) State {
	parts := strings.Split(stateStr, "_")
	if len(parts) < 6 {
		return State{}
	}
	
	statusCode, _ := strconv.ParseInt(parts[0], 10, 64)
	contentLength, _ := strconv.ParseInt(parts[1], 10, 64)
	contentWords, _ := strconv.ParseInt(parts[2], 10, 64)
	contentLines, _ := strconv.ParseInt(parts[3], 10, 64)
	durationMs, _ := strconv.ParseInt(parts[4], 10, 64)
	isMatch, _ := strconv.ParseBool(parts[5])
	
	return State{
		StatusCode:    statusCode,
		ContentLength: contentLength,
		ContentWords:  contentWords,
		ContentLines:  contentLines,
		Duration:      time.Duration(durationMs) * time.Millisecond,
		IsMatch:       isMatch,
	}
}

// UpdateChain updates the Markov chain with a new response state
func (mc *MarkovChain) UpdateChain(newState State) {
	if len(mc.History) > 0 {
		// Update transition from the previous state to the new state
		prevState := mc.History[len(mc.History)-1]
		mc.UpdateTransition(prevState, newState)
	}
	
	// Update current state and history
	mc.CurrentState = newState
	mc.History = append(mc.History, newState)
}

// GetTransitionProbability returns the probability of transitioning from one state to another
func (mc *MarkovChain) GetTransitionProbability(fromState, toState State) float64 {
	fromStr := fromState.String()
	toStr := toState.String()
	
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