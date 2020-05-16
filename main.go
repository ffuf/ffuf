package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/textproto"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"
	"github.com/ffuf/ffuf/pkg/filter"
	"github.com/ffuf/ffuf/pkg/input"
	"github.com/ffuf/ffuf/pkg/output"
	"github.com/ffuf/ffuf/pkg/runner"
)

type cliOptions struct {
	extensions             string
	delay                  string
	filterStatus           string
	filterSize             string
	filterRegexp           string
	filterWords            string
	filterLines            string
	matcherStatus          string
	matcherSize            string
	matcherRegexp          string
	matcherWords           string
	matcherLines           string
	proxyURL               string
	replayProxyURL         string
	request                string
	requestProto           string
	URL                    string
	outputFormat           string
	ignoreBody             bool
	wordlists              multiStringFlag
	inputcommands          multiStringFlag
	headers                multiStringFlag
	cookies                multiStringFlag
	AutoCalibrationStrings multiStringFlag
	showVersion            bool
	debugLog               string
}

type multiStringFlag []string

func (m *multiStringFlag) String() string {
	return ""
}

func (m *multiStringFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf := ffuf.NewConfig(ctx)
	opts := cliOptions{}
	var ignored bool
	flag.BoolVar(&conf.IgnoreWordlistComments, "ic", false, "Ignore wordlist comments")
	flag.StringVar(&opts.extensions, "e", "", "Comma separated list of extensions. Extends FUZZ keyword.")
	flag.BoolVar(&conf.DirSearchCompat, "D", false, "DirSearch wordlist compatibility mode. Used in conjunction with -e flag.")
	flag.Var(&opts.headers, "H", "Header `\"Name: Value\"`, separated by colon. Multiple -H flags are accepted.")
	flag.StringVar(&opts.URL, "u", "", "Target URL")
	flag.Var(&opts.wordlists, "w", "Wordlist file path and (optional) keyword separated by colon. eg. '/path/to/wordlist:KEYWORD'")
	flag.BoolVar(&ignored, "k", false, "Dummy flag for backwards compatibility")
	flag.StringVar(&opts.delay, "p", "", "Seconds of `delay` between requests, or a range of random delay. For example \"0.1\" or \"0.1-2.0\"")
	flag.StringVar(&opts.filterStatus, "fc", "", "Filter HTTP status codes from response. Comma separated list of codes and ranges")
	flag.StringVar(&opts.filterSize, "fs", "", "Filter HTTP response size. Comma separated list of sizes and ranges")
	flag.StringVar(&opts.filterRegexp, "fr", "", "Filter regexp")
	flag.StringVar(&opts.filterWords, "fw", "", "Filter by amount of words in response. Comma separated list of word counts and ranges")
	flag.StringVar(&opts.filterLines, "fl", "", "Filter by amount of lines in response. Comma separated list of line counts and ranges")
	flag.StringVar(&conf.Data, "d", "", "POST data")
	flag.StringVar(&conf.Data, "data", "", "POST data (alias of -d)")
	flag.StringVar(&conf.Data, "data-ascii", "", "POST data (alias of -d)")
	flag.StringVar(&conf.Data, "data-binary", "", "POST data (alias of -d)")
	flag.BoolVar(&conf.Colors, "c", false, "Colorize output.")
	flag.BoolVar(&ignored, "compressed", true, "Dummy flag for copy as curl functionality (ignored)")
	flag.Var(&opts.inputcommands, "input-cmd", "Command producing the input. --input-num is required when using this input method. Overrides -w.")
	flag.IntVar(&conf.InputNum, "input-num", 100, "Number of inputs to test. Used in conjunction with --input-cmd.")
	flag.StringVar(&conf.InputMode, "mode", "clusterbomb", "Multi-wordlist operation mode. Available modes: clusterbomb, pitchfork")
	flag.BoolVar(&ignored, "i", true, "Dummy flag for copy as curl functionality (ignored)")
	flag.Var(&opts.cookies, "b", "Cookie data `\"NAME1=VALUE1; NAME2=VALUE2\"` for copy as curl functionality.")
	flag.Var(&opts.cookies, "cookie", "Cookie data (alias of -b)")
	flag.StringVar(&opts.matcherStatus, "mc", "200,204,301,302,307,401,403", "Match HTTP status codes, or \"all\" for everything.")
	flag.StringVar(&opts.matcherSize, "ms", "", "Match HTTP response size")
	flag.StringVar(&opts.matcherRegexp, "mr", "", "Match regexp")
	flag.StringVar(&opts.matcherWords, "mw", "", "Match amount of words in response")
	flag.StringVar(&opts.matcherLines, "ml", "", "Match amount of lines in response")
	flag.StringVar(&opts.proxyURL, "x", "", "HTTP Proxy URL")
	flag.StringVar(&opts.request, "request", "", "File containing the raw http request")
	flag.StringVar(&opts.requestProto, "request-proto", "https", "Protocol to use along with raw request")
	flag.StringVar(&conf.Method, "X", "GET", "HTTP method to use")
	flag.StringVar(&conf.OutputFile, "o", "", "Write output to file")
	flag.StringVar(&opts.outputFormat, "of", "json", "Output file format. Available formats: json, ejson, html, md, csv, ecsv (or, 'all' for all formats)")
	flag.StringVar(&conf.OutputDirectory, "od", "", "Directory path to store matched results to.")
	flag.BoolVar(&conf.IgnoreBody, "ignore-body", false, "Do not fetch the response content.")
	flag.BoolVar(&conf.Quiet, "s", false, "Do not print additional information (silent mode)")
	flag.BoolVar(&conf.StopOn403, "sf", false, "Stop when > 95% of responses return 403 Forbidden")
	flag.BoolVar(&conf.StopOnErrors, "se", false, "Stop on spurious errors")
	flag.BoolVar(&conf.StopOnAll, "sa", false, "Stop on all error cases. Implies -sf and -se.")
	flag.BoolVar(&conf.FollowRedirects, "r", false, "Follow redirects")
	flag.BoolVar(&conf.Recursion, "recursion", false, "Scan recursively. Only FUZZ keyword is supported, and URL (-u) has to end in it.")
	flag.IntVar(&conf.RecursionDepth, "recursion-depth", 0, "Maximum recursion depth.")
	flag.StringVar(&opts.replayProxyURL, "replay-proxy", "", "Replay matched requests using this proxy.")
	flag.BoolVar(&conf.AutoCalibration, "ac", false, "Automatically calibrate filtering options")
	flag.Var(&opts.AutoCalibrationStrings, "acc", "Custom auto-calibration string. Can be used multiple times. Implies -ac")
	flag.IntVar(&conf.Threads, "t", 40, "Number of concurrent threads.")
	flag.IntVar(&conf.Timeout, "timeout", 10, "HTTP request timeout in seconds.")
	flag.IntVar(&conf.MaxTime, "maxtime", 0, "Maximum running time in seconds for entire process.")
	flag.IntVar(&conf.MaxTimeJob, "maxtime-job", 0, "Maximum running time in seconds per job.")
	flag.BoolVar(&conf.Verbose, "v", false, "Verbose output, printing full URL and redirect location (if any) with the results.")
	flag.BoolVar(&opts.showVersion, "V", false, "Show version information.")
	flag.StringVar(&opts.debugLog, "debug-log", "", "Write all of the internal logging to the specified file.")
	flag.Usage = Usage
	flag.Parse()
	if opts.showVersion {
		fmt.Printf("ffuf version: %s\n", ffuf.VERSION)
		os.Exit(0)
	}
	if len(opts.debugLog) != 0 {
		f, err := os.OpenFile(opts.debugLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Disabling logging, encountered error(s): %s\n", err)
			log.SetOutput(ioutil.Discard)
		} else {
			log.SetOutput(f)
			defer f.Close()
		}
	} else {
		log.SetOutput(ioutil.Discard)
	}
	if err := prepareConfig(&opts, &conf); err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		Usage()
		os.Exit(1)
	}
	job, err := prepareJob(&conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		Usage()
		os.Exit(1)
	}
	if err := prepareFilters(&opts, &conf); err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		Usage()
		os.Exit(1)
	}

	if err := filter.CalibrateIfNeeded(job); err != nil {
		fmt.Fprintf(os.Stderr, "Error in autocalibration, exiting: %s\n", err)
		os.Exit(1)
	}

	// Job handles waiting for goroutines to complete itself
	job.Start()
}

func prepareJob(conf *ffuf.Config) (*ffuf.Job, error) {
	job := &ffuf.Job{
		Config: conf,
	}
	errs := ffuf.NewMultierror()
	var err error
	inputprovider, err := input.NewInputProvider(conf)
	if err != nil {
		errs.Add(err)
	}
	// TODO: implement error handling for runnerprovider and outputprovider
	// We only have http runner right now
	job.Runner = runner.NewRunnerByName("http", conf, false)
	if len(conf.ReplayProxyURL) > 0 {
		job.ReplayRunner = runner.NewRunnerByName("http", conf, true)
	}
	// Initialize the correct inputprovider
	for _, v := range conf.InputProviders {
		err = inputprovider.AddProvider(v)
		if err != nil {
			errs.Add(err)
		}
	}
	job.Input = inputprovider
	// We only have stdout outputprovider right now
	job.Output = output.NewOutputProviderByName("stdout", conf)
	return job, errs.ErrorOrNil()
}

func prepareFilters(parseOpts *cliOptions, conf *ffuf.Config) error {
	errs := ffuf.NewMultierror()
	// If any other matcher is set, ignore -mc default value
	matcherSet := false
	statusSet := false
	warningIgnoreBody := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "mc" {
			statusSet = true
		}
		if f.Name == "ms" {
			matcherSet = true
			warningIgnoreBody = true
		}
		if f.Name == "ml" {
			matcherSet = true
			warningIgnoreBody = true
		}
		if f.Name == "mr" {
			matcherSet = true
		}
		if f.Name == "mw" {
			matcherSet = true
			warningIgnoreBody = true
		}
	})
	if statusSet || !matcherSet {
		if err := filter.AddMatcher(conf, "status", parseOpts.matcherStatus); err != nil {
			errs.Add(err)
		}
	}

	if parseOpts.filterStatus != "" {
		if err := filter.AddFilter(conf, "status", parseOpts.filterStatus); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.filterSize != "" {
		warningIgnoreBody = true
		if err := filter.AddFilter(conf, "size", parseOpts.filterSize); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.filterRegexp != "" {
		if err := filter.AddFilter(conf, "regexp", parseOpts.filterRegexp); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.filterWords != "" {
		warningIgnoreBody = true
		if err := filter.AddFilter(conf, "word", parseOpts.filterWords); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.filterLines != "" {
		warningIgnoreBody = true
		if err := filter.AddFilter(conf, "line", parseOpts.filterLines); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.matcherSize != "" {
		if err := filter.AddMatcher(conf, "size", parseOpts.matcherSize); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.matcherRegexp != "" {
		if err := filter.AddMatcher(conf, "regexp", parseOpts.matcherRegexp); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.matcherWords != "" {
		if err := filter.AddMatcher(conf, "word", parseOpts.matcherWords); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.matcherLines != "" {
		if err := filter.AddMatcher(conf, "line", parseOpts.matcherLines); err != nil {
			errs.Add(err)
		}
	}
	if conf.IgnoreBody && warningIgnoreBody {
		fmt.Printf("*** Warning: possible undesired combination of -ignore-body and the response options: fl,fs,fw,ml,ms and mw.\n")
	}
	return errs.ErrorOrNil()
}

func prepareConfig(parseOpts *cliOptions, conf *ffuf.Config) error {
	//TODO: refactor in a proper flag library that can handle things like required flags
	errs := ffuf.NewMultierror()

	var err error
	var err2 error
	if len(parseOpts.URL) == 0 && parseOpts.request == "" {
		errs.Add(fmt.Errorf("-u flag or -request flag is required"))
	}

	// prepare extensions
	if parseOpts.extensions != "" {
		extensions := strings.Split(parseOpts.extensions, ",")
		conf.Extensions = extensions
	}

	// Convert cookies to a header
	if len(parseOpts.cookies) > 0 {
		parseOpts.headers.Set("Cookie: " + strings.Join(parseOpts.cookies, "; "))
	}

	//Prepare inputproviders
	for _, v := range parseOpts.wordlists {
		var wl []string
		if runtime.GOOS == "windows" {
			// Try to ensure that Windows file paths like C:\path\to\wordlist.txt:KEYWORD are treated properly
			if ffuf.FileExists(v) {
				// The wordlist was supplied without a keyword parameter
				wl = []string{v}
			} else {
				filepart := v[:strings.LastIndex(v, ":")]
				if ffuf.FileExists(filepart) {
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
			conf.InputProviders = append(conf.InputProviders, ffuf.InputProviderConfig{
				Name:    "wordlist",
				Value:   wl[0],
				Keyword: wl[1],
			})
		} else {
			conf.InputProviders = append(conf.InputProviders, ffuf.InputProviderConfig{
				Name:    "wordlist",
				Value:   wl[0],
				Keyword: "FUZZ",
			})
		}
	}
	for _, v := range parseOpts.inputcommands {
		ic := strings.SplitN(v, ":", 2)
		if len(ic) == 2 {
			conf.InputProviders = append(conf.InputProviders, ffuf.InputProviderConfig{
				Name:    "command",
				Value:   ic[0],
				Keyword: ic[1],
			})
			conf.CommandKeywords = append(conf.CommandKeywords, ic[0])
		} else {
			conf.InputProviders = append(conf.InputProviders, ffuf.InputProviderConfig{
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
	if parseOpts.request != "" {
		err := parseRawRequest(parseOpts, conf)
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
	for _, v := range parseOpts.headers {
		hs := strings.SplitN(v, ":", 2)
		if len(hs) == 2 {
			// trim and make canonical
			// except if used in custom defined header
			var CanonicalNeeded bool = true
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
				var CanonicalHeader string = textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(hs[0]))
				conf.Headers[CanonicalHeader] = strings.TrimSpace(hs[1])
			} else {
				conf.Headers[strings.TrimSpace(hs[0])] = strings.TrimSpace(hs[1])
			}
		} else {
			errs.Add(fmt.Errorf("Header defined by -H needs to have a value. \":\" should be used as a separator"))
		}
	}

	//Prepare delay
	d := strings.Split(parseOpts.delay, "-")
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
	} else if len(parseOpts.delay) > 0 {
		conf.Delay.IsRange = false
		conf.Delay.HasDelay = true
		conf.Delay.Min, err = strconv.ParseFloat(parseOpts.delay, 64)
		if err != nil {
			errs.Add(fmt.Errorf("Delay needs to be either a single float: \"0.1\" or a range of floats, delimited by dash: \"0.1-0.8\""))
		}
	}

	// Verify proxy url format
	if len(parseOpts.proxyURL) > 0 {
		_, err := url.Parse(parseOpts.proxyURL)
		if err != nil {
			errs.Add(fmt.Errorf("Bad proxy url (-x) format: %s", err))
		} else {
			conf.ProxyURL = parseOpts.proxyURL
		}
	}

	// Verify replayproxy url format
	if len(parseOpts.replayProxyURL) > 0 {
		_, err := url.Parse(parseOpts.replayProxyURL)
		if err != nil {
			errs.Add(fmt.Errorf("Bad replay-proxy url (-replay-proxy) format: %s", err))
		} else {
			conf.ReplayProxyURL = parseOpts.replayProxyURL
		}
	}

	//Check the output file format option
	if conf.OutputFile != "" {
		//No need to check / error out if output file isn't defined
		outputFormats := []string{"all", "json", "ejson", "html", "md", "csv", "ecsv"}
		found := false
		for _, f := range outputFormats {
			if f == parseOpts.outputFormat {
				conf.OutputFormat = f
				found = true
			}
		}
		if !found {
			errs.Add(fmt.Errorf("Unknown output file format (-of): %s", parseOpts.outputFormat))
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
	if conf.Method == "GET" {
		if len(conf.Data) > 0 {
			conf.Method = "POST"
		}
	}

	conf.CommandLine = strings.Join(os.Args, " ")

	for _, provider := range conf.InputProviders {
		if !keywordPresent(provider.Keyword, conf) {
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

	return errs.ErrorOrNil()
}

func parseRawRequest(parseOpts *cliOptions, conf *ffuf.Config) error {
	file, err := os.Open(parseOpts.request)
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
		conf.Url = parseOpts.requestProto + "://" + conf.Headers["Host"] + parts[1]
	}

	// Set the request body
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return fmt.Errorf("could not read request body: %s", err)
	}
	conf.Data = string(b)

	return nil
}

func keywordPresent(keyword string, conf *ffuf.Config) bool {
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
