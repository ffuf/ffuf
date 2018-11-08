package filter

import (
	"github.com/ffuf/ffuf/pkg/ffuf"
)

func NewFilterByName(name string, value string) (ffuf.FilterProvider, error) {
	if name == "status" {
		return NewStatusFilter(value)
	}
	if name == "size" {
		return NewSizeFilter(value)
	}
	return nil, nil
}
