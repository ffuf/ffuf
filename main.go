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
	showVersion   bool
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
	flag.Var(&opts.headers, "H", "Header `\"Name: Value\"`, separated by colon. Multiple -H flags are accepted.")
	flag.StringVar(&conf.Url, "u", "", "Target URL")
	flag.StringVar(&conf.Wordlist, "w", "", "Wordlist path")
	flag.BoolVar(&conf.TLSSkipVerify, "k", false, "Skip TLS identity verification (insecure)")
	flag.StringVar(&opts.filterStatus, "fc", "", "Filter HTTP status codes from response")
	flag.StringVar(&opts.filterSize, "fs", "", "Filter HTTP response size")
	flag.StringVar(&opts.filterRegexp, "fr", "", "Filter regexp")
	flag.StringVar(&opts.filterWords, "fw", "", "Filter by amount of words in response")
	flag.StringVar(&conf.Data, "d", "", "POST data.")
	flag.BoolVar(&conf.Colors, "c", false, "Colorize output.")
	flag.StringVar(&opts.matcherStatus, "mc", "200,204,301,302,307,401,403", "Match HTTP status codes from respose")
	flag.StringVar(&opts.matcherSize, "ms", "", "Match HTTP response size")
	flag.StringVar(&opts.matcherRegexp, "mr", "", "Match regexp")
	flag.StringVar(&opts.matcherWords, "mw", "", "Match amount of words in response")
	flag.StringVar(&conf.Method, "X", "GET", "HTTP method to use.")
	flag.BoolVar(&conf.Quiet, "s", false, "Do not print additional information (silent mode)")
	flag.IntVar(&conf.Threads, "t", 40, "Number of concurrent threads.")
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
	errs := ffuf.NewMultierror()
	// TODO: implement error handling for runnerprovider and outputprovider
	// We only have http runner right now
	runprovider := runner.NewRunnerByName("http", conf)
	// We only have wordlist inputprovider right now
	inputprovider, err := input.NewInputProviderByName("wordlist", conf)
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

func prepareConfig(parseOpts *cliOptions, conf *ffuf.Config) error {
	//TODO: refactor in a proper flag library that can handle things like required flags
	errs := ffuf.NewMultierror()
	foundkeyword := false
	if len(conf.Url) == 0 {
		errs.Add(fmt.Errorf("-u flag is required"))
	}
	if len(conf.Wordlist) == 0 {
		errs.Add(fmt.Errorf("-w flag is required"))
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

func prepareFilters(parseOpts *cliOptions, conf *ffuf.Config) error {
	errs := ffuf.NewMultierror()
	if parseOpts.filterStatus != "" {
		if err := addFilter(conf, "status", parseOpts.filterStatus); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.filterSize != "" {
		if err := addFilter(conf, "size", parseOpts.filterSize); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.filterRegexp != "" {
		if err := addFilter(conf, "regexp", parseOpts.filterRegexp); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.filterWords != "" {
		if err := addFilter(conf, "word", parseOpts.filterWords); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.matcherStatus != "" {
		if err := addMatcher(conf, "status", parseOpts.matcherStatus); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.matcherSize != "" {
		if err := addMatcher(conf, "size", parseOpts.matcherSize); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.matcherRegexp != "" {
		if err := addMatcher(conf, "regexp", parseOpts.matcherRegexp); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.matcherWords != "" {
		if err := addMatcher(conf, "word", parseOpts.matcherWords); err != nil {
			errs.Add(err)
		}
	}
	return errs.ErrorOrNil()
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
