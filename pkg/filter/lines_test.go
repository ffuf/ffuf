package filter

import (
	"strings"
	"testing"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

func TestNewLineFilter(t *testing.T) {
	f, _ := NewLineFilter("200,301,400-410,500")
	linesRepr := f.Repr()
	if !strings.Contains(linesRepr, "200,301,400-410,500") {
		t.Errorf("Word filter was expected to have 4 values")
	}
}

func TestNewLineFilterError(t *testing.T) {
	_, err := NewLineFilter("invalid")
	if err == nil {
		t.Errorf("Was expecting an error from errenous input data")
	}
}

func TestLineFiltering(t *testing.T) {
	f, _ := NewLineFilter("200,301,402-450,500")
	for i, test := range []struct {
		input  int64
		output bool
	}{
		{200, true},
		{301, true},
		{500, true},
		{4, false},
		{444, true},
		{302, false},
		{401, false},
		{402, true},
		{450, true},
		{451, false},
	} {
		var data []string
		for i := int64(0); i < test.input; i++ {
			data = append(data, "A")
		}
		resp := ffuf.Response{Data: []byte(strings.Join(data, " "))}
		filterReturn, _ := f.Filter(&resp)
		if filterReturn != test.output {
			t.Errorf("Filter test %d: Was expecing filter return value of %t but got %t", i, test.output, filterReturn)
		}
	}
}
