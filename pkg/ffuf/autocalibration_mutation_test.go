package ffuf

import (
	"errors"
	"testing"
)

// stubRunner is a RunnerProvider whose calls fail. It is enough to drive
// Calibrate's loop without real HTTP: the mutation under test happens in
// Calibrate before the request is ever prepared.
type stubRunner struct{}

func (stubRunner) Prepare(map[string][]byte, *Request) (Request, error) {
	return Request{}, errors.New("stub")
}
func (stubRunner) Execute(*Request) (Response, error) { return Response{}, errors.New("stub") }
func (stubRunner) Dump(*Request) ([]byte, error)      { return nil, nil }

// TestCalibrate_DoesNotMutateInput locks the fix for an input-map aliasing bug.
// SimpleRunner.Prepare sets req.Input = input (the same map), so resp.Request.Input
// aliases the caller's input map. Calibrate overwrote input[keyword] with each
// calibration string in place, so a result matched and reported after
// autocalibration showed the calibration string (e.g. ".htaccessXXXX") instead of
// the real payload. Calibrate must copy the map before overwriting, exactly as
// CalibrateForHost already does.
func TestCalibrate_DoesNotMutateInput(t *testing.T) {
	job := &Job{
		Config: &Config{
			Url:                    "http://localhost/FUZZ",
			Method:                 "GET",
			AutoCalibration:        true,
			AutoCalibrationKeyword: "FUZZ",
			AutoCalibrationStrings: []string{"probe-a", "probe-b"},
			MatcherManager:         &fakeMatcherManager{},
		},
		Output: NewNullOutput(),
		Runner: stubRunner{},
	}
	input := map[string][]byte{"FUZZ": []byte("realpayload")}

	if err := job.Calibrate(input); err != nil {
		t.Fatalf("Calibrate: %v", err)
	}
	if got := string(input["FUZZ"]); got != "realpayload" {
		t.Errorf("Calibrate mutated the caller's input map: FUZZ = %q, want \"realpayload\"", got)
	}
}
