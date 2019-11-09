package filter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

func NewFilterByName(name string, value string) (ffuf.FilterProvider, error) {
	if name == "status" {
		return NewStatusFilter(value)
	}
	if name == "size" {
		return NewSizeFilter(value)
	}
	if name == "word" {
		return NewWordFilter(value)
	}
	if name == "line" {
		return NewLineFilter(value)
	}
	if name == "regexp" {
		return NewRegexpFilter(value)
	}
	return nil, fmt.Errorf("Could not create filter with name %s", name)
}

//AddFilter adds a new filter to Config
func AddFilter(conf *ffuf.Config, name string, option string) error {
	newf, err := NewFilterByName(name, option)
	if err == nil {
		conf.Filters = append(conf.Filters, newf)
	}
	return err
}

//AddMatcher adds a new matcher to Config
func AddMatcher(conf *ffuf.Config, name string, option string) error {
	newf, err := NewFilterByName(name, option)
	if err == nil {
		conf.Matchers = append(conf.Matchers, newf)
	}
	return err
}

//CalibrateIfNeeded runs a self-calibration task for filtering options (if needed) by requesting random resources and acting accordingly
func CalibrateIfNeeded(j *ffuf.Job) error {
	if !j.Config.AutoCalibration {
		return nil
	}
	// Handle the calibration
	responses, err := j.CalibrateResponses()
	if err != nil {
		return err
	}
	if len(responses) > 0 {
		calibrateFilters(j, responses)
	}
	return nil
}

func calibrateFilters(j *ffuf.Job, responses []ffuf.Response) {
	sizeCalib := make([]string, 0)
	wordCalib := make([]string, 0)
	lineCalib := make([]string, 0)
	for _, r := range responses {
		if r.ContentLength > 0 {
			// Only add if we have an actual size of responses
			sizeCalib = append(sizeCalib, strconv.FormatInt(r.ContentLength, 10))
		}
		if r.ContentWords > 0 {
			// Only add if we have an actual word length of response
			wordCalib = append(wordCalib, strconv.FormatInt(r.ContentWords, 10))
		}
		if r.ContentLines > 1 {
			// Only add if we have an actual word length of response
			lineCalib = append(lineCalib, strconv.FormatInt(r.ContentLines, 10))
		}
	}

	//Remove duplicates
	sizeCalib = ffuf.UniqStringSlice(sizeCalib)
	wordCalib = ffuf.UniqStringSlice(wordCalib)
	lineCalib = ffuf.UniqStringSlice(lineCalib)

	if len(sizeCalib) > 0 {
		AddFilter(j.Config, "size", strings.Join(sizeCalib, ","))
	}
	if len(wordCalib) > 0 {
		AddFilter(j.Config, "word", strings.Join(wordCalib, ","))
	}
	if len(lineCalib) > 0 {
		AddFilter(j.Config, "line", strings.Join(lineCalib, ","))
	}
}
