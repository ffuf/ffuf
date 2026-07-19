package ffuf

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml"
)

type ConfigOptions struct {
	Filter  FilterOptions  `json:"filters"`
	General GeneralOptions `json:"general"`
	HTTP    HTTPOptions    `json:"http"`
	Input   InputOptions   `json:"input"`
	Matcher MatcherOptions `json:"matchers"`
	Output  OutputOptions  `json:"output"`
}

// The `ffuf`, `section`, `usage`, `kind` and `alias` struct tags are the single
// source of truth for CLI flags (see flags.go / RegisterFlags). A field with an
// `ffuf` tag becomes a flag; the help section, usage text, (for []string) the value
// kind, and any compatibility aliases come from the tags. The only flags NOT backed
// by a field are the dummy `copy as curl` compat flags (-i/-k/-compressed), declared
// explicitly in flags.go because they have no value to bind.
type HTTPOptions struct {
	Cookies           []string `json:"-" ffuf:"b" alias:"cookie" kind:"multistring" section:"http" usage:"Cookie data \"NAME1=VALUE1; NAME2=VALUE2\" for copy as curl functionality."`
	Data              string   `json:"data" ffuf:"d" alias:"data,data-ascii,data-binary" section:"http" usage:"POST data"`
	FollowRedirects   bool     `json:"follow_redirects" ffuf:"r" section:"http" usage:"Follow redirects"`
	Headers           []string `json:"headers" ffuf:"H" kind:"multistring" section:"http" usage:"Header \"Name: Value\", separated by colon. Multiple -H flags are accepted."`
	IgnoreBody        bool     `json:"ignore_body" ffuf:"ignore-body" section:"http" usage:"Do not fetch the response content."`
	Method            string   `json:"method" ffuf:"X" section:"http" usage:"HTTP method to use"`
	ProxyURL          string   `json:"proxy_url" ffuf:"x" section:"http" usage:"Proxy URL (SOCKS5 or HTTP). For example: http://127.0.0.1:8080 or socks5://127.0.0.1:8080"`
	Raw               bool     `json:"raw" ffuf:"raw" section:"http" usage:"Do not encode URI"`
	Recursion         bool     `json:"recursion" ffuf:"recursion" section:"http" usage:"Scan recursively. Only FUZZ keyword is supported, and URL (-u) has to end in it."`
	RecursionDepth    int      `json:"recursion_depth" ffuf:"recursion-depth" section:"http" usage:"Maximum recursion depth."`
	RecursionStrategy string   `json:"recursion_strategy" ffuf:"recursion-strategy" section:"http" usage:"Recursion strategy: \"default\" for a redirect based, and \"greedy\" to recurse on all matches"`
	ReplayProxyURL    string   `json:"replay_proxy_url" ffuf:"replay-proxy" section:"http" usage:"Replay matched requests using this proxy."`
	SNI               string   `json:"sni" ffuf:"sni" section:"http" usage:"Target TLS SNI, does not support FUZZ keyword"`
	Timeout           int      `json:"timeout" ffuf:"timeout" section:"http" usage:"HTTP request timeout in seconds."`
	URL               string   `json:"url" ffuf:"u" section:"http" usage:"Target URL"`
	Http2             bool     `json:"http2" ffuf:"http2" section:"http" usage:"Use HTTP2 protocol"`
	ClientCert        string   `json:"client-cert" ffuf:"cc" section:"http" usage:"Client cert for authentication. Client key needs to be defined as well for this to work"`
	ClientKey         string   `json:"client-key" ffuf:"ck" section:"http" usage:"Client key for authentication. Client certificate needs to be defined as well for this to work"`
	// Preflights/Postflights are not plain flags: -preflight and -preflight-var
	// bind positionally (a -preflight-var attaches to the preceding -preflight), so
	// they are appended by the extraFlags Func callbacks in flags.go rather than a
	// tagged field. The json/toml tags carry them through config files and history.
	Preflights     []PreflightConfig `json:"preflights" toml:"preflights"`
	Postflights    []PreflightConfig `json:"postflights" toml:"postflights"`
	PreflightMode  string            `json:"preflight_mode" toml:"preflight_mode" ffuf:"preflight-mode" section:"http" usage:"Preflight execution mode: \"per-request\" or \"per-thread\""`
	PreflightError string            `json:"preflight_error" toml:"preflight_error" ffuf:"preflight-error" section:"http" usage:"Preflight error handling: \"abort\" or \"ignore\""`
}

type GeneralOptions struct {
	AutoCalibration           bool     `json:"autocalibration" ffuf:"ac" section:"general" usage:"Automatically calibrate filtering options"`
	AutoCalibrationKeyword    string   `json:"autocalibration_keyword" ffuf:"ack" section:"general" usage:"Autocalibration keyword"`
	AutoCalibrationPerHost    bool     `json:"autocalibration_per_host" ffuf:"ach" section:"general" usage:"Per host autocalibration"`
	AutoCalibrationStrategies []string `json:"autocalibration_strategies" ffuf:"acs" kind:"csvreplace" section:"general" usage:"Custom auto-calibration strategies. Can be used multiple times. Implies -ac"`
	AutoCalibrationStrings    []string `json:"autocalibration_strings" ffuf:"acc" kind:"multistring" section:"general" usage:"Custom auto-calibration string. Can be used multiple times. Implies -ac"`
	Colors                    bool     `json:"colors" ffuf:"c" section:"general" usage:"Colorize output."`
	ConfigFile                string   `toml:"-" json:"config_file" ffuf:"config" section:"general" usage:"Load configuration from a file"`
	Delay                     string   `json:"delay" ffuf:"p" section:"general" usage:"Seconds of delay between requests, or a range of random delay. For example \"0.1\" or \"0.1-2.0\""`
	Json                      bool     `json:"json" ffuf:"json" section:"general" usage:"JSON output, printing newline-delimited JSON records"`
	MaxTime                   int      `json:"maxtime" ffuf:"maxtime" section:"general" usage:"Maximum running time in seconds for entire process."`
	MaxTimeJob                int      `json:"maxtime_job" ffuf:"maxtime-job" section:"general" usage:"Maximum running time in seconds per job."`
	Noninteractive            bool     `json:"noninteractive" ffuf:"noninteractive" section:"general" usage:"Disable the interactive console functionality"`
	Quiet                     bool     `json:"quiet" ffuf:"s" section:"general" usage:"Do not print additional information (silent mode)"`
	Rate                      int      `json:"rate" ffuf:"rate" section:"general" usage:"Rate of requests per second"`
	ScraperFile               string   `json:"scraperfile" ffuf:"scraperfile" section:"general" usage:"Custom scraper file path"`
	Scrapers                  string   `json:"scrapers" ffuf:"scrapers" section:"general" usage:"Active scraper groups"`
	Searchhash                string   `json:"-" ffuf:"search" section:"general" usage:"Search for a FFUFHASH payload from ffuf history"`
	ShowVersion               bool     `toml:"-" json:"-" ffuf:"V" section:"general" usage:"Show version information."`
	StopOn403                 bool     `json:"stop_on_403" ffuf:"sf" section:"general" usage:"Stop when > 95% of responses return 403 Forbidden"`
	StopOnAll                 bool     `json:"stop_on_all" ffuf:"sa" section:"general" usage:"Stop on all error cases. Implies -sf and -se."`
	StopOnErrors              bool     `json:"stop_on_errors" ffuf:"se" section:"general" usage:"Stop on spurious errors"`
	Threads                   int      `json:"threads" ffuf:"t" section:"general" usage:"Number of concurrent threads."`
	Verbose                   bool     `json:"verbose" ffuf:"v" section:"general" usage:"Verbose output, printing full URL and redirect location (if any) with the results."`
}

type InputOptions struct {
	DirSearchCompat        bool     `json:"dirsearch_compat" ffuf:"D" section:"input" usage:"DirSearch wordlist compatibility mode. Used in conjunction with -e flag."`
	Encoders               []string `json:"encoders" ffuf:"enc" kind:"wordlist" section:"input" usage:"Encoders for keywords, eg. 'FUZZ:urlencode b64encode'"`
	Extensions             string   `json:"extensions" ffuf:"e" section:"input" usage:"Comma separated list of extensions. Extends FUZZ keyword."`
	IgnoreWordlistComments bool     `json:"ignore_wordlist_comments" ffuf:"ic" section:"input" usage:"Ignore wordlist comments"`
	InputMode              string   `json:"input_mode" ffuf:"mode" section:"input" usage:"Multi-wordlist operation mode. Available modes: clusterbomb, pitchfork, sniper"`
	InputNum               int      `json:"input_num" ffuf:"input-num" section:"input" usage:"Number of inputs to test. Used in conjunction with --input-cmd."`
	InputShell             string   `json:"input_shell" ffuf:"input-shell" section:"input" usage:"Shell to be used for running command"`
	Inputcommands          []string `json:"input_commands" ffuf:"input-cmd" kind:"multistring" section:"input" usage:"Command producing the input. --input-num is required when using this input method. Overrides -w."`
	Request                string   `json:"request_file" ffuf:"request" section:"input" usage:"File containing the raw http request"`
	RequestProto           string   `json:"request_proto" ffuf:"request-proto" section:"input" usage:"Protocol to use along with raw request"`
	Wordlists              []string `json:"wordlists" ffuf:"w" kind:"wordlist" section:"input" usage:"Wordlist file path and (optional) keyword separated by colon. eg. '/path/to/wordlist:KEYWORD'"`
}

type OutputOptions struct {
	AuditLog            string `json:"audit_log" ffuf:"audit-log" section:"output" usage:"Write audit log containing all requests, responses and config"`
	DebugLog            string `json:"debug_log" ffuf:"debug-log" section:"output" usage:"Write all of the internal logging to the specified file."`
	OutputDirectory     string `json:"output_directory" ffuf:"od" section:"output" usage:"Directory path to store matched results to."`
	OutputFile          string `json:"output_file" ffuf:"o" section:"output" usage:"Write output to file"`
	OutputFormat        string `json:"output_format" ffuf:"of" section:"output" usage:"Output file format. Available formats: json, ejson, html, md, csv, ecsv (or, 'all' for all formats)"`
	OutputSkipEmptyFile bool   `json:"output_skip_empty" ffuf:"or" section:"output" usage:"Don't create the output file if we don't have results"`
}

type FilterOptions struct {
	Mode   string `json:"mode" ffuf:"fmode" section:"filter" usage:"Filter set operator. Either of: and, or"`
	Lines  string `json:"lines" ffuf:"fl" section:"filter" usage:"Filter by amount of lines in response. Comma separated list of line counts and ranges"`
	Regexp string `json:"regexp" ffuf:"fr" section:"filter" usage:"Filter regexp"`
	Size   string `json:"size" ffuf:"fs" section:"filter" usage:"Filter HTTP response size. Comma separated list of sizes and ranges"`
	Status string `json:"status" ffuf:"fc" section:"filter" usage:"Filter HTTP status codes from response. Comma separated list of codes and ranges"`
	Time   string `json:"time" ffuf:"ft" section:"filter" usage:"Filter by number of milliseconds to the first response byte, either greater or less than. EG: >100 or <100"`
	Words  string `json:"words" ffuf:"fw" section:"filter" usage:"Filter by amount of words in response. Comma separated list of word counts and ranges"`
}

type MatcherOptions struct {
	Mode   string `json:"mode" ffuf:"mmode" section:"matcher" usage:"Matcher set operator. Either of: and, or"`
	Lines  string `json:"lines" ffuf:"ml" section:"matcher" usage:"Match amount of lines in response"`
	Regexp string `json:"regexp" ffuf:"mr" section:"matcher" usage:"Match regexp"`
	Size   string `json:"size" ffuf:"ms" section:"matcher" usage:"Match HTTP response size"`
	Status string `json:"status" ffuf:"mc" section:"matcher" usage:"Match HTTP status codes, or \"all\" for everything."`
	Time   string `json:"time" ffuf:"mt" section:"matcher" usage:"Match how many milliseconds to the first response byte, either greater or less than. EG: >100 or <100"`
	Words  string `json:"words" ffuf:"mw" section:"matcher" usage:"Match amount of words in response"`
}

// NewConfigOptions returns a newly created ConfigOptions struct with default values
func NewConfigOptions() *ConfigOptions {
	c := &ConfigOptions{}
	c.Filter.Mode = "or"
	c.Filter.Lines = ""
	c.Filter.Regexp = ""
	c.Filter.Size = ""
	c.Filter.Status = ""
	c.Filter.Time = ""
	c.Filter.Words = ""
	c.General.AutoCalibration = false
	c.General.AutoCalibrationKeyword = "FUZZ"
	c.General.AutoCalibrationStrategies = []string{"basic"}
	c.General.Colors = false
	c.General.Delay = ""
	c.General.Json = false
	c.General.MaxTime = 0
	c.General.MaxTimeJob = 0
	c.General.Noninteractive = false
	c.General.Quiet = false
	c.General.Rate = 0
	c.General.Searchhash = ""
	c.General.ScraperFile = ""
	c.General.Scrapers = "all"
	c.General.ShowVersion = false
	c.General.StopOn403 = false
	c.General.StopOnAll = false
	c.General.StopOnErrors = false
	c.General.Threads = 40
	c.General.Verbose = false
	c.HTTP.Preflights = make([]PreflightConfig, 0)
	c.HTTP.Postflights = make([]PreflightConfig, 0)
	c.HTTP.PreflightMode = "per-request"
	c.HTTP.PreflightError = "abort"
	c.HTTP.Data = ""
	c.HTTP.FollowRedirects = false
	c.HTTP.IgnoreBody = false
	c.HTTP.Method = ""
	c.HTTP.ProxyURL = ""
	c.HTTP.Raw = false
	c.HTTP.Recursion = false
	c.HTTP.RecursionDepth = 0
	c.HTTP.RecursionStrategy = "default"
	c.HTTP.ReplayProxyURL = ""
	c.HTTP.Timeout = 10
	c.HTTP.SNI = ""
	c.HTTP.URL = ""
	c.HTTP.Http2 = false
	c.Input.DirSearchCompat = false
	c.Input.Encoders = []string{}
	c.Input.Extensions = ""
	c.Input.IgnoreWordlistComments = false
	c.Input.InputMode = "clusterbomb"
	c.Input.InputNum = 100
	c.Input.Request = ""
	c.Input.RequestProto = "https"
	c.Matcher.Mode = "or"
	c.Matcher.Lines = ""
	c.Matcher.Regexp = ""
	c.Matcher.Size = ""
	c.Matcher.Status = "200-299,301,302,307,401,403,405,500"
	c.Matcher.Time = ""
	c.Matcher.Words = ""
	c.Output.AuditLog = ""
	c.Output.DebugLog = ""
	c.Output.OutputDirectory = ""
	c.Output.OutputFile = ""
	c.Output.OutputFormat = "json"
	c.Output.OutputSkipEmptyFile = false
	return c
}

// cloneStrings returns a copy of s with its own backing array (nil stays nil), so
// the retained options snapshot can't be mutated through the caller's slices.
func cloneStrings(s []string) []string {
	if s == nil {
		return nil
	}
	return append([]string(nil), s...)
}

// clonePreflights deep-copies a preflight/postflight slice, including each entry's
// Vars slice, so the retained options snapshot shares no backing array with the
// caller's options (matching the cloneStrings invariant for the other slice fields).
func clonePreflights(in []PreflightConfig) []PreflightConfig {
	if in == nil {
		return nil
	}
	out := make([]PreflightConfig, len(in))
	for i, pf := range in {
		out[i] = pf
		out[i].Vars = append([]VarExtract(nil), pf.Vars...)
	}
	return out
}

// ConfigFromOptions parses the values in ConfigOptions struct, ensures that the values are sane,
// and creates a Config struct out of them.
func ConfigFromOptions(parseOpts *ConfigOptions, ctx context.Context, cancel context.CancelFunc) (*Config, error) {
	//TODO: refactor in a proper flag library that can handle things like required flags
	errs := NewMultierror()
	conf := NewConfig(ctx, cancel)

	var err error
	var err2 error
	if len(parseOpts.HTTP.URL) == 0 && parseOpts.Input.Request == "" {
		errs.Add(fmt.Errorf("-u flag or -request flag is required"))
	}

	// prepare extensions
	if parseOpts.Input.Extensions != "" {
		extensions := strings.Split(parseOpts.Input.Extensions, ",")
		conf.Extensions = extensions
	}

	// Effective request headers: the -H values plus any -b/-cookie folded in. Built
	// as a fresh slice so ConfigFromOptions never mutates the caller's options (it
	// stays idempotent) and the retained snapshot below shares no backing with it.
	effectiveHeaders := append([]string(nil), parseOpts.HTTP.Headers...)
	if len(parseOpts.HTTP.Cookies) > 0 {
		effectiveHeaders = append(effectiveHeaders, "Cookie: "+strings.Join(parseOpts.HTTP.Cookies, "; "))
	}

	//Prepare inputproviders
	conf.InputMode = parseOpts.Input.InputMode

	validmode := false
	for _, mode := range []string{"clusterbomb", "pitchfork", "sniper"} {
		if conf.InputMode == mode {
			validmode = true
		}
	}
	if !validmode {
		errs.Add(fmt.Errorf("Input mode (-mode) %s not recognized", conf.InputMode))
	}

	template := ""
	// sniper mode needs some additional checking
	if conf.InputMode == "sniper" {
		template = "§"

		if len(parseOpts.Input.Wordlists) > 1 {
			errs.Add(fmt.Errorf("sniper mode only supports one wordlist"))
		}

		if len(parseOpts.Input.Inputcommands) > 1 {
			errs.Add(fmt.Errorf("sniper mode only supports one input command"))
		}
	}
	tmpEncoders := make(map[string]string)
	for _, e := range parseOpts.Input.Encoders {
		if strings.Contains(e, ":") {
			key := strings.Split(e, ":")[0]
			val := strings.Split(e, ":")[1]
			tmpEncoders[key] = val
		}
	}
	tmpWordlists := make([]string, 0)
	for _, v := range parseOpts.Input.Wordlists {
		var wl []string
		if runtime.GOOS == "windows" {
			// Try to ensure that Windows file paths like C:\path\to\wordlist.txt:KEYWORD are treated properly
			if FileExists(v) {
				// The wordlist was supplied without a keyword parameter
				wl = []string{v}
			} else {
				filepart := v
				if strings.Contains(filepart, ":") {
					filepart = v[:strings.LastIndex(filepart, ":")]
				}

				if FileExists(filepart) {
					wl = []string{filepart, v[strings.LastIndex(v, ":")+1:]}
				} else {
					// The file was not found. Use full wordlist parameter value for more concise error message down the line
					wl = []string{v}
				}
			}
		} else {
			wl = strings.SplitN(v, ":", 2)
		}
		// Try to use absolute paths for wordlists
		fullpath := ""
		if wl[0] != "-" {
			fullpath, err = filepath.Abs(wl[0])
		} else {
			fullpath = wl[0]
		}

		if err == nil {
			wl[0] = fullpath
		}
		if len(wl) == 2 {
			if conf.InputMode == "sniper" {
				errs.Add(fmt.Errorf("sniper mode does not support wordlist keywords"))
			} else {
				newp := InputProviderConfig{
					Name:    "wordlist",
					Value:   wl[0],
					Keyword: wl[1],
				}
				// Add encoders if set
				enc, ok := tmpEncoders[wl[1]]
				if ok {
					newp.Encoders = enc
				}
				conf.InputProviders = append(conf.InputProviders, newp)
			}
		} else {
			newp := InputProviderConfig{
				Name:     "wordlist",
				Value:    wl[0],
				Keyword:  "FUZZ",
				Template: template,
			}
			// Add encoders if set
			enc, ok := tmpEncoders["FUZZ"]
			if ok {
				newp.Encoders = enc
			}
			conf.InputProviders = append(conf.InputProviders, newp)
		}
		tmpWordlists = append(tmpWordlists, strings.Join(wl, ":"))
	}
	conf.Wordlists = tmpWordlists

	for _, v := range parseOpts.Input.Inputcommands {
		ic := strings.SplitN(v, ":", 2)
		if len(ic) == 2 {
			if conf.InputMode == "sniper" {
				errs.Add(fmt.Errorf("sniper mode does not support command keywords"))
			} else {
				newp := InputProviderConfig{
					Name:    "command",
					Value:   ic[0],
					Keyword: ic[1],
				}
				enc, ok := tmpEncoders[ic[1]]
				if ok {
					newp.Encoders = enc
				}
				conf.InputProviders = append(conf.InputProviders, newp)
				conf.CommandKeywords = append(conf.CommandKeywords, ic[0])
			}
		} else {
			newp := InputProviderConfig{
				Name:     "command",
				Value:    ic[0],
				Keyword:  "FUZZ",
				Template: template,
			}
			enc, ok := tmpEncoders["FUZZ"]
			if ok {
				newp.Encoders = enc
			}
			conf.InputProviders = append(conf.InputProviders, newp)
			conf.CommandKeywords = append(conf.CommandKeywords, "FUZZ")
		}
	}

	if len(conf.InputProviders) == 0 {
		errs.Add(fmt.Errorf("Either -w or --input-cmd flag is required"))
	}

	// Prepare the request using body
	if parseOpts.Input.Request != "" {
		err := parseRawRequest(parseOpts, &conf)
		if err != nil {
			errmsg := fmt.Sprintf("Could not parse raw request: %s", err)
			errs.Add(fmt.Errorf("%s", errmsg))
		}
	}

	//Prepare URL
	if parseOpts.HTTP.URL != "" {
		conf.Url = parseOpts.HTTP.URL
	}

	// Prepare SNI
	if parseOpts.HTTP.SNI != "" {
		conf.SNI = parseOpts.HTTP.SNI
	}

	// prepare cert
	if parseOpts.HTTP.ClientCert != "" {
		conf.ClientCert = parseOpts.HTTP.ClientCert
	}
	if parseOpts.HTTP.ClientKey != "" {
		conf.ClientKey = parseOpts.HTTP.ClientKey
	}

	//Prepare headers and make canonical
	for _, v := range effectiveHeaders {
		hs := strings.SplitN(v, ":", 2)
		if len(hs) == 2 {
			// trim and make canonical
			// except if used in custom defined header
			var CanonicalNeeded = true
			for _, a := range conf.CommandKeywords {
				if strings.Contains(hs[0], a) {
					CanonicalNeeded = false
				}
			}
			// check if part of InputProviders
			if CanonicalNeeded {
				for _, b := range conf.InputProviders {
					if strings.Contains(hs[0], b.Keyword) {
						CanonicalNeeded = false
					}
				}
			}
			if CanonicalNeeded {
				var CanonicalHeader = textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(hs[0]))
				conf.Headers[CanonicalHeader] = strings.TrimSpace(hs[1])
			} else {
				conf.Headers[strings.TrimSpace(hs[0])] = strings.TrimSpace(hs[1])
			}
		} else {
			errs.Add(fmt.Errorf("Header defined by -H needs to have a value. \":\" should be used as a separator"))
		}
	}

	//Prepare delay
	d := strings.Split(parseOpts.General.Delay, "-")
	if len(d) > 2 {
		errs.Add(fmt.Errorf("Delay needs to be either a single float: \"0.1\" or a range of floats, delimited by dash: \"0.1-0.8\""))
	} else if len(d) == 2 {
		conf.Delay.IsRange = true
		conf.Delay.HasDelay = true
		conf.Delay.Min, err = strconv.ParseFloat(d[0], 64)
		conf.Delay.Max, err2 = strconv.ParseFloat(d[1], 64)
		if err != nil || err2 != nil {
			errs.Add(fmt.Errorf("Delay range min and max values need to be valid floats. For example: 0.1-0.5"))
		}
	} else if len(parseOpts.General.Delay) > 0 {
		conf.Delay.IsRange = false
		conf.Delay.HasDelay = true
		conf.Delay.Min, err = strconv.ParseFloat(parseOpts.General.Delay, 64)
		if err != nil {
			errs.Add(fmt.Errorf("Delay needs to be either a single float: \"0.1\" or a range of floats, delimited by dash: \"0.1-0.8\""))
		}
	}

	// Verify proxy url format
	if len(parseOpts.HTTP.ProxyURL) > 0 {
		u, err := url.Parse(parseOpts.HTTP.ProxyURL)
		if err != nil || u.Opaque != "" || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5") {
			errs.Add(fmt.Errorf("Bad proxy url (-x) format. Expected http, https or socks5 url"))
		} else {
			conf.ProxyURL = parseOpts.HTTP.ProxyURL
		}
	}

	// Verify replayproxy url format
	if len(parseOpts.HTTP.ReplayProxyURL) > 0 {
		u, err := url.Parse(parseOpts.HTTP.ReplayProxyURL)
		if err != nil || u.Opaque != "" || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" && u.Scheme != "socks5h") {
			errs.Add(fmt.Errorf("Bad replay-proxy url (-replay-proxy) format. Expected http, https or socks5 url"))
		} else {
			conf.ReplayProxyURL = parseOpts.HTTP.ReplayProxyURL
		}
	}

	//Check the output file format option
	if parseOpts.Output.OutputFile != "" {
		//No need to check / error out if output file isn't defined
		outputFormats := []string{"all", "json", "ejson", "html", "md", "csv", "ecsv"}
		found := false
		for _, f := range outputFormats {
			if f == parseOpts.Output.OutputFormat {
				conf.OutputFormat = f
				found = true
			}
		}
		if !found {
			errs.Add(fmt.Errorf("Unknown output file format (-of): %s", parseOpts.Output.OutputFormat))
		}
	}

	// Auto-calibration strings
	if len(parseOpts.General.AutoCalibrationStrings) > 0 {
		conf.AutoCalibrationStrings = parseOpts.General.AutoCalibrationStrings
	}
	// Auto-calibration strategies
	if len(parseOpts.General.AutoCalibrationStrategies) > 0 {
		conf.AutoCalibrationStrategies = parseOpts.General.AutoCalibrationStrategies
	}
	// Using -acc implies -ac
	if len(parseOpts.General.AutoCalibrationStrings) > 0 {
		conf.AutoCalibration = true
	}
	// Using -acs implies -ac
	if len(parseOpts.General.AutoCalibrationStrategies) > 0 {
		conf.AutoCalibration = true
	}

	if parseOpts.General.Rate < 0 {
		conf.Rate = 0
	} else {
		conf.Rate = int64(parseOpts.General.Rate)
	}

	if conf.Method == "" {
		if parseOpts.HTTP.Method == "" {
			// Only set if defined on command line, because we might be reparsing the CLI after
			// populating it through raw request in the first iteration
			conf.Method = "GET"
		} else {
			conf.Method = parseOpts.HTTP.Method
		}
	} else {
		if parseOpts.HTTP.Method != "" {
			// Method overridden in CLI
			conf.Method = parseOpts.HTTP.Method
		}
	}

	if parseOpts.HTTP.Data != "" {
		// Only set if defined on command line, because we might be reparsing the CLI after
		// populating it through raw request in the first iteration
		conf.Data = parseOpts.HTTP.Data
	}

	// Common stuff
	conf.IgnoreWordlistComments = parseOpts.Input.IgnoreWordlistComments
	conf.DirSearchCompat = parseOpts.Input.DirSearchCompat
	conf.Colors = parseOpts.General.Colors
	conf.InputNum = parseOpts.Input.InputNum

	conf.InputShell = parseOpts.Input.InputShell
	conf.AuditLog = parseOpts.Output.AuditLog
	conf.OutputFile = parseOpts.Output.OutputFile
	conf.OutputDirectory = parseOpts.Output.OutputDirectory
	conf.OutputSkipEmptyFile = parseOpts.Output.OutputSkipEmptyFile
	conf.IgnoreBody = parseOpts.HTTP.IgnoreBody
	conf.Quiet = parseOpts.General.Quiet
	conf.ScraperFile = parseOpts.General.ScraperFile
	conf.Scrapers = parseOpts.General.Scrapers
	conf.StopOn403 = parseOpts.General.StopOn403
	conf.StopOnAll = parseOpts.General.StopOnAll
	conf.StopOnErrors = parseOpts.General.StopOnErrors
	conf.FollowRedirects = parseOpts.HTTP.FollowRedirects
	conf.Raw = parseOpts.HTTP.Raw
	conf.Recursion = parseOpts.HTTP.Recursion
	conf.RecursionDepth = parseOpts.HTTP.RecursionDepth
	conf.RecursionStrategy = parseOpts.HTTP.RecursionStrategy
	conf.AutoCalibration = parseOpts.General.AutoCalibration
	conf.AutoCalibrationPerHost = parseOpts.General.AutoCalibrationPerHost
	conf.AutoCalibrationStrategies = parseOpts.General.AutoCalibrationStrategies
	conf.Threads = parseOpts.General.Threads
	conf.Timeout = parseOpts.HTTP.Timeout
	conf.MaxTime = parseOpts.General.MaxTime
	conf.MaxTimeJob = parseOpts.General.MaxTimeJob
	conf.Noninteractive = parseOpts.General.Noninteractive
	conf.Verbose = parseOpts.General.Verbose
	conf.Json = parseOpts.General.Json
	conf.Http2 = parseOpts.HTTP.Http2
	conf.Preflights = parseOpts.HTTP.Preflights
	conf.Postflights = parseOpts.HTTP.Postflights

	switch parseOpts.HTTP.PreflightMode {
	case "", "per-request":
		conf.PreflightMode = "per-request"
	case "per-thread":
		conf.PreflightMode = "per-thread"
	default:
		errs.Add(fmt.Errorf("-preflight-mode must be \"per-request\" or \"per-thread\", got %q", parseOpts.HTTP.PreflightMode))
	}

	switch parseOpts.HTTP.PreflightError {
	case "", "abort":
		conf.PreflightError = "abort"
	case "ignore":
		conf.PreflightError = "ignore"
	default:
		errs.Add(fmt.Errorf("-preflight-error must be \"abort\" or \"ignore\", got %q", parseOpts.HTTP.PreflightError))
	}

	// Validate that each preflight/postflight file exists and precompile every
	// extraction regex once here (invalid regex is a config error, not a runtime
	// per-request abort; the runner reuses Compiled so the hot path never recompiles).
	compileFlights := func(kind string, flights []PreflightConfig) {
		for i := range flights {
			if _, err := os.Stat(flights[i].RequestFile); err != nil {
				errs.Add(fmt.Errorf("%s request file #%d %q: %s", kind, i+1, flights[i].RequestFile, err))
			}
			for j := range flights[i].Vars {
				re, cerr := regexp.Compile(flights[i].Vars[j].Regex)
				if cerr != nil {
					errs.Add(fmt.Errorf("%s #%d var %q: invalid regex %q: %s", kind, i+1, flights[i].Vars[j].Name, flights[i].Vars[j].Regex, cerr))
					continue
				}
				flights[i].Vars[j].Compiled = re
			}
		}
	}
	compileFlights("preflight", conf.Preflights)
	compileFlights("postflight", conf.Postflights)

	// Check that fmode and mmode have sane values
	valid_opmodes := []string{"and", "or"}
	fmode_found := false
	mmode_found := false
	for _, v := range valid_opmodes {
		if v == parseOpts.Filter.Mode {
			fmode_found = true
		}
		if v == parseOpts.Matcher.Mode {
			mmode_found = true
		}
	}
	if !fmode_found {
		errmsg := fmt.Sprintf("Unrecognized value for parameter fmode: %s, valid values are: and, or", parseOpts.Filter.Mode)
		errs.Add(fmt.Errorf("%s", errmsg))
	}
	if !mmode_found {
		errmsg := fmt.Sprintf("Unrecognized value for parameter mmode: %s, valid values are: and, or", parseOpts.Matcher.Mode)
		errs.Add(fmt.Errorf("%s", errmsg))
	}
	conf.FilterMode = parseOpts.Filter.Mode
	conf.MatcherMode = parseOpts.Matcher.Mode

	if conf.AutoCalibrationPerHost {
		// AutoCalibrationPerHost implies AutoCalibration
		conf.AutoCalibration = true
	}

	// Handle copy as curl situation where POST method is implied by --data flag. If method is set to anything but GET, NOOP
	if len(conf.Data) > 0 &&
		conf.Method == "GET" &&
		//don't modify the method automatically if a request file is being used as input
		len(parseOpts.Input.Request) == 0 {

		conf.Method = "POST"
	}

	conf.CommandLine = strings.Join(os.Args, " ")

	newInputProviders := []InputProviderConfig{}
	for _, provider := range conf.InputProviders {
		if provider.Template != "" {
			if !templatePresent(provider.Template, &conf) {
				errmsg := fmt.Sprintf("Template %s defined, but not found in pairs in headers, method, URL or POST data.", provider.Template)
				errs.Add(fmt.Errorf("%s", errmsg))
			} else {
				newInputProviders = append(newInputProviders, provider)
			}
		} else {
			if !keywordPresent(provider.Keyword, &conf) {
				errmsg := fmt.Sprintf("Keyword %s defined, but not found in headers, method, URL or POST data.", provider.Keyword)
				_, _ = fmt.Fprintf(os.Stderr, "%s\n", fmt.Errorf("%s", errmsg))
			} else {
				newInputProviders = append(newInputProviders, provider)
			}
		}
	}
	conf.InputProviders = newInputProviders

	// If sniper mode, ensure there is no FUZZ keyword
	if conf.InputMode == "sniper" {
		if keywordPresent("FUZZ", &conf) {
			errs.Add(fmt.Errorf("FUZZ keyword defined, but we are using sniper mode."))
		}
	}

	// Do checks for recursion mode
	if parseOpts.HTTP.Recursion {
		if !strings.HasSuffix(conf.Url, "FUZZ") {
			errmsg := "When using -recursion the URL (-u) must end with FUZZ keyword."
			errs.Add(fmt.Errorf("%s", errmsg))
		}
	}

	// Make verbose mutually exclusive with json
	if parseOpts.General.Verbose && parseOpts.General.Json {
		errs.Add(fmt.Errorf("Cannot have -json and -v"))
	}
	// Retain the source options so the configuration can be re-serialized later
	// (history / FFUFHASH) without a hand-maintained reverse mapper. Deep-copy the
	// slice fields so the retained snapshot shares no backing array with the caller's
	// options; a later mutation of either side can't corrupt the other. Headers holds
	// the effective set (with -b/-cookie folded in) computed above.
	optsCopy := *parseOpts
	optsCopy.HTTP.Headers = effectiveHeaders
	optsCopy.HTTP.Cookies = cloneStrings(parseOpts.HTTP.Cookies)
	optsCopy.Input.Wordlists = cloneStrings(parseOpts.Input.Wordlists)
	optsCopy.Input.Encoders = cloneStrings(parseOpts.Input.Encoders)
	optsCopy.Input.Inputcommands = cloneStrings(parseOpts.Input.Inputcommands)
	optsCopy.General.AutoCalibrationStrings = cloneStrings(parseOpts.General.AutoCalibrationStrings)
	optsCopy.General.AutoCalibrationStrategies = cloneStrings(parseOpts.General.AutoCalibrationStrategies)
	optsCopy.HTTP.Preflights = clonePreflights(parseOpts.HTTP.Preflights)
	optsCopy.HTTP.Postflights = clonePreflights(parseOpts.HTTP.Postflights)
	conf.Options = &optsCopy
	return &conf, errs.ErrorOrNil()
}

func parseRawRequest(parseOpts *ConfigOptions, conf *Config) error {
	conf.RequestFile = parseOpts.Input.Request
	conf.RequestProto = parseOpts.Input.RequestProto
	file, err := os.Open(parseOpts.Input.Request)
	if err != nil {
		return fmt.Errorf("could not open request file: %s", err)
	}
	defer file.Close()

	r := bufio.NewReader(file)

	s, err := r.ReadString('\n')
	if err != nil {
		return fmt.Errorf("could not read request: %s", err)
	}
	parts := strings.Split(s, " ")
	if len(parts) < 3 {
		return fmt.Errorf("malformed request supplied")
	}
	// Set the request Method
	conf.Method = parts[0]

	for {
		line, err := r.ReadString('\n')
		line = strings.TrimSpace(line)

		if err != nil || line == "" {
			break
		}

		p := strings.SplitN(line, ":", 2)
		if len(p) != 2 {
			continue
		}

		if strings.EqualFold(p[0], "content-length") {
			continue
		}

		conf.Headers[strings.TrimSpace(p[0])] = strings.TrimSpace(p[1])
	}

	// Handle case with the full http url in path. In that case,
	// ignore any host header that we encounter and use the path as request URL
	if strings.HasPrefix(parts[1], "http") {
		parsed, err := url.Parse(parts[1])
		if err != nil {
			return fmt.Errorf("could not parse request URL: %s", err)
		}
		conf.Url = parts[1]
		conf.Headers["Host"] = parsed.Host
	} else {
		// Build the request URL from the request
		conf.Url = parseOpts.Input.RequestProto + "://" + conf.Headers["Host"] + parts[1]
	}

	// Set the request body
	b, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("could not read request body: %s", err)
	}
	conf.Data = string(b)

	// Remove newline (typically added by the editor) at the end of the file
	//nolint:gosimple // we specifically want to remove just a single newline, not all of them
	if strings.HasSuffix(conf.Data, "\r\n") {
		conf.Data = conf.Data[:len(conf.Data)-2]
	} else if strings.HasSuffix(conf.Data, "\n") {
		conf.Data = conf.Data[:len(conf.Data)-1]
	}
	return nil
}

func keywordPresent(keyword string, conf *Config) bool {
	//Search for keyword from HTTP method, URL and POST data too
	if strings.Contains(conf.Method, keyword) {
		return true
	}
	if strings.Contains(conf.Url, keyword) {
		return true
	}
	if strings.Contains(conf.Data, keyword) {
		return true
	}
	for k, v := range conf.Headers {
		if strings.Contains(k, keyword) {
			return true
		}
		if strings.Contains(v, keyword) {
			return true
		}
	}
	return false
}

func templatePresent(template string, conf *Config) bool {
	// Search for input location identifiers, these must exist in pairs
	sane := false

	if c := strings.Count(conf.Method, template); c > 0 {
		if c%2 != 0 {
			return false
		}
		sane = true
	}
	if c := strings.Count(conf.Url, template); c > 0 {
		if c%2 != 0 {
			return false
		}
		sane = true
	}
	if c := strings.Count(conf.Data, template); c > 0 {
		if c%2 != 0 {
			return false
		}
		sane = true
	}
	for k, v := range conf.Headers {
		if c := strings.Count(k, template); c > 0 {
			if c%2 != 0 {
				return false
			}
			sane = true
		}
		if c := strings.Count(v, template); c > 0 {
			if c%2 != 0 {
				return false
			}
			sane = true
		}
	}

	return sane
}

func ReadConfig(configFile string) (*ConfigOptions, error) {
	conf := NewConfigOptions()
	configData, err := os.ReadFile(configFile)
	if err == nil {
		err = toml.Unmarshal(configData, conf)
	}
	return conf, err
}

func ReadDefaultConfig() (*ConfigOptions, error) {
	// Try to create configuration directory, ignore the potential error
	_ = CheckOrCreateConfigDir()
	conffile := filepath.Join(CONFIGDIR, "ffufrc")
	if !FileExists(conffile) {
		userhome, err := os.UserHomeDir()
		if err == nil {
			conffile = filepath.Join(userhome, ".ffufrc")
		}
	}
	return ReadConfig(conffile)
}
