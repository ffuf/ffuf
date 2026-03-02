package payloadtamper

import (
	"fmt"
)

func validate(config Config) error {
	if config.Directory == "" {
		return fmt.Errorf("invalid tamper directory, can not be empty.")
	}

	if len(config.Tampers) == 0 {
		return fmt.Errorf("no tampers provided")
	}
	return nil
}
