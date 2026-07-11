package payloadtamper

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

var (
	regexName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

func isValidName(name string) bool {
	return regexName.MatchString(name)
}

func validate(config Config) error {
	// validate directory
	if config.Directory == "" {
		return fmt.Errorf("invalid tamper directory, can not be empty.")
	}

	// Validate that tampers are set
	if len(config.Tampers) == 0 {
		return fmt.Errorf("no tampers provided")
	}

	// Validate tamper name
	for _, t := range config.Tampers {
		if !isValidName(t) {
			return fmt.Errorf("invalid tamper name: %s", t)
		}
	}
	return nil
}

// loadTamper interprets a tamper source file and returns a Tamper.
func loadTamper(path string) (Tamper, error) {
	i := interp.New(interp.Options{})
	i.Use(stdlib.Symbols)

	// Load the interprets tamper from path
	if _, err := i.EvalPath(path); err != nil {
		return nil, fmt.Errorf("failed to interpret tamper %q: %w", path, err)
	}

	// Derive name from filename (e.g. "space2plus.go" -> "space2plus")
	name := strings.TrimSuffix(filepath.Base(path), ".go")

	// Extract Desc as callable Go function
	descV, err := i.Eval("tamper.Tamper.Desc()")
	if err != nil {
		return nil, fmt.Errorf("tamper %q: failed to call Desc(): %w", path, err)
	}

	// Extract Exec as a callable Go function.
	execV, err := i.Eval("tamper.Tamper.Exec")
	if err != nil {
		return nil, fmt.Errorf("tamper %q: failed to get Tamper.Exec: %w", path, err)
	}

	execFn, ok := execV.Interface().(func(string) string)
	if !ok {
		return nil, fmt.Errorf("tamper %q: Tamper.Exec has unexpected type %T", path, execV.Interface())
	}

	// Return the adapter of the Tamper interface
	return &adapter{
		name: name,
		desc: descV.String(),
		exec: execFn,
	}, nil
}
