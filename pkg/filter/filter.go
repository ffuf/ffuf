package filter

import (
	"flag"
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
                // valid filter create or append
                if conf.Filters[name] == nil {
                        conf.Filters[name] = newf
                } else {
                        currentfilter := conf.Filters[name].Repr()
                        newoption := strings.TrimSpace(strings.Split(currentfilter, ":")[1]) + "," + option
                        newerf, err := NewFilterByName(name, newoption)
                        if err == nil {
                                conf.Filters[name] = newerf
                        }
                }
        }
        return err
}

//AddMatcher adds a new matcher to Config
func AddMatcher(conf *ffuf.Config, name string, option string) error {
	newf, err := NewFilterByName(name, option)
	if err == nil {
		conf.Matchers[name] = newf
	}
	return err
}

//CalibrateIfNeeded runs a self-calibration task for filtering options (if needed) by requesting random resources and acting accordingly
func CalibrateIfNeeded(j *ffuf.Job) error {
	var err error
	if !j.Config.AutoCalibration {
		return nil
	}
	// Handle the calibration
	responses, err := j.CalibrateResponses()
	if err != nil {
		return err
	}
	if len(responses) > 0 {
		err = calibrateFilters(j, responses)
	}
	return err
}

func calibrateFilters(j *ffuf.Job, responses []ffuf.Response) error {
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
		err := AddFilter(j.Config, "size", strings.Join(sizeCalib, ","))
		if err != nil {
			return err
		}
	}
	if len(wordCalib) > 0 {
		err := AddFilter(j.Config, "word", strings.Join(wordCalib, ","))
		if err != nil {
			return err
		}
	}
	if len(lineCalib) > 0 {
		err := AddFilter(j.Config, "line", strings.Join(lineCalib, ","))
		if err != nil {
			return err
		}
	}
	return nil
}

func SetupFilters(parseOpts *ffuf.ConfigOptions, conf *ffuf.Config) error {
	errs := ffuf.NewMultierror()
	// If any other matcher is set, ignore -mc default value
	matcherSet := false
	statusSet := false
	warningIgnoreBody := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "mc" {
			statusSet = true
		}
		if f.Name == "ms" {
			matcherSet = true
			warningIgnoreBody = true
		}
		if f.Name == "ml" {
			matcherSet = true
			warningIgnoreBody = true
		}
		if f.Name == "mr" {
			matcherSet = true
		}
		if f.Name == "mw" {
			matcherSet = true
			warningIgnoreBody = true
		}
	})
	if statusSet || !matcherSet {
		if err := AddMatcher(conf, "status", parseOpts.Matcher.Status); err != nil {
			errs.Add(err)
		}
	}

	if parseOpts.Filter.Status != "" {
		if err := AddFilter(conf, "status", parseOpts.Filter.Status); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Size != "" {
		warningIgnoreBody = true
		if err := AddFilter(conf, "size", parseOpts.Filter.Size); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Regexp != "" {
		if err := AddFilter(conf, "regexp", parseOpts.Filter.Regexp); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Words != "" {
		warningIgnoreBody = true
		if err := AddFilter(conf, "word", parseOpts.Filter.Words); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Lines != "" {
		warningIgnoreBody = true
		if err := AddFilter(conf, "line", parseOpts.Filter.Lines); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Size != "" {
		if err := AddMatcher(conf, "size", parseOpts.Matcher.Size); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Regexp != "" {
		if err := AddMatcher(conf, "regexp", parseOpts.Matcher.Regexp); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Words != "" {
		if err := AddMatcher(conf, "word", parseOpts.Matcher.Words); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Lines != "" {
		if err := AddMatcher(conf, "line", parseOpts.Matcher.Lines); err != nil {
			errs.Add(err)
		}
	}
	if conf.IgnoreBody && warningIgnoreBody {
		fmt.Printf("*** Warning: possible undesired combination of -ignore-body and the response options: fl,fs,fw,ml,ms and mw.\n")
	}
	return errs.ErrorOrNil()
}
