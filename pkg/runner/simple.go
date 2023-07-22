package runner

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
	"math/rand"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

var userAgents = []string{
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36 Edg/114.0.1823.86",
    "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/81.0.4044.92 Safari/537.36",
    "Mozilla/5.0 (X11; CrOS armv7l 13597.84.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.5112.105 Safari/537.36",
    "Mozilla/5.0 (Windows NT 6.1; rv:109.0) Gecko/20100101 Firefox/115.0",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/113.0.0.0 Safari/537.36 OPR/99.0.0.0",
    "Mozilla/5.0 (Windows NT 6.3; Win64; x64; rv:82.0) Gecko/20100101 Firefox/82.0",
    "Mozilla/5.0 (iPhone; CPU iPhone OS 16_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.5 Mobile/15E148 Safari/604.1",
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/110.0",
    "Mozilla/5.0 (iPhone; CPU iPhone OS 16_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.5.2 Mobile/15E148 Safari/604.1",
    "Mozilla/5.0 (Linux; Android 8.1.0; SAMSUNG SM-J530F) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/22.0 Chrome/111.0.5563.116 Mobile Safari/537.36",
}


// Download results < 5MB
const MAX_DOWNLOAD_SIZE = 5242880

type SimpleRunner struct {
	config *ffuf.Config
	client *http.Client
}

func NewSimpleRunner(conf *ffuf.Config, replay bool) ffuf.RunnerProvider {
	var simplerunner SimpleRunner
	proxyURL := http.ProxyFromEnvironment
	customProxy := ""

	if replay {
		customProxy = conf.ReplayProxyURL
	} else {
		customProxy = conf.ProxyURL
	}
	if len(customProxy) > 0 {
		pu, err := url.Parse(customProxy)
		if err == nil {
			proxyURL = http.ProxyURL(pu)
		}
	}
	simplerunner.config = conf
	simplerunner.client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       time.Duration(time.Duration(conf.Timeout) * time.Second),
		Transport: &http.Transport{
			ForceAttemptHTTP2:   conf.Http2,
			Proxy:               proxyURL,
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: 500,
			MaxConnsPerHost:     500,
			DialContext: (&net.Dialer{
				Timeout: time.Duration(time.Duration(conf.Timeout) * time.Second),
			}).DialContext,
			TLSHandshakeTimeout: time.Duration(time.Duration(conf.Timeout) * time.Second),
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS10,
				Renegotiation:      tls.RenegotiateOnceAsClient,
				ServerName:         conf.SNI,
			},
		}}

	if conf.FollowRedirects {
		simplerunner.client.CheckRedirect = nil
	}
	return &simplerunner
}

func (r *SimpleRunner) Prepare(input map[string][]byte, basereq *ffuf.Request) (ffuf.Request, error) {
	req := ffuf.CopyRequest(basereq)

	for keyword, inputitem := range input {
		req.Method = strings.ReplaceAll(req.Method, keyword, string(inputitem))
		headers := make(map[string]string, len(req.Headers))
		for h, v := range req.Headers {
			var CanonicalHeader string = textproto.CanonicalMIMEHeaderKey(strings.ReplaceAll(h, keyword, string(inputitem)))
			headers[CanonicalHeader] = strings.ReplaceAll(v, keyword, string(inputitem))
		}
		req.Headers = headers
		req.Url = strings.ReplaceAll(req.Url, keyword, string(inputitem))
		req.Data = []byte(strings.ReplaceAll(string(req.Data), keyword, string(inputitem)))
	}

	req.Input = input
	return req, nil
}

func (r *SimpleRunner) Execute(req *ffuf.Request) (ffuf.Response, error) {
	var httpreq *http.Request
	var err error
	var rawreq []byte
	data := bytes.NewReader(req.Data)

	var start time.Time
	var firstByteTime time.Duration

	trace := &httptrace.ClientTrace{
		WroteRequest: func(wri httptrace.WroteRequestInfo) {
			start = time.Now() // begin the timer after the request is fully written
		},
		GotFirstResponseByte: func() {
			firstByteTime = time.Since(start) // record when the first byte of the response was received
		},
	}

	httpreq, err = http.NewRequestWithContext(r.config.Context, req.Method, req.Url, data)


	if err != nil {
		return ffuf.Response{}, err
	}

	if r.config.RandomAgent {
		rand.Seed(time.Now().UnixNano()) 
		randIndex := rand.Intn(len(userAgents))
		httpreq.Header.Set("User-Agent", userAgents[randIndex]) // Set the agent header
		fmt.Printf("Debug: User-Agent is %s\n", httpreq.Header.Get("User-Agent")) // Debug print
	} else if _, ok := req.Headers["User-Agent"]; !ok {
		// If 'random-agent' is not enabled, set the default User-Agent header if not present
		req.Headers["User-Agent"] = fmt.Sprintf("%s v%s", "Fuzz Faster U Fool", ffuf.Version())
	}

	// Handle Go http.Request special cases
	if _, ok := req.Headers["Host"]; ok {
		httpreq.Host = req.Headers["Host"]
	}

	req.Host = httpreq.Host
	httpreq = httpreq.WithContext(httptrace.WithClientTrace(r.config.Context, trace))
	for k, v := range req.Headers {
		httpreq.Header.Set(k, v)
	}

	if len(r.config.OutputDirectory) > 0 {
		rawreq, _ = httputil.DumpRequestOut(httpreq, true)
	}
	httpresp, err := r.client.Do(httpreq)
	if err != nil {
		return ffuf.Response{}, err
	}

	resp := ffuf.NewResponse(httpresp, req)
	defer httpresp.Body.Close()

	// Check if we should download the resource or not
	size, err := strconv.Atoi(httpresp.Header.Get("Content-Length"))
	if err == nil {
		resp.ContentLength = int64(size)
		if (r.config.IgnoreBody) || (size > MAX_DOWNLOAD_SIZE) {
			resp.Cancelled = true
			return resp, nil
		}
	}

	if len(r.config.OutputDirectory) > 0 {
		rawresp, _ := httputil.DumpResponse(httpresp, true)
		resp.Request.Raw = string(rawreq)
		resp.Raw = string(rawresp)
	}
	var bodyReader io.ReadCloser
	if httpresp.Header.Get("Content-Encoding") == "gzip" {
		bodyReader, err = gzip.NewReader(httpresp.Body)
		if err != nil {
			// fallback to raw data
			bodyReader = httpresp.Body
		}
	} else {
		bodyReader = httpresp.Body
	}

	if respbody, err := io.ReadAll(bodyReader); err == nil {
		resp.ContentLength = int64(len(string(respbody)))
		resp.Data = respbody
	}

	wordsSize := len(strings.Split(string(resp.Data), " "))
	linesSize := len(strings.Split(string(resp.Data), "\n"))
	resp.ContentWords = int64(wordsSize)
	resp.ContentLines = int64(linesSize)
	resp.Time = firstByteTime
	return resp, nil
}

func (r *SimpleRunner) Dump(req *ffuf.Request) ([]byte, error) {
	var httpreq *http.Request
	var err error
	data := bytes.NewReader(req.Data)
	httpreq, err = http.NewRequestWithContext(r.config.Context, req.Method, req.Url, data)
	if err != nil {
		return []byte{}, err
	}

	if r.config.RandomAgent {
		rand.Seed(time.Now().UnixNano()) 
		randIndex := rand.Intn(len(userAgents))
		req.Headers["User-Agent"] = userAgents[randIndex] // Set the agent header
	} else if _, ok := req.Headers["User-Agent"]; !ok {
		// If 'random-agent' is not enabled, set the default User-Agent header if not present
		req.Headers["User-Agent"] = fmt.Sprintf("%s v%s", "Fuzz Faster U Fool", ffuf.Version())
	}



	// Handle Go http.Request special cases
	if _, ok := req.Headers["Host"]; ok {
		httpreq.Host = req.Headers["Host"]
	}

	req.Host = httpreq.Host
	for k, v := range req.Headers {
		httpreq.Header.Set(k, v)
	}
	return httputil.DumpRequestOut(httpreq, true)
}
