package filter

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type WordFilter struct {
	Value []int64
}

func NewWordFilter(value string) (ffuf.FilterProvider, error) {
	var intvals []int64
	for _, sv := range strings.Split(value, ",") {
		intval, err := strconv.ParseInt(sv, 10, 0)
		if err != nil {
			return &WordFilter{}, fmt.Errorf("Word filter or matcher (-fw / -mw): invalid value: %s", value)
		}
		intvals = append(intvals, intval)
	}
	return &WordFilter{Value: intvals}, nil
}

func (f *WordFilter) Filter(response *ffuf.Response) (bool, error) {
	wordsSize := len(strings.Split(string(response.Data), " "))
	for _, iv := range f.Value {
		if iv == int64(wordsSize) {
			return true, nil
		}
	}
	return false, nil
}

func (f *WordFilter) Repr() string {
	var strval []string
	for _, iv := range f.Value {
		strval = append(strval, strconv.Itoa(int(iv)))
	}
	return fmt.Sprintf("Response words: %s", strings.Join(strval, ","))
}
