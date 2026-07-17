// Package assembly wires a ffuf.Job together from a Config: the input provider,
// runner(s), output provider, audit logger, and scraper. It exists as its own
// package because it must import the provider packages (input/runner/output/
// scraper), which themselves import pkg/ffuf - so this composition cannot live in
// pkg/ffuf without an import cycle, and it cannot stay unexported in package main
// without being untestable. Both main and the integration tests call BuildJob, so
// the production and test wiring can never drift.
package assembly

import (
	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/ffuf/ffuf/v2/pkg/input"
	"github.com/ffuf/ffuf/v2/pkg/output"
	"github.com/ffuf/ffuf/v2/pkg/runner"
	"github.com/ffuf/ffuf/v2/pkg/scraper"
)

// BuildJob constructs and wires a Job from conf. It is the single source of truth
// for how a Job is assembled. Matchers/filters are NOT installed here (they need
// the CLI flag state); see filter.FromConfig and main.SetupFilters.
func BuildJob(conf *ffuf.Config) (*ffuf.Job, error) {
	var err error
	job := ffuf.NewJob(conf)
	var errs ffuf.Multierror

	job.Input, errs = input.NewInputProvider(conf)

	// Only the http (SimpleRunner) runner exists today.
	job.Runner = runner.NewSimpleRunner(conf, false)
	if len(conf.ReplayProxyURL) > 0 {
		job.ReplayRunner = runner.NewSimpleRunner(conf, true)
	}

	// Only the stdout output provider exists today.
	job.Output = output.NewStdoutput(conf)

	if len(conf.AuditLog) > 0 {
		job.AuditLogger, err = output.NewAuditLogger(conf.AuditLog)
		if err != nil {
			errs.Add(err)
		} else {
			if err = job.AuditLogger.Write(conf); err != nil {
				errs.Add(err)
			}
		}
	}

	newscraper, scraperErr := scraper.FromDir(ffuf.SCRAPERDIR, conf.Scrapers)
	if scraperErr.ErrorOrNil() != nil {
		errs.Add(scraperErr.ErrorOrNil())
	}
	job.Scraper = newscraper
	if conf.ScraperFile != "" {
		if err = job.Scraper.AppendFromFile(conf.ScraperFile); err != nil {
			errs.Add(err)
		}
	}

	return job, errs.ErrorOrNil()
}
