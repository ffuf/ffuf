package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"
	"github.com/ffuf/ffuf/pkg/filter"
	"github.com/ffuf/ffuf/pkg/input"
	"github.com/ffuf/ffuf/pkg/output"
	"github.com/ffuf/ffuf/pkg/runner"
)

type multiStringFlag []string
type wordlistFlag []string

func (m *multiStringFlag) String() string {
	return ""
}

func (m *wordlistFlag) String() string {
	return ""
}

func (m *multiStringFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func (m *wordlistFlag) Set(value string) error {
	delimited := strings.Split(value, ",")

	if len(delimited) > 1 {
		*m = append(*m, delimited...)
	} else {
		*m = append(*m, value)
	}

	return nil
}

func main() {
	opts := ffuf.ConfigOptions{}
	var ignored bool
	var cookies, autocalibrationstrings, headers, inputcommands multiStringFlag
	var wordlists wordlistFlag
	flag.BoolVar(&ignored, "compressed", true, "Dummy flag for copy as curl functionality (ignored)")
	flag.BoolVar(&ignored, "i", true, "Dummy flag for copy as curl functionality (ignored)")
	flag.BoolVar(&ignored, "k", false, "Dummy flag for backwards compatibility")
	flag.BoolVar(&opts.AutoCalibration, "ac", false, "Automatically calibrate filtering options")
	flag.BoolVar(&opts.Colors, "c", false, "Colorize output.")
	flag.BoolVar(&opts.DirSearchCompat, "D", false, "DirSearch wordlist compatibility mode. Used in conjunction with -e flag.")
	flag.BoolVar(&opts.FollowRedirects, "r", false, "Follow redirects")
	flag.BoolVar(&opts.IgnoreBody, "ignore-body", false, "Do not fetch the response content.")
	flag.BoolVar(&opts.IgnoreWordlistComments, "ic", false, "Ignore wordlist comments")
	flag.BoolVar(&opts.Quiet, "s", false, "Do not print additional information (silent mode)")
	flag.BoolVar(&opts.Recursion, "recursion", false, "Scan recursively. Only FUZZ keyword is supported, and URL (-u) has to end in it.")
	flag.BoolVar(&opts.ShowVersion, "V", false, "Show version information.")
	flag.BoolVar(&opts.StopOn403, "sf", false, "Stop when > 95% of responses return 403 Forbidden")
	flag.BoolVar(&opts.StopOnAll, "sa", false, "Stop on all error cases. Implies -sf and -se.")
	flag.BoolVar(&opts.StopOnErrors, "se", false, "Stop on spurious errors")
	flag.BoolVar(&opts.Verbose, "v", false, "Verbose output, printing full URL and redirect location (if any) with the results.")
	flag.IntVar(&opts.InputNum, "input-num", 100, "Number of inputs to test. Used in conjunction with --input-cmd.")
	flag.IntVar(&opts.MaxTime, "maxtime", 0, "Maximum running time in seconds for entire process.")
	flag.IntVar(&opts.MaxTimeJob, "maxtime-job", 0, "Maximum running time in seconds per job.")
	flag.IntVar(&opts.Rate, "rate", 0, "Rate of requests per second")
	flag.IntVar(&opts.RecursionDepth, "recursion-depth", 0, "Maximum recursion depth.")
	flag.IntVar(&opts.Threads, "t", 40, "Number of concurrent threads.")
	flag.IntVar(&opts.Timeout, "timeout", 10, "HTTP request timeout in seconds.")
	flag.StringVar(&opts.Data, "d", "", "POST data")
	flag.StringVar(&opts.Data, "data", "", "POST data (alias of -d)")
	flag.StringVar(&opts.Data, "data-ascii", "", "POST data (alias of -d)")
	flag.StringVar(&opts.Data, "data-binary", "", "POST data (alias of -d)")
	flag.StringVar(&opts.DebugLog, "debug-log", "", "Write all of the internal logging to the specified file.")
	flag.StringVar(&opts.Delay, "p", "", "Seconds of `delay` between requests, or a range of random delay. For example \"0.1\" or \"0.1-2.0\"")
	flag.StringVar(&opts.Extensions, "e", "", "Comma separated list of extensions. Extends FUZZ keyword.")
	flag.StringVar(&opts.FilterLines, "fl", "", "Filter by amount of lines in response. Comma separated list of line counts and ranges")
	flag.StringVar(&opts.FilterRegexp, "fr", "", "Filter regexp")
	flag.StringVar(&opts.FilterSize, "fs", "", "Filter HTTP response size. Comma separated list of sizes and ranges")
	flag.StringVar(&opts.FilterStatus, "fc", "", "Filter HTTP status codes from response. Comma separated list of codes and ranges")
	flag.StringVar(&opts.FilterWords, "fw", "", "Filter by amount of words in response. Comma separated list of word counts and ranges")
	flag.StringVar(&opts.InputMode, "mode", "clusterbomb", "Multi-wordlist operation mode. Available modes: clusterbomb, pitchfork")
	flag.StringVar(&opts.MatcherLines, "ml", "", "Match amount of lines in response")
	flag.StringVar(&opts.MatcherRegexp, "mr", "", "Match regexp")
	flag.StringVar(&opts.MatcherSize, "ms", "", "Match HTTP response size")
	flag.StringVar(&opts.MatcherStatus, "mc", "200,204,301,302,307,401,403", "Match HTTP status codes, or \"all\" for everything.")
	flag.StringVar(&opts.MatcherWords, "mw", "", "Match amount of words in response")
	flag.StringVar(&opts.Method, "X", "GET", "HTTP method to use")
	flag.StringVar(&opts.OutputDirectory, "od", "", "Directory path to store matched results to.")
	flag.StringVar(&opts.OutputFile, "o", "", "Write output to file")
	flag.StringVar(&opts.OutputFormat, "of", "json", "Output file format. Available formats: json, ejson, html, md, csv, ecsv (or, 'all' for all formats)")
	flag.StringVar(&opts.ProxyURL, "x", "", "HTTP Proxy URL")
	flag.StringVar(&opts.ReplayProxyURL, "replay-proxy", "", "Replay matched requests using this proxy.")
	flag.StringVar(&opts.Request, "request", "", "File containing the raw http request")
	flag.StringVar(&opts.RequestProto, "request-proto", "https", "Protocol to use along with raw request")
	flag.StringVar(&opts.URL, "u", "", "Target URL")
	flag.Var(&autocalibrationstrings, "acc", "Custom auto-calibration string. Can be used multiple times. Implies -ac")
	flag.Var(&cookies, "b", "Cookie data `\"NAME1=VALUE1; NAME2=VALUE2\"` for copy as curl functionality.")
	flag.Var(&cookies, "cookie", "Cookie data (alias of -b)")
	flag.Var(&headers, "H", "Header `\"Name: Value\"`, separated by colon. Multiple -H flags are accepted.")
	flag.Var(&inputcommands, "input-cmd", "Command producing the input. --input-num is required when using this input method. Overrides -w.")
	flag.Var(&wordlists, "w", "Wordlist file path and (optional) keyword separated by colon. eg. '/path/to/wordlist:KEYWORD'")
	flag.Usage = Usage
	flag.Parse()
	opts.AutoCalibrationStrings = autocalibrationstrings
	opts.Cookies = cookies
	opts.Headers = headers
	opts.Inputcommands = inputcommands
	opts.Wordlists = wordlists

	if opts.ShowVersion {
		fmt.Printf("ffuf version: %s\n", ffuf.VERSION)
		os.Exit(0)
	}
	if len(opts.DebugLog) != 0 {
		f, err := os.OpenFile(opts.DebugLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

	// Prepare context and set up Config struct
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf, err := ffuf.ConfigFromOptions(&opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		Usage()
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		os.Exit(1)
	}
	conf.SetContext(ctx)

	job, err := prepareJob(conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		Usage()
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		os.Exit(1)
	}
	if err := filter.SetupFilters(&opts, conf); err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		Usage()
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
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
	job := ffuf.NewJob(conf)
	var errs ffuf.Multierror
	job.Input, errs = input.NewInputProvider(conf)
	// TODO: implement error handling for runnerprovider and outputprovider
	// We only have http runner right now
	job.Runner = runner.NewRunnerByName("http", conf, false)
	if len(conf.ReplayProxyURL) > 0 {
		job.ReplayRunner = runner.NewRunnerByName("http", conf, true)
	}
	// We only have stdout outputprovider right now
	job.Output = output.NewOutputProviderByName("stdout", conf)
	return job, errs.ErrorOrNil()
}
