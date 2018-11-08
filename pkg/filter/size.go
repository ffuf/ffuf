package filter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type SizeFilter struct {
	Value []int64
}

func NewSizeFilter(value string) (ffuf.FilterProvider, error) {
	var intvals []int64
	for _, sv := range strings.Split(value, ",") {
		intval, err := strconv.ParseInt(sv, 10, 0)
		if err != nil {
			return &SizeFilter{}, fmt.Errorf("Size filter (-fs): invalid value: %s", value)
		}
		intvals = append(intvals, intval)
	}
	return &SizeFilter{Value: intvals}, nil
}

func (f *SizeFilter) Filter(response *ffuf.Response) (bool, error) {
	for _, iv := range f.Value {
		if iv == response.ContentLength {
			return true, nil
		}
	}
	return false, nil
}

func (f *SizeFilter) Repr() string {
	var strval []string
	for _, iv := range f.Value {
		strval = append(strval, strconv.Itoa(int(iv)))
	}
	return fmt.Sprintf("Response size: %s", strings.Join(strval, ","))
}
