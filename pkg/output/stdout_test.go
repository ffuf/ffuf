package output

import (
	"strings"
	"testing"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

func TestFormatProgressFitsWidth(t *testing.T) {
	status := ffuf.Progress{
		StartedAt:  time.Now().Add(-15 * time.Second),
		ReqCount:   1234,
		ReqTotal:   12345,
		ReqSec:     78,
		QueuePos:   2,
		QueueTotal: 3,
		ErrorCount: 9,
	}

	cases := []struct {
		name  string
		width int
	}{
		{"unknown_width_returns_full", 0},
		{"wide_terminal_returns_full", 200},
		{"narrow_120", 120},
		{"narrow_80", 80},
		{"narrow_50", 50},
		{"narrow_30", 30},
		{"narrow_15", 15},
		{"narrow_5", 5},
		{"narrow_1", 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := formatProgress(status, tc.width)
			if strings.ContainsAny(out, "\n\r") {
				t.Fatalf("output must not contain newlines or carriage returns: %q", out)
			}
			if tc.width > 0 && len(out) > tc.width {
				t.Fatalf("width=%d but len(output)=%d: %q", tc.width, len(out), out)
			}
		})
	}
}

func TestFormatProgressFullVariantContent(t *testing.T) {
	status := ffuf.Progress{
		StartedAt:  time.Now().Add(-1 * time.Second),
		ReqCount:   100,
		ReqTotal:   200,
		ReqSec:     50,
		QueuePos:   1,
		QueueTotal: 1,
		ErrorCount: 0,
	}

	out := formatProgress(status, 0)
	for _, want := range []string{"Progress:", "[100/200]", "Job [1/1]", "50 req/sec", "Errors: 0"} {
		if !strings.Contains(out, want) {
			t.Errorf("full progress output missing %q: %q", want, out)
		}
	}
}
