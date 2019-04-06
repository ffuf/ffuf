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
	Cancelled     bool
	Url           string
	Request       *Request
}

func NewResponse(httpresp *http.Response, req *Request) Response {
	var resp Response
	resp.Request = req
	resp.StatusCode = int64(httpresp.StatusCode)
	resp.Headers = httpresp.Header
	resp.Cancelled = false
	resp.Url = ""
	return resp
}
