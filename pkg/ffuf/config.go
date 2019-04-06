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
	StaticHeaders   map[string]string
	FuzzHeaders     map[string]string
	Method          string
	Url             string
	TLSSkipVerify   bool
	Data            string
	Quiet           bool
	Colors          bool
	Wordlist        string
	OutputFile      string
	OutputFormat    string
	StopOn403       bool
	StopOnErrors    bool
	StopOnAll       bool
	FollowRedirects bool
	Delay           optRange
	Filters         []FilterProvider
	Matchers        []FilterProvider
	Threads         int
	Context         context.Context
	ProxyURL        func(*http.Request) (*url.URL, error)
	CommandLine     string
}

func NewConfig(ctx context.Context) Config {
	var conf Config
	conf.Context = ctx
	conf.StaticHeaders = make(map[string]string)
	conf.FuzzHeaders = make(map[string]string)
	conf.Method = "GET"
	conf.Url = ""
	conf.TLSSkipVerify = true
	conf.Data = ""
	conf.Quiet = false
	conf.StopOn403 = false
	conf.StopOnErrors = false
	conf.StopOnAll = false
	conf.FollowRedirects = false
	conf.ProxyURL = http.ProxyFromEnvironment
	conf.Filters = make([]FilterProvider, 0)
	conf.Delay = optRange{0, 0, false, false}
	return conf
}
