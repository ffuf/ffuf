package ffuf

import (
	"flag"
	"io"
	"reflect"
	"testing"

	"github.com/pelletier/go-toml"
)

func parsePreflightFlags(t *testing.T, o *ConfigOptions, args []string) error {
	t.Helper()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	RegisterFlags(fs, o)
	return fs.Parse(args)
}

// TestPreflightFlagsPositionalBinding locks the CLI semantics: each -preflight-var
// attaches to the most recent -preflight, in command-line order, via the flag
// callbacks (no os.Args scan).
func TestPreflightFlagsPositionalBinding(t *testing.T) {
	o := NewConfigOptions()
	if err := parsePreflightFlags(t, o, []string{
		"-preflight", "login.txt",
		"-preflight-var", "TOKEN:csrf=([a-f0-9]+)",
		"-preflight-var", "SESSION:sid=(\\w+)",
		"-preflight", "second.txt",
		"-postflight", "logout.txt",
		"-postflight-var", "OUT:done:(\\d+)", // regex contains a colon
	}); err != nil {
		t.Fatalf("parse: %s", err)
	}

	wantPre := []PreflightConfig{
		{RequestFile: "login.txt", Vars: []VarExtract{
			{Name: "TOKEN", Regex: "csrf=([a-f0-9]+)"},
			{Name: "SESSION", Regex: "sid=(\\w+)"},
		}},
		{RequestFile: "second.txt"},
	}
	wantPost := []PreflightConfig{
		{RequestFile: "logout.txt", Vars: []VarExtract{{Name: "OUT", Regex: "done:(\\d+)"}}},
	}
	if !reflect.DeepEqual(o.HTTP.Preflights, wantPre) {
		t.Errorf("preflights =\n  %+v\nwant\n  %+v", o.HTTP.Preflights, wantPre)
	}
	if !reflect.DeepEqual(o.HTTP.Postflights, wantPost) {
		t.Errorf("postflights =\n  %+v\nwant\n  %+v", o.HTTP.Postflights, wantPost)
	}
}

// TestPreflightConfigFilePreserved is the fix for the config-wipe finding: CLI
// preflight flags APPEND to whatever a config file loaded, they don't clobber it.
func TestPreflightConfigFilePreserved(t *testing.T) {
	o := NewConfigOptions()
	o.HTTP.Preflights = []PreflightConfig{{RequestFile: "from-config.txt"}} // as if loaded from a config file
	if err := parsePreflightFlags(t, o, []string{"-preflight", "from-cli.txt"}); err != nil {
		t.Fatalf("parse: %s", err)
	}
	if len(o.HTTP.Preflights) != 2 {
		t.Fatalf("got %d preflights, want 2 (config value must survive)", len(o.HTTP.Preflights))
	}
	if o.HTTP.Preflights[0].RequestFile != "from-config.txt" {
		t.Errorf("config-file preflight was wiped: %+v", o.HTTP.Preflights)
	}
	if o.HTTP.Preflights[1].RequestFile != "from-cli.txt" {
		t.Errorf("CLI preflight not appended: %+v", o.HTTP.Preflights)
	}
}

// TestPreflightVarSpecValidation rejects malformed NAME:regex specs and orphan
// vars with a clear error instead of silently dropping them.
func TestPreflightVarSpecValidation(t *testing.T) {
	for _, bad := range []string{"NOCOLON", ":emptyname", "EMPTY:"} {
		o := NewConfigOptions()
		if err := parsePreflightFlags(t, o, []string{"-preflight", "f.txt", "-preflight-var", bad}); err == nil {
			t.Errorf("var spec %q: expected an error, got none", bad)
		}
	}
	o := NewConfigOptions()
	if err := parsePreflightFlags(t, o, []string{"-preflight-var", "X:y"}); err == nil {
		t.Error("orphan -preflight-var: expected an error, got none")
	}
	// empty -preflight value
	o2 := NewConfigOptions()
	if err := parsePreflightFlags(t, o2, []string{"-preflight", ""}); err == nil {
		t.Error("empty -preflight value: expected an error, got none")
	}
}

// TestPreflightTOMLLoads proves the toml tags wire config-file loading, including
// the nested request_file / vars keys that the json tags alone did not match.
func TestPreflightTOMLLoads(t *testing.T) {
	data := `
[http]
[[http.preflights]]
request_file = "login.txt"
[[http.preflights.vars]]
name = "TOKEN"
regex = "csrf=(\\w+)"
`
	o := NewConfigOptions()
	if err := toml.Unmarshal([]byte(data), o); err != nil {
		t.Fatalf("toml unmarshal: %s", err)
	}
	if len(o.HTTP.Preflights) != 1 || o.HTTP.Preflights[0].RequestFile != "login.txt" {
		t.Fatalf("preflight not loaded from TOML: %+v", o.HTTP.Preflights)
	}
	if len(o.HTTP.Preflights[0].Vars) != 1 || o.HTTP.Preflights[0].Vars[0].Name != "TOKEN" {
		t.Errorf("preflight var not loaded from TOML: %+v", o.HTTP.Preflights[0].Vars)
	}
}
