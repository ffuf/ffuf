package filter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

const AllStatuses = 0

type StatusFilter struct {
	Value []ffuf.ValueRange
}

func NewStatusFilter(value string) (ffuf.FilterProvider, error) {
	var intranges []ffuf.ValueRange
	for _, sv := range strings.Split(value, ",") {
		if sv == "all" {
			intranges = append(intranges, ffuf.ValueRange{AllStatuses, AllStatuses})
		} else {
			vr, err := ffuf.ValueRangeFromString(sv)
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
		if iv.Min == AllStatuses && iv.Max == AllStatuses {
			// Handle the "all" case
			return true, nil
		}
		if iv.Min <= response.StatusCode && response.StatusCode <= iv.Max {
			return true, nil
		}
	}
	return false, nil
}

func (f *StatusFilter) Repr() string {
	var strval []string
	for _, iv := range f.Value {
		if iv.Min == AllStatuses && iv.Max == AllStatuses {
			strval = append(strval, "all")
		} else if iv.Min == iv.Max {
			strval = append(strval, strconv.Itoa(int(iv.Min)))
		} else {
			strval = append(strval, strconv.Itoa(int(iv.Min))+"-"+strconv.Itoa(int(iv.Max)))
		}
	}
	return fmt.Sprintf("Response status: %s", strings.Join(strval, ","))
}
