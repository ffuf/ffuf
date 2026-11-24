package filter

import (
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

func TestNewStatusFilter(t *testing.T) {
	f, _ := NewStatusFilter("200,301,400-410,500")
	statusRepr := f.Repr()
	if !strings.Contains(statusRepr, "200,301,400-410,500") {
		t.Errorf("Status filter was expected to have 4 values")
	}
}

func TestNewStatusFilterSpaces(t *testing.T) {
	// "-mc 200, 301" and config values like status = "200, 301" are common.
	f, err := NewStatusFilter("200, 301 ,400-410, all")
	if err != nil {
		t.Fatalf("Unexpected error for values separated by comma and space: %s", err)
	}
	if repr := f.Repr(); repr != "200,301,400-410,all" {
		t.Errorf("Unexpected repr %q", repr)
	}
}

func TestNewStatusFilterError(t *testing.T) {
	_, err := NewStatusFilter("invalid")
	if err == nil {
		t.Errorf("Was expecting an error from errenous input data")
	}
}

func TestStatusFiltering(t *testing.T) {
	f, _ := NewStatusFilter("200,301,400-498,500")
	for i, test := range []struct {
		input  int64
		output bool
	}{
		{200, true},
		{301, true},
		{500, true},
		{4, false},
		{399, false},
		{400, true},
		{444, true},
		{498, true},
		{499, false},
		{302, false},
	} {
		resp := ffuf.Response{StatusCode: test.input}
		filterReturn, _ := f.Filter(&resp)
		if filterReturn != test.output {
			t.Errorf("Filter test %d: Was expecing filter return value of %t but got %t", i, test.output, filterReturn)
		}
	}
}
