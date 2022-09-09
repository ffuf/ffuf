package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/ffuf/ffuf/pkg/config"
	"github.com/ffuf/ffuf/pkg/ffuf"
	"github.com/ffuf/ffuf/pkg/input"
	"github.com/ffuf/ffuf/pkg/interactive"
	"github.com/ffuf/ffuf/pkg/output"
	"github.com/ffuf/ffuf/pkg/runner"
	"github.com/ffuf/ffuf/pkg/utils"
)

func main() {

	var (
		opts *config.ConfigOptions
		conf *config.Config
		err  error
	)

	opts, conf, err = config.Get()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// if -h or -help is encountered
			config.Usage()
			os.Exit(1)
		} else {
			// other configuration errors
			fmt.Fprintf(os.Stderr, "error(s) encountered during configuration: %s\n", err)
			config.Usage()
			fmt.Fprintf(os.Stderr, "error(s) encountered during configuration: %s\n", err)
			os.Exit(1)
		}
	}

	if opts.General.ShowVersion {
		fmt.Printf("ffuf version: %s\n", utils.Version())
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

	// Prepare context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conf.Context = ctx
	conf.Cancel = cancel

	job, err := prepareJob(conf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encountered error(s): %s\n", err)
		config.Usage()
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

func prepareJob(conf *config.Config) (*ffuf.Job, error) {
	job := ffuf.NewJob(conf)
	var errs utils.Multierror
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
