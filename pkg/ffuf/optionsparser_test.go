package ffuf

import (
	"testing"
)

func TestTemplatePresent(t *testing.T) {
	template := "§"

	headers := make(map[string]string)
	headers["foo"] = "§bar§"
	headers["omg"] = "bbq"
	headers["§world§"] = "Ooo"

	goodConf := Config{
		Url:     "https://example.com/fooo/bar?test=§value§&order[§0§]=§foo§",
		Method:  "PO§ST§",
		Headers: headers,
		Data:    "line=Can we pull back the §veil§ of §static§ and reach in to the source of §all§ being?&commit=true",
	}

	if !templatePresent(template, &goodConf) {
		t.Errorf("Expected-good config failed validation")
	}

	badConfMethod := Config{
		Url:     "https://example.com/fooo/bar?test=§value§&order[§0§]=§foo§",
		Method:  "POST§",
		Headers: headers,
		Data:    "line=Can we pull back the §veil§ of §static§ and reach in to the source of §all§ being?&commit=§true§",
	}

	if templatePresent(template, &badConfMethod) {
		t.Errorf("Expected-bad config (Method) failed validation")
	}

	badConfURL := Config{
		Url:     "https://example.com/fooo/bar?test=§value§&order[0§]=§foo§",
		Method:  "§POST§",
		Headers: headers,
		Data:    "line=Can we pull back the §veil§ of §static§ and reach in to the source of §all§ being?&commit=§true§",
	}

	if templatePresent(template, &badConfURL) {
		t.Errorf("Expected-bad config (URL) failed validation")
	}

	badConfData := Config{
		Url:     "https://example.com/fooo/bar?test=§value§&order[§0§]=§foo§",
		Method:  "§POST§",
		Headers: headers,
		Data:    "line=Can we pull back the §veil of §static§ and reach in to the source of §all§ being?&commit=§true§",
	}

	if templatePresent(template, &badConfData) {
		t.Errorf("Expected-bad config (Data) failed validation")
	}

	headers["kingdom"] = "§candy"

	badConfHeaderValue := Config{
		Url:     "https://example.com/fooo/bar?test=§value§&order[§0§]=§foo§",
		Method:  "PO§ST§",
		Headers: headers,
		Data:    "line=Can we pull back the §veil§ of §static§ and reach in to the source of §all§ being?&commit=true",
	}

	if templatePresent(template, &badConfHeaderValue) {
		t.Errorf("Expected-bad config (Header value) failed validation")
	}

	headers["kingdom"] = "candy"
	headers["§kingdom"] = "candy"

	badConfHeaderKey := Config{
		Url:     "https://example.com/fooo/bar?test=§value§&order[§0§]=§foo§",
		Method:  "PO§ST§",
		Headers: headers,
		Data:    "line=Can we pull back the §veil§ of §static§ and reach in to the source of §all§ being?&commit=true",
	}

	if templatePresent(template, &badConfHeaderKey) {
		t.Errorf("Expected-bad config (Header key) failed validation")
	}
}
