package ffuf

import (
	"context"
)

type Config struct {
	AutoCalibration        bool                      `json:"autocalibration"`
	AutoCalibrationStrings []string                  `json:"autocalibration_strings"`
	Cancel                 context.CancelFunc        `json:"-"`
	Colors                 bool                      `json:"colors"`
	CommandKeywords        []string                  `json:"-"`
	CommandLine            string                    `json:"cmdline"`
	ConfigFile             string                    `json:"configfile"`
	Context                context.Context           `json:"-"`
	Data                   string                    `json:"postdata"`
	Delay                  optRange                  `json:"delay"`
	DirSearchCompat        bool                      `json:"dirsearch_compatibility"`
	Extensions             []string                  `json:"extensions"`
	Filters                map[string]FilterProvider `json:"filters"`
	FollowRedirects        bool                      `json:"follow_redirects"`
	Headers                map[string]string         `json:"headers"`
	IgnoreBody             bool                      `json:"ignorebody"`
	IgnoreWordlistComments bool                      `json:"ignore_wordlist_comments"`
	InputMode              string                    `json:"inputmode"`
	InputNum               int                       `json:"cmd_inputnum"`
	InputProviders         []InputProviderConfig     `json:"inputproviders"`
	InputShell             string                    `json:"inputshell"`
	Matchers               map[string]FilterProvider `json:"matchers"`
	MaxTime                int                       `json:"maxtime"`
	MaxTimeJob             int                       `json:"maxtime_job"`
	Method                 string                    `json:"method"`
	Noninteractive         bool                      `json:"noninteractive"`
	OutputDirectory        string                    `json:"outputdirectory"`
	OutputFile             string                    `json:"outputfile"`
	OutputFormat           string                    `json:"outputformat"`
	OutputSkipEmptyFile    bool                      `json:"OutputSkipEmptyFile"`
	ProgressFrequency      int                       `json:"-"`
	ProxyURL               string                    `json:"proxyurl"`
	Quiet                  bool                      `json:"quiet"`
	Rate                   int64                     `json:"rate"`
	Recursion              bool                      `json:"recursion"`
	RecursionDepth         int                       `json:"recursion_depth"`
	RecursionStrategy      string                    `json:"recursion_strategy"`
	ReplayProxyURL         string                    `json:"replayproxyurl"`
	SNI                    string                    `json:"sni"`
	StopOn403              bool                      `json:"stop_403"`
	StopOnAll              bool                      `json:"stop_all"`
	StopOnErrors           bool                      `json:"stop_errors"`
	Threads                int                       `json:"threads"`
	Timeout                int                       `json:"timeout"`
	Url                    string                    `json:"url"`
	Verbose                bool                      `json:"verbose"`
}

type InputProviderConfig struct {
	Name    string `json:"name"`
	Keyword string `json:"keyword"`
	Value   string `json:"value"`
}

func NewConfig(ctx context.Context, cancel context.CancelFunc) Config {
	var conf Config
	conf.AutoCalibrationStrings = make([]string, 0)
	conf.CommandKeywords = make([]string, 0)
	conf.Context = ctx
	conf.Cancel = cancel
	conf.Data = ""
	conf.Delay = optRange{0, 0, false, false}
	conf.DirSearchCompat = false
	conf.Extensions = make([]string, 0)
	conf.Filters = make(map[string]FilterProvider)
	conf.FollowRedirects = false
	conf.Headers = make(map[string]string)
	conf.IgnoreWordlistComments = false
	conf.InputMode = "clusterbomb"
	conf.InputNum = 0
	conf.InputShell = ""
	conf.InputProviders = make([]InputProviderConfig, 0)
	conf.Matchers = make(map[string]FilterProvider)
	conf.MaxTime = 0
	conf.MaxTimeJob = 0
	conf.Method = "GET"
	conf.Noninteractive = false
	conf.ProgressFrequency = 125
	conf.ProxyURL = ""
	conf.Quiet = false
	conf.Rate = 0
	conf.Recursion = false
	conf.RecursionDepth = 0
	conf.RecursionStrategy = "default"
	conf.SNI = ""
	conf.StopOn403 = false
	conf.StopOnAll = false
	conf.StopOnErrors = false
	conf.Timeout = 10
	conf.Url = ""
	conf.Verbose = false
	return conf
}

func (c *Config) SetContext(ctx context.Context, cancel context.CancelFunc) {
	c.Context = ctx
	c.Cancel = cancel
}
