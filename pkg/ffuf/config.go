package ffuf

import (
	"context"
)

type Config struct {
	StaticHeaders map[string]string
	FuzzHeaders   map[string]string
	Method        string
	Url           string
	TLSSkipVerify bool
	Data          string
	Quiet         bool
	Colors        bool
	Wordlist      string
	Filters       []FilterProvider
	Matchers      []FilterProvider
	Threads       int
	Context       context.Context
}

func NewConfig(ctx context.Context) Config {
	var conf Config
	conf.Context = ctx
	conf.StaticHeaders = make(map[string]string)
	conf.FuzzHeaders = make(map[string]string)
	conf.Method = "GET"
	conf.Url = ""
	conf.TLSSkipVerify = false
	conf.Data = ""
	conf.Quiet = false
	conf.Filters = make([]FilterProvider, 0)
	return conf
}
