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

type HTTPOptions struct {
	Cookies           []string `json:"-"` // this is appended in headers
	Data              string   `json:"data"`
	FollowRedirects   bool     `json:"follow_redirects"`
	Headers           []string `json:"headers"`
	IgnoreBody        bool     `json:"ignore_body"`
	Method            string   `json:"method"`
	ProxyURL          string   `json:"proxy_url"`
	Recursion         bool     `json:"recursion"`
	RecursionDepth    int      `json:"recursion_depth"`
	RecursionStrategy string   `json:"recursion_strategy"`
	ReplayProxyURL    string   `json:"replay_proxy_url"`
	SNI               string   `json:"sni"`
	Timeout           int      `json:"timeout"`
	URL               string   `json:"url"`
	Http2             bool     `json:"http2"`
}

type GeneralOptions struct {
	AutoCalibration         bool     `json:"autocalibration"`
	AutoCalibrationKeyword  string   `json:"autocalibration_keyword"`
	AutoCalibrationPerHost  bool     `json:"autocalibration_per_host"`
	AutoCalibrationStrategy string   `json:"autocalibration_strategy"`
	AutoCalibrationStrings  []string `json:"autocalibration_strings"`
	Colors                  bool     `json:"colors"`
	ConfigFile              string   `toml:"-" json:"config_file"`
	Delay                   string   `json:"delay"`
	Json                    bool     `json:"json"`
	MaxTime                 int      `json:"maxtime"`
	MaxTimeJob              int      `json:"maxtime_job"`
	Noninteractive          bool     `json:"noninteractive"`
	Quiet                   bool     `json:"quiet"`
	Rate                    int      `json:"rate"`
	ScraperFile             string   `json:"scraperfile"`
	Scrapers                string   `json:"scrapers"`
	Searchhash              string   `json:"-"`
	ShowVersion             bool     `toml:"-" json:"-"`
	StopOn403               bool     `json:"stop_on_403"`
	StopOnAll               bool     `json:"stop_on_all"`
	StopOnErrors            bool     `json:"stop_on_errors"`
	Threads                 int      `json:"threads"`
	Verbose                 bool     `json:"verbose"`
}

type InputOptions struct {
	DirSearchCompat        bool     `json:"dirsearch_compat"`
	Extensions             string   `json:"extensions"`
	IgnoreWordlistComments bool     `json:"ignore_wordlist_comments"`
	InputMode              string   `json:"input_mode"`
	InputNum               int      `json:"input_num"`
	InputShell             string   `json:"input_shell"`
	Inputcommands          []string `json:"input_commands"`
	Request                string   `json:"request_file"`
	RequestProto           string   `json:"request_proto"`
	Wordlists              []string `json:"wordlists"`
}

type OutputOptions struct {
	DebugLog            string `json:"debug_log"`
	OutputDirectory     string `json:"output_directory"`
	OutputFile          string `json:"output_file"`
	OutputFormat        string `json:"output_format"`
	OutputSkipEmptyFile bool   `json:"output_skip_empty"`
}

type FilterOptions struct {
	Mode   string `json:"mode"`
	Lines  string `json:"lines"`
	Regexp string `json:"regexp"`
	Size   string `json:"size"`
	Status string `json:"status"`
	Time   string `json:"time"`
	Words  string `json:"words"`
}

type MatcherOptions struct {
	Mode   string `json:"mode"`
	Lines  string `json:"lines"`
	Regexp string `json:"regexp"`
	Size   string `json:"size"`
	Status string `json:"status"`
	Time   string `json:"time"`
	Words  string `json:"words"`
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
	c.General.AutoCalibrationStrategy = "basic"
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
	c.HTTP.Data = ""
	c.HTTP.FollowRedirects = false
	c.HTTP.IgnoreBody = false
	c.HTTP.Method = ""
	c.HTTP.ProxyURL = ""
	c.HTTP.Recursion = false
	c.HTTP.RecursionDepth = 0
	c.HTTP.RecursionStrategy = "default"
	c.HTTP.ReplayProxyURL = ""
	c.HTTP.Timeout = 10
	c.HTTP.SNI = ""
	c.HTTP.URL = ""
	c.HTTP.Http2 = false
	c.Input.DirSearchCompat = false
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
	c.Matcher.Status = "200,204,301,302,307,401,403,405,500"
	c.Matcher.Time = ""
	c.Matcher.Words = ""
	c.Output.DebugLog = ""
	c.Output.OutputDirectory = ""
	c.Output.OutputFile = ""
	c.Output.OutputFormat = "json"
	c.Output.OutputSkipEmptyFile = false
	return c
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

	// Convert cookies to a header
	if len(parseOpts.HTTP.Cookies) > 0 {
		parseOpts.HTTP.Headers = append(parseOpts.HTTP.Headers, "Cookie: "+strings.Join(parseOpts.HTTP.Cookies, "; "))
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
				conf.InputProviders = append(conf.InputProviders, InputProviderConfig{
					Name:    "wordlist",
					Value:   wl[0],
					Keyword: wl[1],
				})
			}
		} else {
			conf.InputProviders = append(conf.InputProviders, InputProviderConfig{
				Name:     "wordlist",
				Value:    wl[0],
				Keyword:  "FUZZ",
				Template: template,
			})
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
				conf.InputProviders = append(conf.InputProviders, InputProviderConfig{
					Name:    "command",
					Value:   ic[0],
					Keyword: ic[1],
				})
				conf.CommandKeywords = append(conf.CommandKeywords, ic[0])
			}
		} else {
			conf.InputProviders = append(conf.InputProviders, InputProviderConfig{
				Name:     "command",
				Value:    ic[0],
				Keyword:  "FUZZ",
				Template: template,
			})
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
			errs.Add(fmt.Errorf(errmsg))
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

	//Prepare headers and make canonical
	for _, v := range parseOpts.HTTP.Headers {
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
	// Using -acc implies -ac
	if len(parseOpts.General.AutoCalibrationStrings) > 0 {
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
	conf.Recursion = parseOpts.HTTP.Recursion
	conf.RecursionDepth = parseOpts.HTTP.RecursionDepth
	conf.RecursionStrategy = parseOpts.HTTP.RecursionStrategy
	conf.AutoCalibration = parseOpts.General.AutoCalibration
	conf.AutoCalibrationPerHost = parseOpts.General.AutoCalibrationPerHost
	conf.AutoCalibrationStrategy = parseOpts.General.AutoCalibrationStrategy
	conf.Threads = parseOpts.General.Threads
	conf.Timeout = parseOpts.HTTP.Timeout
	conf.MaxTime = parseOpts.General.MaxTime
	conf.MaxTimeJob = parseOpts.General.MaxTimeJob
	conf.Noninteractive = parseOpts.General.Noninteractive
	conf.Verbose = parseOpts.General.Verbose
	conf.Json = parseOpts.General.Json
	conf.Http2 = parseOpts.HTTP.Http2

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
		errs.Add(fmt.Errorf(errmsg))
	}
	if !mmode_found {
		errmsg := fmt.Sprintf("Unrecognized value for parameter mmode: %s, valid values are: and, or", parseOpts.Matcher.Mode)
		errs.Add(fmt.Errorf(errmsg))
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

	for _, provider := range conf.InputProviders {
		if provider.Template != "" {
			if !templatePresent(provider.Template, &conf) {
				errmsg := fmt.Sprintf("Template %s defined, but not found in pairs in headers, method, URL or POST data.", provider.Template)
				errs.Add(fmt.Errorf(errmsg))
			}
		} else {
			if !keywordPresent(provider.Keyword, &conf) {
				errmsg := fmt.Sprintf("Keyword %s defined, but not found in headers, method, URL or POST data.", provider.Keyword)
				errs.Add(fmt.Errorf(errmsg))
			}
		}
	}

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
			errs.Add(fmt.Errorf(errmsg))
		}
	}

	// Make verbose mutually exclusive with json
	if parseOpts.General.Verbose && parseOpts.General.Json {
		errs.Add(fmt.Errorf("Cannot have -json and -v"))
	}
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
