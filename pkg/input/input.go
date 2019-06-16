package input

import (
	"github.com/ffuf/ffuf/pkg/ffuf"
)

func NewInputProviderByName(name string, conf *ffuf.Config) (ffuf.InputProvider, error) {
	if name == "command" {
		return NewCommandInput(conf)
	} else {
		// Default to wordlist
		return NewWordlistInput(conf)
	}
}
