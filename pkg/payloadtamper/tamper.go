package payloadtamper

import (
	"fmt"
	"path/filepath"
	"plugin"
)

// Tamper is the interface that custom tampers must implement.
type Tamper interface {
	// Name returns the name of the tamper.
	Name() string
	// Desc represent the description of the tamper
	Desc() string
	// Exec takes an original payload and returns the tampered payload.
	Exec(payload string) string
}
type PayloadTamper struct {
	tampers  map[string]Tamper
	pipeline []string
	config   Config
}

type Config struct {
	Directory string
	Tampers   []string
}

// TamperInfo holds metadata about a tamper plugin.
type TamperInfo struct {
	Name string
	Desc string
}

// New creates a new PayloadTamper.
func New(config Config) (*PayloadTamper, error) {
	if err := validate(config); err != nil {
		return &PayloadTamper{}, err

	}
	return &PayloadTamper{
		config:   config,
		pipeline: config.Tampers,
		tampers:  make(map[string]Tamper),
	}, nil
}

// Load all tampers in the configured directory based on their filenames
func (pt *PayloadTamper) LoadTampers() error {
	for _, name := range pt.config.Tampers {
		path := filepath.Join(pt.config.Directory, name+".so")

		p, err := plugin.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open tamper plugin %q: %w", name, err)
		}

		sym, err := p.Lookup("Tamper")
		if err != nil {
			return fmt.Errorf("plugin %q missing Tamper symbol: %w", name, err)
		}

		t, ok := sym.(Tamper)
		if !ok {
			return fmt.Errorf("plugin %q: Tamper symbol does not implement Tamper interface", name)
		}

		// Register the tamper
		if err := pt.AddTamper(t); err != nil {
			return err
		}
	}
	return nil
}

// Add a compiled tamper. Returns an error if a tamper
// with the same name is already registered.
func (pt *PayloadTamper) AddTamper(t Tamper) error {
	if t == nil {
		return fmt.Errorf("cannot register nil tamper")
	}
	name := t.Name()
	if _, exists := pt.tampers[name]; exists {
		return fmt.Errorf("tamper already registered: %s", name)
	}
	pt.tampers[name] = t
	return nil
}

// Execute all tampers against the given payload in the configured order.
// Return the final tampered payload.
func (pt *PayloadTamper) Execute(payload string) string {
	for _, name := range pt.pipeline {
		t := pt.tampers[name]
		payload = t.Exec(payload)
	}
	return payload
}

// Returns the names of all registered tampers.
func (pt *PayloadTamper) GetTampersName() []string {
	names := make([]string, 0, len(pt.tampers))
	for name := range pt.tampers {
		names = append(names, name)
	}
	return names
}

// Returns the names of all registered tampers.
func (pt *PayloadTamper) GetTampers() []*Tamper {
	tampers := make([]*Tamper, 0, len(pt.tampers))
	for _, tamper := range pt.tampers {
		tampers = append(tampers, &tamper)
	}
	return tampers
}

// ListTampers scans a directory for .so tamper plugins and returns their metadata.
func ListTampers(directory string) ([]TamperInfo, error) {
	pattern := filepath.Join(directory, "*.so")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob tamper directory: %w", err)
	}

	var tampers []TamperInfo
	for _, path := range matches {
		p, err := plugin.Open(path)
		if err != nil {
			return nil, fmt.Errorf("failed to open tamper plugin %q: %w", path, err)
		}

		sym, err := p.Lookup("Tamper")
		if err != nil {
			return nil, fmt.Errorf("plugin %q missing Tamper symbol: %w", path, err)
		}

		t, ok := sym.(Tamper)
		if !ok {
			return nil, fmt.Errorf("plugin %q: Tamper symbol does not implement Tamper interface", path)
		}

		tampers = append(tampers, TamperInfo{Name: t.Name(), Desc: t.Desc()})
	}
	return tampers, nil
}
