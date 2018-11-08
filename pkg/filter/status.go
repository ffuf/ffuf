package filter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type StatusFilter struct {
	Value []int64
}

func NewStatusFilter(value string) (ffuf.FilterProvider, error) {
	var intvals []int64
	for _, sv := range strings.Split(value, ",") {
		intval, err := strconv.ParseInt(sv, 10, 0)
		if err != nil {
			return &StatusFilter{}, fmt.Errorf("Status filter (-fc): invalid value %s", value)
		}
		intvals = append(intvals, intval)
	}
	return &StatusFilter{Value: intvals}, nil
}

func (f *StatusFilter) Filter(response *ffuf.Response) (bool, error) {
	for _, iv := range f.Value {
		if iv == response.StatusCode {
			return true, nil
		}
	}
	return false, nil
}

func (f *StatusFilter) Repr() string {
	var strval []string
	for _, iv := range f.Value {
		strval = append(strval, strconv.Itoa(int(iv)))
	}
	return fmt.Sprintf("Response status: %s", strings.Join(strval, ","))
}
