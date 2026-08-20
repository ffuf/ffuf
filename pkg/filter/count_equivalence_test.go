package filter

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// The word/line counts are computed with bytes.Count(data, sep)+1 instead of
// len(strings.Split(string(data), sep)) to avoid allocating a []string over the
// whole response body. These tests pin the two to the same value across the edge
// cases where an off-by-one would hide: empty body, only separators, and
// leading/trailing separators. The filter is built with an exact [n,n] range so a
// match proves the production count equals the old strings.Split count, and the
// [n+1,n+1] miss proves it is not off by one.

var countEquivalenceBodies = []string{
	"",
	"a",
	"a b c",
	"     ",
	" a ",
	"a\nb\nc",
	"\n\n",
	"abc",
	"a\n",
	"one two\nthree four\n",
}

func TestWordFilterCountMatchesSplit(t *testing.T) {
	for _, body := range countEquivalenceBodies {
		expected := len(strings.Split(body, " "))
		resp := ffuf.Response{Data: []byte(body)}

		hit, _ := NewWordFilter(strconv.Itoa(expected))
		if match, _ := hit.Filter(&resp); !match {
			t.Errorf("word count for %q: expected filter to match %d", body, expected)
		}
		miss, _ := NewWordFilter(strconv.Itoa(expected + 1))
		if match, _ := miss.Filter(&resp); match {
			t.Errorf("word count for %q: filter matched %d, count is off by one", body, expected+1)
		}
	}
}

func TestLineFilterCountMatchesSplit(t *testing.T) {
	for _, body := range countEquivalenceBodies {
		expected := len(strings.Split(body, "\n"))
		resp := ffuf.Response{Data: []byte(body)}

		hit, _ := NewLineFilter(strconv.Itoa(expected))
		if match, _ := hit.Filter(&resp); !match {
			t.Errorf("line count for %q: expected filter to match %d", body, expected)
		}
		miss, _ := NewLineFilter(strconv.Itoa(expected + 1))
		if match, _ := miss.Filter(&resp); match {
			t.Errorf("line count for %q: filter matched %d, count is off by one", body, expected+1)
		}
	}
}
