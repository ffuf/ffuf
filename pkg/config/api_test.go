// tests the integration of the config package

package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// TestSourcesOverride tests if the high level config.Get() function produces the
// correct output after the default configuration sources have been overridden.
// This also tests the config package integration.
func TestSourcesOverride(t *testing.T) {
	SetSources(
		SourceFromCmdline([]string{"-u", "https://override.me/FUZZ", "-v"}),
		SourceFromFileByCmdline([]string{"-config", "ffufrc.test"}),
	)

	want := setupFileTestCase()
	want.General.Verbose = true

	result, _, err := Get()
	if err != nil {
		t.Errorf("could not get config: %v", err)
	}

	rv := reflect.ValueOf(result).Elem()
	wv := reflect.ValueOf(want).Elem()

	if !reflect.DeepEqual(rv.Interface(), wv.Interface()) {
		t.Error("merged options from ffufrc.test and commandline diverge from expected value")
	}
}

func TestCustomTranslator(t *testing.T) {

	addition := "/added/by/custom/translator"

	custom_translator := func(opts *ConfigOptions, conf *Config) error {
		conf.Url = opts.HTTP.URL + addition
		return nil
	}

	SetSources(
		SourceFromCmdline([]string{"-u", "https://override.me/FUZZ", "-v"}),
		SourceFromFileByCmdline([]string{"-config", "ffufrc.test"}),
	)

	AddTranslator(custom_translator)

	opts, conf, err := Get()
	if err != nil {
		t.Errorf("error while testing custom translator: %v", err)
	}

	if conf.Url != opts.HTTP.URL+addition {
		t.Errorf("custom translator was not executed: wanted %q, got %q", opts.HTTP.URL+addition, conf.Url)
	}
}

//////////////
// EXAMPLES //
//////////////

// ExampleGetConfigSimple demonstrates how to get the configuration with a single
// call to config.Get().
func ExampleGet() {
	// config.Get() from outside the package
	opts, conf, err := Get()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// handle -help flag
			Usage() // config.Usage outside of package
			os.Exit(1)
		} else {
			// handle configuration errors or display to user
			fmt.Fprintf(os.Stderr, "error(s) encountered during configuration: %s\n", err)
			Usage()
			fmt.Fprintf(os.Stderr, "error(s) encountered during configuration: %s\n", err)
			os.Exit(1)
		}
	}

	if opts.General.Verbose {
		fmt.Println("verbose flag set")
	}

	if conf.Colors {
		fmt.Println("colors flag set")
	}
}

// ExampleSetSources demonstrates how to customize configuration sources and how
// to change the hierarchy.
func ExampleSetSources() {
	// These options get merged into one, where conflicting options are overriden
	// by the lower source, except for slices and maps, which get merged.
	SetSources(
		SourceFromCmdline(os.Args[1:]),
		SourceFromFileByCmdline(os.Args[1:]),              // these options override commandline options
		SourceFromRequest("/path/to/my/http-request.txt"), // these options override config file options
	)

	_, _, _ = Get() // returns opts, conf, err
}
