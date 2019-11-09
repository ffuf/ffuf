package input

import (
	"github.com/ffuf/ffuf/pkg/ffuf"
)

type MainInputProvider struct {
	Providers []ffuf.InternalInputProvider
	Config    *ffuf.Config
	position  int
}

func NewInputProvider(conf *ffuf.Config) ffuf.InputProvider {
	return &MainInputProvider{Config: conf}
}

func (i *MainInputProvider) AddProvider(provider ffuf.InputProviderConfig) error {
	if provider.Name == "command" {
		newcomm, _ := NewCommandInput(provider.Keyword, provider.Value, i.Config)
		i.Providers = append(i.Providers, newcomm)
	} else {
		// Default to wordlist
		newwl, err := NewWordlistInput(provider.Keyword, provider.Value, i.Config)
		if err != nil {
			return err
		}
		i.Providers = append(i.Providers, newwl)
	}
	return nil
}

//Position will return the current position of progress
func (i *MainInputProvider) Position() int {
	return i.position
}

//Next will increment the cursor position, and return a boolean telling if there's inputs left
func (i *MainInputProvider) Next() bool {
	if i.position >= i.Total() {
		return false
	}
	i.position++
	return true
}

//Value returns a map of keyword:value pairs including all inputs
func (i *MainInputProvider) Value() map[string][]byte {
	values := make(map[string][]byte)
	for _, p := range i.Providers {
		if !p.Next() {
			// Loop to beginning if the inputprovider has been exhausted
			p.ResetPosition()
		}
		values[p.Keyword()] = p.Value()
	}
	return values
}

//Total returns the amount of input combinations available
func (i *MainInputProvider) Total() int {
	count := 1
	for _, p := range i.Providers {
		count = count * p.Total()
	}
	return count
}
