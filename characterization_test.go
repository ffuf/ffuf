package main

// Characterization ("golden master") harness for the CLI flag + config pipeline.
//
// It pins the CURRENT observable behavior of ffuf's option handling so the planned
// flag/config refactor can be developed against a safety net: any change to help
// output, flag parsing, aliases, config-file precedence, or the resulting Config is
// surfaced as a test failure.
//
// Regenerate goldens after an INTENTIONAL behavior change:
//
//	go test -run Characterization -update-golden
//
// A diff in any golden that you did not intend is a regression.

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

var updateGolden = flag.Bool("update-golden", false, "regenerate .golden characterization files")

// withCleanFlagState saves/restores the global flag + os.Args state around fn, and
// installs a fresh ContinueOnError FlagSet so a parse error can't os.Exit the tests.
func withCleanFlagState(args []string, fn func()) {
	oldArgs, oldCmd, oldUsage := os.Args, flag.CommandLine, flag.Usage
	defer func() { os.Args, flag.CommandLine, flag.Usage = oldArgs, oldCmd, oldUsage }()

	flag.CommandLine = flag.NewFlagSet("ffuf", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = append([]string{"ffuf"}, args...)
	fn()
}

func compareGolden(t *testing.T, path, got string) {
	t.Helper()
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s — run: go test -run Characterization -update-golden\n(%v)", path, err)
	}
	if got != string(want) {
		t.Errorf("characterization drift in %s\n--- got ---\n%s\n--- want ---\n%s", path, got, string(want))
	}
}

var versionLine = regexp.MustCompile(`(?m)^Fuzz Faster U Fool - .*$`)

// TestCharacterization_HelpGolden pins the exact `ffuf -h` output (sections, order,
// flags, defaults, examples). The build-dependent version line is normalized out.
func TestCharacterization_HelpGolden(t *testing.T) {
	var out string
	withCleanFlagState(nil, func() {
		opts := ffuf.NewConfigOptions()
		ParseFlags(opts) // registers every flag on the fresh FlagSet
		out = captureStdout(Usage)
	})
	out = versionLine.ReplaceAllString(out, "Fuzz Faster U Fool - <VERSION>")
	compareGolden(t, "testdata/help.golden", out)
}

type configCase struct {
	name        string
	args        []string
	toml        string // optional config-file contents; when set, loaded before flags (file < CLI)
	requestBody string // optional raw HTTP request; when set, written to a temp file and passed via -request
}

// TestCharacterization_ConfigGolden pins the Config produced by the full parse
// pipeline (ReadConfig/defaults -> ParseFlags -> ConfigFromOptions -> SetupFilters)
// for a table of argvs covering every flag shape, aliases, precedence, and errors.
func TestCharacterization_ConfigGolden(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("config goldens are recorded on unix; ConfigFromOptions applies windows-specific wordlist path handling that diverges from them")
	}
	cwd, _ := os.Getwd()
	cases := []configCase{
		{name: "basic", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt"}},
		{name: "alias_data_binary", args: []string{"-u", "https://example.org/", "-w", "/tmp/wl.txt", "-X", "POST", "-data-binary", "name=FUZZ"}},
		{name: "alias_cookie", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt", "-cookie", "SESSION=abc"}},
		{name: "matchers_filters", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt", "-mc", "200,301", "-fc", "404", "-fs", "42", "-ml", "5"}},
		{name: "multi_wordlist_pitchfork", args: []string{"-u", "https://example.org/W1/W2", "-w", "/tmp/a.txt:W1", "-w", "/tmp/b.txt:W2", "-mode", "pitchfork"}},
		{name: "headers_multi", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt", "-H", "X-A: 1", "-H", "X-B: 2"}},
		{name: "delay_range", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt", "-p", "0.1-0.8"}},
		{name: "extensions", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt", "-e", ".php,.bak"}},
		{name: "encoders", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt:FUZZ", "-enc", "FUZZ:b64encode"}},
		{name: "autocalibration", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt", "-ac", "-acc", "custom1", "-acc", "custom2"}},
		{name: "acs_strategies", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt", "-acs", "advanced,greedy", "-acs", "custom"}},
		{name: "compat_data_alias", args: []string{"-u", "https://example.org/", "-w", "/tmp/wl.txt", "-data", "x=FUZZ"}},
		// Locks the -ach behavior: a config-file -ac must NOT auto-enable per-host calibration.
		{name: "config_ach_no_autoenable", toml: "[general]\nautocalibration = true\n", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt"}},
		// Locks the -cc/-ck behavior: a config-file client cert/key survives (is not wiped by the flag default).
		{name: "config_client_cert", toml: "[http]\nclientcert = \"/tmp/cert.pem\"\nclientkey = \"/tmp/key.pem\"\n", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt"}},
		// Broad coverage of otherwise-unexercised flags through to Config.
		{name: "many_flags", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt", "-x", "http://127.0.0.1:8080", "-replay-proxy", "http://127.0.0.1:9090", "-sni", "example.com", "-timeout", "15", "-rate", "50", "-recursion", "-recursion-depth", "3", "-recursion-strategy", "greedy", "-of", "json", "-od", "/tmp/out", "-maxtime", "60", "-json", "-r", "-raw", "-http2", "-ic", "-D", "-sf"}},
		// Exercises the raw-request parse path (parseRawRequest).
		{name: "raw_request", args: []string{"-w", "/tmp/wl.txt"}, requestBody: "POST /submit HTTP/1.1\nHost: example.org\nContent-Type: application/json\n\n{\"q\":\"FUZZ\"}\n"},
		{name: "precedence_cli_overrides_file", toml: "[general]\nthreads = 99\n", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt", "-t", "5"}},
		{name: "precedence_file_used", toml: "[general]\nthreads = 99\n", args: []string{"-u", "https://example.org/FUZZ", "-w", "/tmp/wl.txt"}},
		{name: "error_missing_url", args: []string{"-w", "/tmp/wl.txt"}},
		{name: "error_missing_wordlist", args: []string{"-u", "https://example.org/FUZZ"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := c.args
			reqPath := ""
			if c.requestBody != "" {
				reqPath = filepath.Join(t.TempDir(), "request.txt")
				if err := os.WriteFile(reqPath, []byte(c.requestBody), 0644); err != nil {
					t.Fatal(err)
				}
				args = append(append([]string{}, c.args...), "-request", reqPath)
			}
			var snap string
			withCleanFlagState(args, func() {
				opts := loadOpts(t, c.toml)
				ParseFlags(opts)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				conf, err := ffuf.ConfigFromOptions(opts, ctx, cancel)
				snap = snapshotConfig(conf, err, cwd)
				if err == nil && conf != nil {
					_ = captureStdout(func() { _ = SetupFilters(opts, conf) }) // swallow warning prints
					snap += "\n--- matchers after SetupFilters ---\n" + matcherSnapshot(conf)
				}
			})
			if reqPath != "" {
				snap = strings.ReplaceAll(snap, reqPath, "$REQFILE") // temp path is machine-specific
			}
			compareGolden(t, "testdata/config_"+c.name+".golden", snap)
		})
	}
}

func loadOpts(t *testing.T, tomlBody string) *ffuf.ConfigOptions {
	t.Helper()
	if tomlBody == "" {
		return ffuf.NewConfigOptions()
	}
	f := filepath.Join(t.TempDir(), "ffufrc")
	if err := os.WriteFile(f, []byte(tomlBody), 0644); err != nil {
		t.Fatal(err)
	}
	opts, err := ffuf.ReadConfig(f)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	return opts
}

func snapshotConfig(conf *ffuf.Config, err error, cwd string) string {
	var b strings.Builder
	if err != nil {
		fmt.Fprintf(&b, "error: %s\n", err.Error())
	}
	if conf == nil {
		return b.String()
	}
	j, _ := json.MarshalIndent(conf, "", "  ")
	s := string(j)
	if cwd != "" {
		s = strings.ReplaceAll(s, cwd, "$CWD")
	}
	b.WriteString(s)
	b.WriteString("\n")
	return b.String()
}

func matcherSnapshot(conf *ffuf.Config) string {
	if conf.MatcherManager == nil {
		return "(no matcher manager)\n"
	}
	var b strings.Builder
	dump := func(title string, providers map[string]ffuf.FilterProvider) {
		reprs := make([]string, 0, len(providers))
		for name, p := range providers {
			reprs = append(reprs, fmt.Sprintf("  %s = %s", name, p.Repr()))
		}
		sort.Strings(reprs)
		fmt.Fprintf(&b, "%s:\n%s\n", title, strings.Join(reprs, "\n"))
	}
	dump("matchers", conf.MatcherManager.GetMatchers())
	dump("filters", conf.MatcherManager.GetFilters())
	return b.String()
}

// captureStdout redirects os.Stdout for the duration of fn and returns what it wrote.
func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}
