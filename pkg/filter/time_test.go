package filter

import (
	"testing"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

func TestNewTimeFilter(t *testing.T) {
	fp, _ := NewTimeFilter(">100")

	f := fp.(*TimeFilter)

	if !f.gt || f.lt {
		t.Errorf("Time filter was expected to have greater-than")
	}

	if f.ms != 100 {
		t.Errorf("Time filter was expected to have ms == 100")
	}
}

func TestNewTimeFilterError(t *testing.T) {
	_, err := NewTimeFilter("100>")
	if err == nil {
		t.Errorf("Was expecting an error from errenous input data")
	}
}

func TestTimeFiltering(t *testing.T) {
	f, _ := NewTimeFilter(">100")

	for i, test := range []struct {
		input  int64
		output bool
	}{
		{1342, true},
		{2000, true},
		{35000, true},
		{1458700, true},
		{99, false},
		{2, false},
	} {
		resp := ffuf.Response{
			Data: []byte("dahhhhhtaaaaa"),
			Time: time.Duration(test.input * int64(time.Millisecond)),
		}
		filterReturn, _ := f.Filter(&resp)
		if filterReturn != test.output {
			t.Errorf("Filter test %d: Was expecing filter return value of %t but got %t", i, test.output, filterReturn)
		}
	}
}
