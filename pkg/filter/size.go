package filter

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type SizeFilter struct {
	Value []ffuf.ValueRange
}

func NewSizeFilter(value string) (ffuf.FilterProvider, error) {
	var intranges []ffuf.ValueRange
	for _, sv := range strings.Split(value, ",") {
		vr, err := ffuf.ValueRangeFromString(sv)
		if err != nil {
			return &SizeFilter{}, fmt.Errorf("Size filter or matcher (-fs / -ms): invalid value: %s", sv)
		}

		intranges = append(intranges, vr)
	}
	return &SizeFilter{Value: intranges}, nil
}

func (f *SizeFilter) MarshalJSON() ([]byte, error) {
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

func (f *SizeFilter) Filter(response *ffuf.Response) (bool, error) {
	for _, iv := range f.Value {
		if iv.Min <= response.ContentLength && response.ContentLength <= iv.Max {
			return true, nil
		}
	}
	return false, nil
}

func (f *SizeFilter) Repr() string {
	var strval []string
	for _, iv := range f.Value {
		if iv.Min == iv.Max {
			strval = append(strval, strconv.Itoa(int(iv.Min)))
		} else {
			strval = append(strval, strconv.Itoa(int(iv.Min))+"-"+strconv.Itoa(int(iv.Max)))
		}
	}
	return fmt.Sprintf("Response size: %s", strings.Join(strval, ","))
}
