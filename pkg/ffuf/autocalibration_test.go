package ffuf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// NullOutput is a dummy output provider that does nothing
type NullOutput struct {
	Results []Result
}

func NewNullOutput() *NullOutput                             { return &NullOutput{} }
func (o *NullOutput) Banner()                                {}
func (o *NullOutput) Finalize() error                        { return nil }
func (o *NullOutput) Progress(status Progress)               {}
func (o *NullOutput) Info(infostring string)                 {}
func (o *NullOutput) Error(errstring string)                 {}
func (o *NullOutput) Raw(output string)                      {}
func (o *NullOutput) Warning(warnstring string)              {}
func (o *NullOutput) Result(resp Response)                   {}
func (o *NullOutput) PrintResult(res Result)                 {}
func (o *NullOutput) SaveFile(filename, format string) error { return nil }
func (o *NullOutput) GetCurrentResults() []Result            { return o.Results }
func (o *NullOutput) SetCurrentResults(results []Result)     { o.Results = results }
func (o *NullOutput) Reset()                                 {}
func (o *NullOutput) Cycle()                                 {}

func TestAutoCalibrationStrings(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "ffuf-test")
	AUTOCALIBDIR = tmpDir
	if err != nil {
		t.Fatalf("Failed to create temporary directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test strategy file
	strategy := AutocalibrationStrategy{
		"test": {"foo", "bar"},
	}
	strategyJSON, err := json.Marshal(strategy)
	if err != nil {
		t.Fatalf("Failed to marshal strategy to JSON: %v", err)
	}
	strategyFile := filepath.Join(tmpDir, "test.json")
	err = os.WriteFile(strategyFile, strategyJSON, 0644)
	if err != nil {
		t.Fatalf("Failed to write strategy file: %v", err)
	}

	// Create a test job with the strategy
	job := &Job{
		Config: &Config{
			AutoCalibrationStrategies: []string{"test"},
		},
		Output: NewNullOutput(),
	}
	cInputs := job.autoCalibrationStrings()

	// Verify that the custom strategy was added
	if len(cInputs["custom"]) != 0 {
		t.Errorf("Expected custom strategy to be empty, but got %v", cInputs["custom"])
	}

	// Verify that the test strategy was added
	expected := []string{"foo", "bar"}
	if len(cInputs["test"]) != len(expected) {
		t.Errorf("Expected test strategy to have %d inputs, but got %d", len(expected), len(cInputs["test"]))
	}
	for i, input := range cInputs["test"] {
		if input != expected[i] {
			t.Errorf("Expected test strategy input %d to be %q, but got %q", i, expected[i], input)
		}
	}

	// Verify that a missing strategy is skipped
	job = &Job{
		Config: &Config{
			AutoCalibrationStrategies: []string{"missing"},
		},
		Output: NewNullOutput(),
	}
	cInputs = job.autoCalibrationStrings()
	if len(cInputs) != 0 {
		t.Errorf("Expected missing strategy to be skipped, but got %v", cInputs)
	}

	// Verify that a malformed strategy is skipped
	malformedStrategy := []byte(`{"test": "foo"}`)
	malformedFile := filepath.Join(tmpDir, "malformed.json")
	err = os.WriteFile(malformedFile, malformedStrategy, 0644)
	if err != nil {
		t.Fatalf("Failed to write malformed strategy file: %v", err)
	}
	job = &Job{
		Config: &Config{
			AutoCalibrationStrategies: []string{"malformed"},
		},
		Output: NewNullOutput(),
	}
	cInputs = job.autoCalibrationStrings()
	if len(cInputs) != 0 {
		t.Errorf("Expected malformed strategy to be skipped, but got %v", cInputs)
	}
}
