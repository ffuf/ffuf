package utils

import (
	"reflect"
	"testing"
)

// TestTokenizeArgsCorrect tests valid inputs to TokenizeArgs.
func TestTokenizeArgsCorrect(t *testing.T) {

	tests := []struct {
		input string
		want  []string
	}{
		{"hello", []string{"hello"}},
		{"hello  ", []string{"hello"}},
		{"  hello   ", []string{"hello"}},
		{` "hello" `, []string{`hello`}},
		{`'hello' `, []string{`hello`}},
		{`"he'll'o"`, []string{`he'll'o`}},
		{`'he"ll"o'`, []string{`he"ll"o`}},
		{`he\"llo`, []string{`he"llo`}},
		// test line break
		{`hel
lo`, []string{`hel`, `lo`}},
		{"hel\r\nlo", []string{`hel`, `lo`}}, // carriage return an line feed
		{
			`-w    "/path/to/vhost/wor'dlist" -u  https://target -H   'Host: FUZZ' -fs 4242`,
			[]string{"-w", "/path/to/vhost/wor'dlist", "-u", "https://target", "-H", "Host: FUZZ", "-fs", "4242"},
		},
	}

	for i, test := range tests {
		result, err := TokenizeArgs(test.input)
		if err != nil {
			t.Errorf("[%d] error tokenizing args: %v", i, err)
		}

		v_res := reflect.ValueOf(result).Interface()
		v_want := reflect.ValueOf(test.want).Interface()

		if !reflect.DeepEqual(v_res, v_want) {
			t.Errorf(
				"[%d] tokens differ for %q: want %q, got %q",
				i,
				test.input,
				test.want,
				result)
		}
	}
}

// TestTokenizeArgsCorrect tests malformed inputs to TokenizeArgs like unbalanced
// quotes or an escape character at the end.
func TestTokenizeArgsMalformed(t *testing.T) {

	tests := []string{
		`hello\`,
		`hello'`,
		`hello"`,
		`'hello`,
		`"hello`,
		`'he"llo`,
		`"hell'o`,
		`-w    /path/to/vhost/wor'dlist -u  https://target -H   'Host: FUZZ' -fs 4242`,
	}

	for i, test := range tests {
		_, err := TokenizeArgs(test)
		if err == nil {
			t.Errorf("[%d] TokenizeArgs didn't detect malformed string: %q", i, test)
		}
	}
}
