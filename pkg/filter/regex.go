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
		return &RegexpFilter{}, fmt.Errorf("Size filter or matcher (-fs / -ms): invalid value: %s", value)
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
	return f.Value.Match(matchdata), nil
}

func (f *RegexpFilter) Repr() string {
	return fmt.Sprintf("Regexp: %s", f.valueRaw)
}
