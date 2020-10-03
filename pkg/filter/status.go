package filter

import (
	"encoding/json"
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
			intranges = append(intranges, ffuf.ValueRange{Min: AllStatuses, Max: AllStatuses})
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

func (f *StatusFilter) MarshalJSON() ([]byte, error) {
	value := make([]string, 0)
	for _, v := range f.Value {
		if v.Min == 0 && v.Max == 0 {
			value = append(value, "all")
		} else {
			if v.Min == v.Max {
				value = append(value, strconv.FormatInt(v.Min, 10))
			} else {
				value = append(value, fmt.Sprintf("%d-%d", v.Min, v.Max))
			}
		}
	}
	return json.Marshal(&struct {
		Value string `json:"value"`
	}{
		Value: strings.Join(value, ","),
	})
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
