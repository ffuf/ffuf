package config

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ffuf/ffuf/pkg/utils"
	"github.com/pelletier/go-toml"
)

/////////////
// EXPORTS //
/////////////

// The ConfigSource type repreresents a function closure which carries in it's
// environment all necessary information to produce ConfigOptions or nil and an
// error message. Values of this type should be set to nil if the config source
// is not available.
// This type is used by the package to obtain options from
// different configuration sources, while decoupling the parameters of where to
// get them from the process of how to produce them.
type ConfigSource func() (opts *ConfigOptions, err error)

// SourceFromCmdline returns a ConfigSource closure which carries in it's
// environment a reference to the supplied commandline arguments. These must be
// passed without the command name.
//
// When the returned ConfigSource is called, it extracts configuration options
// from commandline arguments passed by the args parameter. If an error occurs
// during parsing, program execution will not be terminated and the error
// printed, but the error message supplied in the err return value. If the -h or
// -help flag is encoutered, the error type will be flag.ErrHelp. Thus the
// printing of usage must be invoked when flag.ErrHelp is encountered.
func SourceFromCmdline(args []string) ConfigSource {

	return func() (opts *ConfigOptions, err error) {
		opts = NewConfigOptions()

		err = opts.setFromCmdlineArgs(args)
		if err != nil {
			return nil, err
		}

		return opts, nil
	}
}

// SourceLikeCmdline returns a ConfigSource closure which carries in it's
// environment a string which must mimic the commandline arguments. It must not
// contain the command name, e.g. ffuf. Like on the commandline, arguments
// containing whitespace can be grouped either by escaping the whitespace with a
// backslash or by putting it inside single or double quotes. Single quotes
// inside double quotes and vice versa are considered part of the argument. It is
// recommended to use raw string literals. If an error occurs, like unbalanced
// quotes, SourceLikeCmdline panics.
// SourceLikeCmdline is convenient as a quick way to define some options from
// inside the program, without building the ConfigOptions struct by hand.
// A ConfigSource is returned.
//
// When the returned ConfigSource is called, it splits the string into arguments
// from commandline arguments passed by the args parameter. If an error occurs
// during parsing, program execution will not be terminated and the error
// printed, but the error message supplied in the err return value. If the -h or
// -help flag is encoutered, the error type will be flag.ErrHelp. Thus the
// printing of usage must be invoked when flag.ErrHelp is encountered.
func SourceLikeCmdline(cmdline string) ConfigSource {

	args, err := utils.TokenizeArgs(cmdline)
	if err != nil {
		panic(fmt.Errorf("error with supplied configuration string: %v", err))
	}

	return func() (opts *ConfigOptions, err error) {
		opts = NewConfigOptions()

		err = opts.setFromCmdlineArgs(args)
		if err != nil {
			return nil, err
		}

		return opts, nil
	}
}

// SourceFromFile returns a ConfigSource closure which carries in it's
// environment the path to the config file or nil, if the file under the path is
// inaccessible.
// When the returned ConfigSource is called, it extracts configuration parameters
// from the configuration file pointed to by path. If an error occurs during
// parsing is encountered, err will be set. It is nil otherwise.
func SourceFromFile(path string) ConfigSource {

	// check of the file's existence occurs during parsing inside
	// opts.setFromConfigFile

	return func() (opts *ConfigOptions, err error) {
		opts = NewConfigOptions()

		err = opts.setFromConfigFile(path)
		if err != nil {
			return nil, fmt.Errorf("error reading config from file: %v", err)
		}

		return opts, nil
	}
}

// SourceFromFileByCmdline returns a ConfigSource closure which carries in it's
// environment the path to the config file, which is first extracted from the
// commandline arguments. These should be passed without the command name.
// When the returned ConfigSource is called, it extracts configuration parameters
// from the configuration file pointed to by path. If an error occurs during
// parsing is encountered, err will be set. It is nil otherwise.
func SourceFromFileByCmdline(args []string) ConfigSource {

	// this may be inefficient, but it keeps the definition of commandline args
	// in one place: setFromCmdline.
	tmp := NewConfigOptions()
	tmp.setFromCmdlineArgs(args)
	path := tmp.General.ConfigFile

	if path == "" {
		return nil
	}

	return SourceFromFile(path)
}

// SourceFromFileUserHome returns a ConfigSource closure which carries in it's
// environment the path to the .ffufrc file or nil, if the file is inaccessible.
// First the current directory, then the user configuration directory as defined
// by os.UserConfigDir and finally the user home directory as definded by
// os.UserHomeDir is searched.
// When the returned ConfigSource is called, it extracts configuration parameters
// from the configuration file pointed to by path. If an error occurs during
// parsing is encountered, err will be set. It is nil otherwise.
func SourceFromFileUserHome() ConfigSource {

	var (
		file string = ".ffufrc"
		path string
	)

	curdir, err := os.Getwd()
	if err != nil {
		goto confdir // don't worry
	}
	if path = filepath.Join(curdir, file); utils.FileExists(path) {
		return SourceFromFile(path)
	}

confdir:
	confdir, err := os.UserConfigDir()
	if err != nil {
		goto homedir
	}
	if path = filepath.Join(confdir, file); utils.FileExists(path) {
		return SourceFromFile(path)
	}

homedir:
	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	if path = filepath.Join(homedir, file); utils.FileExists(path) {
		return SourceFromFile(path)
	}

	return nil
}

// SourceFromRequest returns a ConfigSource closure which carries in it's
// environment the path to the HTTP request template file. When the ConfigSource
// is called, it extracts configuration arguments from a raw HTTP request
// file pointed to by path.
// Beware that not all required options can be extracted from an HTTP template.
// The rest of the options need to be supplemented and merged from other
// configuration sources.
// SourceFromRequest will also set the URL, even if it is relative. If this is
// not desired, the opts should be merged with another configuration source.
func SourceFromRequest(path string) ConfigSource {

	return func() (opts *ConfigOptions, err error) {
		opts = NewConfigOptions()

		err = opts.setFromHttpTemplate(path)
		if err != nil {
			return nil, fmt.Errorf("error reading config from raw http request: %v", err)
		}

		return opts, nil
	}
}

// SourceFromRequestByCmdline extracts configuration parameters from a raw HTTP
// request file where the path to the file is extracted from commandline
// arguments. The args must be passed without the command name.
// Beware that not all required options can be extracted from an HTTP template.
// The rest of the options need to be supplemented and merged from other
// configuration sources.
// SourceFromRequest will also set the URL, even if it is relative. If this is
// not desired, the opts should be merged with another configuration source.
func SourceFromRequestByCmdline(args []string) ConfigSource {

	// this may be inefficient, but it keeps the definition of commandline args
	// in one place: setFromCmdline.
	tmp := NewConfigOptions()
	tmp.setFromCmdlineArgs(args)
	path := tmp.Input.Request

	if path == "" {
		return nil
	}

	return SourceFromRequest(path)
}

// A MultiStringFlag represents a commandline option which carries multiple
// values delimited by commas.
type MultiStringFlag []string

// A WorldlistFlag represents a commandline option which carries one or more
// wordlist parameters seperated by commas.
type WordlistFlag []string

// String serialized a MultiStringFlag type into a string, delimited by commas.
func (m *MultiStringFlag) String() string {
	return strings.Join(*m, ",")
}

// String serialized a WorlistFlag type into a string, delimited by commas.
func (m *WordlistFlag) String() string {
	return strings.Join(*m, ",")
}

// Set splits a comma delimited string into a slice during commandline parsing.
func (m *MultiStringFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

// Set splits a comma delimited string into a slice during commandline parsing.
func (m *WordlistFlag) Set(value string) error {
	delimited := strings.Split(value, ",")

	if len(delimited) > 1 {
		*m = append(*m, delimited...)
	} else {
		*m = append(*m, value)
	}

	return nil
}

////////////////
// AUXILIARY  //
////////////////

// setFromCmdlineArgs invokes commandline parsing of args and stores the result
// in it's ConfigOptions receiver. args is a reference to a slice so that it can
// be distinguished from os.Args
func (opts *ConfigOptions) setFromCmdlineArgs(args []string) error {

	// set package wide flagset
	flagset = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	defineCmdlineOptions(flagset, opts)
	return flagset.Parse(args)
}

func (opts *ConfigOptions) setFromConfigFile(path string) error {
	raw, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("file error: %v", err)
	}

	err = toml.Unmarshal(raw, opts)
	if err != nil {
		return fmt.Errorf("unmarshal error: %v", err)
	}

	return nil
}

func (opts *ConfigOptions) setFromHttpTemplate(path string) error {

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("could not open request file: %s", err)
	}

	req, err := utils.ParseRawRequest(file)
	if err != nil {
		return fmt.Errorf("error parsing HTTP template: %v", err)
	}
	defer req.Body.Close()

	// Set cookies
	for _, cookie := range req.Cookies() {
		opts.HTTP.Cookies = append(opts.HTTP.Cookies, cookie.String())
	}

	// Set data. It is generally wrong to assume that the request only carries
	// string data. Here it is done for compatibility reasons.
	bytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("error reading request body: %v", err)
	}
	opts.HTTP.Data = trim_trailing_newline(string(bytes))

	// Set headers. Should they be a map??
	for k, v := range req.Header {
		opts.HTTP.Headers = append(opts.HTTP.Headers, fmt.Sprintf("%s: %s", k, strings.Join(v, ",")))
	}

	opts.HTTP.Method = req.Method
	opts.HTTP.URL = req.URL.String() // might be relative
	opts.HTTP.Http2 = req.ProtoMajor == 2

	return nil
}

// Remove newline (typically added by the editor) at the end of the file
// we specifically want to remove just a single newline, not all of them
func trim_trailing_newline(str string) string {

	if strings.HasSuffix(str, "\r\n") {
		return str[:len(str)-2]
	} else if strings.HasSuffix(str, "\n") {
		return str[:len(str)-1]
	} else {
		return str
	}
}

func remove_quotes(s string) (clean string) {
	clean = strings.ReplaceAll(s, "'", "") // remove single quotes
	clean = strings.ReplaceAll(clean, "\"", "")
	return clean
}
