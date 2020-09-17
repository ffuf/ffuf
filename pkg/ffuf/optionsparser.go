package ffuf

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/textproto"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type ConfigOptions struct {
	AutoCalibration        bool
	AutoCalibrationStrings []string
	Colors                 bool
	Cookies                []string
	Data                   string
	DebugLog               string
	Delay                  string
	DirSearchCompat        bool
	Extensions             string
	FilterLines            string
	FilterRegexp           string
	FilterSize             string
	FilterStatus           string
	FilterWords            string
	FollowRedirects        bool
	Headers                []string
	IgnoreBody             bool
	IgnoreWordlistComments bool
	InputMode              string
	InputNum               int
	Inputcommands          []string
	MatcherLines           string
	MatcherRegexp          string
	MatcherSize            string
	MatcherStatus          string
	MatcherWords           string
	MaxTime                int
	MaxTimeJob             int
	Method                 string
	OutputDirectory        string
	OutputFile             string
	OutputFormat           string
	ProxyURL               string
	Quiet                  bool
	Rate                   int
	Recursion              bool
	RecursionDepth         int
	ReplayProxyURL         string
	Request                string
	RequestProto           string
	ShowVersion            bool
	StopOn403              bool
	StopOnAll              bool
	StopOnErrors           bool
	Threads                int
	Timeout                int
	URL                    string
	Verbose                bool
	Wordlists              []string
}

//ConfigFromOptions parses the values in ConfigOptions struct, ensures that the values are sane,
// and creates a Config struct out of them.
func ConfigFromOptions(parseOpts *ConfigOptions) (*Config, error) {
	//TODO: refactor in a proper flag library that can handle things like required flags
	errs := NewMultierror()
	conf := NewConfig()

	var err error
	var err2 error
	if len(parseOpts.URL) == 0 && parseOpts.Request == "" {
		errs.Add(fmt.Errorf("-u flag or -request flag is required"))
	}

	// prepare extensions
	if parseOpts.Extensions != "" {
		extensions := strings.Split(parseOpts.Extensions, ",")
		conf.Extensions = extensions
	}

	// Convert cookies to a header
	if len(parseOpts.Cookies) > 0 {
		parseOpts.Headers = append(parseOpts.Headers, "Cookie: "+strings.Join(parseOpts.Cookies, "; "))
	}

	//Prepare inputproviders
	for _, v := range parseOpts.Wordlists {
		var wl []string
		if runtime.GOOS == "windows" {
			// Try to ensure that Windows file paths like C:\path\to\wordlist.txt:KEYWORD are treated properly
			if FileExists(v) {
				// The wordlist was supplied without a keyword parameter
				wl = []string{v}
			} else {
				filepart := v[:strings.LastIndex(v, ":")]
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
	for _, v := range parseOpts.Inputcommands {
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
	if parseOpts.Request != "" {
		err := parseRawRequest(parseOpts, &conf)
		if err != nil {
			errmsg := fmt.Sprintf("Could not parse raw request: %s", err)
			errs.Add(fmt.Errorf(errmsg))
		}
	}

	//Prepare URL
	if parseOpts.URL != "" {
		conf.Url = parseOpts.URL
	}

	//Prepare headers and make canonical
	for _, v := range parseOpts.Headers {
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
	d := strings.Split(parseOpts.Delay, "-")
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
	} else if len(parseOpts.Delay) > 0 {
		conf.Delay.IsRange = false
		conf.Delay.HasDelay = true
		conf.Delay.Min, err = strconv.ParseFloat(parseOpts.Delay, 64)
		if err != nil {
			errs.Add(fmt.Errorf("Delay needs to be either a single float: \"0.1\" or a range of floats, delimited by dash: \"0.1-0.8\""))
		}
	}

	// Verify proxy url format
	if len(parseOpts.ProxyURL) > 0 {
		_, err := url.Parse(parseOpts.ProxyURL)
		if err != nil {
			errs.Add(fmt.Errorf("Bad proxy url (-x) format: %s", err))
		} else {
			conf.ProxyURL = parseOpts.ProxyURL
		}
	}

	// Verify replayproxy url format
	if len(parseOpts.ReplayProxyURL) > 0 {
		_, err := url.Parse(parseOpts.ReplayProxyURL)
		if err != nil {
			errs.Add(fmt.Errorf("Bad replay-proxy url (-replay-proxy) format: %s", err))
		} else {
			conf.ReplayProxyURL = parseOpts.ReplayProxyURL
		}
	}

	//Check the output file format option
	if conf.OutputFile != "" {
		//No need to check / error out if output file isn't defined
		outputFormats := []string{"all", "json", "ejson", "html", "md", "csv", "ecsv"}
		found := false
		for _, f := range outputFormats {
			if f == parseOpts.OutputFormat {
				conf.OutputFormat = f
				found = true
			}
		}
		if !found {
			errs.Add(fmt.Errorf("Unknown output file format (-of): %s", parseOpts.OutputFormat))
		}
	}

	// Auto-calibration strings
	if len(parseOpts.AutoCalibrationStrings) > 0 {
		conf.AutoCalibrationStrings = parseOpts.AutoCalibrationStrings
	}
	// Using -acc implies -ac
	if len(conf.AutoCalibrationStrings) > 0 {
		conf.AutoCalibration = true
	}

	// Handle copy as curl situation where POST method is implied by --data flag. If method is set to anything but GET, NOOP
	if len(conf.Data) > 0 &&
		conf.Method == "GET" &&
		//don't modify the method automatically if a request file is being used as input
		len(parseOpts.Request) == 0 {

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
	if conf.Recursion {
		if !strings.HasSuffix(conf.Url, "FUZZ") {
			errmsg := fmt.Sprintf("When using -recursion the URL (-u) must end with FUZZ keyword.")
			errs.Add(fmt.Errorf(errmsg))
		}
	}

	if parseOpts.Rate < 0 {
		conf.Rate = 0
	} else {
		conf.Rate = int64(parseOpts.Rate)
	}

	// Common stuff
	conf.IgnoreWordlistComments = parseOpts.IgnoreWordlistComments
	conf.DirSearchCompat = parseOpts.DirSearchCompat
	conf.Data = parseOpts.Data
	conf.Colors = parseOpts.Colors
	conf.InputNum = parseOpts.InputNum
	conf.InputMode = parseOpts.InputMode
	conf.Method = parseOpts.Method
	conf.OutputFile = parseOpts.OutputFile
	conf.OutputDirectory = parseOpts.OutputDirectory
	conf.IgnoreBody = parseOpts.IgnoreBody
	conf.Quiet = parseOpts.Quiet
	conf.StopOn403 = parseOpts.StopOn403
	conf.StopOnAll = parseOpts.StopOnAll
	conf.StopOnErrors = parseOpts.StopOnErrors
	conf.FollowRedirects = parseOpts.FollowRedirects
	conf.Recursion = parseOpts.Recursion
	conf.RecursionDepth = parseOpts.RecursionDepth
	conf.AutoCalibration = parseOpts.AutoCalibration
	conf.Threads = parseOpts.Threads
	conf.Timeout = parseOpts.Timeout
	conf.MaxTime = parseOpts.MaxTime
	conf.MaxTimeJob = parseOpts.MaxTimeJob
	conf.Verbose = parseOpts.Verbose
	return &conf, errs.ErrorOrNil()
}

func parseRawRequest(parseOpts *ConfigOptions, conf *Config) error {
	file, err := os.Open(parseOpts.Request)
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
		conf.Url = parseOpts.RequestProto + "://" + conf.Headers["Host"] + parts[1]
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
	if strings.Index(conf.Method, keyword) != -1 {
		return true
	}
	if strings.Index(conf.Url, keyword) != -1 {
		return true
	}
	if strings.Index(conf.Data, keyword) != -1 {
		return true
	}
	for k, v := range conf.Headers {
		if strings.Index(k, keyword) != -1 {
			return true
		}
		if strings.Index(v, keyword) != -1 {
			return true
		}
	}
	return false
}
