package integration

import (
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"github.com/ffuf/ffuf/v2/pkg/testtarget"
)

// TestEncoderApplied checks that -enc transforms the payload before it is sent:
// with FUZZ:b64encode, "hello" goes on the wire as its base64 form, which the
// recorder confirms.
func TestEncoderApplied(t *testing.T) {
	target := testtarget.New()
	defer target.Close()

	runScan(t, target.URL+"/reflect/FUZZ",
		[]string{"hello"},
		func(o *ffuf.ConfigOptions) { o.Input.Encoders = []string{"FUZZ:b64encode"} },
		func(mm ffuf.MatcherManager) { mustMatch(t, mm, "status", "all") },
	)

	paths := recordedPaths(target.Requests())
	// base64("hello") == "aGVsbG8="
	assertContainsAll(t, paths, []string{"/reflect/aGVsbG8="})
	assertContainsNone(t, paths, []string{"/reflect/hello"})
}
