package ffuf

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
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
	Filter  FilterOptions
	General GeneralOptions
	HTTP    HTTPOptions
	Input   InputOptions
	Matcher MatcherOptions
	Output  OutputOptions
}

type HTTPOptions struct {
	Cookies         []string
	Data            string
	FollowRedirects bool
	Headers         []string
	IgnoreBody      bool
	Method          string
	ProxyURL        string
	Recursion       bool
	RecursionDepth  int
	ReplayProxyURL  string
	Timeout         int
	URL             string
}

type GeneralOptions struct {
	AutoCalibration        bool
	AutoCalibrationStrings []string
	Colors                 bool
	ConfigFile             string `toml:"-"`
	Delay                  string
	MaxTime                int
	MaxTimeJob             int
	Quiet                  bool
	Rate                   int
	ShowVersion            bool `toml:"-"`
	StopOn403              bool
	StopOnAll              bool
	StopOnErrors           bool
	Threads                int
	Verbose                bool
}

type InputOptions struct {
	DirSearchCompat        bool
	Extensions             string
	IgnoreWordlistComments bool
	InputMode              string
	InputNum               int
	Inputcommands          []string
	Request                string
	RequestProto           string
	Wordlists              []string
}

type OutputOptions struct {
	DebugLog        string
	OutputDirectory string
	OutputFile      string
	OutputFormat    string
	OutputCreateEmptyFile	bool
}

type FilterOptions struct {
	Lines  string
	Regexp string
	Size   string
	Status string
	Words  string
}

type MatcherOptions struct {
	Lines  string
	Regexp string
	Size   string
	Status string
	Words  string
}

//NewConfigOptions returns a newly created ConfigOptions struct with default values
func NewConfigOptions() *ConfigOptions {
	c := &ConfigOptions{}
	c.Filter.Lines = ""
	c.Filter.Regexp = ""
	c.Filter.Size = ""
	c.Filter.Status = ""
	c.Filter.Words = ""
	c.General.AutoCalibration = false
	c.General.Colors = false
	c.General.Delay = ""
	c.General.MaxTime = 0
	c.General.MaxTimeJob = 0
	c.General.Quiet = false
	c.General.Rate = 0
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
	c.HTTP.ReplayProxyURL = ""
	c.HTTP.Timeout = 10
	c.HTTP.URL = ""
	c.Input.DirSearchCompat = false
	c.Input.Extensions = ""
	c.Input.IgnoreWordlistComments = false
	c.Input.InputMode = "clusterbomb"
	c.Input.InputNum = 100
	c.Input.Request = ""
	c.Input.RequestProto = "https"
	c.Matcher.Lines = ""
	c.Matcher.Regexp = ""
	c.Matcher.Size = ""
	c.Matcher.Status = "200,204,301,302,307,401,403"
	c.Matcher.Words = ""
	c.Output.DebugLog = ""
	c.Output.OutputDirectory = ""
	c.Output.OutputFile = ""
	c.Output.OutputFormat = "json"
	c.Output.OutputCreateEmptyFile = false
	return c
}

//ConfigFromOptions parses the values in ConfigOptions struct, ensures that the values are sane,
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
		if len(wl) == 2 {
			conf.InputProviders = append(conf.InputProviders, InputProviderConfig{
				Name:    "wordlist",
				Value:   wl[0],
				Keyword: wl[1],
			})
		} else {
			conf.InputProviders = append(conf.InputProviders, InputProviderConfig{
				Name:    "wordlist",
				Value:   wl[0],
				Keyword: "FUZZ",
			})
		}
	}
	for _, v := range parseOpts.Input.Inputcommands {
		ic := strings.SplitN(v, ":", 2)
		if len(ic) == 2 {
			conf.InputProviders = append(conf.InputProviders, InputProviderConfig{
				Name:    "command",
				Value:   ic[0],
				Keyword: ic[1],
			})
			conf.CommandKeywords = append(conf.CommandKeywords, ic[0])
		} else {
			conf.InputProviders = append(conf.InputProviders, InputProviderConfig{
				Name:    "command",
				Value:   ic[0],
				Keyword: "FUZZ",
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

	//Prepare headers and make canonical
	for _, v := range parseOpts.HTTP.Headers {
		hs := strings.SplitN(v, ":", 2)
		if len(hs) == 2 {
			// trim and make canonical
			// except if used in custom defined header
			var CanonicalNeeded = true
			for _, a := range conf.CommandKeywords {
				if a == hs[0] {
					CanonicalNeeded = false
				}
			}
			// check if part of InputProviders
			if CanonicalNeeded {
				for _, b := range conf.InputProviders {
					if b.Keyword == hs[0] {
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
		_, err := url.Parse(parseOpts.HTTP.ProxyURL)
		if err != nil {
			errs.Add(fmt.Errorf("Bad proxy url (-x) format: %s", err))
		} else {
			conf.ProxyURL = parseOpts.HTTP.ProxyURL
		}
	}

	// Verify replayproxy url format
	if len(parseOpts.HTTP.ReplayProxyURL) > 0 {
		_, err := url.Parse(parseOpts.HTTP.ReplayProxyURL)
		if err != nil {
			errs.Add(fmt.Errorf("Bad replay-proxy url (-replay-proxy) format: %s", err))
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
	conf.InputMode = parseOpts.Input.InputMode
	conf.OutputFile = parseOpts.Output.OutputFile
	conf.OutputDirectory = parseOpts.Output.OutputDirectory
	conf.OutputCreateEmptyFile = parseOpts.Output.OutputCreateEmptyFile
	conf.IgnoreBody = parseOpts.HTTP.IgnoreBody
	conf.Quiet = parseOpts.General.Quiet
	conf.StopOn403 = parseOpts.General.StopOn403
	conf.StopOnAll = parseOpts.General.StopOnAll
	conf.StopOnErrors = parseOpts.General.StopOnErrors
	conf.FollowRedirects = parseOpts.HTTP.FollowRedirects
	conf.Recursion = parseOpts.HTTP.Recursion
	conf.RecursionDepth = parseOpts.HTTP.RecursionDepth
	conf.AutoCalibration = parseOpts.General.AutoCalibration
	conf.Threads = parseOpts.General.Threads
	conf.Timeout = parseOpts.HTTP.Timeout
	conf.MaxTime = parseOpts.General.MaxTime
	conf.MaxTimeJob = parseOpts.General.MaxTimeJob
	conf.Verbose = parseOpts.General.Verbose

	// Handle copy as curl situation where POST method is implied by --data flag. If method is set to anything but GET, NOOP
	if len(conf.Data) > 0 &&
		conf.Method == "GET" &&
		//don't modify the method automatically if a request file is being used as input
		len(parseOpts.Input.Request) == 0 {

		conf.Method = "POST"
	}

	conf.CommandLine = strings.Join(os.Args, " ")

	for _, provider := range conf.InputProviders {
		if !keywordPresent(provider.Keyword, &conf) {
			errmsg := fmt.Sprintf("Keyword %s defined, but not found in headers, method, URL or POST data.", provider.Keyword)
			errs.Add(fmt.Errorf(errmsg))
		}
	}

	// Do checks for recursion mode
	if parseOpts.HTTP.Recursion {
		if !strings.HasSuffix(conf.Url, "FUZZ") {
			errmsg := "When using -recursion the URL (-u) must end with FUZZ keyword."
			errs.Add(fmt.Errorf(errmsg))
		}
	}
	return &conf, errs.ErrorOrNil()
}

func parseRawRequest(parseOpts *ConfigOptions, conf *Config) error {
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
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return fmt.Errorf("could not read request body: %s", err)
	}
	conf.Data = string(b)

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

func ReadConfig(configFile string) (*ConfigOptions, error) {
	conf := NewConfigOptions()
	configData, err := ioutil.ReadFile(configFile)
	if err == nil {
		err = toml.Unmarshal(configData, conf)
	}
	return conf, err
}

func ReadDefaultConfig() (*ConfigOptions, error) {
	userhome, err := os.UserHomeDir()
	if err != nil {
		return NewConfigOptions(), err
	}
	defaultconf := filepath.Join(userhome, ".ffufrc")
	return ReadConfig(defaultconf)
}
