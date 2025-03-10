package ffuf

import (
	"net/http"
	"net/url"
	"time"
)

// Response struct holds the meaningful data returned from request and is meant for passing to filters
type Response struct {
	StatusCode    int64
	Headers       map[string][]string
	Data          []byte
	ContentLength int64
	ContentWords  int64
	ContentLines  int64
	ContentType   string
	Cancelled     bool
	Request       *Request
	Raw           string
	ResultFile    string
	ScraperData   map[string][]string
	Duration      time.Duration
	Timestamp     time.Time
}

// ResponseStatistics is a reduced struct containing information used to calculate run statistics
// A copy of this struct will exist for every response in memory
type ResponseStatistics struct {
	StatusCode           int64
	ContentLength        int64
	Duration             time.Duration
	Timestamp            time.Time
	PayloadResponseDelta int64
	Input                map[string][]byte
}

// GetRedirectLocation returns the redirect location for a 3xx redirect HTTP response
func (resp *Response) GetRedirectLocation(absolute bool) string {

	redirectLocation := ""
	if resp.StatusCode >= 300 && resp.StatusCode <= 399 {
		if loc, ok := resp.Headers["Location"]; ok {
			if len(loc) > 0 {
				redirectLocation = loc[0]
			}
		}
	}

	if absolute {
		redirectUrl, err := url.Parse(redirectLocation)
		if err != nil {
			return redirectLocation
		}
		baseUrl, err := url.Parse(resp.Request.Url)
		if err != nil {
			return redirectLocation
		}
		if redirectUrl.IsAbs() && UrlEqual(redirectUrl, baseUrl) {
			redirectLocation = redirectUrl.Scheme + "://" +
				baseUrl.Host + redirectUrl.Path
		} else {
			redirectLocation = baseUrl.ResolveReference(redirectUrl).String()
		}
	}

	return redirectLocation
}

func UrlEqual(url1, url2 *url.URL) bool {
	if url1.Hostname() != url2.Hostname() {
		return false
	}
	if url1.Scheme != url2.Scheme {
		return false
	}
	p1, p2 := getUrlPort(url1), getUrlPort(url2)
	return p1 == p2
}

func getUrlPort(url *url.URL) string {
	var portMap = map[string]string{
		"http":  "80",
		"https": "443",
	}
	p := url.Port()
	if p == "" {
		p = portMap[url.Scheme]
	}
	return p
}

func NewResponse(httpresp *http.Response, req *Request) Response {
	var resp Response
	resp.Request = req
	resp.StatusCode = int64(httpresp.StatusCode)
	resp.ContentType = httpresp.Header.Get("Content-Type")
	resp.Headers = httpresp.Header
	resp.Cancelled = false
	resp.Raw = ""
	resp.ResultFile = ""
	resp.ScraperData = make(map[string][]string)
	return resp
}

func GetResponseStats(resp *Response) ResponseStatistics {
	var respStats ResponseStatistics
	respStats.StatusCode = resp.StatusCode
	respStats.Duration = resp.Duration
	respStats.ContentLength = resp.ContentLength
	respStats.Input = resp.Request.Input
	respStats.Timestamp = resp.Timestamp

	inputs := make(map[string][]byte, len(resp.Request.Input))
	inputLength := 0

	for k, v := range resp.Request.Input {
		inputs[k] = v

		// calculate the sum of all input payloads, excluding the FFUFHASH input
		if k != "FFUFHASH" {
			inputLength = inputLength + len(v)
		}
	}
	respStats.PayloadResponseDelta = resp.ContentLength - int64(inputLength)

	return respStats
}
