package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

type UsageSection struct {
	Name        string
	Description string
	Flags       []UsageFlag
	Hidden      bool
	Key         string
}

// PrintSection prints out the section name, description and each of the flags
func (u *UsageSection) PrintSection(max_length int, extended bool) {
	// Do not print if extended usage not requested and section marked as hidden
	if !extended && u.Hidden {
		return
	}
	fmt.Printf("%s:\n", u.Name)
	for _, f := range u.Flags {
		f.PrintFlag(max_length)
	}
	fmt.Printf("\n")
}

type UsageFlag struct {
	Name        string
	Description string
	Default     string
}

// PrintFlag prints out the flag name, usage string and default value
func (f *UsageFlag) PrintFlag(max_length int) {
	// Create format string, used for padding
	format := fmt.Sprintf("  -%%-%ds %%s", max_length)
	if f.Default != "" {
		format = format + " (default: %s)\n"
		fmt.Printf(format, f.Name, f.Description, f.Default)
	} else {
		format = format + "\n"
		fmt.Printf(format, f.Name, f.Description)
	}
}

func Usage() {
	// Section order and metadata; membership comes from the flag registry.
	sections := []UsageSection{
		{Name: "HTTP OPTIONS", Description: "Options controlling the HTTP request and its parts.", Flags: make([]UsageFlag, 0), Hidden: false, Key: ffuf.SectionHTTP},
		{Name: "GENERAL OPTIONS", Description: "", Flags: make([]UsageFlag, 0), Hidden: false, Key: ffuf.SectionGeneral},
		{Name: "COMPATIBILITY OPTIONS", Description: "Options to ensure compatibility with other pieces of software.", Flags: make([]UsageFlag, 0), Hidden: true, Key: ffuf.SectionCompat},
		{Name: "MATCHER OPTIONS", Description: "Matchers for the response filtering.", Flags: make([]UsageFlag, 0), Hidden: false, Key: ffuf.SectionMatcher},
		{Name: "FILTER OPTIONS", Description: "Filters for the response filtering.", Flags: make([]UsageFlag, 0), Hidden: false, Key: ffuf.SectionFilter},
		{Name: "INPUT OPTIONS", Description: "Options for input data for fuzzing. Wordlists and input generators.", Flags: make([]UsageFlag, 0), Hidden: false, Key: ffuf.SectionInput},
		{Name: "OUTPUT OPTIONS", Description: "Options for output. Output file formats, file names and debug file locations.", Flags: make([]UsageFlag, 0), Hidden: false, Key: ffuf.SectionOutput},
	}
	byKey := make(map[string]int, len(sections))
	for i, s := range sections {
		byKey[s.Key] = i
	}

	// Populate the flag sections
	max_length := 0
	flag.VisitAll(func(f *flag.Flag) {
		section, ok := flagRegistry.SectionOf(f.Name)
		if !ok {
			fmt.Printf("DEBUG: Flag %s was found but not defined in the flag registry.\n", f.Name)
			os.Exit(1)
		}
		i := byKey[section]
		sections[i].Flags = append(sections[i].Flags, UsageFlag{
			Name:        f.Name,
			Description: f.Usage,
			Default:     f.DefValue,
		})
		if len(f.Name) > max_length {
			max_length = len(f.Name)
		}
	})

	fmt.Printf("Fuzz Faster U Fool - %s\n\n", ffuf.FormattedVersion())

	// Print out the sections
	for _, section := range sections {
		section.PrintSection(max_length, false)
	}

	// Usage examples.
	fmt.Printf("EXAMPLE USAGE:\n")

	fmt.Printf("  Fuzz file paths from wordlist.txt, match all responses but filter out those with content-size 42.\n")
	fmt.Printf("  Colored, verbose output.\n")
	fmt.Printf("    ffuf -w wordlist.txt -u https://example.org/FUZZ -mc all -fs 42 -c -v\n\n")

	fmt.Printf("  Fuzz Host-header, match HTTP 200 responses.\n")
	fmt.Printf("    ffuf -w hosts.txt -u https://example.org/ -H \"Host: FUZZ\" -mc 200\n\n")

	fmt.Printf("  Fuzz POST JSON data. Match all responses not containing text \"error\".\n")
	fmt.Printf("    ffuf -w entries.txt -u https://example.org/ -X POST -H \"Content-Type: application/json\" \\\n")
	fmt.Printf("      -d '{\"name\": \"FUZZ\", \"anotherkey\": \"anothervalue\"}' -fr \"error\"\n\n")

	fmt.Printf("  Fuzz multiple locations. Match only responses reflecting the value of \"VAL\" keyword. Colored.\n")
	fmt.Printf("    ffuf -w params.txt:PARAM -w values.txt:VAL -u https://example.org/?PARAM=VAL -mr \"VAL\" -c\n\n")

	fmt.Printf("  More information and examples: https://github.com/ffuf/ffuf\n\n")
}
