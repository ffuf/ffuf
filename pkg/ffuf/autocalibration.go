package ffuf

import (
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"time"
)

func (j *Job) autoCalibrationStrings() map[string][]string {
	rand.Seed(time.Now().UnixNano())
	cInputs := make(map[string][]string)
	if len(j.Config.AutoCalibrationStrings) < 1 {
		cInputs["basic_admin"] = append(cInputs["basic_admin"], "admin"+RandomString(16))
		cInputs["basic_admin"] = append(cInputs["basic_admin"], "admin"+RandomString(8))
		cInputs["htaccess"] = append(cInputs["htaccess"], ".htaccess"+RandomString(16))
		cInputs["htaccess"] = append(cInputs["htaccess"], ".htaccess"+RandomString(8))
		cInputs["basic_random"] = append(cInputs["basic_random"], RandomString(16))
		cInputs["basic_random"] = append(cInputs["basic_random"], RandomString(8))
		if j.Config.AutoCalibrationStrategy == "advanced" {
			// Add directory tests and .htaccess too
			cInputs["admin_dir"] = append(cInputs["admin_dir"], "admin"+RandomString(16)+"/")
			cInputs["admin_dir"] = append(cInputs["admin_dir"], "admin"+RandomString(8)+"/")
			cInputs["random_dir"] = append(cInputs["random_dir"], RandomString(16)+"/")
			cInputs["random_dir"] = append(cInputs["random_dir"], RandomString(8)+"/")
		}
	} else {
		cInputs["custom"] = append(cInputs["custom"], j.Config.AutoCalibrationStrings...)
	}
	return cInputs
}

func (j *Job) calibrationRequest(inputs map[string][]byte) (Response, error) {
	basereq := BaseRequest(j.Config)
	req, err := j.Runner.Prepare(inputs, &basereq)
	if err != nil {
		j.Output.Error(fmt.Sprintf("Encountered an error while preparing autocalibration request: %s\n", err))
		j.incError()
		log.Printf("%s", err)
		return Response{}, err
	}
	resp, err := j.Runner.Execute(&req)
	if err != nil {
		j.Output.Error(fmt.Sprintf("Encountered an error while executing autocalibration request: %s\n", err))
		j.incError()
		log.Printf("%s", err)
		return Response{}, err
	}
	// Only calibrate on responses that would be matched otherwise
	if j.isMatch(resp) {
		return resp, nil
	}
	return resp, fmt.Errorf("Response wouldn't be matched")
}

// CalibrateForHost runs autocalibration for a specific host
func (j *Job) CalibrateForHost(host string, baseinput map[string][]byte) error {
	if j.Config.MatcherManager.CalibratedForDomain(host) {
		return nil
	}
	if baseinput[j.Config.AutoCalibrationKeyword] == nil {
		return fmt.Errorf("Autocalibration keyword \"%s\" not found in the request.", j.Config.AutoCalibrationKeyword)
	}
	cStrings := j.autoCalibrationStrings()
	input := make(map[string][]byte)
	for k, v := range baseinput {
		input[k] = v
	}
	for _, v := range cStrings {
		responses := make([]Response, 0)
		for _, cs := range v {
			input[j.Config.AutoCalibrationKeyword] = []byte(cs)
			resp, err := j.calibrationRequest(input)
			if err != nil {
				continue
			}
			responses = append(responses, resp)
			err = j.calibrateFilters(responses, true)
			if err != nil {
				j.Output.Error(fmt.Sprintf("%s", err))
			}
		}
	}
	j.Config.MatcherManager.SetCalibratedForHost(host, true)
	return nil
}

// CalibrateResponses returns slice of Responses for randomly generated filter autocalibration requests
func (j *Job) Calibrate(input map[string][]byte) error {
	if j.Config.MatcherManager.Calibrated() {
		return nil
	}
	cInputs := j.autoCalibrationStrings()

	for _, v := range cInputs {
		responses := make([]Response, 0)
		for _, cs := range v {
			input[j.Config.AutoCalibrationKeyword] = []byte(cs)
			resp, err := j.calibrationRequest(input)
			if err != nil {
				continue
			}
			responses = append(responses, resp)
		}
		_ = j.calibrateFilters(responses, false)
	}
	j.Config.MatcherManager.SetCalibrated(true)
	return nil
}

// CalibrateIfNeeded runs a self-calibration task for filtering options (if needed) by requesting random resources and
//
//	configuring the filters accordingly
func (j *Job) CalibrateIfNeeded(host string, input map[string][]byte) error {
	j.calibMutex.Lock()
	defer j.calibMutex.Unlock()
	if !j.Config.AutoCalibration {
		return nil
	}
	if j.Config.AutoCalibrationPerHost {
		return j.CalibrateForHost(host, input)
	}
	return j.Calibrate(input)
}

func (j *Job) calibrateFilters(responses []Response, perHost bool) error {
	// Work down from the most specific common denominator
	if len(responses) > 0 {
		// Content length
		baselineSize := responses[0].ContentLength
		sizeMatch := true
		for _, r := range responses {
			if baselineSize != r.ContentLength {
				sizeMatch = false
			}
		}
		if sizeMatch {
			if perHost {
				// Check if already filtered
				for _, f := range j.Config.MatcherManager.FiltersForDomain(HostURLFromRequest(*responses[0].Request)) {
					match, _ := f.Filter(&responses[0])
					if match {
						// Already filtered
						return nil
					}
				}
				_ = j.Config.MatcherManager.AddPerDomainFilter(HostURLFromRequest(*responses[0].Request), "size", strconv.FormatInt(baselineSize, 10))
				return nil
			} else {
				// Check if already filtered
				for _, f := range j.Config.MatcherManager.GetFilters() {
					match, _ := f.Filter(&responses[0])
					if match {
						// Already filtered
						return nil
					}
				}
				_ = j.Config.MatcherManager.AddFilter("size", strconv.FormatInt(baselineSize, 10), false)
				return nil
			}
		}

		// Content words
		baselineWords := responses[0].ContentWords
		wordsMatch := true
		for _, r := range responses {
			if baselineWords != r.ContentWords {
				wordsMatch = false
			}
		}
		if wordsMatch {
			if perHost {
				// Check if already filtered
				for _, f := range j.Config.MatcherManager.FiltersForDomain(HostURLFromRequest(*responses[0].Request)) {
					match, _ := f.Filter(&responses[0])
					if match {
						// Already filtered
						return nil
					}
				}
				_ = j.Config.MatcherManager.AddPerDomainFilter(HostURLFromRequest(*responses[0].Request), "word", strconv.FormatInt(baselineWords, 10))
				return nil
			} else {
				// Check if already filtered
				for _, f := range j.Config.MatcherManager.GetFilters() {
					match, _ := f.Filter(&responses[0])
					if match {
						// Already filtered
						return nil
					}
				}
				_ = j.Config.MatcherManager.AddFilter("word", strconv.FormatInt(baselineWords, 10), false)
				return nil
			}
		}

		// Content lines
		baselineLines := responses[0].ContentLines
		linesMatch := true
		for _, r := range responses {
			if baselineLines != r.ContentLines {
				linesMatch = false
			}
		}
		if linesMatch {
			if perHost {
				// Check if already filtered
				for _, f := range j.Config.MatcherManager.FiltersForDomain(HostURLFromRequest(*responses[0].Request)) {
					match, _ := f.Filter(&responses[0])
					if match {
						// Already filtered
						return nil
					}
				}
				_ = j.Config.MatcherManager.AddPerDomainFilter(HostURLFromRequest(*responses[0].Request), "line", strconv.FormatInt(baselineLines, 10))
				return nil
			} else {
				// Check if already filtered
				for _, f := range j.Config.MatcherManager.GetFilters() {
					match, _ := f.Filter(&responses[0])
					if match {
						// Already filtered
						return nil
					}
				}
				_ = j.Config.MatcherManager.AddFilter("line", strconv.FormatInt(baselineLines, 10), false)
				return nil
			}
		}
	}
	return fmt.Errorf("No common filtering values found")
}
