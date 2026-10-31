package output

import (
	"testing"
)

func TestTruncateProgressLine(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		width int
		want  string
	}{
		{name: "shorter than width is unchanged", line: "hello", width: 10, want: "hello"},
		{name: "exactly width is unchanged", line: "hello", width: 5, want: "hello"},
		{name: "longer than width is truncated", line: "hello world", width: 8, want: "hello w~"},
		{name: "width one keeps a single rune", line: "hello", width: 1, want: "h"},
		{name: "non-positive width disables truncation", line: "hello", width: 0, want: "hello"},
		{name: "unicode is truncated by rune count", line: "héllo", width: 4, want: "hél~"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := truncateProgressLine(test.line, test.width); got != test.want {
				t.Fatalf("truncateProgressLine(%q, %d) = %q, want %q", test.line, test.width, got, test.want)
			}
		})
	}
}
