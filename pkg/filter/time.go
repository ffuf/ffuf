package filter

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

type TimeFilter struct {
	ms       int64 // milliseconds since first response byte
	gt       bool  // filter if response time is greater than
	lt       bool  // filter if response time is less than
	valueRaw string
}

func NewTimeFilter(value string) (ffuf.FilterProvider, error) {
	var milliseconds int64
	gt, lt := false, false

	gt = strings.HasPrefix(value, ">")
	lt = strings.HasPrefix(value, "<")

	if (!lt && !gt) || (lt && gt) {
		return &TimeFilter{}, fmt.Errorf("Time filter or matcher (-ft / -mt): invalid value: %s", value)
	}

	milliseconds, err := strconv.ParseInt(value[1:], 10, 64)
	if err != nil {
		return &TimeFilter{}, fmt.Errorf("Time filter or matcher (-ft / -mt): invalid value: %s", value)
	}
	return &TimeFilter{ms: milliseconds, gt: gt, lt: lt, valueRaw: value}, nil
}

func (f *TimeFilter) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Value string `json:"value"`
	}{
		Value: f.valueRaw,
	})
}

func (f *TimeFilter) Filter(response *ffuf.Response) (bool, error) {
	if f.gt {
		if response.Time.Milliseconds() > f.ms {
			return true, nil
		}

	} else if f.lt {
		if response.Time.Milliseconds() < f.ms {
			return true, nil
		}
	}

	return false, nil
}

func (f *TimeFilter) Repr() string {
	return f.valueRaw
}

func (f *TimeFilter) ReprVerbose() string {
	return fmt.Sprintf("Response time: %s", f.Repr())
}
