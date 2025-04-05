package ffuf

import (
	"fmt"
)

type Multierror struct {
	errors []error
}

// NewMultierror returns a new Multierror
func NewMultierror() Multierror {
	return Multierror{}
}

func (m *Multierror) Add(err error) {
	m.errors = append(m.errors, err)
}

func (m *Multierror) ErrorOrNil() error {
	var errString string
	if len(m.errors) > 0 {
		errString += fmt.Sprintf("%d errors occured.\n", len(m.errors))
		for _, e := range m.errors {
			errString += fmt.Sprintf("\t* %s\n", e)
		}
		return fmt.Errorf("%s", errString)
	}
	return nil
}
