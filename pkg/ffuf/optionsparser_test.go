package ffuf

import (
	"strings"
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

func TestProxyParsing(t *testing.T) {
	configOptions := NewConfigOptions()
	errorString := "Bad proxy url (-x) format. Expected http, https or socks5 url"

	// http should work
	configOptions.HTTP.ProxyURL = "http://127.0.0.1:8080"
	_, err := ConfigFromOptions(configOptions, nil, nil)
	if strings.Contains(err.Error(), errorString) {
		t.Errorf("Expected http proxy string to work")
	}

	// https should work
	configOptions.HTTP.ProxyURL = "https://127.0.0.1"
	_, err = ConfigFromOptions(configOptions, nil, nil)
	if strings.Contains(err.Error(), errorString) {
		t.Errorf("Expected https proxy string to work")
	}

	// socks5 should work
	configOptions.HTTP.ProxyURL = "socks5://127.0.0.1"
	_, err = ConfigFromOptions(configOptions, nil, nil)
	if strings.Contains(err.Error(), errorString) {
		t.Errorf("Expected socks5 proxy string to work")
	}

	// garbage data should FAIL
	configOptions.HTTP.ProxyURL = "Y0 y0 it's GREASE"
	_, err = ConfigFromOptions(configOptions, nil, nil)
	if !strings.Contains(err.Error(), errorString) {
		t.Errorf("Expected garbage proxy string to fail")
	}

	// Opaque URLs with the right scheme should FAIL
	configOptions.HTTP.ProxyURL = "http:sixhours@dungeon"
	_, err = ConfigFromOptions(configOptions, nil, nil)
	if !strings.Contains(err.Error(), errorString) {
		t.Errorf("Expected opaque proxy string to fail")
	}

	// Unsupported protocols should FAIL
	configOptions.HTTP.ProxyURL = "imap://127.0.0.1"
	_, err = ConfigFromOptions(configOptions, nil, nil)
	if !strings.Contains(err.Error(), errorString) {
		t.Errorf("Expected proxy string with unsupported protocol to fail")
	}
}

func TestReplayProxyParsing(t *testing.T) {
	configOptions := NewConfigOptions()
	errorString := "Bad replay-proxy url (-replay-proxy) format. Expected http, https or socks5 url"

	// http should work
	configOptions.HTTP.ReplayProxyURL = "http://127.0.0.1:8080"
	_, err := ConfigFromOptions(configOptions, nil, nil)
	if strings.Contains(err.Error(), errorString) {
		t.Errorf("Expected http replay proxy string to work")
	}

	// https should work
	configOptions.HTTP.ReplayProxyURL = "https://127.0.0.1"
	_, err = ConfigFromOptions(configOptions, nil, nil)
	if strings.Contains(err.Error(), errorString) {
		t.Errorf("Expected https proxy string to work")
	}

	// socks5 should work
	configOptions.HTTP.ReplayProxyURL = "socks5://127.0.0.1"
	_, err = ConfigFromOptions(configOptions, nil, nil)
	if strings.Contains(err.Error(), errorString) {
		t.Errorf("Expected socks5 proxy string to work")
	}

	// garbage data should FAIL
	configOptions.HTTP.ReplayProxyURL = "Y0 y0 it's GREASE"
	_, err = ConfigFromOptions(configOptions, nil, nil)
	if !strings.Contains(err.Error(), errorString) {
		t.Errorf("Expected garbage proxy string to fail")
	}

	// Opaque URLs with the right scheme should FAIL
	configOptions.HTTP.ReplayProxyURL = "http:sixhours@dungeon"
	_, err = ConfigFromOptions(configOptions, nil, nil)
	if !strings.Contains(err.Error(), errorString) {
		t.Errorf("Expected opaque proxy string to fail")
	}

	// Unsupported protocols should FAIL
	configOptions.HTTP.ReplayProxyURL = "imap://127.0.0.1"
	_, err = ConfigFromOptions(configOptions, nil, nil)
	if !strings.Contains(err.Error(), errorString) {
		t.Errorf("Expected proxy string with unsupported protocol to fail")
	}
}
