package config

import (
	"fmt"
	"testing"
)

func TestTemplatePresent(t *testing.T) {
	template := "§"

	headers := []string{"foo: §bar§", "omg: bbq", "§world§: Ooo"}

	goodOpts := ConfigOptions{
		HTTP: HTTPOptions{
			URL:     "https://example.com/fooo/bar?test=§value§&order[§0§]=§foo§",
			Method:  "PO§ST§",
			Headers: headers,
			Data:    "line=Can we pull back the §veil§ of §static§ and reach in to the source of §all§ being?&commit=true",
		},
	}

	if !templatePresent(template, &goodOpts) {
		t.Errorf("Expected-good config failed validation")
	}

	badConfMethod := ConfigOptions{
		HTTP: HTTPOptions{
			URL:     "https://example.com/fooo/bar?test=§value§&order[§0§]=§foo§",
			Method:  "POST§",
			Headers: headers,
			Data:    "line=Can we pull back the §veil§ of §static§ and reach in to the source of §all§ being?&commit=§true§",
		},
	}

	if templatePresent(template, &badConfMethod) {
		t.Errorf("Expected-bad config (Method) failed validation")
	}

	badConfURL := ConfigOptions{
		HTTP: HTTPOptions{
			URL:     "https://example.com/fooo/bar?test=§value§&order[0§]=§foo§",
			Method:  "§POST§",
			Headers: headers,
			Data:    "line=Can we pull back the §veil§ of §static§ and reach in to the source of §all§ being?&commit=§true§",
		},
	}

	if templatePresent(template, &badConfURL) {
		t.Errorf("Expected-bad config (URL) failed validation")
	}

	badConfData := ConfigOptions{
		HTTP: HTTPOptions{
			URL:     "https://example.com/fooo/bar?test=§value§&order[§0§]=§foo§",
			Method:  "§POST§",
			Headers: headers,
			Data:    "line=Can we pull back the §veil of §static§ and reach in to the source of §all§ being?&commit=§true§",
		},
	}

	if templatePresent(template, &badConfData) {
		t.Errorf("Expected-bad config (Data) failed validation")
	}

	headers_0 := append(headers, "kingdom: §candy")

	badConfHeaderValue := ConfigOptions{
		HTTP: HTTPOptions{
			URL:     "https://example.com/fooo/bar?test=§value§&order[§0§]=§foo§",
			Method:  "PO§ST§",
			Headers: headers_0,
			Data:    "line=Can we pull back the §veil§ of §static§ and reach in to the source of §all§ being?&commit=true",
		},
	}

	if templatePresent(template, &badConfHeaderValue) {
		t.Errorf("Expected-bad config (Header value) failed validation")
	}

	headers_1 := append(headers, "kingdom: candy", "§kingdom: candy")

	badConfHeaderKey := ConfigOptions{
		HTTP: HTTPOptions{
			URL:     "https://example.com/fooo/bar?test=§value§&order[§0§]=§foo§",
			Method:  "PO§ST§",
			Headers: headers_1,
			Data:    "line=Can we pull back the §veil§ of §static§ and reach in to the source of §all§ being?&commit=true",
		},
	}

	if templatePresent(template, &badConfHeaderKey) {
		t.Errorf("Expected-bad config (Header key) failed validation")
	}
}

//////////////
// EXAMPLES //
//////////////

func ExampleCreateOwnTranslator() {

	// Done as function expression because it's in an Example function. See
	// source code from translateFilterMatcherModes for a more accurate example.

	// Takes as arguments references to *ConfigOptions and *Config, chooses one
	// aspect of the options, validates that they are sane and writes them into
	// the Config struct. If the options are bad, the translator returns an error
	// which will be collected into a list of errors. A translator never
	// terminates the program.
	http_get_translator := func(opts *ConfigOptions, conf *Config) error {

		// check if options are sane
		if opts.HTTP.Method != "GET" {
			return fmt.Errorf("method must be GET for some reason!")
		}

		// translate into configuration
		conf.Method = opts.HTTP.Method

		return nil
	}

	// add the translator to the predefined translators
	AddTranslator(http_get_translator)
}
