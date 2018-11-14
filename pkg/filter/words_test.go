package filter

import (
	"strings"
	"testing"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

func TestNewWordFilter(t *testing.T) {
	f, _ := NewWordFilter("200,301,500")
	wordsRepr := f.Repr()
	if strings.Index(wordsRepr, "200,301,500") == -1 {
		t.Errorf("Word filter was expected to have 3 values")
	}
}

func TestNewWordFilterError(t *testing.T) {
	_, err := NewWordFilter("invalid")
	if err == nil {
		t.Errorf("Was expecting an error from errenous input data")
	}
}

func TestWordFiltering(t *testing.T) {
	f, _ := NewWordFilter("200,301,500")
	for i, test := range []struct {
		input  int64
		output bool
	}{
		{200, true},
		{301, true},
		{500, true},
		{4, false},
		{444, false},
		{302, false},
		{401, false},
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
