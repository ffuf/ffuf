package filter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type SizeFilter struct {
	Value []ValueRange
}

func NewSizeFilter(value string) (ffuf.FilterProvider, error) {
	var intranges []ValueRange
	for _, sv := range strings.Split(value, ",") {
		vr, err := ValueRangeFromString(sv)
		if err != nil {
			return &SizeFilter{}, fmt.Errorf("Size filter or matcher (-fs / -ms): invalid value: %s", sv)
		}

		intranges = append(intranges, vr)
	}
	return &SizeFilter{Value: intranges}, nil
}

func (f *SizeFilter) Filter(response *ffuf.Response) (bool, error) {
	for _, iv := range f.Value {
		if iv.min <= response.ContentLength && response.ContentLength <= iv.max {
			return true, nil
		}
	}
	return false, nil
}

func (f *SizeFilter) Repr() string {
	var strval []string
	for _, iv := range f.Value {
		if iv.min == iv.max {
			strval = append(strval, strconv.Itoa(int(iv.min)))
		} else {
			strval = append(strval, strconv.Itoa(int(iv.min))+"-"+strconv.Itoa(int(iv.max)))
		}
	}
	return fmt.Sprintf("Response size: %s", strings.Join(strval, ","))
}
