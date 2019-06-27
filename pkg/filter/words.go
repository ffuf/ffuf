package filter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type WordFilter struct {
	Value []ffuf.ValueRange
}

func NewWordFilter(value string) (ffuf.FilterProvider, error) {
	var intranges []ffuf.ValueRange
	for _, sv := range strings.Split(value, ",") {
		vr, err := ffuf.ValueRangeFromString(sv)
		if err != nil {
			return &WordFilter{}, fmt.Errorf("Word filter or matcher (-fw / -mw): invalid value: %s", sv)
		}
		intranges = append(intranges, vr)
	}
	return &WordFilter{Value: intranges}, nil
}

func (f *WordFilter) Filter(response *ffuf.Response) (bool, error) {
	wordsSize := len(strings.Split(string(response.Data), " "))
	for _, iv := range f.Value {
		if iv.Min <= int64(wordsSize) && int64(wordsSize) <= iv.Max {
			return true, nil
		}
	}
	return false, nil
}

func (f *WordFilter) Repr() string {
	var strval []string
	for _, iv := range f.Value {
		if iv.Min == iv.Max {
			strval = append(strval, strconv.Itoa(int(iv.Min)))
		} else {
			strval = append(strval, strconv.Itoa(int(iv.Min))+"-"+strconv.Itoa(int(iv.Max)))
		}
	}
	return fmt.Sprintf("Response words: %s", strings.Join(strval, ","))
}
