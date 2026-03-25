package payloadtamper

import (
	"fmt"
	"path/filepath"
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
		path := filepath.Join(pt.config.Directory, name+".go")

		t, err := loadTamper(path)
		if err != nil {
			return err
		}

		// Register the tamper
		if err := pt.registerTamper(t); err != nil {
			return err
		}
	}
	return nil
}

// Regsiter a tamper. Returns an error if a tamper
// with the same name is already registered.
func (pt *PayloadTamper) registerTamper(t Tamper) error {
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

// GetTampersInfo scans a directory for .go tamper scripts and returns their metadata.
func GetTampersInfo(directory string) ([]TamperInfo, error) {
	matches, err := filepath.Glob(filepath.Join(directory, "*.go"))
	if err != nil {
		return nil, fmt.Errorf("failed to scan tamper directory %q: %w", directory, err)
	}

	var tampersInfo []TamperInfo
	for _, path := range matches {
		t, err := loadTamper(path)
		if err != nil {
			return nil, err
		}
		tampersInfo = append(tampersInfo, TamperInfo{
			Name: t.Name(),
			Desc: t.Desc(),
		})
	}
	return tampersInfo, nil
}
