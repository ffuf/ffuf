package ffuf

import (
	"math/rand"
	"testing"
)

func TestRandomString(t *testing.T) {
	length := 1 + rand.Intn(65535)
	str := RandomString(length)

	if len(str) != length {
		t.Errorf("Length of generated string was %d, was expecting %d", len(str), length)
	}
}

func TestUniqStringSlice(t *testing.T) {
	slice := []string{"foo", "foo", "bar", "baz", "baz", "foo", "baz", "baz", "foo"}
	expectedLength := 3

	uniqSlice := UniqStringSlice(slice)

	if len(uniqSlice) != expectedLength {
		t.Errorf("Length of slice was %d, was expecting %d", len(uniqSlice), expectedLength)
	}
}

func TestKeyword(t *testing.T) {
	testreq := Request{Method: "FUZZ",
		Url:  "http://example.com/aaaa?param=lemony",
		Data: []byte("line=mnrrrrr"),
	}

	if !RequestContainsKeyword(testreq, "FUZZ") {
		t.Errorf("FUZZ keyword in method not found")
	}

	testreq = Request{Method: "POST",
		Url:  "http://example.com/FUZZ?param=lemony",
		Data: []byte("line=mnrrrrr"),
	}

	if !RequestContainsKeyword(testreq, "FUZZ") {
		t.Errorf("FUZZ keyword in URL not found")
	}

	testreq = Request{Method: "POST",
		Url:  "http://example.com/aaaa?param=lemony",
		Host: "FUZZ.com",
		Data: []byte("line=mnrrrrr"),
	}

	if !RequestContainsKeyword(testreq, "FUZZ") {
		t.Errorf("FUZZ keyword in Host not found")
	}

	testreq = Request{Method: "POST",
		Url:  "http://example.com/aaaa?param=lemony",
		Data: []byte("line=FUZZ"),
	}

	if !RequestContainsKeyword(testreq, "FUZZ") {
		t.Errorf("FUZZ keyword in data not found")
	}

	testreq = Request{Method: "POST",
		Url: "http://example.com/aaaa?param=lemony",
		Headers: map[string][]string{
			"FUZZ": {"bar"},
		},
		Data: []byte("line=wehhh"),
	}

	if !RequestContainsKeyword(testreq, "FUZZ") {
		t.Errorf("FUZZ keyword in header key not found")
	}

	testreq = Request{Method: "POST",
		Url: "http://example.com/aaaa?param=lemony",
		Headers: map[string][]string{
			"bar": {"FUZZ"},
		},
		Data: []byte("line=To live life you need problems."),
	}

	if !RequestContainsKeyword(testreq, "FUZZ") {
		t.Errorf("FUZZ keyword in header value not found")
	}

	testreq = Request{Method: "POST",
		Url: "http://example.com/aaaa?param=lemony",
		Headers: map[string][]string{
			"bar": {"foo"},
		},
		Data: []byte("line=To live life you need problems."),
	}

	if RequestContainsKeyword(testreq, "FUZZ") {
		t.Errorf("FUZZ keyword found when not expected")
	}
}
