package filter

import (
	"strings"
	"testing"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

func TestNewDynamicSizeFilter(t *testing.T) {
	f, err := NewDynamicSizeFilter("173:FUZZ")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if !strings.Contains(f.Repr(), "173:FUZZ") {
		t.Errorf("Expected repr to contain \"173:FUZZ\", got %q", f.Repr())
	}
}

func TestNewDynamicSizeFilterMultipleEntries(t *testing.T) {
	f, err := NewDynamicSizeFilter("173:FUZZ,42:FUZZ2")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	df, ok := f.(*DynamicSizeFilter)
	if !ok {
		t.Fatalf("Expected *DynamicSizeFilter, got %T", f)
	}
	if len(df.Entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(df.Entries))
	}
}

func TestNewDynamicSizeFilterErrors(t *testing.T) {
	for _, value := range []string{
		"",
		"notanumber:FUZZ",
		"173",                  // missing keyword
		"173:",                 // empty keyword
		"173:FUZZ:extra:stuff", // still valid: SplitN(2) keeps "FUZZ:extra:stuff" as keyword
	} {
		_, err := NewDynamicSizeFilter(value)
		if value == "173:FUZZ:extra:stuff" {
			if err != nil {
				t.Errorf("Value %q: expected no error (extra colons belong to the keyword), got %s", value, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("Value %q: expected an error, got none", value)
		}
	}
}

func TestDynamicSizeFilterMatches(t *testing.T) {
	f, _ := NewDynamicSizeFilter("100:FUZZ")

	for i, test := range []struct {
		fuzzValue     string
		contentLength int64
		expected      bool
	}{
		{"admin", 105, true},         // 100 + len("admin") == 105
		{"administrator", 113, true}, // 100 + len("administrator") == 113
		{"admin", 999, false},        // wrong length for this fuzz value
		{"", 100, true},              // empty fuzz value still honours the offset
	} {
		resp := &ffuf.Response{
			ContentLength: test.contentLength,
			Request: &ffuf.Request{
				Input: map[string][]byte{"FUZZ": []byte(test.fuzzValue)},
			},
		}
		got, err := f.Filter(resp)
		if err != nil {
			t.Errorf("Test %d: unexpected error: %s", i, err)
		}
		if got != test.expected {
			t.Errorf("Test %d: expected %t, got %t", i, test.expected, got)
		}
	}
}

func TestDynamicSizeFilterNoRequest(t *testing.T) {
	f, _ := NewDynamicSizeFilter("100:FUZZ")
	resp := &ffuf.Response{ContentLength: 105}
	got, err := f.Filter(resp)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if got {
		t.Errorf("Expected no match when response has no associated Request")
	}
}

func TestDynamicSizeFilterUnknownKeyword(t *testing.T) {
	f, _ := NewDynamicSizeFilter("100:FUZZ")
	resp := &ffuf.Response{
		ContentLength: 105,
		Request: &ffuf.Request{
			Input: map[string][]byte{"OTHER": []byte("admin")},
		},
	}
	got, err := f.Filter(resp)
	if err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
	if got {
		t.Errorf("Expected no match when the entry's keyword is absent from the request input")
	}
}
