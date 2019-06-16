package main

import (
	"context"
	"flag"
	"fmt"
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

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf := ffuf.NewConfig(ctx)
	opts := cliOptions{}
	var ignored bool
	flag.StringVar(&opts.extensions, "e", "", "Comma separated list of extensions to apply. Each extension provided will extend the wordlist entry once.")
	flag.BoolVar(&conf.DirSearchCompat, "D", false, "DirSearch style wordlist compatibility mode. Used in conjunction with -e flag. Replaces %EXT% in wordlist entry with each of the extensions provided by -e.")
	flag.Var(&opts.headers, "H", "Header `\"Name: Value\"`, separated by colon. Multiple -H flags are accepted.")
	flag.StringVar(&conf.Url, "u", "", "Target URL")
	flag.StringVar(&conf.Wordlist, "w", "", "Wordlist path")
	flag.BoolVar(&conf.TLSVerify, "k", false, "TLS identity verification")
	flag.StringVar(&opts.delay, "p", "", "Seconds of `delay` between requests, or a range of random delay. For example \"0.1\" or \"0.1-2.0\"")
	flag.StringVar(&opts.filterStatus, "fc", "", "Filter HTTP status codes from response")
	flag.StringVar(&opts.filterSize, "fs", "", "Filter HTTP response size")
	flag.StringVar(&opts.filterRegexp, "fr", "", "Filter regexp")
	flag.StringVar(&opts.filterWords, "fw", "", "Filter by amount of words in response")
	flag.StringVar(&conf.Data, "d", "", "POST data")
	flag.StringVar(&conf.Data, "data", "", "POST data (alias of -d)")
	flag.BoolVar(&conf.Colors, "c", false, "Colorize output.")
	flag.BoolVar(&ignored, "compressed", true, "Dummy flag for copy as curl functionality (ignored)")
	flag.StringVar(&conf.InputCommand, "input-cmd", "", "Command producing the input. --input-num is required when using this input method. Overrides -w.")
	flag.IntVar(&conf.InputNum, "input-num", 100, "Number of inputs to test. Used in conjunction with --input-cmd.")
	flag.StringVar(&opts.matcherStatus, "mc", "200,204,301,302,307,401,403", "Match HTTP status codes from respose, use \"all\" to match every response code.")
	flag.StringVar(&opts.matcherSize, "ms", "", "Match HTTP response size")
	flag.StringVar(&opts.matcherRegexp, "mr", "", "Match regexp")
	flag.StringVar(&opts.matcherWords, "mw", "", "Match amount of words in response")
	flag.StringVar(&opts.proxyURL, "x", "", "HTTP Proxy URL")
	flag.StringVar(&conf.Method, "X", "GET", "HTTP method to use")
	flag.StringVar(&conf.OutputFile, "o", "", "Write output to file")
	flag.StringVar(&opts.outputFormat, "of", "json", "Output file format. Available formats: json, csv, ecsv")
	flag.BoolVar(&conf.Quiet, "s", false, "Do not print additional information (silent mode)")
	flag.BoolVar(&conf.StopOn403, "sf", false, "Stop when > 95% of responses return 403 Forbidden")
	flag.BoolVar(&conf.StopOnErrors, "se", false, "Stop on spurious errors")
	flag.BoolVar(&conf.StopOnAll, "sa", false, "Stop on all error cases. Implies -sf and -se")
	flag.BoolVar(&conf.FollowRedirects, "r", false, "Follow redirects")
	flag.BoolVar(&conf.AutoCalibration, "ac", false, "Automatically calibrate filtering options")
	flag.IntVar(&conf.Threads, "t", 40, "Number of concurrent threads.")
	flag.IntVar(&conf.Timeout, "timeout", 10, "HTTP request timeout in seconds.")
	flag.BoolVar(&opts.showVersion, "V", false, "Show version information.")
	flag.Parse()
	if opts.showVersion {
		fmt.Printf("ffuf version: %s\n", ffuf.VERSION)
		os.Exit(0)
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
	var inputprovider ffuf.InputProvider
	// TODO: implement error handling for runnerprovider and outputprovider
	// We only have http runner right now
	runprovider := runner.NewRunnerByName("http", conf)
	// Initialize the correct inputprovider
	if len(conf.InputCommand) > 0 {
		inputprovider, err = input.NewInputProviderByName("command", conf)
	} else {
		inputprovider, err = input.NewInputProviderByName("wordlist", conf)
	}
	if err != nil {
		errs.Add(fmt.Errorf("%s", err))
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
	return errs.ErrorOrNil()
}

func prepareConfig(parseOpts *cliOptions, conf *ffuf.Config) error {
	//TODO: refactor in a proper flag library that can handle things like required flags
	errs := ffuf.NewMultierror()
	foundkeyword := false

	var err error
	var err2 error
	if len(conf.Url) == 0 {
		errs.Add(fmt.Errorf("-u flag is required"))
	}
	if len(conf.Wordlist) == 0 && len(conf.InputCommand) == 0 {
		errs.Add(fmt.Errorf("Either -w or --input-cmd flag is required"))
	}
	// prepare extensions
	if parseOpts.extensions != "" {
		extensions := strings.Split(parseOpts.extensions, ",")
		conf.Extensions = extensions
	}

	//Prepare headers
	for _, v := range parseOpts.headers {
		hs := strings.SplitN(v, ":", 2)
		if len(hs) == 2 {
			fuzzedheader := false
			for _, fv := range hs {
				if strings.Index(fv, "FUZZ") != -1 {
					// Add to fuzzheaders
					fuzzedheader = true
				}
			}
			if fuzzedheader {
				conf.FuzzHeaders[strings.TrimSpace(hs[0])] = strings.TrimSpace(hs[1])
				foundkeyword = true
			} else {
				conf.StaticHeaders[strings.TrimSpace(hs[0])] = strings.TrimSpace(hs[1])
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
		outputFormats := []string{"json", "csv", "ecsv"}
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

	// Handle copy as curl situation where POST method is implied by --data flag. If method is set to anything bug GET, NOOP
	if conf.Method == "GET" {
		if len(conf.Data) > 0 {
			conf.Method = "POST"
		}
	}

	conf.CommandLine = strings.Join(os.Args, " ")

	//Search for keyword from URL and POST data too
	if strings.Index(conf.Url, "FUZZ") != -1 {
		foundkeyword = true
	}
	if strings.Index(conf.Data, "FUZZ") != -1 {
		foundkeyword = true
	}

	if !foundkeyword {
		errs.Add(fmt.Errorf("No FUZZ keyword(s) found in headers, URL or POST data, nothing to do"))
	}

	return errs.ErrorOrNil()
}
