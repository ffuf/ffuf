package ffuf

import (
	"fmt"

	"github.com/ffuf/ffuf/v2/pkg/input"
	"github.com/ffuf/ffuf/v2/pkg/output"
	"github.com/ffuf/ffuf/v2/pkg/runner"
)

func WordlistInputFromConfig(conf *Config) (*input.WordlistInput, error) {
	wl, err := input.NewWordlistInput("wordlist", "FUZZ", conf)
	if err != nil {
		return nil, fmt.Errorf("creating new wordlist input: %w", err)
	}
	return wl, nil
}

func PrepareJob(conf *Config, wl *input.WordlistInput) (*Job, error) {
	job := NewJob(conf)
	var errs Multierror

	job.Input, errs = input.NewWordlistInputProvider(conf, wl)

	job.Runner = runner.NewRunnerByName("http", conf, false)
	if len(conf.ReplayProxyURL) > 0 {
		job.ReplayRunner = runner.NewRunnerByName("http", conf, true)
	}
	// We only have stdout outputprovider right now
	job.Output = output.NewOutputProviderByName("stdout", conf)
	return job, errs.ErrorOrNil()
}
