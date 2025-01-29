package input

import (
	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

type CsrfInput struct {
	config  *ffuf.Config
	count   int
	active  bool
	keyword string
	value   string
}

func NewCsrfInput(keyword string, value string, conf *ffuf.Config) (*CsrfInput, error) {
	var csrf CsrfInput
	csrf.active = true
	csrf.keyword = keyword
	csrf.config = conf
	csrf.value = value
	csrf.count = 0
	return &csrf, nil
}

// Keyword returns the keyword assigned to this InternalInputProvider
func (c *CsrfInput) Keyword() string {
	return c.keyword
}

// Position will return the current position in the input list
func (c *CsrfInput) Position() int {
	return c.count
}

// SetPosition will set the current position of the inputprovider
func (c *CsrfInput) SetPosition(pos int) {
	c.count = pos
}

// ResetPosition will reset the current position of the InternalInputProvider
func (c *CsrfInput) ResetPosition() {
	c.count = 0
}

// IncrementPosition increments the current position in the inputprovider
func (c *CsrfInput) IncrementPosition() {
	c.count += 1
}

// Next will increment the cursor position, and return a boolean telling if there's iterations left
func (c *CsrfInput) Next() bool {
	return c.count < c.config.InputNum
}

// Value returns the input from command stdoutput
func (c *CsrfInput) Value() []byte {
	return []byte(c.value)
}

// Total returns the size of wordlist
func (c *CsrfInput) Total() int {
	return 1
}

func (c *CsrfInput) Active() bool {
	return c.active
}

func (c *CsrfInput) Enable() {
	c.active = true
}

func (c *CsrfInput) Disable() {
	c.active = false
}
