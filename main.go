package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"
	"github.com/ffuf/ffuf/pkg/filter"
	"github.com/ffuf/ffuf/pkg/input"
	"github.com/ffuf/ffuf/pkg/output"
	"github.com/ffuf/ffuf/pkg/runner"

	"github.com/hashicorp/go-multierror"
)

type cliOptions struct {
	filterStatus  string
	filterSize    string
	filterRegexp  string
	filterWords   string
	matcherStatus string
	matcherSize   string
	matcherRegexp string
	matcherWords  string
	headers       headerFlags
}

type headerFlags []string

func (h *headerFlags) String() string {
	return ""
}

func (h *headerFlags) Set(value string) error {
	*h = append(*h, value)
	return nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf := ffuf.NewConfig(ctx)
	opts := cliOptions{}
	flag.Var(&opts.headers, "H", "Header name and value, separated by colon. Multiple -H flags are accepted.")
	flag.StringVar(&conf.Url, "u", "", "Target URL")
	flag.StringVar(&conf.Wordlist, "w", "", "Wordlist path")
	flag.BoolVar(&conf.TLSSkipVerify, "k", false, "Skip TLS identity verification (insecure)")
	flag.StringVar(&opts.filterStatus, "fc", "", "Filter HTTP status codes from response")
	flag.StringVar(&opts.filterSize, "fs", "", "Filter HTTP response size")
	flag.StringVar(&opts.filterRegexp, "fr", "", "Filter regexp")
	flag.StringVar(&opts.filterWords, "fw", "", "Filter by amount of words in response")
	flag.StringVar(&conf.Data, "d", "", "POST data.")
	flag.BoolVar(&conf.Colors, "c", false, "Colorize output.")
	flag.StringVar(&opts.matcherStatus, "mc", "200,204,301,302,307,401", "Match HTTP status codes from respose")
	flag.StringVar(&opts.matcherSize, "ms", "", "Match HTTP response size")
	flag.StringVar(&opts.matcherRegexp, "mr", "", "Match regexp")
	flag.StringVar(&opts.matcherWords, "mw", "", "Match amount of words in response")
	flag.StringVar(&conf.Method, "X", "GET", "HTTP method to use.")
	flag.BoolVar(&conf.Quiet, "s", false, "Do not print additional information (silent mode)")
	flag.IntVar(&conf.Threads, "t", 20, "Number of concurrent threads.")
	flag.Parse()
	if err := prepareConfig(&opts, &conf); err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		flag.Usage()
		os.Exit(1)
	}
	if err := prepareFilters(&opts, &conf); err != nil {
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
	// Job handles waiting for goroutines to complete itself
	job.Start()
}

func prepareJob(conf *ffuf.Config) (*ffuf.Job, error) {
	var errlist *multierror.Error
	// TODO: implement error handling for runnerprovider and outputprovider
	// We only have http runner right now
	runprovider := runner.NewRunnerByName("http", conf)
	// We only have wordlist inputprovider right now
	inputprovider, err := input.NewInputProviderByName("wordlist", conf)
	if err != nil {
		errlist = multierror.Append(errlist, fmt.Errorf("%s", err))
	}
	// We only have stdout outputprovider right now
	outprovider := output.NewOutputProviderByName("stdout", conf)
	return &ffuf.Job{
		Config: conf,
		Runner: runprovider,
		Output: outprovider,
		Input:  inputprovider,
	}, errlist.ErrorOrNil()
}

func prepareConfig(parseOpts *cliOptions, conf *ffuf.Config) error {
	//TODO: refactor in a proper flag library that can handle things like required flags
	var errlist *multierror.Error
	foundkeyword := false
	if len(conf.Url) == 0 {
		errlist = multierror.Append(errlist, fmt.Errorf("-u flag is required"))
	}
	if len(conf.Wordlist) == 0 {
		errlist = multierror.Append(errlist, fmt.Errorf("-w flag is required"))
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
			errlist = multierror.Append(errlist, fmt.Errorf("Header defined by -H needs to have a value. \":\" should be used as a separator"))
		}
	}
	//Search for keyword from URL and POST data too
	if strings.Index(conf.Url, "FUZZ") != -1 {
		foundkeyword = true
	}
	if strings.Index(conf.Data, "FUZZ") != -1 {
		foundkeyword = true
	}

	if !foundkeyword {
		errlist = multierror.Append(errlist, fmt.Errorf("No FUZZ keywords found in headers or URL, nothing to do"))
	}
	return errlist.ErrorOrNil()
}

func prepareFilters(parseOpts *cliOptions, conf *ffuf.Config) error {
	var errlist *multierror.Error
	if parseOpts.filterStatus != "" {
		if err := addFilter(conf, "status", parseOpts.filterStatus); err != nil {
			errlist = multierror.Append(errlist, err)
		}
	}
	if parseOpts.filterSize != "" {
		if err := addFilter(conf, "size", parseOpts.filterSize); err != nil {
			errlist = multierror.Append(errlist, err)
		}
	}
	if parseOpts.filterRegexp != "" {
		if err := addFilter(conf, "regexp", parseOpts.filterRegexp); err != nil {
			errlist = multierror.Append(errlist, err)
		}
	}
	if parseOpts.filterWords != "" {
		if err := addFilter(conf, "word", parseOpts.filterWords); err != nil {
			errlist = multierror.Append(errlist, err)
		}
	}
	if parseOpts.matcherStatus != "" {
		if err := addMatcher(conf, "status", parseOpts.matcherStatus); err != nil {
			errlist = multierror.Append(errlist, err)
		}
	}
	if parseOpts.matcherSize != "" {
		if err := addMatcher(conf, "size", parseOpts.matcherSize); err != nil {
			errlist = multierror.Append(errlist, err)
		}
	}
	if parseOpts.matcherRegexp != "" {
		if err := addMatcher(conf, "regexp", parseOpts.matcherRegexp); err != nil {
			errlist = multierror.Append(errlist, err)
		}
	}
	if parseOpts.matcherWords != "" {
		if err := addMatcher(conf, "word", parseOpts.matcherWords); err != nil {
			errlist = multierror.Append(errlist, err)
		}
	}
	return errlist.ErrorOrNil()
}

func addFilter(conf *ffuf.Config, name string, option string) error {
	newf, err := filter.NewFilterByName(name, option)
	if err == nil {
		conf.Filters = append(conf.Filters, newf)
	}
	return err
}

func addMatcher(conf *ffuf.Config, name string, option string) error {
	newf, err := filter.NewFilterByName(name, option)
	if err == nil {
		conf.Matchers = append(conf.Matchers, newf)
	}
	return err
}
