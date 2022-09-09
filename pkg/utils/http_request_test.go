package utils

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

var (
	rawHttpRequest = `GET http://www.example.com/FUZZ HTTP/1.1
Host: www.example.com
User-Agent: Mozilla/5.0 (X11; Linux x86_64; rv:105.0) Gecko/20100101 Firefox/105.0
Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8
Accept-Language: en-US,en;q=0.5
Referer: https://google.com
DNT: 1
Connection: keep-alive
Upgrade-Insecure-Requests: 1
`
)

func TestParseRawRequest(t *testing.T) {
	reader := io.NopCloser(strings.NewReader(rawHttpRequest))

	want, err := http.NewRequest("GET", "http://www.example.com/FUZZ", nil)
	if err != nil {
		t.Errorf("could not create empty http.Request: %v", err)
	}
	want.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:105.0) Gecko/20100101 Firefox/105.0")
	want.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	want.Header.Add("Accept-Language", "en-US,en;q=0.5")
	want.Header.Add("Referer", "https://google.com")
	want.Header.Add("DNT", "1")
	want.Header.Add("Connection", "keep-alive")
	want.Header.Add("Upgrade-Insecure-Requests", "1")

	got, err := ParseRawRequest(reader)
	if err != nil {
		t.Errorf("error during request parsing: %v", err)
	}

	if got.Method != want.Method {
		t.Errorf("headers differ. Got: %q, want: %q", got.Method, want.Method)
	}

	if got.URL.String() != want.URL.String() {
		t.Errorf("URLs differ. Got: %q, want: %q", got.URL, want.URL)
	}

	if got.Proto != want.Proto {
		t.Errorf("protocol versions differ. Got: %q, want: %q", got.Proto, want.Proto)
	}

	for key := range want.Header {
		want_val := want.Header.Get(key)
		got_val := got.Header.Get(key)
		if got_val != want_val {
			t.Errorf("%q header differs. Got: %q, want: %q", key, got_val, want_val)
		}
	}
}
