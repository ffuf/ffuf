package ffuf

import (
	"fmt"
	"log"
)

func (j *Job) blackListFiles() map[string][]string {
	blackListedFiles := make(map[string][]string)

	blackListedFiles["FUZZ"] = append(blackListedFiles["FUZZ"], ".htaccess")
	blackListedFiles["FUZZ"] = append(blackListedFiles["FUZZ"], "web.config")
	blackListedFiles["FUZZ"] = append(blackListedFiles["FUZZ"], ".bash_history")
	blackListedFiles["FUZZ"] = append(blackListedFiles["FUZZ"], ".git/HEAD")
	blackListedFiles["FUZZ"] = append(blackListedFiles["FUZZ"], ".ssh/id_rsa")

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
	blackListedResponses := make([]Response, 0)
	for _, r := range responses {
		if r.StatusCode != 200 {
			blackListedResponses = append(blackListedResponses, r)
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

	err := j.calibrateFilters(responses, j.Config.AutoCalibrationPerHost /*, j.Config.AutoCalibrationPerPath*/)
	if err != nil {
		j.Output.Error(fmt.Sprintf("%s", err))
	}

	j.blacklistChecked = true
	return nil
}
