package utils

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ParseRawRequest parses it's reader parameter into an http.Request struct from net/http
// and returns it. Req contains the io.ReadCloser in the req.Body field, which might
// be an open file. It is up to the caller to close it via req.Body.Close().
func ParseRawRequest(r io.ReadCloser) (req *http.Request, err error) {
	// Parsing is done via this routine instead of http.ReadRequest from net/http,
	// because the latter produces a standards-conforming request, which might be
	// undesirable while fuzzig.

	var reader *bufio.Reader = bufio.NewReader(r)

	s, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("could not read request: %s", err)
	}
	parts := strings.Split(s, " ")
	if len(parts) < 3 {
		return nil, fmt.Errorf("malformed request supplied")
	}
	// Set the request Method
	req.Method = parts[0]

	var ok bool
	req.ProtoMajor, req.ProtoMinor, ok = http.ParseHTTPVersion(parts[3])
	if !ok {
		return nil, fmt.Errorf("malformed http protocol version")
	}

	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if err != nil || line == "" {
			break
		}

		p := strings.SplitN(line, ":", 2)
		if len(p) != 2 {
			continue
		}

		if strings.EqualFold(p[0], "content-length") {
			continue
		}

		req.Header.Add(strings.TrimSpace(p[0]), strings.TrimSpace(p[1]))
	}

	req.URL, err = url.Parse(parts[1])
	if err != nil {
		return nil, fmt.Errorf("could not parse request URL: %s", err)
	}

	// Set the request body: The ReadCloser parameter r was read up to the body
	// of the HTTP request.
	req.Body = r

	return req, nil
}
