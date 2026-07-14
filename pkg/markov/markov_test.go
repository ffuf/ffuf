package markov

import (
	"testing"
	"time"
	
	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// Mock InputProvider for testing
type MockInputProvider struct {
	inputs []map[string][]byte
	pos    int
}

func (m *MockInputProvider) Value() map[string][]byte {
	if m.pos >= len(m.inputs) {
		return nil
	}
	return m.inputs[m.pos]
}

func (m *MockInputProvider) Position() int {
	return m.pos
}

func (m *MockInputProvider) Next() bool {
	if m.pos < len(m.inputs)-1 {
		m.pos++
		return true
	}
	return false
}

func (m *MockInputProvider) Total() int {
	return len(m.inputs)
}

func (m *MockInputProvider) Reset() {
	m.pos = 0
}

func (m *MockInputProvider) ActivateKeywords(keywords []string) {
	// Not implemented for this test
}

func (m *MockInputProvider) Keywords() []string {
	return []string{"FUZZ"}
}

// TestMarkovChain tests the basic functionality of the Markov Chain
func TestMarkovChain(t *testing.T) {
	chain := NewMarkovChain()
	
	state1 := State{
		StatusCode:    200,
		ContentLength: 100,
		ContentWords:  20,
		ContentLines:  10,
		Duration:      time.Millisecond * 50,
		IsMatch:       true,
	}
	
	state2 := State{
		StatusCode:    404,
		ContentLength: 50,
		ContentWords:  10,
		ContentLines:  5,
		Duration:      time.Millisecond * 30,
		IsMatch:       false,
	}
	
	// Update the chain with transitions
	chain.UpdateChain(state1)
	chain.UpdateChain(state2)
	
	// Check that we have transitions
	if len(chain.History) != 2 {
		t.Errorf("Expected 2 states in history, got %d", len(chain.History))
	}
}

// TestFeedbackController tests the feedback controller
func TestFeedbackController(t *testing.T) {
	config := &ffuf.Config{}
	controller := NewFeedbackController(config)
	
	response := ffuf.Response{
		StatusCode: 200,
	}
	
	// Test updating with response
	controller.UpdateWithResponse(response, true)
	
	if len(controller.responseHistory) != 1 {
		t.Errorf("Expected 1 response in history, got %d", len(controller.responseHistory))
	}
}

// TestGetNextInput tests the input selection mechanism
func TestGetNextInput(t *testing.T) {
	config := &ffuf.Config{}
	controller := NewFeedbackController(config)
	
	// Add a matched input
	matchedInput := map[string][]byte{
		"FUZZ": []byte("test"),
	}
	controller.UpdateWithMatchedInput(matchedInput)
	
	// Set up mock input provider
	mockProvider := &MockInputProvider{
		inputs: []map[string][]byte{
			{"FUZZ": []byte("default")},
		},
		pos: 0,
	}
	
	// Get next input
	nextInput, useFeedback := controller.GetNextInput(mockProvider)
	
	if useFeedback && nextInput != nil {
		// The function should return a selected input
	} else {
		t.Log("Using base input provider (this is expected based on probability)")
	}
}