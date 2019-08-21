package ffuf

import (
	"context"
	"net/http"
	"net/url"
)

//optRange stores either a single float, in which case the value is stored in min and IsRange is false,
//or a range of floats, in which case IsRange is true
type optRange struct {
	Min      float64
	Max      float64
	IsRange  bool
	HasDelay bool
}

type Config struct {
	StaticHeaders        map[string]string
	FuzzHeaders          map[string]string
	Extensions           []string
	DirSearchCompat      bool
	Method               string
	Url                  string
	TLSVerify            bool
	Data                 string
	Quiet                bool
	Colors               bool
	Wordlist             string
	InputCommand         string
	InputNum             int
	OutputFile           string
	OutputFormat         string
	StopOn403            bool
	StopOnErrors         bool
	StopOnAll            bool
	FollowRedirects      bool
	AutoCalibration      bool
	ShowRedirectLocation bool
	Timeout              int
	ProgressFrequency    int
	Delay                optRange
	Filters              []FilterProvider
	Matchers             []FilterProvider
	Threads              int
	Context              context.Context
	ProxyURL             func(*http.Request) (*url.URL, error)
	CommandLine          string
}

func NewConfig(ctx context.Context) Config {
	var conf Config
	conf.Context = ctx
	conf.StaticHeaders = make(map[string]string)
	conf.FuzzHeaders = make(map[string]string)
	conf.Method = "GET"
	conf.Url = ""
	conf.TLSVerify = false
	conf.Data = ""
	conf.Quiet = false
	conf.StopOn403 = false
	conf.StopOnErrors = false
	conf.StopOnAll = false
	conf.ShowRedirectLocation = false
	conf.FollowRedirects = false
	conf.InputCommand = ""
	conf.InputNum = 0
	conf.ProxyURL = http.ProxyFromEnvironment
	conf.Filters = make([]FilterProvider, 0)
	conf.Delay = optRange{0, 0, false, false}
	conf.Extensions = make([]string, 0)
	conf.Timeout = 10
	// Progress update frequency, in milliseconds
	conf.ProgressFrequency = 100
	conf.DirSearchCompat = false
	return conf
}

type CliOptions struct {
	extensions    string
	delay         string
	filterStatus  string
	filterSize    string
	filterRegexp  string
	filterWords   string
	matcherStatus string
	matcherSize   string
	matcherRegexp string
	matcherWords  string
	proxyURL      string
	outputFormat  string
	headers       multiStringFlag
	showVersion   bool
}

type multiStringFlag []string

func (m *multiStringFlag) String() string {
	return ""
}

func (m *multiStringFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}
