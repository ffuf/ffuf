package interactive

import (
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// TestResultProbeUsesLineCount guards the fl-filter bug: the probe rebuilt to
// re-evaluate filters against an already-collected result must take ContentLines
// from the result's line count, not its length. Before the fix an in-console
// "fl" (line count) filter compared against ContentLength.
func TestResultProbeUsesLineCount(t *testing.T) {
	res := ffuf.Result{
		StatusCode:    200,
		ContentLength: 4096,
		ContentWords:  100,
		ContentLines:  7,
	}

	probe := resultProbe(res)

	if probe.ContentLines != 7 {
		t.Errorf("resultProbe ContentLines = %d, want 7 (fed ContentLength instead of ContentLines)", probe.ContentLines)
	}
	if probe.ContentLength != 4096 {
		t.Errorf("resultProbe ContentLength = %d, want 4096", probe.ContentLength)
	}
	if probe.ContentWords != 100 {
		t.Errorf("resultProbe ContentWords = %d, want 100", probe.ContentWords)
	}
	if probe.StatusCode != 200 {
		t.Errorf("resultProbe StatusCode = %d, want 200", probe.StatusCode)
	}
}
