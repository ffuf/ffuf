package ffuf

import (
	"context"
	"regexp"
)

// VarExtract names a variable to capture from a preflight/postflight response
// using the first capture group of Regex. The captured value is substituted into
// the keyword Name wherever it appears in later requests.
type VarExtract struct {
	Name  string `json:"name" toml:"name"`
	Regex string `json:"regex" toml:"regex"`
	// Compiled is the precompiled Regex. ConfigFromOptions sets it once so the hot
	// path never recompiles per request; nil when a VarExtract is built directly.
	Compiled *regexp.Regexp `json:"-" toml:"-"`
}

// PreflightConfig is one raw HTTP request file executed around the fuzzing
// request, with optional variable extractions from its response.
type PreflightConfig struct {
	RequestFile string       `json:"request_file" toml:"request_file"`
	Vars        []VarExtract `json:"vars" toml:"vars"`
}

type Config struct {
	AuditLog                  string                `json:"auditlog"`
	AutoCalibration           bool                  `json:"autocalibration"`
	AutoCalibrationKeyword    string                `json:"autocalibration_keyword"`
	AutoCalibrationPerHost    bool                  `json:"autocalibration_perhost"`
	AutoCalibrationStrategies []string              `json:"autocalibration_strategies"`
	AutoCalibrationStrings    []string              `json:"autocalibration_strings"`
	Cancel                    context.CancelFunc    `json:"-"`
	Colors                    bool                  `json:"colors"`
	CommandKeywords           []string              `json:"-"`
	CommandLine               string                `json:"cmdline"`
	ConfigFile                string                `json:"configfile"`
	Context                   context.Context       `json:"-"`
	Data                      string                `json:"postdata"`
	Debuglog                  string                `json:"debuglog"`
	Delay                     optRange              `json:"delay"`
	DirSearchCompat           bool                  `json:"dirsearch_compatibility"`
	Encoders                  []string              `json:"encoders"`
	Extensions                []string              `json:"extensions"`
	FilterMode                string                `json:"fmode"`
	FollowRedirects           bool                  `json:"follow_redirects"`
	Headers                   map[string]string     `json:"headers"`
	IgnoreBody                bool                  `json:"ignorebody"`
	IgnoreWordlistComments    bool                  `json:"ignore_wordlist_comments"`
	InputMode                 string                `json:"inputmode"`
	InputNum                  int                   `json:"cmd_inputnum"`
	InputProviders            []InputProviderConfig `json:"inputproviders"`
	InputShell                string                `json:"inputshell"`
	Json                      bool                  `json:"json"`
	MatcherManager            MatcherManager        `json:"matchers"`
	MatcherMode               string                `json:"mmode"`
	MaxTime                   int                   `json:"maxtime"`
	MaxTimeJob                int                   `json:"maxtime_job"`
	Method                    string                `json:"method"`
	Noninteractive            bool                  `json:"noninteractive"`
	OutputDirectory           string                `json:"outputdirectory"`
	OutputFile                string                `json:"outputfile"`
	OutputFormat              string                `json:"outputformat"`
	OutputSkipEmptyFile       bool                  `json:"OutputSkipEmptyFile"`
	ProgressFrequency         int                   `json:"-"`
	ProxyURL                  string                `json:"proxyurl"`
	Quiet                     bool                  `json:"quiet"`
	Rate                      int64                 `json:"rate"`
	Raw                       bool                  `json:"raw"`
	Recursion                 bool                  `json:"recursion"`
	RecursionDepth            int                   `json:"recursion_depth"`
	RecursionStrategy         string                `json:"recursion_strategy"`
	ReplayProxyURL            string                `json:"replayproxyurl"`
	RequestFile               string                `json:"requestfile"`
	RequestProto              string                `json:"requestproto"`
	ScraperFile               string                `json:"scraperfile"`
	Scrapers                  string                `json:"scrapers"`
	SNI                       string                `json:"sni"`
	StopOn403                 bool                  `json:"stop_403"`
	StopOnAll                 bool                  `json:"stop_all"`
	StopOnErrors              bool                  `json:"stop_errors"`
	Threads                   int                   `json:"threads"`
	Timeout                   int                   `json:"timeout"`
	Url                       string                `json:"url"`
	Verbose                   bool                  `json:"verbose"`
	Wordlists                 []string              `json:"wordlists"`
	Http2                     bool                  `json:"http2"`
	ClientCert                string                `json:"client-cert"`
	ClientKey                 string                `json:"client-key"`
	Preflights                []PreflightConfig     `json:"preflights"`
	Postflights               []PreflightConfig     `json:"postflights"`
	PreflightMode             string                `json:"preflight_mode"`
	PreflightError            string                `json:"preflight_error"`
	// RateLimitFunc blocks until the shared rate limiter allows another request.
	// The engine sets it so preflight/postflight requests (sent from the runner,
	// outside the dispatch loop) also honor -rate and -p. Nil means unmetered.
	RateLimitFunc func() `json:"-" toml:"-"`
	// Options retains the raw ConfigOptions this Config was built from, so the
	// configuration can be re-serialized (FFUFHASH history and similar features)
	// directly, with no hand-maintained reverse mapper. Set by ConfigFromOptions;
	// nil for a Config assembled by other means.
	Options *ConfigOptions `json:"-"`
}

type InputProviderConfig struct {
	Name     string `json:"name"`
	Keyword  string `json:"keyword"`
	Value    string `json:"value"`
	Encoders string `json:"encoders"`
	Template string `json:"template"` // the templating string used for sniper mode (usually "§")
}

// NewConfig returns a Config ready for ConfigFromOptions to populate. It sets ONLY
// what ConfigFromOptions does not: the context, non-nil slice/map fields (which are
// appended to or indexed during parsing), and the few defaults for fields that are
// not copied from the options. Every other default lives once in NewConfigOptions
// and flows in through ConfigFromOptions — listing it here too would just duplicate
// that single source.
func NewConfig(ctx context.Context, cancel context.CancelFunc) Config {
	var conf Config
	conf.Context = ctx
	conf.Cancel = cancel

	conf.AutoCalibrationStrings = make([]string, 0)
	conf.CommandKeywords = make([]string, 0)
	conf.Encoders = make([]string, 0)
	conf.Extensions = make([]string, 0)
	conf.Headers = make(map[string]string)
	conf.InputProviders = make([]InputProviderConfig, 0)
	conf.Wordlists = []string{}

	conf.AutoCalibrationKeyword = "FUZZ"
	conf.Method = "GET"
	conf.ProgressFrequency = 125
	conf.RequestProto = "https"
	return conf
}

func (c *Config) SetContext(ctx context.Context, cancel context.CancelFunc) {
	c.Context = ctx
	c.Cancel = cancel
}
