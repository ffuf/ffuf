package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/ffuf/ffuf/v2/pkg/filter"
	"github.com/ffuf/ffuf/v2/pkg/input"
	"github.com/ffuf/ffuf/v2/pkg/interactive"
	"github.com/ffuf/ffuf/v2/pkg/output"
	"github.com/ffuf/ffuf/v2/pkg/runner"
	"github.com/ffuf/ffuf/v2/pkg/scraper"
)

// flagRegistry describes the registered flags for the segmented help output
// (see Usage in help.go). It is (re)populated on every ParseFlags call.
var flagRegistry *ffuf.FlagRegistry

// ParseFlags registers every CLI flag from ffuf.RegisterFlags — the single source
// of truth — onto the global flag set and parses the command line into opts.
// Flag defaults are the current field values, so a value loaded from a config
// file is used unless the same flag is given on the command line (file < CLI).
func ParseFlags(opts *ffuf.ConfigOptions) *ffuf.ConfigOptions {
	flagRegistry = ffuf.RegisterFlags(flag.CommandLine, opts)
	flag.Usage = Usage
	flag.Parse()
	return opts
}

func main() {

	var err, optserr error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// prepare the default config options from default config file
	var opts *ffuf.ConfigOptions
	opts, optserr = ffuf.ReadDefaultConfig()

	opts = ParseFlags(opts)

	// Handle searchhash functionality and exit
	if opts.General.Searchhash != "" {
		coptions, pos, err := ffuf.SearchHash(opts.General.Searchhash)
		if err != nil {
			fmt.Printf("[ERR] %s\n", err)
			os.Exit(1)
		}
		if len(coptions) > 0 {
			fmt.Printf("Request candidate(s) for hash %s\n", opts.General.Searchhash)
		}
		for _, copt := range coptions {
			conf, err := ffuf.ConfigFromOptions(&copt.ConfigOptions, ctx, cancel)
			if err != nil {
				continue
			}
			ok, reason := ffuf.HistoryReplayable(conf)
			if ok {
				printSearchResults(conf, pos, copt.Time, opts.General.Searchhash)
			} else {
				fmt.Printf("[ERR] Hash cannot be mapped back because %s\n", reason)
			}

		}
		if err != nil {
			fmt.Printf("[ERR] %s\n", err)
		}
		os.Exit(0)
	}

	if opts.General.ShowVersion {
		fmt.Printf("ffuf version: %s\n", ffuf.Version())
		os.Exit(0)
	}
	if len(opts.Output.DebugLog) != 0 {
		f, err := os.OpenFile(opts.Output.DebugLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Disabling logging, encountered error(s): %s\n", err)
			log.SetOutput(io.Discard)
		} else {
			log.SetOutput(f)
			defer f.Close()
		}
	} else {
		log.SetOutput(io.Discard)
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

	// Set up Config struct
	conf, err := ffuf.ConfigFromOptions(opts, ctx, cancel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		Usage()
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		os.Exit(1)
	}

	job, err := prepareJob(conf)

	if job.AuditLogger != nil {
		defer job.AuditLogger.Close()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		Usage()
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		os.Exit(1)
	}
	if err := SetupFilters(opts, conf); err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		Usage()
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		os.Exit(1)
	}

	if !conf.Noninteractive {
		go func() {
			err := interactive.Handle(job)
			if err != nil {
				log.Printf("Error while trying to initialize interactive session: %s", err)
			}
		}()
	}

	// Job handles waiting for goroutines to complete itself
	job.Start()
}

func prepareJob(conf *ffuf.Config) (*ffuf.Job, error) {
	var err error
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

	// Initialize the audit logger if specified
	if len(conf.AuditLog) > 0 {
		job.AuditLogger, err = output.NewAuditLogger(conf.AuditLog)
		if err != nil {
			errs.Add(err)
		} else {
			err = job.AuditLogger.Write(conf)
			if err != nil {
				errs.Add(err)
			}
		}
	}

	// Initialize scraper
	newscraper, scraper_err := scraper.FromDir(ffuf.SCRAPERDIR, conf.Scrapers)
	if scraper_err.ErrorOrNil() != nil {
		errs.Add(scraper_err.ErrorOrNil())
	}
	job.Scraper = newscraper
	if conf.ScraperFile != "" {
		err = job.Scraper.AppendFromFile(conf.ScraperFile)
		if err != nil {
			errs.Add(err)
		}
	}
	return job, errs.ErrorOrNil()
}

func SetupFilters(parseOpts *ffuf.ConfigOptions, conf *ffuf.Config) error {
	errs := ffuf.NewMultierror()
	conf.MatcherManager = filter.NewMatcherManager()
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
		if f.Name == "mt" {
			matcherSet = true
		}
		if f.Name == "mw" {
			matcherSet = true
			warningIgnoreBody = true
		}
	})
	// Only set default matchers if no
	if statusSet || !matcherSet {
		if err := conf.MatcherManager.AddMatcher("status", parseOpts.Matcher.Status); err != nil {
			errs.Add(err)
		}
	}

	if parseOpts.Filter.Status != "" {
		if err := conf.MatcherManager.AddFilter("status", parseOpts.Filter.Status, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Size != "" {
		warningIgnoreBody = true
		if err := conf.MatcherManager.AddFilter("size", parseOpts.Filter.Size, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Regexp != "" {
		if err := conf.MatcherManager.AddFilter("regexp", parseOpts.Filter.Regexp, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Words != "" {
		warningIgnoreBody = true
		if err := conf.MatcherManager.AddFilter("word", parseOpts.Filter.Words, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Lines != "" {
		warningIgnoreBody = true
		if err := conf.MatcherManager.AddFilter("line", parseOpts.Filter.Lines, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Time != "" {
		if err := conf.MatcherManager.AddFilter("time", parseOpts.Filter.Time, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Size != "" {
		if err := conf.MatcherManager.AddMatcher("size", parseOpts.Matcher.Size); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Regexp != "" {
		if err := conf.MatcherManager.AddMatcher("regexp", parseOpts.Matcher.Regexp); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Words != "" {
		if err := conf.MatcherManager.AddMatcher("word", parseOpts.Matcher.Words); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Lines != "" {
		if err := conf.MatcherManager.AddMatcher("line", parseOpts.Matcher.Lines); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Time != "" {
		if err := conf.MatcherManager.AddMatcher("time", parseOpts.Matcher.Time); err != nil {
			errs.Add(err)
		}
	}
	if conf.IgnoreBody && warningIgnoreBody {
		fmt.Printf("*** Warning: possible undesired combination of -ignore-body and the response options: fl,fs,fw,ml,ms and mw.\n")
	}
	return errs.ErrorOrNil()
}

func printSearchResults(conf *ffuf.Config, pos int, exectime time.Time, hash string) {
	inp, err := input.NewInputProvider(conf)
	if err.ErrorOrNil() != nil {
		fmt.Printf("-------------------------------------------\n")
		fmt.Println("Encountered error that prevents reproduction of the request:")
		fmt.Println(err.ErrorOrNil())
		return
	}
	inp.SetPosition(pos)
	inputdata := inp.Value()
	inputdata["FFUFHASH"] = []byte(hash)
	basereq := ffuf.BaseRequest(conf)
	dummyrunner := runner.NewRunnerByName("simple", conf, false)
	ffufreq, _ := dummyrunner.Prepare(inputdata, &basereq)
	rawreq, _ := dummyrunner.Dump(&ffufreq)
	fmt.Printf("-------------------------------------------\n")
	fmt.Printf("ffuf job started at: %s\n\n", exectime.Format(time.RFC3339))
	fmt.Printf("%s\n", string(rawreq))
}
