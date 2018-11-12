package filter

import (
	"fmt"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

func NewFilterByName(name string, value string) (ffuf.FilterProvider, error) {
	if name == "status" {
		return NewStatusFilter(value)
	}
	if name == "size" {
		return NewSizeFilter(value)
	}
	if name == "word" {
		return NewWordFilter(value)
	}
	if name == "regexp" {
		return NewRegexpFilter(value)
	}
	return nil, fmt.Errorf("Could not create filter with name %s", name)
}
