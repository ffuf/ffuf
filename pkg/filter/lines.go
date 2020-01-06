package filter

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type LineFilter struct {
	Value []ffuf.ValueRange
}

func NewLineFilter(value string) (ffuf.FilterProvider, error) {
	var intranges []ffuf.ValueRange
	for _, sv := range strings.Split(value, ",") {
		vr, err := ffuf.ValueRangeFromString(sv)
		if err != nil {
			return &LineFilter{}, fmt.Errorf("Line filter or matcher (-fl / -ml): invalid value: %s", sv)
		}
		intranges = append(intranges, vr)
	}
	return &LineFilter{Value: intranges}, nil
}

func (f *LineFilter) MarshalJSON() ([]byte, error) {
	value := make([]string, 0)
	for _, v := range f.Value {
		if v.Min == v.Max {
			value = append(value, strconv.FormatInt(v.Min, 10))
		} else {
			value = append(value, fmt.Sprintf("%d-%d", v.Min, v.Max))
		}
	}
	return json.Marshal(&struct {
		Value string `json:"value"`
	}{
		Value: strings.Join(value, ","),
	})
}

func (f *LineFilter) Filter(response *ffuf.Response) (bool, error) {
	linesSize := len(strings.Split(string(response.Data), "\n"))
	for _, iv := range f.Value {
		if iv.Min <= int64(linesSize) && int64(linesSize) <= iv.Max {
			return true, nil
		}
	}
	return false, nil
}

func (f *LineFilter) Repr() string {
	var strval []string
	for _, iv := range f.Value {
		if iv.Min == iv.Max {
			strval = append(strval, strconv.Itoa(int(iv.Min)))
		} else {
			strval = append(strval, strconv.Itoa(int(iv.Min))+"-"+strconv.Itoa(int(iv.Max)))
		}
	}
	return fmt.Sprintf("Response lines: %s", strings.Join(strval, ","))
}
