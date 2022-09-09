package input

import (
	"github.com/ffuf/ffuf/pkg/config"
)

// InputProvider interface handles the input data for RunnerProvider
type InputProvider interface {
	ActivateKeywords([]string)
	AddProvider(config.InputProviderConfig) error
	Keywords() []string
	Next() bool
	Position() int
	Reset()
	Value() map[string][]byte
	Total() int
}

// InternalInputProvider interface handles providing input data to InputProvider
type InternalInputProvider interface {
	Keyword() string
	Next() bool
	Position() int
	ResetPosition()
	IncrementPosition()
	Value() []byte
	Total() int
	Active() bool
	Enable()
	Disable()
}
