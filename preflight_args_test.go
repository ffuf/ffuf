package main

import (
	"reflect"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// TestParsePreflightArgs locks the positional parsing: each -preflight-var binds
// to the most recent -preflight, both "-flag val" and "-flag=val" forms are
// accepted, the var value splits on the first colon (so a regex may contain
// colons), and a stray -preflight-var with no preceding -preflight is dropped
// rather than crashing.
func TestParsePreflightArgs(t *testing.T) {
	args := []string{
		"-u", "http://target/FUZZ",
		"-preflight", "login.txt",
		"-preflight-var", "TOKEN:csrf=([a-f0-9]+)",
		"-preflight-var=SESSION:sid=(\\w+)",
		"-preflight", "second.txt",
		"-postflight", "logout.txt",
		"-postflight-var", "OUT:done:(\\d+)", // regex contains a colon
		"-preflight-var", "ORPHAN:x", // no preceding -preflight at this point? there is (second.txt)
	}

	pre, post := parsePreflightArgs(args)

	wantPre := []ffuf.PreflightConfig{
		{RequestFile: "login.txt", Vars: []ffuf.VarExtract{
			{Name: "TOKEN", Regex: "csrf=([a-f0-9]+)"},
			{Name: "SESSION", Regex: "sid=(\\w+)"},
		}},
		{RequestFile: "second.txt", Vars: []ffuf.VarExtract{
			{Name: "ORPHAN", Regex: "x"},
		}},
	}
	wantPost := []ffuf.PreflightConfig{
		{RequestFile: "logout.txt", Vars: []ffuf.VarExtract{
			{Name: "OUT", Regex: "done:(\\d+)"},
		}},
	}

	if !reflect.DeepEqual(pre, wantPre) {
		t.Errorf("preflights =\n  %+v\nwant\n  %+v", pre, wantPre)
	}
	if !reflect.DeepEqual(post, wantPost) {
		t.Errorf("postflights =\n  %+v\nwant\n  %+v", post, wantPost)
	}
}

// TestParsePreflightArgsOrphanVarDropped confirms a -preflight-var before any
// -preflight is ignored (with a warning), not a crash, and yields empty chains.
func TestParsePreflightArgsOrphanVarDropped(t *testing.T) {
	pre, post := parsePreflightArgs([]string{"-preflight-var", "X:y", "-postflight-var", "A:b"})
	if len(pre) != 0 || len(post) != 0 {
		t.Errorf("orphan vars should produce empty chains, got pre=%v post=%v", pre, post)
	}
}
