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
	opts := ffuf.NewConfigOptions()
	var ignored bool
	var cookies, autocalibrationstrings, headers, inputcommands multiStringFlag
	var wordlists wordlistFlag
	flag.BoolVar(&ignored, "compressed", true, "Dummy flag for copy as curl functionality (ignored)")
	flag.BoolVar(&ignored, "i", true, "Dummy flag for copy as curl functionality (ignored)")
	flag.BoolVar(&ignored, "k", false, "Dummy flag for backwards compatibility")
	flag.BoolVar(&opts.AutoCalibration, "ac", opts.AutoCalibration, "Automatically calibrate filtering options")
	flag.BoolVar(&opts.Colors, "c", opts.Colors, "Colorize output.")
	flag.BoolVar(&opts.DirSearchCompat, "D", opts.DirSearchCompat, "DirSearch wordlist compatibility mode. Used in conjunction with -e flag.")
	flag.BoolVar(&opts.FollowRedirects, "r", opts.FollowRedirects, "Follow redirects")
	flag.BoolVar(&opts.IgnoreBody, "ignore-body", opts.IgnoreBody, "Do not fetch the response content.")
	flag.BoolVar(&opts.IgnoreWordlistComments, "ic", opts.IgnoreWordlistComments, "Ignore wordlist comments")
	flag.BoolVar(&opts.Quiet, "s", opts.Quiet, "Do not print additional information (silent mode)")
	flag.BoolVar(&opts.Recursion, "recursion", opts.Recursion, "Scan recursively. Only FUZZ keyword is supported, and URL (-u) has to end in it.")
	flag.BoolVar(&opts.ShowVersion, "V", opts.ShowVersion, "Show version information.")
	flag.BoolVar(&opts.StopOn403, "sf", opts.StopOn403, "Stop when > 95% of responses return 403 Forbidden")
	flag.BoolVar(&opts.StopOnAll, "sa", opts.StopOnAll, "Stop on all error cases. Implies -sf and -se.")
	flag.BoolVar(&opts.StopOnErrors, "se", opts.StopOnErrors, "Stop on spurious errors")
	flag.BoolVar(&opts.Verbose, "v", opts.Verbose, "Verbose output, printing full URL and redirect location (if any) with the results.")
	flag.IntVar(&opts.InputNum, "input-num", opts.InputNum, "Number of inputs to test. Used in conjunction with --input-cmd.")
	flag.IntVar(&opts.MaxTime, "maxtime", opts.MaxTime, "Maximum running time in seconds for entire process.")
	flag.IntVar(&opts.MaxTimeJob, "maxtime-job", opts.MaxTimeJob, "Maximum running time in seconds per job.")
	flag.IntVar(&opts.Rate, "rate", opts.Rate, "Rate of requests per second")
	flag.IntVar(&opts.RecursionDepth, "recursion-depth", opts.RecursionDepth, "Maximum recursion depth.")
	flag.IntVar(&opts.Threads, "t", opts.Threads, "Number of concurrent threads.")
	flag.IntVar(&opts.Timeout, "timeout", opts.Timeout, "HTTP request timeout in seconds.")
	flag.StringVar(&opts.Data, "d", opts.Data, "POST data")
	flag.StringVar(&opts.Data, "data", opts.Data, "POST data (alias of -d)")
	flag.StringVar(&opts.Data, "data-ascii", opts.Data, "POST data (alias of -d)")
	flag.StringVar(&opts.Data, "data-binary", opts.Data, "POST data (alias of -d)")
	flag.StringVar(&opts.DebugLog, "debug-log", opts.DebugLog, "Write all of the internal logging to the specified file.")
	flag.StringVar(&opts.Delay, "p", opts.Delay, "Seconds of `delay` between requests, or a range of random delay. For example \"0.1\" or \"0.1-2.0\"")
	flag.StringVar(&opts.Extensions, "e", opts.Extensions, "Comma separated list of extensions. Extends FUZZ keyword.")
	flag.StringVar(&opts.FilterLines, "fl", opts.FilterLines, "Filter by amount of lines in response. Comma separated list of line counts and ranges")
	flag.StringVar(&opts.FilterRegexp, "fr", opts.FilterRegexp, "Filter regexp")
	flag.StringVar(&opts.FilterSize, "fs", opts.FilterSize, "Filter HTTP response size. Comma separated list of sizes and ranges")
	flag.StringVar(&opts.FilterStatus, "fc", opts.FilterStatus, "Filter HTTP status codes from response. Comma separated list of codes and ranges")
	flag.StringVar(&opts.FilterWords, "fw", opts.FilterWords, "Filter by amount of words in response. Comma separated list of word counts and ranges")
	flag.StringVar(&opts.InputMode, "mode", opts.InputMode, "Multi-wordlist operation mode. Available modes: clusterbomb, pitchfork")
	flag.StringVar(&opts.MatcherLines, "ml", opts.MatcherLines, "Match amount of lines in response")
	flag.StringVar(&opts.MatcherRegexp, "mr", opts.MatcherRegexp, "Match regexp")
	flag.StringVar(&opts.MatcherSize, "ms", opts.MatcherSize, "Match HTTP response size")
	flag.StringVar(&opts.MatcherStatus, "mc", opts.MatcherSize, "Match HTTP status codes, or \"all\" for everything.")
	flag.StringVar(&opts.MatcherWords, "mw", opts.MatcherWords, "Match amount of words in response")
	flag.StringVar(&opts.Method, "X", opts.Method, "HTTP method to use")
	flag.StringVar(&opts.OutputDirectory, opts.OutputDirectory, "", "Directory path to store matched results to.")
	flag.StringVar(&opts.OutputFile, "o", opts.OutputFile, "Write output to file")
	flag.StringVar(&opts.OutputFormat, "of", opts.OutputFormat, "Output file format. Available formats: json, ejson, html, md, csv, ecsv (or, 'all' for all formats)")
	flag.StringVar(&opts.ProxyURL, "x", opts.ProxyURL, "HTTP Proxy URL")
	flag.StringVar(&opts.ReplayProxyURL, "replay-proxy", opts.ReplayProxyURL, "Replay matched requests using this proxy.")
	flag.StringVar(&opts.Request, "request", opts.Request, "File containing the raw http request")
	flag.StringVar(&opts.RequestProto, "request-proto", opts.RequestProto, "Protocol to use along with raw request")
	flag.StringVar(&opts.URL, "u", opts.URL, "Target URL")
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
	conf, err := ffuf.ConfigFromOptions(opts)
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
	if err := filter.SetupFilters(opts, conf); err != nil {
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
