// high level functionality to get a configuration without much fuss

package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/ffuf/ffuf/pkg/utils"
)

//////////////////
// PACKAGE VARS //
//////////////////

var (
	// configSources represent the default selection of configuration sources and
	// their hierarchy: request < file < cmdline.
	// If SetSources is used, this value is overwritten by the user definded
	// sources and their hierarchy.
	configSources = []ConfigSource{
		SourceFromFileUserHome(),
		SourceFromRequestByCmdline(os.Args[1:]),
		SourceFromFileByCmdline(os.Args[1:]),
		SourceFromCmdline(os.Args[1:]),
	}

	_translators = []Translator{
		translateFilterMatcherModes,
		translateFilterMatcherOptions,
		translateExtensions,
		translateInputMode,
		translateInputProvidersAndHttpHeaders,
		translateInputCommon,
		translateHttpParams,
		translateCookies,
		translateHttpMethodCurlCompat,
		translateDelay,
		translateOutputFormat,
		translateAutoCalibration,
		translateGeneral,
		translateOutput,
		translateCmdline,
		translateRate,
	}

	// has to be a package variable so that Usage can walk it
	// has to be a flagset instead of just flag so that configuration might be
	// supplied programatically
	flagset *flag.FlagSet
)

/////////////
// EXPORTS //
/////////////

// SetSources takes an arbitrary amount of ConfigSources closures and uses their
// order as the hierarchy of configuration sources. The last one set is the most
// dominant.
func SetSources(sources ...ConfigSource) {
	configSources = sources
}

// AddTranslator appends a Translator function t to the Translators used by the
// package for validation and translation of the options into the configuration.
// If SetTranslators was not used, t is appended to a default set of translators.
func AddTranslator(t Translator) {
	_translators = append(_translators, t)
}

// SetTranslators overwrites the existing (and default, if called for the first
// time) set of translators with the given ones. The order of the transaltors
// should not matter for their correct functioning.
// To merely add a translator use the AddTranslator function, which does not
// overrride the default translators.
func SetTranslators(translators ...Translator) {
	_translators = translators
}

// Get produces a Config object representing the final job configuration and a
// ConfigOptions object representing the merged options from all configuration
// sources, or both nil and a descriptive error message, if they cannot be
// obtained.
// Get may be called without prior use of any other functionality of the package,
// defaulting to getting the configuration sources from the standard hierarchy
// (ascending) of sources if present, which is:
//   - config file in default locations: user home folder < user config folder
//     < current working directory
//   - HTTP request file
//   - an explicit config file set via the commandline
//   - commandline args
func Get() (opts *ConfigOptions, conf *Config, err error) {
	conf = NewConfig()

	merged, err := getMerged(configSources)
	if err != nil {
		// return in case of errors with config sources or merging
		return nil, nil, fmt.Errorf("could not obtain merged options: %w", err)
	}

	if err = runTranslators(merged, conf); err != nil {
		return nil, nil, err
	}

	return merged, conf, nil
}

///////////////
// AUXILIARY //
///////////////

func getMerged(sources []ConfigSource) (merged *ConfigOptions, err error) {

	opts := make([]*ConfigOptions, 0, 5)

	for _, src := range sources {
		if src != nil {
			opt, err := src()
			if err != nil {
				return nil, fmt.Errorf("configuration not available: %w", err)
			}
			opts = append(opts, opt)
		}
	}
	return Merge(opts...), nil
}

func runTranslators(opts *ConfigOptions, conf *Config) error {

	var (
		trans_err error
		errs      utils.Multierror = utils.NewMultierror()
	)

	for _, translator := range _translators {
		trans_err = translator(opts, conf)
		if trans_err != nil {
			errs.Add(trans_err)
		}
	}
	return errs.ErrorOrNil()
}
