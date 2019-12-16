package filter

import (
	"fmt"
	"regexp"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type RegexpFilter struct {
	Value    *regexp.Regexp
	valueRaw string
}

func NewRegexpFilter(value string) (ffuf.FilterProvider, error) {
	re, err := regexp.Compile(value)
	if err != nil {
		return &RegexpFilter{}, fmt.Errorf("Regexp filter or matcher (-fr / -mr): invalid value: %s", value)
	}
	return &RegexpFilter{Value: re, valueRaw: value}, nil
}

func (f *RegexpFilter) Filter(response *ffuf.Response) (bool, error) {
	matchheaders := ""
	for k, v := range response.Headers {
		for _, iv := range v {
			matchheaders += k + ": " + iv + "\r\n"
		}
	}
	matchdata := []byte(matchheaders)
	matchdata = append(matchdata, response.Data...)
	// is the raw value a fuzzing keyword
	if _, fuzzed := inputs[f.valueRaw]; fuzzed {
		matched, err := regexp.Match(string(inputs[f.valueRaw]), matchdata)
		if err != nil {
			return false, nil
		}
		return matched, nil
	}
	return f.Value.Match(matchdata), nil
}

func (f *RegexpFilter) Repr() string {
	return fmt.Sprintf("Regexp: %s", f.valueRaw)
}
