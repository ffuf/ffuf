package ffuf

import (
	"flag"
	"testing"
)

// TestRegisterFlags_Wellformed runs RegisterFlags against the real ConfigOptions
// struct so any malformed declaration — a slice field missing its `kind`, an
// unsupported field type, an unexported tagged field, or a duplicate flag name —
// fails here in CI instead of panicking at runtime. This is what buys back the
// compile-time safety that reflection-based registration gives up.
func TestRegisterFlags_Wellformed(t *testing.T) {
	fs := flag.NewFlagSet("ffuf", flag.ContinueOnError)
	opts := NewConfigOptions()

	var reg *FlagRegistry
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("RegisterFlags panicked (malformed tag or unsupported field type): %v", r)
			}
		}()
		reg = RegisterFlags(fs, opts)
	}()

	validSection := map[string]bool{
		SectionHTTP: true, SectionGeneral: true, SectionCompat: true, SectionMatcher: true,
		SectionFilter: true, SectionInput: true, SectionOutput: true,
	}

	count := 0
	fs.VisitAll(func(f *flag.Flag) {
		count++
		section, ok := reg.SectionOf(f.Name)
		if !ok {
			t.Errorf("flag -%s is registered but missing from the registry (no section)", f.Name)
			return
		}
		if !validSection[section] {
			t.Errorf("flag -%s has unknown section %q", f.Name, section)
		}
	})

	// Guard against silent flag drops or duplicate registrations: ffuf currently
	// ships 79 flags (72 visible + 7 hidden compatibility flags). Update this
	// number deliberately when adding or removing a flag.
	if count != 79 {
		t.Errorf("expected 79 registered flags, got %d — a flag was added, dropped, or duplicated", count)
	}
}
