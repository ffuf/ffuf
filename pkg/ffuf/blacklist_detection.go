package ffuf

import (
	"fmt"
	"log"
)

func (j *Job) blackListFiles() map[string][]string {
	blackListedFiles := make(map[string][]string)

	// if a couple of those returns same size, its probably a blacklisted response.
	blackListedFiles["BLACKLIST"] = append(blackListedFiles["BLACKLIST"], ".htaccess")
	blackListedFiles["BLACKLIST"] = append(blackListedFiles["BLACKLIST"], "web.config")
	blackListedFiles["BLACKLIST"] = append(blackListedFiles["BLACKLIST"], ".bash_history")
	blackListedFiles["BLACKLIST"] = append(blackListedFiles["BLACKLIST"], ".git/HEAD")
	blackListedFiles["BLACKLIST"] = append(blackListedFiles["BLACKLIST"], ".ssh/id_rsa")

	// if a couple of those returns same size, its probably a wildcard not found response.
	blackListedFiles["WILDCARD"] = append(blackListedFiles["WILDCARD"], "abc.xyz")
	blackListedFiles["WILDCARD"] = append(blackListedFiles["WILDCARD"], "abc2.xyz")
	blackListedFiles["WILDCARD"] = append(blackListedFiles["WILDCARD"], "abc3.xyzz")
	blackListedFiles["WILDCARD"] = append(blackListedFiles["WILDCARD"], "xyz1.abc")

	return blackListedFiles
}

func (j *Job) blackListRequest(input map[string][]byte) (Response, error) {
	basereq := BaseRequest(j.Config)
	req, err := j.Runner.Prepare(input, &basereq)
	if err != nil {
		j.Output.Error(fmt.Sprintf("Encountered an error while preparing blacklist detection request: %s\n", err))
		j.incError()
		log.Printf("%s", err)
		return Response{}, err
	}

	resp, err := j.Runner.Execute(&req)
	if err != nil {
		j.Output.Error(fmt.Sprintf("Encountered an error while executing blacklist detection request: %s\n", err))
		j.incError()
		log.Printf("%s", err)
		return Response{}, err
	}

	return resp, nil
}

func (j *Job) DetectBlacklist(host string, inputs map[string][]byte) error {
	j.blacklistMutex.Lock()
	defer j.blacklistMutex.Unlock()
	if j.blacklistChecked {
		return nil
	}

	responses := make([]Response, 0)
	input := make(map[string][]byte)

	blacklist := j.blackListFiles()
	for _, f := range blacklist {
		for _, cs := range f {
			input[j.Config.AutoCalibrationKeyword] = []byte(cs)

			resp, err := j.blackListRequest(input)
			if err != nil {
				continue
			}
			responses = append(responses, resp)
		}
	}

	successResponses := make(map[int64][]Response, 0)
	blackListedResponses := make(map[int64][]Response, 0)
	for _, r := range responses {
		if r.StatusCode != 200 {
			blackListedResponses[r.ContentWords] = append(blackListedResponses[r.ContentWords], r)
		} else {
			successResponses[r.ContentWords] = append(successResponses[r.ContentWords], r)
		}
	}

	for _, r := range successResponses {
		if len(r) > 1 {
			err := j.calibrateFilters(r, j.Config.AutoCalibrationPerHost /*, j.Config.AutoCalibrationPerPath*/)
			if err != nil {
				j.Output.Error(fmt.Sprintf("%s", err))
			}
		}
	}

	for _, r := range blackListedResponses {
		if len(r) > 1 {
			err := j.calibrateFilters(r, j.Config.AutoCalibrationPerHost /*, j.Config.AutoCalibrationPerPath */)
			if err != nil {
				j.Output.Error(fmt.Sprintf("%s", err))
			}
		}
	}

	j.blacklistChecked = true
	return nil
}
