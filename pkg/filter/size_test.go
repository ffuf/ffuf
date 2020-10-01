package filter

import (
	"strings"
	"testing"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

func TestNewSizeFilter(t *testing.T) {
	f, _ := NewSizeFilter("1,2,3,444,5-90")
	sizeRepr := f.Repr()
	if !strings.Contains(sizeRepr, "1,2,3,444,5-90") {
		t.Errorf("Size filter was expected to have 5 values")
	}
}

func TestNewSizeFilterError(t *testing.T) {
	_, err := NewSizeFilter("invalid")
	if err == nil {
		t.Errorf("Was expecting an error from errenous input data")
	}
}

func TestFiltering(t *testing.T) {
	f, _ := NewSizeFilter("1,2,3,5-90,444")
	for i, test := range []struct {
		input  int64
		output bool
	}{
		{1, true},
		{2, true},
		{3, true},
		{4, false},
		{5, true},
		{70, true},
		{90, true},
		{91, false},
		{444, true},
	} {
		resp := ffuf.Response{ContentLength: test.input}
		filterReturn, _ := f.Filter(&resp)
		if filterReturn != test.output {
			t.Errorf("Filter test %d: Was expecing filter return value of %t but got %t", i, test.output, filterReturn)
		}
	}
}
