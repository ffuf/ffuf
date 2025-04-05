package ffuf

import (
	"reflect"
	"testing"
)

func TestBaseRequest(t *testing.T) {
	headers := make(map[string]string)
	headers["foo"] = "bar"
	headers["baz"] = "wibble"
	headers["Content-Type"] = "application/json"

	data := "{\"quote\":\"I'll still be here tomorrow to high five you yesterday, my friend. Peace.\"}"

	expectedreq := Request{Method: "POST", Url: "http://example.com/aaaa", Headers: headers, Data: []byte(data)}
	config := Config{Method: "POST", Url: "http://example.com/aaaa", Headers: headers, Data: data}
	basereq := BaseRequest(&config)

	if !reflect.DeepEqual(basereq, expectedreq) {
		t.Errorf("BaseRequest does not return a struct with expected values")
	}

}

func TestCopyRequest(t *testing.T) {
	headers := make(map[string]string)
	headers["foo"] = "bar"
	headers["omg"] = "bbq"

	data := "line=Is+that+where+creativity+comes+from?+From+sad+biz?"

	input := make(map[string][]byte)
	input["matthew"] = []byte("If you are the head that floats atop the §ziggurat§, then the stairs that lead to you must be infinite.")

	basereq := Request{Method: "POST",
		Host:     "testhost.local",
		Url:      "http://example.com/aaaa",
		Headers:  headers,
		Data:     []byte(data),
		Input:    input,
		Position: 2,
		Raw:      "We're not oil and water, we're oil and vinegar! It's good. It's yummy.",
	}

	copiedreq := CopyRequest(&basereq)

	if !reflect.DeepEqual(basereq, copiedreq) {
		t.Errorf("CopyRequest does not return an equal struct")
	}
}

func TestSniperRequests(t *testing.T) {
	headers := make(map[string]string)
	headers["foo"] = "§bar§"
	headers["§omg§"] = "bbq"

	testreq := Request{
		Method:  "§POST§",
		Url:     "http://example.com/aaaa?param=§lemony§",
		Headers: headers,
		Data:    []byte("line=§yo yo, it's grease§"),
	}

	requests := SniperRequests(&testreq, "§")

	if len(requests) != 5 {
		t.Errorf("SniperRequests returned an incorrect number of requests")
	}

	headers = make(map[string]string)
	headers["foo"] = "bar"
	headers["omg"] = "bbq"

	var expected Request
	expected = Request{ // Method
		Method:  "FUZZ",
		Url:     "http://example.com/aaaa?param=lemony",
		Headers: headers,
		Data:    []byte("line=yo yo, it's grease"),
	}

	pass := false
	for _, req := range requests {
		if reflect.DeepEqual(req, expected) {
			pass = true
		}
	}

	if !pass {
		t.Errorf("SniperRequests does not return expected values (Method)")
	}

	expected = Request{ // URL
		Method:  "POST",
		Url:     "http://example.com/aaaa?param=FUZZ",
		Headers: headers,
		Data:    []byte("line=yo yo, it's grease"),
	}

	pass = false
	for _, req := range requests {
		if reflect.DeepEqual(req, expected) {
			pass = true
		}
	}

	if !pass {
		t.Errorf("SniperRequests does not return expected values (Url)")
	}

	expected = Request{ // Data
		Method:  "POST",
		Url:     "http://example.com/aaaa?param=lemony",
		Headers: headers,
		Data:    []byte("line=FUZZ"),
	}

	pass = false
	for _, req := range requests {
		if reflect.DeepEqual(req, expected) {
			pass = true
		}
	}

	if !pass {
		t.Errorf("SniperRequests does not return expected values (Data)")
	}

	headers = make(map[string]string)
	headers["foo"] = "FUZZ"
	headers["omg"] = "bbq"

	expected = Request{ // Header value
		Method:  "POST",
		Url:     "http://example.com/aaaa?param=lemony",
		Headers: headers,
		Data:    []byte("line=yo yo, it's grease"),
	}

	pass = false
	for _, req := range requests {
		if reflect.DeepEqual(req, expected) {
			pass = true
		}
	}

	if !pass {
		t.Errorf("SniperRequests does not return expected values (Header value)")
	}

	headers = make(map[string]string)
	headers["foo"] = "bar"
	headers["FUZZ"] = "bbq"

	expected = Request{ // Header key
		Method:  "POST",
		Url:     "http://example.com/aaaa?param=lemony",
		Headers: headers,
		Data:    []byte("line=yo yo, it's grease"),
	}

	pass = false
	for _, req := range requests {
		if reflect.DeepEqual(req, expected) {
			pass = true
		}
	}

	if !pass {
		t.Errorf("SniperRequests does not return expected values (Header key)")
	}

}

func TestTemplateLocations(t *testing.T) {
	test := "this is my 1§template locator§ test"
	arr := templateLocations("§", test)
	expected := []int{12, 29}
	if !reflect.DeepEqual(arr, expected) {
		t.Errorf("templateLocations does not return expected values")
	}

	test2 := "§template locator§"
	arr = templateLocations("§", test2)
	expected = []int{0, 17}
	if !reflect.DeepEqual(arr, expected) {
		t.Errorf("templateLocations does not return expected values")
	}

	if len(templateLocations("§", "te§st2")) != 1 {
		t.Errorf("templateLocations does not return expected values")
	}
}

func TestInjectKeyword(t *testing.T) {
	input := "§Greetings, creator§"
	offsetTuple := templateLocations("§", input)
	expected := "FUZZ"

	result := injectKeyword(input, "FUZZ", offsetTuple[0], offsetTuple[1])
	if result != expected {
		t.Errorf("injectKeyword returned unexpected result: " + result)
	}

	if injectKeyword(input, "FUZZ", -32, 44) != input {
		t.Errorf("injectKeyword offset validation failed")
	}

	if injectKeyword(input, "FUZZ", 12, 2) != input {
		t.Errorf("injectKeyword offset validation failed")
	}

	if injectKeyword(input, "FUZZ", 0, 25) != input {
		t.Errorf("injectKeyword offset validation failed")
	}

	input = "id=§a§&sort=desc"
	offsetTuple = templateLocations("§", input)
	expected = "id=FUZZ&sort=desc"

	result = injectKeyword(input, "FUZZ", offsetTuple[0], offsetTuple[1])
	if result != expected {
		t.Errorf("injectKeyword returned unexpected result: " + result)
	}

	input = "feature=aaa&thingie=bbb&array[§0§]=baz"
	offsetTuple = templateLocations("§", input)
	expected = "feature=aaa&thingie=bbb&array[FUZZ]=baz"

	result = injectKeyword(input, "FUZZ", offsetTuple[0], offsetTuple[1])
	if result != expected {
		t.Errorf("injectKeyword returned unexpected result: " + result)
	}
}

func TestScrubTemplates(t *testing.T) {
	headers := make(map[string]string)
	headers["foo"] = "§bar§"
	headers["§omg§"] = "bbq"

	testreq := Request{Method: "§POST§",
		Url:     "http://example.com/aaaa?param=§lemony§",
		Headers: headers,
		Data:    []byte("line=§yo yo, it's grease§"),
	}

	headers = make(map[string]string)
	headers["foo"] = "bar"
	headers["omg"] = "bbq"

	expectedreq := Request{Method: "POST",
		Url:     "http://example.com/aaaa?param=lemony",
		Headers: headers,
		Data:    []byte("line=yo yo, it's grease"),
	}

	scrubTemplates(&testreq, "§")

	if !reflect.DeepEqual(testreq, expectedreq) {
		t.Errorf("scrubTemplates does not return expected values")
	}
}
