package input

import (
	"fmt"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

type MainInputProvider struct {
	Providers   []ffuf.InternalInputProvider
	Config      *ffuf.Config
	position    int
	msbIterator int
}

func NewInputProvider(conf *ffuf.Config) (ffuf.InputProvider, ffuf.Multierror) {
	validmode := false
	errs := ffuf.NewMultierror()
	for _, mode := range []string{"clusterbomb", "pitchfork"} {
		if conf.InputMode == mode {
			validmode = true
		}
	}
	if !validmode {
		errs.Add(fmt.Errorf("Input mode (-mode) %s not recognized", conf.InputMode))
		return &MainInputProvider{}, errs
	}
	mainip := MainInputProvider{Config: conf, msbIterator: 0}
	// Initialize the correct inputprovider
	for _, v := range conf.InputProviders {
		err := mainip.AddProvider(v)
		if err != nil {
			errs.Add(err)
		}
	}
	return &mainip, errs
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

//Value returns a map of inputs for keywords
func (i *MainInputProvider) Value() map[string][]byte {
	retval := make(map[string][]byte)
	if i.Config.InputMode == "clusterbomb" {
		retval = i.clusterbombValue()
	}
	if i.Config.InputMode == "pitchfork" {
		retval = i.pitchforkValue()
	}
	return retval
}

//Reset resets all the inputproviders and counters
func (i *MainInputProvider) Reset() {
	for _, p := range i.Providers {
		p.ResetPosition()
	}
	i.position = 0
	i.msbIterator = 0
}

//pitchforkValue returns a map of keyword:value pairs including all inputs.
//This mode will iterate through wordlists in lockstep.
func (i *MainInputProvider) pitchforkValue() map[string][]byte {
	values := make(map[string][]byte)
	for _, p := range i.Providers {
		if !p.Next() {
			// Loop to beginning if the inputprovider has been exhausted
			p.ResetPosition()
		}
		values[p.Keyword()] = p.Value()
		p.IncrementPosition()
	}
	return values
}

//clusterbombValue returns map of keyword:value pairs including all inputs.
//this mode will iterate through all possible combinations.
func (i *MainInputProvider) clusterbombValue() map[string][]byte {
	values := make(map[string][]byte)
	// Should we signal the next InputProvider in the slice to increment
	signalNext := false
	first := true
	for index, p := range i.Providers {
		if signalNext {
			p.IncrementPosition()
			signalNext = false
		}
		if !p.Next() {
			// No more inputs in this inputprovider
			if index == i.msbIterator {
				// Reset all previous wordlists and increment the msb counter
				i.msbIterator += 1
				i.clusterbombIteratorReset()
				// Start again
				return i.clusterbombValue()
			}
			p.ResetPosition()
			signalNext = true
		}
		values[p.Keyword()] = p.Value()
		if first {
			p.IncrementPosition()
			first = false
		}
	}
	return values
}

func (i *MainInputProvider) clusterbombIteratorReset() {
	for index, p := range i.Providers {
		if index < i.msbIterator {
			p.ResetPosition()
		}
		if index == i.msbIterator {
			p.IncrementPosition()
		}
	}
}

//Total returns the amount of input combinations available
func (i *MainInputProvider) Total() int {
	count := 0
	if i.Config.InputMode == "pitchfork" {
		for _, p := range i.Providers {
			if p.Total() > count {
				count = p.Total()
			}
		}
	}
	if i.Config.InputMode == "clusterbomb" {
		count = 1
		for _, p := range i.Providers {
			count = count * p.Total()
		}
	}
	return count
}
