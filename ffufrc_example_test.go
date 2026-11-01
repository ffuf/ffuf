package main

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/pelletier/go-toml"
)

// TestFfufrcExampleKeysAreRead decodes ffufrc.example in strict mode, which
// fails on any key that matches no ConfigOptions field. go-toml otherwise
// ignores such keys silently, so a misspelled key in the example would just do
// nothing.
func TestFfufrcExampleKeysAreRead(t *testing.T) {
	f, err := os.Open("ffufrc.example")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := toml.NewDecoder(f).Strict(true).Decode(ffuf.NewConfigOptions()); err != nil {
		t.Errorf("ffufrc.example: %s", err)
	}
}

// TestFfufrcExampleValuesReachConfig loads ffufrc.example the way -config does
// (ReadConfig, then ConfigFromOptions) and checks that the renamed keys set
// the Config fields they are meant for. A key that decodes into the wrong field
// passes the strict check above. The example ships the default values, which a
// misrouted key would leave in place, so each one is first changed to a
// non-default value.
func TestFfufrcExampleValuesReachConfig(t *testing.T) {
	example, err := os.ReadFile("ffufrc.example")
	if err != nil {
		t.Fatal(err)
	}
	body := string(example)
	for _, r := range []struct{ old, new string }{
		{"recursiondepth = 0", "recursiondepth = 3"},
		{"autocalibrationstrategies = [", `autocalibrationstrategies = ["advanced",`},
		{"outputskipemptyfile = false", "outputskipemptyfile = true"},
	} {
		if !strings.Contains(body, r.old) {
			t.Fatalf("ffufrc.example: %q not found", r.old)
		}
		body = strings.Replace(body, r.old, r.new, 1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The example names files that do not exist (wordlists, request file), so
	// ConfigFromOptions returns an error. The Config it returns is still filled.
	conf, _ := ffuf.ConfigFromOptions(loadOpts(t, body), ctx, cancel)

	if conf.RecursionDepth != 3 {
		t.Errorf("RecursionDepth = %d, want 3", conf.RecursionDepth)
	}
	if want := []string{"advanced", "basic"}; !reflect.DeepEqual(conf.AutoCalibrationStrategies, want) {
		t.Errorf("AutoCalibrationStrategies = %q, want %q", conf.AutoCalibrationStrategies, want)
	}
	if !conf.OutputSkipEmptyFile {
		t.Errorf("OutputSkipEmptyFile = false, want true")
	}
}
