package runner

import (
	"bytes"
	"compress/flate"
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

	"github.com/ffuf/ffuf/v2/pkg/ffuf"

	"github.com/andybalholm/brotli"
)

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
	cert := []tls.Certificate{}

	if conf.ClientCert != "" && conf.ClientKey != "" {
		tmp, _ := tls.LoadX509KeyPair(conf.ClientCert, conf.ClientKey)
		cert = []tls.Certificate{tmp}
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
				Certificates:       cert,
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

	// set default User-Agent header if not present
	if _, ok := req.Headers["User-Agent"]; !ok {
		req.Headers["User-Agent"] = fmt.Sprintf("%s v%s", "Fuzz Faster U Fool", ffuf.Version())
	}

	// Handle Go http.Request special cases
	if _, ok := req.Headers["Host"]; ok {
		httpreq.Host = req.Headers["Host"]
	}

	req.Host = httpreq.Host
	httpreq = httpreq.WithContext(httptrace.WithClientTrace(r.config.Context, trace))

	if r.config.Raw {
		httpreq.URL.Opaque = req.Url
	}

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
	} else if httpresp.Header.Get("Content-Encoding") == "br" {
		bodyReader = io.NopCloser(brotli.NewReader(httpresp.Body))
		if err != nil {
			// fallback to raw data
			bodyReader = httpresp.Body
		}
	} else if httpresp.Header.Get("Content-Encoding") == "deflate" {
		bodyReader = flate.NewReader(httpresp.Body)
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

	// set default User-Agent header if not present
	if _, ok := req.Headers["User-Agent"]; !ok {
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
