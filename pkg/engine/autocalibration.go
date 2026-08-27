package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

func (j *Job) autoCalibrationStrings() map[string][]string {
	// The global rand is seeded once in Job.Start; re-seeding here on every call
	// reset the sequence and could hand two near-simultaneous calibrations the
	// same "random" strings.
	cInputs := make(map[string][]string)

	if len(j.Config.AutoCalibrationStrings) > 0 {
		cInputs["custom"] = append(cInputs["custom"], j.Config.AutoCalibrationStrings...)
		return cInputs

	}

	for _, strategy := range j.Config.AutoCalibrationStrategies {
		jsonStrategy, err := os.ReadFile(filepath.Join(ffuf.AUTOCALIBDIR, strategy+".json"))
		if err != nil {
			j.Output.Warning(fmt.Sprintf("Skipping strategy \"%s\" because of error: %s\n", strategy, err))
			continue
		}

		tmpStrategy := ffuf.AutocalibrationStrategy{}
		err = json.Unmarshal(jsonStrategy, &tmpStrategy)
		if err != nil {
			j.Output.Warning(fmt.Sprintf("Skipping strategy \"%s\" because of error: %s\n", strategy, err))
			continue
		}

		cInputs = mergeMaps(cInputs, tmpStrategy)
	}

	return cInputs
}

func mergeMaps(m1 map[string][]string, m2 map[string][]string) map[string][]string {
	merged := make(map[string][]string)
	for k, v := range m1 {
		merged[k] = v
	}
	for key, value := range m2 {
		if _, ok := merged[key]; !ok {
			// Key not found, add it
			merged[key] = value
			continue
		}
		for _, entry := range value {
			if !ffuf.StrInSlice(entry, merged[key]) {
				merged[key] = append(merged[key], entry)
			}
		}
	}
	return merged
}

func (j *Job) calibrationRequest(inputs map[string][]byte) (ffuf.Response, error) {
	basereq := ffuf.BaseRequest(j.Config)
	req, err := j.Runner.Prepare(inputs, &basereq)
	if err != nil {
		j.Output.Error(fmt.Sprintf("Encountered an error while preparing autocalibration request: %s\n", err))
		j.incError()
		log.Printf("%s", err)
		return ffuf.Response{}, err
	}
	resp, err := j.Runner.Execute(&req)
	if err != nil {
		j.Output.Error(fmt.Sprintf("Encountered an error while executing autocalibration request: %s\n", err))
		j.incError()
		log.Printf("%s", err)
		return ffuf.Response{}, err
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
		responses := make([]ffuf.Response, 0)
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

	// Copy the caller's inputs before overwriting the calibration keyword below.
	// SimpleRunner.Prepare sets req.Input to the map it is given, so the map the
	// caller passes here is the same one used for result attribution; mutating it
	// in place makes a later matched result report the calibration string instead
	// of the real payload. CalibrateForHost copies for the same reason.
	cinput := make(map[string][]byte, len(input))
	for k, v := range input {
		cinput[k] = v
	}

	for _, v := range cInputs {
		responses := make([]ffuf.Response, 0)
		for _, cs := range v {
			cinput[j.Config.AutoCalibrationKeyword] = []byte(cs)
			resp, err := j.calibrationRequest(cinput)
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

// registerCalibrationFilter adds a calibration-derived filter (name/value) for
// the current scope (global, or per-host when perHost is set), unless a filter
// already installed for that scope would filter the sample response - in which
// case a new, redundant filter is skipped.
func (j *Job) registerCalibrationFilter(name, value string, perHost bool, sample ffuf.Response) error {
	if perHost {
		host := ffuf.HostURLFromRequest(*sample.Request)
		for _, f := range j.Config.MatcherManager.FiltersForDomain(host) {
			if match, _ := f.Filter(&sample); match {
				return nil // Already filtered
			}
		}
		return j.Config.MatcherManager.AddPerDomainFilter(host, name, value)
	}
	for _, f := range j.Config.MatcherManager.GetFilters() {
		if match, _ := f.Filter(&sample); match {
			return nil // Already filtered
		}
	}
	return j.Config.MatcherManager.AddFilter(name, value, false)
}

func (j *Job) calibrateFilters(responses []ffuf.Response, perHost bool) error {
	if len(responses) == 0 {
		return fmt.Errorf("No common filtering values found")
	}

	// Work down from the most specific common denominator.

	// Content length
	baselineSize := responses[0].ContentLength
	sizeMatch := true
	for _, r := range responses {
		if baselineSize != r.ContentLength {
			sizeMatch = false
			break
		}
	}
	if sizeMatch {
		return j.registerCalibrationFilter("size", strconv.FormatInt(baselineSize, 10), perHost, responses[0])
	}

	// Content words
	baselineWords := responses[0].ContentWords
	wordsMatch := true
	for _, r := range responses {
		if baselineWords != r.ContentWords {
			wordsMatch = false
			break
		}
	}
	if wordsMatch {
		return j.registerCalibrationFilter("word", strconv.FormatInt(baselineWords, 10), perHost, responses[0])
	}

	// Content lines
	baselineLines := responses[0].ContentLines
	linesMatch := true
	for _, r := range responses {
		if baselineLines != r.ContentLines {
			linesMatch = false
			break
		}
	}
	if linesMatch {
		return j.registerCalibrationFilter("line", strconv.FormatInt(baselineLines, 10), perHost, responses[0])
	}

	// Dynamic (reflected) size: none of the dimensions above were constant,
	// which is exactly what happens with a custom error page that echoes the
	// requested value back into the body - e.g. an nginx/Express/Flask 404
	// handler rendering "Cannot find /adminXXXXXXXXXXXXXXXX". Content-Length
	// then tracks len(fuzz value) plus a fixed offset instead of being
	// constant outright. Detect that relationship by subtracting each
	// response's fuzz-value length from its ContentLength: if the *normalized*
	// value is constant, the page is a reflected-but-otherwise-static 404, and
	// a DynamicSizeFilter (pkg/filter/dynamicsize.go) can recognize it for any
	// future fuzzed value, not just the calibration strings used here.
	if offset, ok := j.reflectedSizeOffset(responses); ok {
		value := fmt.Sprintf("%d:%s", offset, j.Config.AutoCalibrationKeyword)
		return j.registerCalibrationFilter("dynamicsize", value, perHost, responses[0])
	}

	return fmt.Errorf("No common filtering values found")
}

// reflectedSizeOffset checks whether every response's ContentLength equals a
// constant offset plus the length of the calibration value that produced it
// (i.e. the fuzzed value was reflected into the response body). It returns
// that offset and true when the relationship holds for every response, or
// (0, false) when responses are missing the calibration input or the
// normalized sizes disagree.
func (j *Job) reflectedSizeOffset(responses []ffuf.Response) (int64, bool) {
	keyword := j.Config.AutoCalibrationKeyword
	var offset int64
	for i, r := range responses {
		if r.Request == nil {
			return 0, false
		}
		fuzzval, ok := r.Request.Input[keyword]
		if !ok {
			return 0, false
		}
		normalized := r.ContentLength - int64(len(fuzzval))
		if i == 0 {
			offset = normalized
			continue
		}
		if normalized != offset {
			return 0, false
		}
	}
	return offset, true
}
