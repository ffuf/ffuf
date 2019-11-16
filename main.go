package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
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
	outputFormat           string
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
	flag.StringVar(&opts.extensions, "e", "", "Comma separated list of extensions to apply. Each extension provided will extend the wordlist entry once. Only extends a wordlist with (default) FUZZ keyword.")
	flag.BoolVar(&conf.DirSearchCompat, "D", false, "DirSearch style wordlist compatibility mode. Used in conjunction with -e flag. Replaces %EXT% in wordlist entry with each of the extensions provided by -e.")
	flag.Var(&opts.headers, "H", "Header `\"Name: Value\"`, separated by colon. Multiple -H flags are accepted.")
	flag.StringVar(&conf.Url, "u", "", "Target URL")
	flag.Var(&opts.wordlists, "w", "Wordlist file path and (optional) custom fuzz keyword, using colon as delimiter. Use file path '-' to read from standard input. Can be supplied multiple times. Format: '/path/to/wordlist:KEYWORD'")
	flag.BoolVar(&conf.TLSVerify, "k", false, "TLS identity verification")
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
	flag.Var(&opts.cookies, "b", "Cookie data `\"NAME1=VALUE1; NAME2=VALUE2\"` for copy as curl functionality.\nResults unpredictable when combined with -H \"Cookie: ...\"")
	flag.Var(&opts.cookies, "cookie", "Cookie data (alias of -b)")
	flag.StringVar(&opts.matcherStatus, "mc", "200,204,301,302,307,401,403", "Match HTTP status codes from respose, use \"all\" to match every response code.")
	flag.StringVar(&opts.matcherSize, "ms", "", "Match HTTP response size")
	flag.StringVar(&opts.matcherRegexp, "mr", "", "Match regexp")
	flag.StringVar(&opts.matcherWords, "mw", "", "Match amount of words in response")
	flag.StringVar(&opts.matcherLines, "ml", "", "Match amount of lines in response")
	flag.StringVar(&opts.proxyURL, "x", "", "HTTP Proxy URL")
	flag.StringVar(&conf.Method, "X", "GET", "HTTP method to use")
	flag.StringVar(&conf.OutputFile, "o", "", "Write output to file")
	flag.StringVar(&opts.outputFormat, "of", "json", "Output file format. Available formats: json, ejson, html, md, csv, ecsv")
	flag.BoolVar(&conf.Quiet, "s", false, "Do not print additional information (silent mode)")
	flag.BoolVar(&conf.StopOn403, "sf", false, "Stop when > 95% of responses return 403 Forbidden")
	flag.BoolVar(&conf.StopOnErrors, "se", false, "Stop on spurious errors")
	flag.BoolVar(&conf.StopOnAll, "sa", false, "Stop on all error cases. Implies -sf and -se")
	flag.BoolVar(&conf.FollowRedirects, "r", false, "Follow redirects")
	flag.BoolVar(&conf.AutoCalibration, "ac", false, "Automatically calibrate filtering options")
	flag.Var(&opts.AutoCalibrationStrings, "acc", "Custom auto-calibration string. Can be used multiple times. Implies -ac")
	flag.IntVar(&conf.Threads, "t", 40, "Number of concurrent threads.")
	flag.IntVar(&conf.Timeout, "timeout", 10, "HTTP request timeout in seconds.")
	flag.BoolVar(&conf.Verbose, "v", false, "Verbose output, printing full URL and redirect location (if any) with the results.")
	flag.BoolVar(&opts.showVersion, "V", false, "Show version information.")
	flag.StringVar(&opts.debugLog, "debug-log", "", "Write all of the internal logging to the specified file.")
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
		flag.Usage()
		os.Exit(1)
	}
	job, err := prepareJob(&conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		flag.Usage()
		os.Exit(1)
	}
	if err := prepareFilters(&opts, &conf); err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		flag.Usage()
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
	errs := ffuf.NewMultierror()
	var err error
	inputprovider, err := input.NewInputProvider(conf)
	if err != nil {
		errs.Add(err)
	}
	// TODO: implement error handling for runnerprovider and outputprovider
	// We only have http runner right now
	runprovider := runner.NewRunnerByName("http", conf)
	// Initialize the correct inputprovider
	for _, v := range conf.InputProviders {
		err = inputprovider.AddProvider(v)
		if err != nil {
			errs.Add(err)
		}
	}
	// We only have stdout outputprovider right now
	outprovider := output.NewOutputProviderByName("stdout", conf)
	return &ffuf.Job{
		Config: conf,
		Runner: runprovider,
		Output: outprovider,
		Input:  inputprovider,
	}, errs.ErrorOrNil()
}

func prepareFilters(parseOpts *cliOptions, conf *ffuf.Config) error {
	errs := ffuf.NewMultierror()
	if parseOpts.filterStatus != "" {
		if err := filter.AddFilter(conf, "status", parseOpts.filterStatus); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.filterSize != "" {
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
		if err := filter.AddFilter(conf, "word", parseOpts.filterWords); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.filterLines != "" {
		if err := filter.AddFilter(conf, "line", parseOpts.filterLines); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.matcherStatus != "" {
		if err := filter.AddMatcher(conf, "status", parseOpts.matcherStatus); err != nil {
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
	return errs.ErrorOrNil()
}

func prepareConfig(parseOpts *cliOptions, conf *ffuf.Config) error {
	//TODO: refactor in a proper flag library that can handle things like required flags
	errs := ffuf.NewMultierror()

	var err error
	var err2 error
	if len(conf.Url) == 0 {
		errs.Add(fmt.Errorf("-u flag is required"))
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
		wl := strings.SplitN(v, ":", 2)
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

	//Prepare headers
	for _, v := range parseOpts.headers {
		hs := strings.SplitN(v, ":", 2)
		if len(hs) == 2 {
			conf.Headers[strings.TrimSpace(hs[0])] = strings.TrimSpace(hs[1])
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
		pu, err := url.Parse(parseOpts.proxyURL)
		if err != nil {
			errs.Add(fmt.Errorf("Bad proxy url (-x) format: %s", err))
		} else {
			conf.ProxyURL = http.ProxyURL(pu)
		}
	}

	//Check the output file format option
	if conf.OutputFile != "" {
		//No need to check / error out if output file isn't defined
		outputFormats := []string{"json", "ejson", "html", "md", "csv", "ecsv"}
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
	conf.AutoCalibrationStrings = parseOpts.AutoCalibrationStrings
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

	return errs.ErrorOrNil()
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
