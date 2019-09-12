package output

import (
	"encoding/json"
	"io/ioutil"
	"time"

	"github.com/fang0654/ffuf/pkg/ffuf"
)

type jsonFileOutput struct {
	CommandLine string   `json:"commandline"`
	Time        string   `json:"time"`
	Results     []Result `json:"results"`
}

func writeJSON(config *ffuf.Config, res []Result) error {
	t := time.Now()
	outJSON := jsonFileOutput{
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
