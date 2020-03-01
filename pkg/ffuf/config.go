package ffuf

import (
	"context"
)

type Config struct {
	Headers                map[string]string         `json:"headers"`
	Extensions             []string                  `json:"extensions"`
	DirSearchCompat        bool                      `json:"dirsearch_compatibility"`
	Method                 string                    `json:"method"`
	Url                    string                    `json:"url"`
	Data                   string                    `json:"postdata"`
	Quiet                  bool                      `json:"quiet"`
	Colors                 bool                      `json:"colors"`
	InputProviders         []InputProviderConfig     `json:"inputproviders"`
	CommandKeywords        []string                  `json:"-"`
	InputNum               int                       `json:"cmd_inputnum"`
	InputMode              string                    `json:"inputmode"`
	OutputDirectory        string                    `json:"outputdirectory"`
	OutputFile             string                    `json:"outputfile"`
	OutputFormat           string                    `json:"outputformat"`
	IgnoreBody             bool                      `json:"ignorebody"`
	IgnoreWordlistComments bool                      `json:"ignore_wordlist_comments"`
	StopOn403              bool                      `json:"stop_403"`
	StopOnErrors           bool                      `json:"stop_errors"`
	StopOnAll              bool                      `json:"stop_all"`
	FollowRedirects        bool                      `json:"follow_redirects"`
	AutoCalibration        bool                      `json:"autocalibration"`
	AutoCalibrationStrings []string                  `json:"autocalibration_strings"`
	Timeout                int                       `json:"timeout"`
	ProgressFrequency      int                       `json:"-"`
	Delay                  optRange                  `json:"delay"`
	Filters                map[string]FilterProvider `json:"filters"`
	Matchers               map[string]FilterProvider `json:"matchers"`
	Threads                int                       `json:"threads"`
	Context                context.Context           `json:"-"`
	ProxyURL               string                    `json:"proxyurl"`
	ReplayProxyURL         string                    `json:"replayproxyurl"`
	CommandLine            string                    `json:"cmdline"`
	Verbose                bool                      `json:"verbose"`
	MaxTime                int                       `json:"maxtime"`
	MaxTimeJob             int                       `json:"maxtime_job"`
	Recursion              bool                      `json:"recursion"`
	RecursionDepth         int                       `json:"recursion_depth"`
}

type InputProviderConfig struct {
	Name    string `json:"name"`
	Keyword string `json:"keyword"`
	Value   string `json:"value"`
}

func NewConfig(ctx context.Context) Config {
	var conf Config
	conf.Context = ctx
	conf.Headers = make(map[string]string)
	conf.Method = "GET"
	conf.Url = ""
	conf.Data = ""
	conf.Quiet = false
	conf.IgnoreWordlistComments = false
	conf.StopOn403 = false
	conf.StopOnErrors = false
	conf.StopOnAll = false
	conf.FollowRedirects = false
	conf.InputProviders = make([]InputProviderConfig, 0)
	conf.CommandKeywords = make([]string, 0)
	conf.AutoCalibrationStrings = make([]string, 0)
	conf.InputNum = 0
	conf.InputMode = "clusterbomb"
	conf.ProxyURL = ""
	conf.Filters = make(map[string]FilterProvider)
	conf.Matchers = make(map[string]FilterProvider)
	conf.Delay = optRange{0, 0, false, false}
	conf.Extensions = make([]string, 0)
	conf.Timeout = 10
	// Progress update frequency, in milliseconds
	conf.ProgressFrequency = 100
	conf.DirSearchCompat = false
	conf.Verbose = false
	conf.MaxTime = 0
	conf.MaxTimeJob = 0
	conf.Recursion = false
	conf.RecursionDepth = 0
	return conf
}
