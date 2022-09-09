package config

import (
	"fmt"
	"net/textproto"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/pkg/filter"
	"github.com/ffuf/ffuf/pkg/utils"
)

// The Translator type is a function which takes a reference to opts and to conf,
// picks one aspect of the options, translates it to a configuration, thereby
// either validating it or returning an error in case of illegal values or an
// incompatibility with other options. The opts parameter should not be mutated.
// A Translator should not terminate the program but rather return a descriptive
// error message. Further, a Translator should be self-contained and thus not
// depend on the prior execution of other Translators for it's operation.
type Translator func(opts *ConfigOptions, conf *Config) error

/////////////////////////
// DEFAULT TRANSLATORS //
/////////////////////////

// translateFilterMatcherModes is a Translator function which validates and sets
// the filter and matcher options.
func translateFilterMatcherModes(opts *ConfigOptions, conf *Config) error {

	valid_opmodes := []string{"and", "or"}
	fmode_found := false
	mmode_found := false
	for _, op := range valid_opmodes {
		if op == opts.Filter.Mode {
			fmode_found = true
		}
		if op == opts.Matcher.Mode {
			mmode_found = true
		}
	}
	if !fmode_found {
		return fmt.Errorf("unrecognized value for parameter fmode: %s, valid values are: and, or", opts.Filter.Mode)
	}
	if !mmode_found {
		return fmt.Errorf("unrecognized value for parameter mmode: %s, valid values are: and, or", opts.Matcher.Mode)
	}

	conf.FilterMode = opts.Filter.Mode
	conf.MatcherMode = opts.Matcher.Mode

	return nil
}

// translateFilterMatcherOptions is a Translator function which validates and
// sets matcher and filter options.
func translateFilterMatcherOptions(opts *ConfigOptions, conf *Config) error {

	// If any other matcher is set, ignore -mc default value
	var (
		matcherSet        bool = false
		statusSet         bool = false
		warningIgnoreBody bool = false

		default_opts *ConfigOptions   = NewConfigOptions()
		errs         utils.Multierror = utils.NewMultierror()
	)

	conf.MatcherManager = filter.NewMatcherManager()

	if opts.Matcher.Status != default_opts.Matcher.Status {
		statusSet = true
	}
	if opts.Matcher.Size != default_opts.Matcher.Size {
		matcherSet = true
		warningIgnoreBody = true
	}
	if opts.Matcher.Lines != default_opts.Matcher.Lines {
		matcherSet = true
		warningIgnoreBody = true
	}
	if opts.Matcher.Regexp != default_opts.Matcher.Regexp {
		matcherSet = true
	}
	if opts.Matcher.Time != default_opts.Matcher.Time {
		matcherSet = true
	}
	if opts.Matcher.Words != default_opts.Matcher.Words {
		matcherSet = true
		warningIgnoreBody = true
	}

	// Only set default matchers if no
	if statusSet || !matcherSet {
		if err := conf.MatcherManager.AddMatcher("status", opts.Matcher.Status); err != nil {
			errs.Add(err)
		}
	}

	if opts.Filter.Status != "" {
		if err := conf.MatcherManager.AddFilter("status", opts.Filter.Status, false); err != nil {
			errs.Add(err)
		}
	}
	if opts.Filter.Size != "" {
		warningIgnoreBody = true
		if err := conf.MatcherManager.AddFilter("size", opts.Filter.Size, false); err != nil {
			errs.Add(err)
		}
	}
	if opts.Filter.Regexp != "" {
		if err := conf.MatcherManager.AddFilter("regexp", opts.Filter.Regexp, false); err != nil {
			errs.Add(err)
		}
	}
	if opts.Filter.Words != "" {
		warningIgnoreBody = true
		if err := conf.MatcherManager.AddFilter("word", opts.Filter.Words, false); err != nil {
			errs.Add(err)
		}
	}
	if opts.Filter.Lines != "" {
		warningIgnoreBody = true
		if err := conf.MatcherManager.AddFilter("line", opts.Filter.Lines, false); err != nil {
			errs.Add(err)
		}
	}
	if opts.Filter.Time != "" {
		if err := conf.MatcherManager.AddFilter("time", opts.Filter.Time, false); err != nil {
			errs.Add(err)
		}
	}
	if opts.Matcher.Size != "" {
		if err := conf.MatcherManager.AddMatcher("size", opts.Matcher.Size); err != nil {
			errs.Add(err)
		}
	}
	if opts.Matcher.Regexp != "" {
		if err := conf.MatcherManager.AddMatcher("regexp", opts.Matcher.Regexp); err != nil {
			errs.Add(err)
		}
	}
	if opts.Matcher.Words != "" {
		if err := conf.MatcherManager.AddMatcher("word", opts.Matcher.Words); err != nil {
			errs.Add(err)
		}
	}
	if opts.Matcher.Lines != "" {
		if err := conf.MatcherManager.AddMatcher("line", opts.Matcher.Lines); err != nil {
			errs.Add(err)
		}
	}
	if opts.Matcher.Time != "" {
		if err := conf.MatcherManager.AddFilter("time", opts.Matcher.Time, false); err != nil {
			errs.Add(err)
		}
	}
	if conf.IgnoreBody && warningIgnoreBody {
		fmt.Printf("*** Warning: possible undesired combination of -ignore-body and the response options: fl,fs,fw,ml,ms and mw.\n")
	}
	return errs.ErrorOrNil()
}

// translateExtensions is a Translator function which sets the extensions option.
func translateExtensions(opts *ConfigOptions, conf *Config) error {

	if opts.Input.Extensions != "" {
		conf.Extensions = strings.Split(opts.Input.Extensions, ",")
	}

	return nil
}

// translateInputMode is a Translator function which validates and sets
// everything concerning the input mode
func translateInputMode(opts *ConfigOptions, conf *Config) error {

	validmode := false
	for _, mode := range []string{"clusterbomb", "pitchfork", "sniper"} {
		if opts.Input.InputMode == mode {
			validmode = true
		}
	}
	if !validmode {
		return fmt.Errorf("input mode (-mode) %s not recognized", conf.InputMode)
	}

	// sniper mode needs some additional checking
	if opts.Input.InputMode == "sniper" {

		if len(opts.Input.Wordlists) > 1 {
			return fmt.Errorf("sniper mode only supports one wordlist")
		}

		if len(opts.Input.Inputcommands) > 1 {
			return fmt.Errorf("sniper mode only supports one input command")
		}
	}

	conf.InputMode = opts.Input.InputMode
	return nil
}

// translateInputProvidersAndHttpHeaders validates and sets the input provider
// configuration. Because the canonicalization of HTTP Headers depends on the
// keyword supplied by input providers, translateInputProvidersAndHttpHeaders
// also sets and validates the HTTP headers.
func translateInputProvidersAndHttpHeaders(opts *ConfigOptions, conf *Config) error {

	// TODO: refactor further: Decouple the setting of HTTP headers and input
	// providers.

	if len(opts.Input.Wordlists)+len(opts.Input.Inputcommands) == 0 {
		return fmt.Errorf("either -w or --input-cmd flag is required")
	}

	template := ""
	// sniper mode needs some additional checking
	if opts.Input.InputMode == "sniper" {
		template = "§"
	}

	for _, wordlist := range opts.Input.Wordlists {
		var wl []string
		if runtime.GOOS == "windows" {
			// Try to ensure that Windows file paths like C:\path\to\wordlist.txt:KEYWORD are treated properly
			if utils.FileExists(wordlist) {
				// The wordlist was supplied without a keyword parameter
				wl = []string{wordlist}
			} else {
				filepart := wordlist
				if strings.Contains(filepart, ":") {
					filepart = wordlist[:strings.LastIndex(filepart, ":")]
				}

				if utils.FileExists(filepart) {
					wl = []string{filepart, wordlist[strings.LastIndex(wordlist, ":")+1:]}
				} else {
					// The file was not found. Use full wordlist parameter value for more concise error message down the line
					wl = []string{wordlist}
				}
			}
		} else {
			wl = strings.SplitN(wordlist, ":", 2)
		}

		if len(wl) == 2 {
			if conf.InputMode == "sniper" {
				return fmt.Errorf("sniper mode does not support wordlist keywords")
			} else {
				conf.InputProviders = append(conf.InputProviders, InputProviderConfig{
					Name:    "wordlist",
					Value:   wl[0],
					Keyword: wl[1],
				})
			}
		} else {
			conf.InputProviders = append(conf.InputProviders, InputProviderConfig{
				Name:     "wordlist",
				Value:    wl[0],
				Keyword:  "FUZZ",
				Template: template,
			})
		}
	}

	for _, v := range opts.Input.Inputcommands {
		ic := strings.SplitN(v, ":", 2)
		if len(ic) == 2 {
			if conf.InputMode == "sniper" {
				return fmt.Errorf("sniper mode does not support command keywords")
			} else {
				conf.InputProviders = append(conf.InputProviders, InputProviderConfig{
					Name:    "command",
					Value:   ic[0],
					Keyword: ic[1],
				})
				// BUG? Should be ic[1]
				//conf.CommandKeywords = append(conf.CommandKeywords, ic[0])
				conf.CommandKeywords = append(conf.CommandKeywords, ic[1])
			}
		} else {
			conf.InputProviders = append(conf.InputProviders, InputProviderConfig{
				Name:     "command",
				Value:    ic[0],
				Keyword:  "FUZZ",
				Template: template,
			})
			conf.CommandKeywords = append(conf.CommandKeywords, "FUZZ")
		}
	}

	for _, provider := range conf.InputProviders {
		if provider.Template != "" {
			if !templatePresent(provider.Template, opts) {
				return fmt.Errorf("template %s defined, but not found in pairs in headers, method, URL or POST data", provider.Template)
			}
		} else {
			if !keywordPresent(provider.Keyword, opts) {
				return fmt.Errorf("keyword %s defined, but not found in headers, method, URL or POST data", provider.Keyword)
			}
		}
	}

	// this depends on conf.CommandKeywords, thus it is called here.
	if err := _translateHttpHeaders(opts, conf); err != nil {
		return err
	}

	return nil
}

// translateInputCommon is a Translator function which sets common input params.
func translateInputCommon(opts *ConfigOptions, conf *Config) error {
	conf.IgnoreWordlistComments = opts.Input.IgnoreWordlistComments
	conf.DirSearchCompat = opts.Input.DirSearchCompat
	conf.InputNum = opts.Input.InputNum
	conf.InputShell = opts.Input.InputShell

	return nil
}

// translateHttpParams is a Translator function which validates and sets the
// HTTP options.
func translateHttpParams(opts *ConfigOptions, conf *Config) error {

	conf.Data = opts.HTTP.Data
	conf.FollowRedirects = opts.HTTP.FollowRedirects
	conf.IgnoreBody = opts.HTTP.IgnoreBody
	conf.Method = opts.HTTP.Method

	// Verify proxy url format
	if len(opts.HTTP.ProxyURL) > 0 {
		_, err := url.Parse(opts.HTTP.ProxyURL)
		if err != nil {
			return fmt.Errorf("bad proxy url (-x) format: %s", err)
		} else {
			conf.ProxyURL = opts.HTTP.ProxyURL
		}
	}

	// Do checks for recursion mode
	if opts.HTTP.Recursion {
		if !strings.HasSuffix(conf.Url, "FUZZ") {
			return fmt.Errorf("when using -recursion the URL (-u) must end with FUZZ keyword")
		}
	}

	conf.Recursion = opts.HTTP.Recursion
	conf.RecursionDepth = opts.HTTP.RecursionDepth
	conf.RecursionStrategy = opts.HTTP.RecursionStrategy

	// Verify replayproxy url format
	if len(opts.HTTP.ReplayProxyURL) > 0 {
		_, err := url.Parse(opts.HTTP.ReplayProxyURL)
		if err != nil {
			return fmt.Errorf("bad replay-proxy url (-replay-proxy) format: %s", err)
		} else {
			conf.ReplayProxyURL = opts.HTTP.ReplayProxyURL
		}
	}

	conf.Timeout = opts.HTTP.Timeout
	conf.SNI = opts.HTTP.SNI

	// TODO: What if the URL is relative?
	if len(opts.HTTP.URL) == 0 {
		return fmt.Errorf("a URL is required (-u flag)")
	}
	conf.Url = opts.HTTP.URL

	conf.Http2 = opts.HTTP.Http2

	return nil
}

// translateCookies is a Translator function which sets the cookie header.
func translateCookies(opts *ConfigOptions, conf *Config) error {

	if len(opts.HTTP.Cookies) > 0 {
		conf.Headers["Cookie"] = strings.Join(opts.HTTP.Cookies, "; ")
	}

	return nil
}

// _translateHttpHeaders validates, canonicalizes and sets HTTP Headers.
// Caution: Depends on prior run of translateInputProviders. Thus, it is not
// called in api.go but at the end of translateInputProviders.
func _translateHttpHeaders(opts *ConfigOptions, conf *Config) error {

	for _, hdr := range opts.HTTP.Headers {

		hdr_split := strings.SplitN(hdr, ":", 2)

		if len(hdr_split) != 2 {
			return fmt.Errorf("header %q defined by -H needs to have a value. \":\" should be used as a separator", hdr)
		}

		// trim and make canonical
		// except if used in custom defined header
		var CanonicalNeeded = true
		for _, cmd_keyword := range conf.CommandKeywords {
			if strings.Contains(hdr_split[0], cmd_keyword) {
				CanonicalNeeded = false
			}
		}
		// check if part of InputProviders
		if CanonicalNeeded {

			for _, in_prov := range conf.InputProviders {
				if strings.Contains(hdr_split[0], in_prov.Keyword) {
					CanonicalNeeded = false
				}
			}

			var CanonicalHeader = textproto.CanonicalMIMEHeaderKey(strings.TrimSpace(hdr_split[0]))
			conf.Headers[CanonicalHeader] = strings.TrimSpace(hdr_split[1])
		} else {
			conf.Headers[strings.TrimSpace(hdr_split[0])] = strings.TrimSpace(hdr_split[1])
		}
	}

	return nil
}

// translateHttpMethodCurlCompat is a Translator function which sets the HTTP
// method to POST if the default method is GET but a body is present, like curl
// does.
func translateHttpMethodCurlCompat(opts *ConfigOptions, conf *Config) error {

	// Is this really necessary? Curl is not a fuzzing tool but ffuf is.

	// Handle copy as curl situation where POST method is implied by --data flag.
	// If method is set to anything but GET, NOOP
	if len(opts.HTTP.Data) > 0 &&
		opts.HTTP.Data == "GET" &&
		//don't modify the method automatically if a request file is being used as input
		len(opts.Input.Request) == 0 {

		conf.Method = "POST"
	}
	return nil
}

// translateDelay is a Translator function which validates and sets the delay.
func translateDelay(opts *ConfigOptions, conf *Config) error {

	var err, err2 error

	if opts.General.Delay == "" {
		return nil
	}

	delay_split := strings.Split(opts.General.Delay, "-")

	if len(delay_split) == 1 {

		conf.Delay.IsRange = false
		conf.Delay.HasDelay = true
		conf.Delay.Min, err = strconv.ParseFloat(opts.General.Delay, 64)
		if err != nil {
			return fmt.Errorf("delay needs to be either a single float: \"0.1\" or a range of floats, delimited by dash: \"0.1-0.8\"")
		}
	} else if len(delay_split) == 2 {

		conf.Delay.IsRange = true
		conf.Delay.HasDelay = true
		conf.Delay.Min, err = strconv.ParseFloat(delay_split[0], 64)
		conf.Delay.Max, err2 = strconv.ParseFloat(delay_split[1], 64)
		if err != nil || err2 != nil {
			return fmt.Errorf("delay range min and max values need to be valid floats. For example: 0.1-0.5")
		}
	} else {
		return fmt.Errorf("delay needs to be either a single float: \"0.1\" or a range of floats, delimited by dash: \"0.1-0.8\"")
	}

	return nil
}

// translateOutputFormat is a Translator function which validates and sets the
// output format.
func translateOutputFormat(opts *ConfigOptions, conf *Config) error {

	//Check the output file format option
	if opts.Output.OutputFile == "" {
		//No need to check / error out if output file isn't defined
		return nil
	}

	// TODO: Define supported output formats somewhere else
	formats := []string{"all", "json", "ejson", "html", "md", "csv", "ecsv"}

	for _, format := range formats {
		if format == opts.Output.OutputFormat {
			conf.OutputFormat = format
			return nil
		}
	}

	return fmt.Errorf("unknown output file format (-of): %s", opts.Output.OutputFormat)
}

// translateAutoCalibration is a Translator function which validates and sets the
// autocalibration parameters.
func translateAutoCalibration(opts *ConfigOptions, conf *Config) error {

	conf.AutoCalibrationStrings = opts.General.AutoCalibrationStrings

	// Using -acc implies -ac
	if len(opts.General.AutoCalibrationStrings) > 0 {
		conf.AutoCalibration = true
	}

	// AutoCalibrationPerHost implies AutoCalibration
	if opts.General.AutoCalibrationPerHost {
		conf.AutoCalibration = true
	}

	conf.AutoCalibration = opts.General.AutoCalibration
	conf.AutoCalibrationPerHost = opts.General.AutoCalibrationPerHost
	conf.AutoCalibrationStrategy = opts.General.AutoCalibrationStrategy

	return nil
}

// translateGeneral is a Translator function setting general options.
func translateGeneral(opts *ConfigOptions, conf *Config) error {

	// Make verbose mutually exclusive with json
	if opts.General.Verbose && opts.General.Json {
		return fmt.Errorf("cannot have -json and -v")
	}

	conf.Colors = opts.General.Colors
	conf.Quiet = opts.General.Quiet
	conf.StopOn403 = opts.General.StopOn403
	conf.StopOnAll = opts.General.StopOnAll
	conf.StopOnErrors = opts.General.StopOnErrors
	conf.Threads = opts.General.Threads
	conf.MaxTime = opts.General.MaxTime
	conf.MaxTimeJob = opts.General.MaxTimeJob
	conf.Noninteractive = opts.General.Noninteractive
	conf.Verbose = opts.General.Verbose
	conf.Json = opts.General.Json

	return nil
}

// translateOutput is a Translator function setting general options.
func translateOutput(opts *ConfigOptions, conf *Config) error {

	conf.OutputFile = opts.Output.OutputFile
	conf.OutputDirectory = opts.Output.OutputDirectory
	conf.OutputSkipEmptyFile = opts.Output.OutputSkipEmptyFile

	return nil
}

// translateCmdline is a Translator function which sets the supplied commandline
func translateCmdline(opts *ConfigOptions, conf *Config) error {

	// This should not be relied upon. The conf package tries to abstract from
	// os.Args so that any string slice with options might be used as a config
	// source.

	conf.CommandLine = strings.Join(os.Args, " ")
	return nil
}

// translateRate is a Translator function which validates and sets rate.
func translateRate(opts *ConfigOptions, conf *Config) error {

	if opts.General.Rate < 0 {
		conf.Rate = 0
	} else {
		conf.Rate = int64(opts.General.Rate)
	}

	return nil
}

/////////////////////////
// AUXILIARY FUNCTIONS //
/////////////////////////

// templatePresent checks if the sniper mode delimiters, usually §, are present
// and come in pairs.
func templatePresent(template string, opts *ConfigOptions) bool {
	// Search for input location identifiers, these must exist in pairs
	sane := false

	if c := strings.Count(opts.HTTP.Method, template); c > 0 {
		if c%2 != 0 {
			return false
		}
		sane = true
	}
	if c := strings.Count(opts.HTTP.URL, template); c > 0 {
		if c%2 != 0 {
			return false
		}
		sane = true
	}
	if c := strings.Count(opts.HTTP.Data, template); c > 0 {
		if c%2 != 0 {
			return false
		}
		sane = true
	}

	for _, hdr := range opts.HTTP.Headers {
		hkey, hval, found := strings.Cut(hdr, ":")
		if !found {
			return strings.Count(hkey, template)%2 == 0
		}
		if c := strings.Count(hkey, template); c > 0 {
			if c%2 != 0 {
				return false
			}
			sane = true
		}
		if c := strings.Count(hval, template); c > 0 {
			if c%2 != 0 {
				return false
			}
			sane = true
		}
	}

	return sane
}

// keywordPresent checks if the fuzzing keyword is present in atleast some part
// of the HTTP request.
func keywordPresent(keyword string, opts *ConfigOptions) bool {
	//Search for keyword from HTTP method, URL and POST data too
	if strings.Contains(opts.HTTP.Method, keyword) {
		return true
	}
	if strings.Contains(opts.HTTP.URL, keyword) {
		return true
	}
	if strings.Contains(opts.HTTP.Data, keyword) {
		return true
	}
	for _, hdr := range opts.HTTP.Headers {
		return strings.Contains(hdr, keyword)
	}
	return false
}
