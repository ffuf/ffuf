package filter

import (
	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// FromConfig builds a MatcherManager from the parsed ConfigOptions. It is the pure,
// global-free core of what main.SetupFilters used to do inline, so matcher/filter
// setup is now unit-testable without the CLI flag package. The one CLI-explicit
// decision - whether to install the default status matcher - needs the global flag
// set, so it stays in main and is passed in as addDefaultStatusMatcher.
func FromConfig(opts *ffuf.ConfigOptions, addDefaultStatusMatcher bool) (ffuf.MatcherManager, error) {
	errs := ffuf.NewMultierror()
	mm := NewMatcherManager()

	if addDefaultStatusMatcher {
		if err := mm.AddMatcher("status", opts.Matcher.Status); err != nil {
			errs.Add(err)
		}
	}

	if opts.Filter.Status != "" {
		if err := mm.AddFilter("status", opts.Filter.Status, false); err != nil {
			errs.Add(err)
		}
	}
	if opts.Filter.Size != "" {
		if err := mm.AddFilter("size", opts.Filter.Size, false); err != nil {
			errs.Add(err)
		}
	}
	if opts.Filter.Regexp != "" {
		if err := mm.AddFilter("regexp", opts.Filter.Regexp, false); err != nil {
			errs.Add(err)
		}
	}
	if opts.Filter.Words != "" {
		if err := mm.AddFilter("word", opts.Filter.Words, false); err != nil {
			errs.Add(err)
		}
	}
	if opts.Filter.Lines != "" {
		if err := mm.AddFilter("line", opts.Filter.Lines, false); err != nil {
			errs.Add(err)
		}
	}
	if opts.Filter.Time != "" {
		if err := mm.AddFilter("time", opts.Filter.Time, false); err != nil {
			errs.Add(err)
		}
	}

	if opts.Matcher.Size != "" {
		if err := mm.AddMatcher("size", opts.Matcher.Size); err != nil {
			errs.Add(err)
		}
	}
	if opts.Matcher.Regexp != "" {
		if err := mm.AddMatcher("regexp", opts.Matcher.Regexp); err != nil {
			errs.Add(err)
		}
	}
	if opts.Matcher.Words != "" {
		if err := mm.AddMatcher("word", opts.Matcher.Words); err != nil {
			errs.Add(err)
		}
	}
	if opts.Matcher.Lines != "" {
		if err := mm.AddMatcher("line", opts.Matcher.Lines); err != nil {
			errs.Add(err)
		}
	}
	if opts.Matcher.Time != "" {
		if err := mm.AddMatcher("time", opts.Matcher.Time); err != nil {
			errs.Add(err)
		}
	}

	return mm, errs.ErrorOrNil()
}
