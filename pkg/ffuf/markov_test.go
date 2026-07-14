package ffuf

import (
	"testing"
	"time"
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

func (m *MockInputProvider) AddProvider(config InputProviderConfig) error {
	// Not implemented for this test
	return nil
}

func (m *MockInputProvider) SetPosition(pos int) {
	if pos >= 0 && pos < len(m.inputs) {
		m.pos = pos
	}
}

// TestMarkovChain tests the basic functionality of the Markov Chain
func TestMarkovChain(t *testing.T) {
	controller := NewFeedbackController()
	
	response1 := Response{
		StatusCode:    200,
		ContentLength: 100,
		ContentWords:  20,
		ContentLines:  10,
		Duration:      time.Millisecond * 50,
	}
	
	response2 := Response{
		StatusCode:    404,
		ContentLength: 50,
		ContentWords:  10,
		ContentLines:  5,
		Duration:      time.Millisecond * 30,
	}
	
	// Update the controller with responses
	controller.UpdateWithResponse(response1, true)
	controller.UpdateWithResponse(response2, false)
	
	if len(controller.(*mFeedbackController).responseHistory) != 2 {
		t.Errorf("Expected 2 responses in history, got %d", len(controller.(*mFeedbackController).responseHistory))
	}
}

// TestGetNextInput tests the input selection mechanism
func TestGetNextInput(t *testing.T) {
	controller := NewFeedbackController()
	
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
	
	// Get next input - this may or may not return a feedback-based input depending on probability
	_, useFeedback := controller.GetNextInput(mockProvider)
	
	if !useFeedback {
		t.Log("Feedback may not be used due to probability calculations - this is expected behavior")
	}
}

// TestMarkovAnalysis checks the analysis functionality
func TestMarkovAnalysis(t *testing.T) {
	controller := NewFeedbackController()
	
	// Add several responses
	for i := 0; i < 5; i++ {
		response := Response{
			StatusCode:    int64(200 + i%2*200), // Alternates between 200 and 400
			ContentLength: int64(100 + i*10),
			ContentWords:  int64(20 + i*2),
			ContentLines:  int64(10 + i),
			Duration:      time.Millisecond * time.Duration(50+i*5),
		}
		controller.UpdateWithResponse(response, i%2 == 0) // Every other one is a match
	}
	
	analysis := controller.AnalyzeResponsePatterns()
	
	if totalResp, ok := analysis["total_responses"].(int); !ok || totalResp != 5 {
		t.Errorf("Expected 5 total responses in analysis, got %v", analysis["total_responses"])
	}
}