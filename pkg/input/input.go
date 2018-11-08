package input

import (
	"github.com/ffuf/ffuf/pkg/ffuf"
)

func NewInputProviderByName(name string, conf *ffuf.Config) (ffuf.InputProvider, error) {
	// We have only one inputprovider at the moment
	return NewWordlistInput(conf)
}
