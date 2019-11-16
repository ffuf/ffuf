package output

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type ejsonFileOutput struct {
	CommandLine string   `json:"commandline"`
	Time        string   `json:"time"`
	Results     []Result `json:"results"`
}

type JsonResult struct {
	Input            map[string]string `json:"input"`
	Position         int               `json:"position"`
	StatusCode       int64             `json:"status"`
	ContentLength    int64             `json:"length"`
	ContentWords     int64             `json:"words"`
	ContentLines     int64             `json:"lines"`
	RedirectLocation string            `json:"redirectlocation"`
	Url              string            `json:"url"`
}

type jsonFileOutput struct {
	CommandLine string       `json:"commandline"`
	Time        string       `json:"time"`
	Results     []JsonResult `json:"results"`
}

func writeEJSON(config *ffuf.Config, res []Result) error {
	t := time.Now()
	outJSON := ejsonFileOutput{
		CommandLine: config.CommandLine,
		Time:        t.Format(time.RFC3339),
		Results:     res,
	}

	outBytes, err := json.Marshal(outJSON)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(config.OutputFile, outBytes, 0644)
	if err != nil {
		return err
	}
	return nil
}

func writeJSON(config *ffuf.Config, res []Result) error {
	t := time.Now()
	jsonRes := make([]JsonResult, 0)
	for _, r := range res {
		strinput := make(map[string]string)
		for k, v := range r.Input {
			strinput[k] = string(v)
		}
		jsonRes = append(jsonRes, JsonResult{
			Input:            strinput,
			Position:         r.Position,
			StatusCode:       r.StatusCode,
			ContentLength:    r.ContentLength,
			ContentWords:     r.ContentWords,
			ContentLines:     r.ContentLines,
			RedirectLocation: r.RedirectLocation,
			Url:              r.Url,
		})
	}
	outJSON := jsonFileOutput{
		CommandLine: config.CommandLine,
		Time:        t.Format(time.RFC3339),
		Results:     jsonRes,
	}
	outBytes, err := json.Marshal(outJSON)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(config.OutputFile, outBytes, 0644)
	if err != nil {
		return err
	}
	return nil
}
