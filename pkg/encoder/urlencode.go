package encoder

import (
	"net/url"
	"strings"
)

// urlencoding algorithm below is a slightly modified version found in
// the standard library to allow for which chars to encode to be
// parameterised: https://golang.org/src/net/url/url.go
const upperhex = "0123456789ABCDEF"

func escape(s string, encodeChars string) string {
	spaceCount, hexCount := 0, 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.Contains(encodeChars, string(c)) {
			if c == ' ' {
				spaceCount++
			} else {
				hexCount++
			}
		}
	}

	if spaceCount == 0 && hexCount == 0 {
		return s
	}

	var buf [64]byte
	var t []byte

	required := len(s) + 2*hexCount
	if required <= len(buf) {
		t = buf[:required]
	} else {
		t = make([]byte, required)
	}

	if hexCount == 0 {
		copy(t, s)
		for i := 0; i < len(s); i++ {
			if s[i] == ' ' {
				t[i] = '+'
			}
		}
		return string(t)
	}

	j := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == ' ':
			t[j] = '+'
			j++
		case strings.Contains(encodeChars, string(c)):
			t[j] = '%'
			t[j+1] = upperhex[c>>4]
			t[j+2] = upperhex[c&15]
			j += 3
		default:
			t[j] = s[i]
			j++
		}
	}
	return string(t)
}

func urlencode(ep EncoderParameters, data []byte) ([]byte, error) {
	chars, ok := ep["urlenc_chars"]
	if ok {
		return []byte(escape(string(data), chars)), nil
	}
	return []byte(url.QueryEscape(string(data))), nil
}
