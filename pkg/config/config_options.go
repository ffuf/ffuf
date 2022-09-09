package config

import (
	"context"

	"github.com/ffuf/ffuf/pkg/filter"
)

type ConfigOptions struct {
	// Caution: Unexported fields cannot be merged via reflection.
	// The field must be visible from the outside if it is relevant to the configuration.
	Filter  FilterOptions
	General GeneralOptions
	HTTP    HTTPOptions
	Input   InputOptions
	Matcher MatcherOptions
	Output  OutputOptions
}

type FilterOptions struct {
	Mode   string
	Lines  string
	Regexp string
	Size   string
	Status string
	Time   string
	Words  string
}

type GeneralOptions struct {
	AutoCalibration         bool
	AutoCalibrationKeyword  string
	AutoCalibrationPerHost  bool
	AutoCalibrationStrategy string
	AutoCalibrationStrings  MultiStringFlag
	Colors                  bool
	ConfigFile              string `toml:"-"`
	Delay                   string
	Json                    bool
	MaxTime                 int
	MaxTimeJob              int
	Noninteractive          bool
	Quiet                   bool
	Rate                    int
	ShowVersion             bool `toml:"-"`
	StopOn403               bool
	StopOnAll               bool
	StopOnErrors            bool
	Threads                 int
	Verbose                 bool
}

type HTTPOptions struct {
	Cookies           MultiStringFlag
	Data              string
	FollowRedirects   bool
	Headers           MultiStringFlag
	IgnoreBody        bool
	Method            string
	ProxyURL          string
	Recursion         bool
	RecursionDepth    int
	RecursionStrategy string
	ReplayProxyURL    string
	SNI               string
	Timeout           int
	URL               string
	Http2             bool
}

type InputOptions struct {
	DirSearchCompat        bool
	Extensions             string
	IgnoreWordlistComments bool
	InputMode              string
	InputNum               int
	InputShell             string
	Inputcommands          MultiStringFlag
	Request                string
	RequestProto           string
	Wordlists              WordlistFlag
}

type MatcherOptions struct {
	Mode   string
	Lines  string
	Regexp string
	Size   string
	Status string
	Time   string
	Words  string
}

type OutputOptions struct {
	DebugLog            string
	OutputDirectory     string
	OutputFile          string
	OutputFormat        string
	OutputSkipEmptyFile bool
}

type Config struct {
	AutoCalibration         bool                       `json:"autocalibration"`
	AutoCalibrationKeyword  string                     `json:"autocalibration_keyword"`
	AutoCalibrationPerHost  bool                       `json:"autocalibration_perhost"`
	AutoCalibrationStrategy string                     `json:"autocalibration_strategy"`
	AutoCalibrationStrings  []string                   `json:"autocalibration_strings"`
	Cancel                  context.CancelFunc         `json:"-"`
	Colors                  bool                       `json:"colors"`
	CommandKeywords         []string                   `json:"-"`
	CommandLine             string                     `json:"cmdline"`
	ConfigFile              string                     `json:"configfile"`
	Context                 context.Context            `json:"-"`
	Data                    string                     `json:"postdata"`
	Delay                   optRange                   `json:"delay"`
	DirSearchCompat         bool                       `json:"dirsearch_compatibility"`
	Extensions              []string                   `json:"extensions"`
	FilterMode              string                     `json:"fmode"`
	FollowRedirects         bool                       `json:"follow_redirects"`
	Headers                 map[string]string          `json:"headers"`
	IgnoreBody              bool                       `json:"ignorebody"`
	IgnoreWordlistComments  bool                       `json:"ignore_wordlist_comments"`
	InputMode               string                     `json:"inputmode"`
	InputNum                int                        `json:"cmd_inputnum"`
	InputProviders          []InputProviderConfig      `json:"inputproviders"`
	InputShell              string                     `json:"inputshell"`
	Json                    bool                       `json:"json"`
	MatcherManager          filter.MatcherManagerIface `json:"matchers"`
	MatcherMode             string                     `json:"mmode"`
	MaxTime                 int                        `json:"maxtime"`
	MaxTimeJob              int                        `json:"maxtime_job"`
	Method                  string                     `json:"method"`
	Noninteractive          bool                       `json:"noninteractive"`
	OutputDirectory         string                     `json:"outputdirectory"`
	OutputFile              string                     `json:"outputfile"`
	OutputFormat            string                     `json:"outputformat"`
	OutputSkipEmptyFile     bool                       `json:"OutputSkipEmptyFile"`
	ProgressFrequency       int                        `json:"-"`
	ProxyURL                string                     `json:"proxyurl"`
	Quiet                   bool                       `json:"quiet"`
	Rate                    int64                      `json:"rate"`
	Recursion               bool                       `json:"recursion"`
	RecursionDepth          int                        `json:"recursion_depth"`
	RecursionStrategy       string                     `json:"recursion_strategy"`
	ReplayProxyURL          string                     `json:"replayproxyurl"`
	SNI                     string                     `json:"sni"`
	StopOn403               bool                       `json:"stop_403"`
	StopOnAll               bool                       `json:"stop_all"`
	StopOnErrors            bool                       `json:"stop_errors"`
	Threads                 int                        `json:"threads"`
	Timeout                 int                        `json:"timeout"`
	Url                     string                     `json:"url"`
	Verbose                 bool                       `json:"verbose"`
	Http2                   bool                       `json:"http2"`
}

type InputProviderConfig struct {
	Name     string `json:"name"`
	Keyword  string `json:"keyword"`
	Value    string `json:"value"`
	Template string `json:"template"` // the templating string used for sniper mode (usually "§")
}

// NewConfigOptions returns a newly created ConfigOptions struct with default values
func NewConfigOptions() *ConfigOptions {
	opts := new(ConfigOptions)
	opts.Filter.Mode = "or"
	opts.Filter.Lines = ""
	opts.Filter.Regexp = ""
	opts.Filter.Size = ""
	opts.Filter.Status = ""
	opts.Filter.Time = ""
	opts.Filter.Words = ""
	opts.General.AutoCalibration = false
	opts.General.AutoCalibrationKeyword = "FUZZ"
	opts.General.AutoCalibrationStrategy = "basic"
	opts.General.AutoCalibrationStrings = make(MultiStringFlag, 0, 2)
	opts.General.Colors = false
	opts.General.Delay = ""
	opts.General.Json = false
	opts.General.MaxTime = 0
	opts.General.MaxTimeJob = 0
	opts.General.Noninteractive = false
	opts.General.Quiet = false
	opts.General.Rate = 0
	opts.General.ShowVersion = false
	opts.General.StopOn403 = false
	opts.General.StopOnAll = false
	opts.General.StopOnErrors = false
	opts.General.Threads = 40
	opts.General.Verbose = false
	opts.HTTP.Cookies = make(MultiStringFlag, 0, 2)
	opts.HTTP.Data = ""
	opts.HTTP.FollowRedirects = false
	opts.HTTP.Headers = make(MultiStringFlag, 0, 2)
	opts.HTTP.IgnoreBody = false
	opts.HTTP.Method = ""
	opts.HTTP.ProxyURL = ""
	opts.HTTP.Recursion = false
	opts.HTTP.RecursionDepth = 0
	opts.HTTP.RecursionStrategy = "default"
	opts.HTTP.ReplayProxyURL = ""
	opts.HTTP.Timeout = 10
	opts.HTTP.SNI = ""
	opts.HTTP.URL = ""
	opts.HTTP.Http2 = false
	opts.Input.DirSearchCompat = false
	opts.Input.Extensions = ""
	opts.Input.IgnoreWordlistComments = false
	opts.Input.InputMode = "clusterbomb"
	opts.Input.InputNum = 100
	opts.Input.InputShell = ""
	opts.Input.Inputcommands = make(MultiStringFlag, 0, 2)
	opts.Input.Request = ""
	opts.Input.RequestProto = "https"
	opts.Input.Wordlists = make(WordlistFlag, 0, 2)
	opts.Matcher.Mode = "or"
	opts.Matcher.Lines = ""
	opts.Matcher.Regexp = ""
	opts.Matcher.Size = ""
	opts.Matcher.Status = "200,204,301,302,307,401,403,405,500"
	opts.Matcher.Time = ""
	opts.Matcher.Words = ""
	opts.Output.DebugLog = ""
	opts.Output.OutputDirectory = ""
	opts.Output.OutputFile = ""
	opts.Output.OutputFormat = "json"
	opts.Output.OutputSkipEmptyFile = false
	return opts
}

func NewConfig() *Config {
	var conf = new(Config)
	conf.AutoCalibrationKeyword = "FUZZ"
	conf.AutoCalibrationStrategy = "basic"
	conf.AutoCalibrationStrings = make([]string, 0)
	conf.CommandKeywords = make([]string, 0)
	conf.Data = ""
	conf.Delay = optRange{0, 0, false, false}
	conf.DirSearchCompat = false
	conf.Extensions = make([]string, 0)
	conf.FilterMode = "or"
	conf.FollowRedirects = false
	conf.Headers = make(map[string]string)
	conf.IgnoreWordlistComments = false
	conf.InputMode = "clusterbomb"
	conf.InputNum = 0
	conf.InputShell = ""
	conf.InputProviders = make([]InputProviderConfig, 0)
	conf.Json = false
	conf.MatcherMode = "or"
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
	conf.Http2 = false
	return conf
}
