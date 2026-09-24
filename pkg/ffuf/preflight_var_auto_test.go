package ffuf

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml"
)

// TestPreflightVarAutoFlags locks the -preflight-var-auto grammar: a bare key is
// auto, a "[source]key" prefix pins the source, only the first "]" closes the tag
// (PHP array names end in brackets) and the key may contain colons (JSF ids).
func TestPreflightVarAutoFlags(t *testing.T) {
	o := NewConfigOptions()
	if err := parsePreflightFlags(t, o, []string{
		"-preflight", "login.txt",
		"-preflight-var-auto", "CSRF:csrf_token",
		"-preflight-var-auto", "FT:[form]data[_Token][key]",
		"-preflight-var-auto", "JSF:[form]loginForm:token",
		"-preflight-var-auto", "META:[meta]csrf-token",
		"-preflight-var", "RE:tok=(\\w+)",
		"-postflight", "logout.txt",
		"-postflight-var-auto", "SESS:[cookie]sessionid",
		"-postflight-var-auto", "HDR:[header]X-Csrf-Token",
	}); err != nil {
		t.Fatalf("parse: %s", err)
	}
	wantPre := []PreflightConfig{{RequestFile: "login.txt", Vars: []VarExtract{
		{Name: "CSRF", Source: VarSourceAuto, Key: "csrf_token"},
		{Name: "FT", Source: VarSourceForm, Key: "data[_Token][key]"},
		{Name: "JSF", Source: VarSourceForm, Key: "loginForm:token"},
		{Name: "META", Source: VarSourceMeta, Key: "csrf-token"},
		{Name: "RE", Regex: "tok=(\\w+)"},
	}}}
	wantPost := []PreflightConfig{{RequestFile: "logout.txt", Vars: []VarExtract{
		{Name: "SESS", Source: VarSourceCookie, Key: "sessionid"},
		{Name: "HDR", Source: VarSourceHeader, Key: "X-Csrf-Token"},
	}}}
	if !reflect.DeepEqual(o.HTTP.Preflights, wantPre) {
		t.Errorf("preflights =\n  %+v\nwant\n  %+v", o.HTTP.Preflights, wantPre)
	}
	if !reflect.DeepEqual(o.HTTP.Postflights, wantPost) {
		t.Errorf("postflights =\n  %+v\nwant\n  %+v", o.HTTP.Postflights, wantPost)
	}
}

// TestPreflightVarAutoFlagErrors rejects bad specs at parse time instead of
// failing on every request later.
func TestPreflightVarAutoFlagErrors(t *testing.T) {
	cases := map[string]string{
		"NOCOLON":      "must be",
		":key":         "must be",
		"X:":           "must be",
		"X:[body]foo":  "unknown source",
		"X:[]foo":      "unknown source",
		"X:[form":      "unterminated",
		"X:[header]":   "non-empty key",
		"X:[auto]csrf": "", // [auto] spelled out is the same as a bare key
	}
	for spec, want := range cases {
		o := NewConfigOptions()
		err := parsePreflightFlags(t, o, []string{"-preflight", "f.txt", "-preflight-var-auto", spec})
		if want == "" {
			if err != nil {
				t.Errorf("%q: unexpected error %s", spec, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error = %v, want it to mention %q", spec, err, want)
		}
	}
	o := NewConfigOptions()
	if err := parsePreflightFlags(t, o, []string{"-preflight-var-auto", "X:y"}); err == nil {
		t.Error("orphan -preflight-var-auto: expected an error, got none")
	}
}

// TestPreflightVarAutoTOML loads source/key vars from a config file and checks
// that ConfigFromOptions validates them the same way the flag does.
func TestPreflightVarAutoTOML(t *testing.T) {
	data := `
[http]
[[http.preflights]]
request_file = "login.txt"
[[http.preflights.vars]]
name = "CSRF"
source = "cookie"
key = "XSRF-TOKEN"
`
	o := NewConfigOptions()
	if err := toml.Unmarshal([]byte(data), o); err != nil {
		t.Fatalf("toml unmarshal: %s", err)
	}
	want := []VarExtract{{Name: "CSRF", Source: VarSourceCookie, Key: "XSRF-TOKEN"}}
	if !reflect.DeepEqual(o.HTTP.Preflights[0].Vars, want) {
		t.Errorf("vars = %+v, want %+v", o.HTTP.Preflights[0].Vars, want)
	}
}

func TestPreflightVarConfigValidation(t *testing.T) {
	cases := []struct {
		ve   VarExtract
		want string
	}{
		{VarExtract{Name: "A", Source: "body", Key: "x"}, `unknown source "body"`},
		{VarExtract{Name: "B", Source: VarSourceForm}, "non-empty key"},
		{VarExtract{Name: "C", Source: VarSourceForm, Key: "x", Regex: "(x)"}, "not both"},
		{VarExtract{Name: "D"}, "needs a regex or a source"},
	}
	for _, c := range cases {
		o := NewConfigOptions()
		o.HTTP.Preflights = []PreflightConfig{{RequestFile: "f.txt", Vars: []VarExtract{c.ve}}}
		_, err := ConfigFromOptions(o, context.Background(), func() {})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("var %+v: error = %v, want it to mention %q", c.ve, err, c.want)
		}
	}
}
