package ffuf

import (
	"fmt"
	"strings"
)

func (c *Config) ToOptions() ConfigOptions {
	o := ConfigOptions{}
	// HTTP options
	o.HTTP.Cookies = []string{}
	o.HTTP.Data = c.Data
	o.HTTP.FollowRedirects = c.FollowRedirects
	o.HTTP.Headers = make([]string, 0)
	for k, v := range c.Headers {
		o.HTTP.Headers = append(o.HTTP.Headers, fmt.Sprintf("%s: %s", k, v))
	}
	o.HTTP.IgnoreBody = c.IgnoreBody
	o.HTTP.Method = c.Method
	o.HTTP.ProxyURL = c.ProxyURL
	o.HTTP.Raw = c.Raw
	o.HTTP.Recursion = c.Recursion
	o.HTTP.RecursionDepth = c.RecursionDepth
	o.HTTP.RecursionStrategy = c.RecursionStrategy
	o.HTTP.ReplayProxyURL = c.ReplayProxyURL
	o.HTTP.SNI = c.SNI
	o.HTTP.Timeout = c.Timeout
	o.HTTP.URL = c.Url
	o.HTTP.Http2 = c.Http2

	o.General.AutoCalibration = c.AutoCalibration
	o.General.AutoCalibrationKeyword = c.AutoCalibrationKeyword
	o.General.AutoCalibrationPerHost = c.AutoCalibrationPerHost
	o.General.AutoCalibrationStrategies = c.AutoCalibrationStrategies
	o.General.AutoCalibrationStrings = c.AutoCalibrationStrings
	o.General.Colors = c.Colors
	o.General.ConfigFile = ""
	if c.Delay.HasDelay {
		if c.Delay.IsRange {
			o.General.Delay = fmt.Sprintf("%.2f-%.2f", c.Delay.Min, c.Delay.Max)
		} else {
			o.General.Delay = fmt.Sprintf("%.2f", c.Delay.Min)
		}
	} else {
		o.General.Delay = ""
	}
	o.General.Json = c.Json
	o.General.MaxTime = c.MaxTime
	o.General.MaxTimeJob = c.MaxTimeJob
	o.General.Noninteractive = c.Noninteractive
	o.General.Quiet = c.Quiet
	o.General.Rate = int(c.Rate)
	o.General.ScraperFile = c.ScraperFile
	o.General.Scrapers = c.Scrapers
	o.General.StopOn403 = c.StopOn403
	o.General.StopOnAll = c.StopOnAll
	o.General.StopOnErrors = c.StopOnErrors
	o.General.Threads = c.Threads
	o.General.Verbose = c.Verbose

	o.Input.DirSearchCompat = c.DirSearchCompat
	o.Input.Extensions = strings.Join(c.Extensions, ",")
	o.Input.IgnoreWordlistComments = c.IgnoreWordlistComments
	o.Input.InputMode = c.InputMode
	o.Input.InputNum = c.InputNum
	o.Input.InputShell = c.InputShell
	o.Input.Inputcommands = []string{}
	for _, v := range c.InputProviders {
		if v.Name == "command" {
			o.Input.Inputcommands = append(o.Input.Inputcommands, fmt.Sprintf("%s:%s", v.Value, v.Keyword))
		}
	}
	o.Input.Request = c.RequestFile
	o.Input.RequestProto = c.RequestProto
	o.Input.Wordlists = c.Wordlists

	o.Output.DebugLog = c.Debuglog
	o.Output.OutputDirectory = c.OutputDirectory
	o.Output.OutputFile = c.OutputFile
	o.Output.OutputFormat = c.OutputFormat
	o.Output.OutputSkipEmptyFile = c.OutputSkipEmptyFile

	o.Filter.Mode = c.FilterMode
	o.Filter.Lines = ""
	o.Filter.Regexp = ""
	o.Filter.Size = ""
	o.Filter.Status = ""
	o.Filter.Time = ""
	o.Filter.Words = ""
	for name, filter := range c.MatcherManager.GetFilters() {
		switch name {
		case "line":
			o.Filter.Lines = filter.Repr()
		case "regexp":
			o.Filter.Regexp = filter.Repr()
		case "size":
			o.Filter.Size = filter.Repr()
		case "status":
			o.Filter.Status = filter.Repr()
		case "time":
			o.Filter.Time = filter.Repr()
		case "words":
			o.Filter.Words = filter.Repr()
		}
	}
	o.Matcher.Mode = c.MatcherMode
	o.Matcher.Lines = ""
	o.Matcher.Regexp = ""
	o.Matcher.Size = ""
	o.Matcher.Status = ""
	o.Matcher.Time = ""
	o.Matcher.Words = ""
	for name, filter := range c.MatcherManager.GetMatchers() {
		switch name {
		case "line":
			o.Matcher.Lines = filter.Repr()
		case "regexp":
			o.Matcher.Regexp = filter.Repr()
		case "size":
			o.Matcher.Size = filter.Repr()
		case "status":
			o.Matcher.Status = filter.Repr()
		case "time":
			o.Matcher.Time = filter.Repr()
		case "words":
			o.Matcher.Words = filter.Repr()
		}
	}
	return o
}
