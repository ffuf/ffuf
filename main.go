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

//ParseFlags parses the command line flags and (re)populates the ConfigOptions struct
func ParseFlags(opts *ffuf.ConfigOptions) *ffuf.ConfigOptions {
	var ignored bool
	var cookies, autocalibrationstrings, headers, inputcommands multiStringFlag
	var wordlists wordlistFlag

	cookies = opts.HTTP.Cookies
	autocalibrationstrings = opts.General.AutoCalibrationStrings
	headers = opts.HTTP.Headers
	inputcommands = opts.Input.Inputcommands

	flag.BoolVar(&ignored, "compressed", true, "Dummy flag for copy as curl functionality (ignored)")
	flag.BoolVar(&ignored, "i", true, "Dummy flag for copy as curl functionality (ignored)")
	flag.BoolVar(&ignored, "k", false, "Dummy flag for backwards compatibility")
	flag.BoolVar(&opts.Output.OutputCreateEmptyFile, "or", opts.Output.OutputCreateEmptyFile, "Don't create the output file if we don't have results")
	flag.BoolVar(&opts.General.AutoCalibration, "ac", opts.General.AutoCalibration, "Automatically calibrate filtering options")
	flag.BoolVar(&opts.General.Colors, "c", opts.General.Colors, "Colorize output.")
	flag.BoolVar(&opts.General.Quiet, "s", opts.General.Quiet, "Do not print additional information (silent mode)")
	flag.BoolVar(&opts.General.ShowVersion, "V", opts.General.ShowVersion, "Show version information.")
	flag.BoolVar(&opts.General.StopOn403, "sf", opts.General.StopOn403, "Stop when > 95% of responses return 403 Forbidden")
	flag.BoolVar(&opts.General.StopOnAll, "sa", opts.General.StopOnAll, "Stop on all error cases. Implies -sf and -se.")
	flag.BoolVar(&opts.General.StopOnErrors, "se", opts.General.StopOnErrors, "Stop on spurious errors")
	flag.BoolVar(&opts.General.Verbose, "v", opts.General.Verbose, "Verbose output, printing full URL and redirect location (if any) with the results.")
	flag.BoolVar(&opts.HTTP.FollowRedirects, "r", opts.HTTP.FollowRedirects, "Follow redirects")
	flag.BoolVar(&opts.HTTP.IgnoreBody, "ignore-body", opts.HTTP.IgnoreBody, "Do not fetch the response content.")
	flag.BoolVar(&opts.HTTP.Recursion, "recursion", opts.HTTP.Recursion, "Scan recursively. Only FUZZ keyword is supported, and URL (-u) has to end in it.")
	flag.BoolVar(&opts.Input.DirSearchCompat, "D", opts.Input.DirSearchCompat, "DirSearch wordlist compatibility mode. Used in conjunction with -e flag.")
	flag.BoolVar(&opts.Input.IgnoreWordlistComments, "ic", opts.Input.IgnoreWordlistComments, "Ignore wordlist comments")
	flag.IntVar(&opts.General.MaxTime, "maxtime", opts.General.MaxTime, "Maximum running time in seconds for entire process.")
	flag.IntVar(&opts.General.MaxTimeJob, "maxtime-job", opts.General.MaxTimeJob, "Maximum running time in seconds per job.")
	flag.IntVar(&opts.General.Rate, "rate", opts.General.Rate, "Rate of requests per second")
	flag.IntVar(&opts.General.Threads, "t", opts.General.Threads, "Number of concurrent threads.")
	flag.IntVar(&opts.HTTP.RecursionDepth, "recursion-depth", opts.HTTP.RecursionDepth, "Maximum recursion depth.")
	flag.IntVar(&opts.HTTP.Timeout, "timeout", opts.HTTP.Timeout, "HTTP request timeout in seconds.")
	flag.IntVar(&opts.Input.InputNum, "input-num", opts.Input.InputNum, "Number of inputs to test. Used in conjunction with --input-cmd.")
	flag.StringVar(&opts.General.ConfigFile, "config", "", "Load configuration from a file")
	flag.StringVar(&opts.Filter.Lines, "fl", opts.Filter.Lines, "Filter by amount of lines in response. Comma separated list of line counts and ranges")
	flag.StringVar(&opts.Filter.Regexp, "fr", opts.Filter.Regexp, "Filter regexp")
	flag.StringVar(&opts.Filter.Size, "fs", opts.Filter.Size, "Filter HTTP response size. Comma separated list of sizes and ranges")
	flag.StringVar(&opts.Filter.Status, "fc", opts.Filter.Status, "Filter HTTP status codes from response. Comma separated list of codes and ranges")
	flag.StringVar(&opts.Filter.Words, "fw", opts.Filter.Words, "Filter by amount of words in response. Comma separated list of word counts and ranges")
	flag.StringVar(&opts.General.Delay, "p", opts.General.Delay, "Seconds of `delay` between requests, or a range of random delay. For example \"0.1\" or \"0.1-2.0\"")
	flag.StringVar(&opts.HTTP.Data, "d", opts.HTTP.Data, "POST data")
	flag.StringVar(&opts.HTTP.Data, "data", opts.HTTP.Data, "POST data (alias of -d)")
	flag.StringVar(&opts.HTTP.Data, "data-ascii", opts.HTTP.Data, "POST data (alias of -d)")
	flag.StringVar(&opts.HTTP.Data, "data-binary", opts.HTTP.Data, "POST data (alias of -d)")
	flag.StringVar(&opts.HTTP.Method, "X", opts.HTTP.Method, "HTTP method to use")
	flag.StringVar(&opts.HTTP.ProxyURL, "x", opts.HTTP.ProxyURL, "HTTP Proxy URL")
	flag.StringVar(&opts.HTTP.ReplayProxyURL, "replay-proxy", opts.HTTP.ReplayProxyURL, "Replay matched requests using this proxy.")
	flag.StringVar(&opts.HTTP.URL, "u", opts.HTTP.URL, "Target URL")
	flag.StringVar(&opts.Input.Extensions, "e", opts.Input.Extensions, "Comma separated list of extensions. Extends FUZZ keyword.")
	flag.StringVar(&opts.Input.InputMode, "mode", opts.Input.InputMode, "Multi-wordlist operation mode. Available modes: clusterbomb, pitchfork")
	flag.StringVar(&opts.Input.Request, "request", opts.Input.Request, "File containing the raw http request")
	flag.StringVar(&opts.Input.RequestProto, "request-proto", opts.Input.RequestProto, "Protocol to use along with raw request")
	flag.StringVar(&opts.Matcher.Lines, "ml", opts.Matcher.Lines, "Match amount of lines in response")
	flag.StringVar(&opts.Matcher.Regexp, "mr", opts.Matcher.Regexp, "Match regexp")
	flag.StringVar(&opts.Matcher.Size, "ms", opts.Matcher.Size, "Match HTTP response size")
	flag.StringVar(&opts.Matcher.Status, "mc", opts.Matcher.Status, "Match HTTP status codes, or \"all\" for everything.")
	flag.StringVar(&opts.Matcher.Words, "mw", opts.Matcher.Words, "Match amount of words in response")
	flag.StringVar(&opts.Output.DebugLog, "debug-log", opts.Output.DebugLog, "Write all of the internal logging to the specified file.")
	flag.StringVar(&opts.Output.OutputDirectory, "od", opts.Output.OutputDirectory, "Directory path to store matched results to.")
	flag.StringVar(&opts.Output.OutputFile, "o", opts.Output.OutputFile, "Write output to file")
	flag.StringVar(&opts.Output.OutputFormat, "of", opts.Output.OutputFormat, "Output file format. Available formats: json, ejson, html, md, csv, ecsv (or, 'all' for all formats)")
	flag.Var(&autocalibrationstrings, "acc", "Custom auto-calibration string. Can be used multiple times. Implies -ac")
	flag.Var(&cookies, "b", "Cookie data `\"NAME1=VALUE1; NAME2=VALUE2\"` for copy as curl functionality.")
	flag.Var(&cookies, "cookie", "Cookie data (alias of -b)")
	flag.Var(&headers, "H", "Header `\"Name: Value\"`, separated by colon. Multiple -H flags are accepted.")
	flag.Var(&inputcommands, "input-cmd", "Command producing the input. --input-num is required when using this input method. Overrides -w.")
	flag.Var(&wordlists, "w", "Wordlist file path and (optional) keyword separated by colon. eg. '/path/to/wordlist:KEYWORD'")
	flag.Usage = Usage
	flag.Parse()

	opts.General.AutoCalibrationStrings = autocalibrationstrings
	opts.HTTP.Cookies = cookies
	opts.HTTP.Headers = headers
	opts.Input.Inputcommands = inputcommands
	opts.Input.Wordlists = wordlists
	return opts
}

func main() {

	var err, optserr error

	// prepare the default config options from default config file
	var opts *ffuf.ConfigOptions
	opts, optserr = ffuf.ReadDefaultConfig()

	opts = ParseFlags(opts)

	if opts.General.ShowVersion {
		fmt.Printf("ffuf version: %s\n", ffuf.VERSION)
		os.Exit(0)
	}
	if len(opts.Output.DebugLog) != 0 {
		f, err := os.OpenFile(opts.Output.DebugLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
	if optserr != nil {
		log.Printf("Error while opening default config file: %s", optserr)
	}

	if opts.General.ConfigFile != "" {
		opts, err = ffuf.ReadConfig(opts.General.ConfigFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Encoutered error(s): %s\n", err)
			Usage()
			fmt.Fprintf(os.Stderr, "Encoutered error(s): %s\n", err)
			os.Exit(1)
		}
		// Reset the flag package state
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		// Re-parse the cli options
		opts = ParseFlags(opts)
	}

	// Prepare context and set up Config struct
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf, err := ffuf.ConfigFromOptions(opts, ctx, cancel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		Usage()
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		os.Exit(1)
	}
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
