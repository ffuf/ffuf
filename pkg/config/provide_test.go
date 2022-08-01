package config

import (
	"errors"
	"flag"
	"os"
	"reflect"
	"strings"
	"testing"
)

// TestCmdlineHelpFlag tests if the -h flag raises the flag.ErrHelp error.
func TestCmdlineHelpFlag(t *testing.T) {
	args := []string{"-h"}

	FromCmdline := SourceFromCmdline(args)
	_, err := FromCmdline()

	if !errors.Is(err, flag.ErrHelp) {
		t.Error("-h flag raises no error of type fmt.ErrHelp")
	}
}

func TestSourceLikeCmdline(t *testing.T) {

	dir_disc := NewConfigOptions()
	dir_disc.General.Colors = true
	dir_disc.Input.Wordlists = []string{"/path/to/wordlist"}
	dir_disc.HTTP.URL = "https://ffuf.io.fi/FUZZ"

	vhost_disc := NewConfigOptions()
	vhost_disc.Input.Wordlists = []string{"/path/to/vhost/wordlist"}
	vhost_disc.HTTP.URL = "https://target"
	vhost_disc.HTTP.Headers = []string{"Host: FUZZ"}
	vhost_disc.Filter.Size = "4242"

	external_mutator := NewConfigOptions()
	external_mutator.Input.Inputcommands = []string{"radamsa --seed $FFUF_NUM example1.txt example2.txt"}
	external_mutator.HTTP.Headers = []string{"Content-Type: application/json"}
	external_mutator.HTTP.Method = "POST"
	external_mutator.HTTP.URL = "https://ffuf.io.fi/FUZZ"
	external_mutator.Matcher.Status = "all"
	external_mutator.Filter.Status = "400"

	// set config options as test cases
	tests := []struct {
		args string
		want *ConfigOptions
	}{
		// 	{ // typical directory discovery
		// 			" -c   -w /path/to/wordlist\n -u 'https://ffuf.io.fi/FUZZ'",
		// 			dir_disc,
		// 		},
		// 		{ // virtual host discovery (without DNS records)
		// 			`
		// -w /path/to/vhost/wordlist
		// -u https://target
		// -H Host:\ FUZZ
		// -fs 4242`,
		// 			vhost_disc,
		// 		},
		{ // using external mutator to produce test cases
			"--input-cmd \"radamsa --seed $FFUF_NUM example1.txt example2.txt\" -H Content-Type:\\ application/json -X POST -u https://ffuf.io.fi/FUZZ -mc all -fc 400",
			external_mutator,
		},
	}

	// run tests
	for _, test := range tests {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("test paniced: %v", r)
			}
		}()

		FromCmdline := SourceLikeCmdline(test.args)
		result, err := FromCmdline()
		if err != nil {
			t.Errorf("an error occured during commandline parsing: %v", err)
		}

		val_res := reflect.ValueOf(result).Elem()
		val_want := reflect.ValueOf(test.want).Elem()

		if !reflect.DeepEqual(val_res.Interface(), val_want.Interface()) {
			t.Errorf("ConfigOptions not in expected state for the following options: %q", test.args)
		}
	}

}

// TestCmdlineExampleUsage tests all examples from the Example Usage section in
// README.md.
func TestSourceFromCmdlineExampleUsage(t *testing.T) {

	// create config options for testing
	dir_disc := NewConfigOptions()
	dir_disc.General.Colors = true
	dir_disc.Input.Wordlists = []string{"/path/to/wordlist"}
	dir_disc.HTTP.URL = "https://ffuf.io.fi/FUZZ"

	vhost_disc := NewConfigOptions()
	vhost_disc.Input.Wordlists = []string{"/path/to/vhost/wordlist"}
	vhost_disc.HTTP.URL = "https://target"
	vhost_disc.HTTP.Headers = []string{"Host: FUZZ"}
	vhost_disc.Filter.Size = "4242"

	get_param_fuzzing1 := NewConfigOptions()
	get_param_fuzzing1.Input.Wordlists = []string{"/path/to/paramnames.txt"}
	get_param_fuzzing1.HTTP.URL = "https://target/script.php?FUZZ=test_value"
	get_param_fuzzing1.Filter.Size = "4242"

	get_param_fuzzing2 := NewConfigOptions()
	get_param_fuzzing2.Input.Wordlists = []string{"/path/to/values.txt"}
	get_param_fuzzing2.HTTP.URL = "https://target/script.php?valid_name=FUZZ"
	get_param_fuzzing2.Filter.Status = "401"

	post_data_fuzzing := NewConfigOptions()
	post_data_fuzzing.Input.Wordlists = []string{"/path/to/postdata.txt"}
	post_data_fuzzing.HTTP.Method = "POST"
	post_data_fuzzing.HTTP.Data = "username=admin&password=FUZZ"
	post_data_fuzzing.HTTP.URL = "https://target/login.php"
	post_data_fuzzing.Filter.Status = "401"

	max_exec_time1 := NewConfigOptions()
	max_exec_time1.Input.Wordlists = []string{"/path/to/wordlist"}
	max_exec_time1.HTTP.URL = "https://target/FUZZ"
	max_exec_time1.General.MaxTime = 60

	max_exec_time2 := NewConfigOptions()
	max_exec_time2.Input.Wordlists = []string{"/path/to/wordlist"}
	max_exec_time2.HTTP.URL = "https://target/FUZZ"
	max_exec_time2.General.MaxTimeJob = 60
	max_exec_time2.HTTP.Recursion = true
	max_exec_time2.HTTP.RecursionDepth = 2

	external_mutator := NewConfigOptions()
	external_mutator.Input.Inputcommands = []string{"radamsa --seed $FFUF_NUM example1.txt example2.txt"}
	external_mutator.HTTP.Headers = []string{"Content-Type: application/json"}
	external_mutator.HTTP.Method = "POST"
	external_mutator.HTTP.URL = "https://ffuf.io.fi/FUZZ"
	external_mutator.Matcher.Status = "all"
	external_mutator.Filter.Status = "400"

	// set config options as test cases
	tests := []struct {
		args []string
		want *ConfigOptions
	}{
		{ // typical directory discovery
			[]string{"-c", "-w", "/path/to/wordlist", "-u", "https://ffuf.io.fi/FUZZ"},
			dir_disc,
		},
		{ // virtual host discovery (without DNS records)
			[]string{"-w", "/path/to/vhost/wordlist", "-u", "https://target", "-H", "Host: FUZZ", "-fs", "4242"},
			vhost_disc,
		},
		{ // GET parameter fuzzing #1
			strings.Split("-w /path/to/paramnames.txt -u https://target/script.php?FUZZ=test_value -fs 4242", " "),
			get_param_fuzzing1,
		},
		{ // GET parameter fuzzing #2
			strings.Split("-w /path/to/values.txt -u https://target/script.php?valid_name=FUZZ -fc 401", " "),
			get_param_fuzzing2,
		},
		{ // POST data fuzzing
			strings.Split("-w /path/to/postdata.txt -X POST -d username=admin&password=FUZZ -u https://target/login.php -fc 401", " "),
			post_data_fuzzing,
		},
		{ // maximum execution time #1
			strings.Split("-w /path/to/wordlist -u https://target/FUZZ -maxtime 60", " "),
			max_exec_time1,
		},
		{ // maximum execution time #2
			strings.Split("-w /path/to/wordlist -u https://target/FUZZ -maxtime-job 60 -recursion -recursion-depth 2", " "),
			max_exec_time2,
		},
		{ // using external mutator to produce test cases
			[]string{"--input-cmd", "radamsa --seed $FFUF_NUM example1.txt example2.txt", "-H", "Content-Type: application/json", "-X", "POST", "-u", "https://ffuf.io.fi/FUZZ", "-mc", "all", "-fc", "400"},
			external_mutator,
		},
	}

	// run tests
	for _, test := range tests {
		FromCmdline := SourceFromCmdline(test.args)
		result, err := FromCmdline()
		if err != nil {
			t.Errorf("an error occured during commandline parsing: %v", err)
		}

		val_res := reflect.ValueOf(result).Elem()
		val_want := reflect.ValueOf(test.want).Elem()

		if !reflect.DeepEqual(val_res.Interface(), val_want.Interface()) {
			t.Errorf("ConfigOptions not in expected state for the following options: %q", strings.Join(test.args, " "))
		}
	}
}

func setupFileTestCase() (want *ConfigOptions) {
	want = NewConfigOptions()

	want.HTTP.Cookies = []string{"cookiename=cookievalue"}
	want.HTTP.Data = "post=data&key=value"
	want.HTTP.FollowRedirects = false
	want.HTTP.Headers = []string{"X-Header-Name: value", "X-Another-Header: value"}
	want.HTTP.IgnoreBody = false
	want.HTTP.Method = "GET"
	want.HTTP.ProxyURL = "http://127.0.0.1:8080"
	want.HTTP.Recursion = false
	want.HTTP.RecursionDepth = 0
	want.HTTP.RecursionStrategy = "default"
	want.HTTP.URL = "https://example.org/FUZZ/HOST/CUSTOMKEYWORD"
	want.General.AutoCalibrationStrings = []string{"randomtest", "admin"}
	want.Input.Inputcommands = []string{"seq 1 100:CUSTOMKEYWORD"}
	want.Input.Wordlists = []string{"/path/to/wordlist:FUZZ", "/path/to/hostlist:HOST"}

	return want
}

// TestSourceFromFileByCmdlineArgs tests if options can be extracted from a file
// pointed to by the -config flag and if a config file can be parsed in general.
func TestSourceFromFileByCmdlineArgs(t *testing.T) {

	want := setupFileTestCase()

	FromFile := SourceFromFileByCmdline([]string{"-config", "ffufrc.test"})
	result, err := FromFile()
	if err != nil {
		t.Errorf("could not read ffufrc.test: %v", err)
	}

	rv := reflect.ValueOf(result).Elem()
	wv := reflect.ValueOf(want).Elem()

	if !reflect.DeepEqual(rv.Interface(), wv.Interface()) {
		t.Error("options from ffufrc.test diverge from expected value. Did ffufrc.test change?")
	}
}

//////////////
// EXAMPLES //
//////////////

// This example demonstrates how to create a ConfigSouce producer.
func ExampleCreateOwnConfigSource() {

	// Done as function expression because it's in an Example function. See
	// source code form SourceFromCmdline for a more accurate example.

	// Takes required parameters as input. Defaults need none
	SourceFromDefaults := func() ConfigSource {
		// returns a ConfigSource, which itself returns *ConfigOptions and an
		// error.
		return func() (opts *ConfigOptions, err error) {
			// Must take parameters from the enclosing function, not via the
			// function arguments.

			opts = NewConfigOptions() // initializes default

			return opts, nil
		}
	}

	// Use it inside SetSources: only default config will be considered
	SetSources(
		SourceFromDefaults(),
	)

}

// ExampleSourceLikeCmdline demonstrates how to get a configuration
// programatically, by configuring ffuf like from the commandline.
func ExampleSourceLikeCmdline() {

	// must not include program name in args
	// should be used with a raw string literal to avoid double escaping
	SetSources(
		SourceFromCmdline(os.Args[1:]), // set anything via cmdline
		SourceLikeCmdline(`-w "/path/to/vhost/wordlist" -u https://target -H Host:\ FUZZ -fs 4242`), // but prefer these
	)

	_, _, _ = Get() // returns opts, conf, err
}
