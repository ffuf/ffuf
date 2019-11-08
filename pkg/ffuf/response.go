package ffuf

import (
	"net/http"
)

// Response struct holds the meaningful data returned from request and is meant for passing to filters
type Response struct {
	StatusCode    int64
	Headers       map[string][]string
	Data          []byte
	ContentLength int64
	ContentWords  int64
	ContentLines  int64
	Cancelled     bool
	Request       *Request
}

// GetRedirectLocation returns the redirect location for a 3xx redirect HTTP response
func (resp *Response) GetRedirectLocation() string {

	redirectLocation := ""
	if resp.StatusCode >= 300 && resp.StatusCode <= 399 {
		redirectLocation = resp.Headers["Location"][0]
	}

	return redirectLocation
}

func NewResponse(httpresp *http.Response, req *Request) Response {
	var resp Response
	resp.Request = req
	resp.StatusCode = int64(httpresp.StatusCode)
	resp.Headers = httpresp.Header
	resp.Cancelled = false
	return resp
}
