package filter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

const AllStatuses = 0

type StatusFilter struct {
	Value []ValueRange
}

func NewStatusFilter(value string) (ffuf.FilterProvider, error) {
	var intranges []ValueRange
	for _, sv := range strings.Split(value, ",") {
		if sv == "all" {
			intranges = append(intranges, ValueRange{AllStatuses, AllStatuses})
		} else {
			vr, err := ValueRangeFromString(sv)
			if err != nil {
				return &StatusFilter{}, fmt.Errorf("Status filter or matcher (-fc / -mc): invalid value %s", sv)
			}
			intranges = append(intranges, vr)
		}
	}
	return &StatusFilter{Value: intranges}, nil
}

func (f *StatusFilter) Filter(response *ffuf.Response) (bool, error) {
	for _, iv := range f.Value {
		if iv.min == AllStatuses && iv.max == AllStatuses {
			// Handle the "all" case
			return true, nil
		}
		if iv.min <= response.StatusCode && response.StatusCode <= iv.max {
			return true, nil
		}
	}
	return false, nil
}

func (f *StatusFilter) Repr() string {
	var strval []string
	for _, iv := range f.Value {
		if iv.min == AllStatuses && iv.max == AllStatuses {
			strval = append(strval, "all")
		} else if iv.min == iv.max {
			strval = append(strval, strconv.Itoa(int(iv.min)))
		} else {
			strval = append(strval, strconv.Itoa(int(iv.min))+"-"+strconv.Itoa(int(iv.max)))
		}
	}
	return fmt.Sprintf("Response status: %s", strings.Join(strval, ","))
}
