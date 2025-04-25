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
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"

	"github.com/andybalholm/brotli"
	"github.com/virusvfv/go-ntlmssp"
)

// Download results < 5MB
const MAX_DOWNLOAD_SIZE = 5242880

type SimpleRunner struct {
	config *ffuf.Config
	client *http.Client
}

func NewHttpClient(conf *ffuf.Config, replay bool) *http.Client {
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

	if conf.ClientCert != "" && conf.ClientKey != "" {
		tmp, _ := tls.LoadX509KeyPair(conf.ClientCert, conf.ClientKey)
		cert = []tls.Certificate{tmp}
	}

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       time.Duration(time.Duration(conf.Timeout) * time.Second),
		Transport: ntlmssp.Negotiator{
			RoundTripper: &http.Transport{
				ForceAttemptHTTP2:   false,
				Proxy:               proxyURL,
				MaxIdleConns:        1000,
				MaxIdleConnsPerHost: 500,
				MaxConnsPerHost:     500,
				//IdleConnTimeout:     5 * time.Second,
				///DisableKeepAlives:   true,

				DialContext: (&net.Dialer{
					Timeout: time.Duration(time.Duration(conf.Timeout) * time.Second),
					//KeepAlive: 5 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: time.Duration(time.Duration(conf.Timeout) * time.Second),
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
					MinVersion:         tls.VersionTLS10,
					Renegotiation:      tls.RenegotiateOnceAsClient,
					ServerName:         conf.SNI,
					Certificates:       cert,
				},
			},
		}}
	return client
}

func NewSimpleRunner(conf *ffuf.Config, replay bool) ffuf.RunnerProvider {
	var simplerunner SimpleRunner

	simplerunner.config = conf
	simplerunner.client = NewHttpClient(conf, replay)

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
		req.Auth = strings.ReplaceAll(req.Auth, keyword, string(inputitem))
		req.Data = []byte(strings.ReplaceAll(string(req.Data), keyword, string(inputitem)))
	}

	req.Input = input
	return req, nil
}

func (r *SimpleRunner) GetCSRF(req *ffuf.Request) []string {
	var csrf []string
	var httpreq *http.Request
	var err error
	var prebodyReader io.ReadCloser

	httpreq, err = http.NewRequestWithContext(r.config.Context, http.MethodGet, r.config.Preflight, nil)
	//httpreq, err = http.NewRequest(http.MethodGet, r.config.Preflight, nil)

	//set basic/ntlm auth
	if req.Auth != "" && len(strings.Split(req.Auth, ":")) > 1 {
		httpreq.SetBasicAuth(strings.SplitN(req.Auth, ":", 2)[0], strings.SplitN(req.Auth, ":", 2)[1])
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
	//httpreq = httpreq.WithContext(httptrace.WithClientTrace(r.config.Context, trace))

	if r.config.Raw {
		httpreq.URL.Opaque = req.Url
	}

	for k, v := range req.Headers {
		httpreq.Header.Set(k, v)
	}

	for k, v := range r.config.PreflightHeader {
		httpreq.Header.Set(k, v)
	}

	prehttpresp, err := r.client.Do(httpreq)
	if err != nil {
		return csrf
	}

	resp := ffuf.NewResponse(prehttpresp, req)
	defer prehttpresp.Body.Close()

	if err == nil {

		if prehttpresp.Header.Get("Content-Encoding") == "gzip" {
			prebodyReader, err = gzip.NewReader(prehttpresp.Body)
			if err != nil {
				// fallback to raw data
				prebodyReader = prehttpresp.Body
			}
		} else if prehttpresp.Header.Get("Content-Encoding") == "br" {
			prebodyReader = io.NopCloser(brotli.NewReader(prehttpresp.Body))
			if err != nil {
				// fallback to raw data
				prebodyReader = prehttpresp.Body
			}
		} else if prehttpresp.Header.Get("Content-Encoding") == "deflate" {
			prebodyReader = flate.NewReader(prehttpresp.Body)
			if err != nil {
				// fallback to raw data
				prebodyReader = prehttpresp.Body
			}
		} else {
			prebodyReader = prehttpresp.Body
		}

		respbody, err := io.ReadAll(prebodyReader)
		if err == nil {
			resp.ContentLength = int64(len(string(respbody)))
			resp.Data = respbody
		}

		//if err == nil {
		for _, elem := range r.config.Capregex {
			if len(strings.Split(elem, ":")) > 1 {
				val := strings.Split(elem, ":")
				elem = strings.Join(val[:len(val)-1], ":")
				r := regexp.MustCompile(elem)
				//looking in body
				matches := r.FindAllSubmatch(resp.Data, -1)
				if len(matches) > 0 {
					if len(matches[0]) > 1 {
						csrf = append(csrf, strings.Trim(string(matches[0][1]), "\r\n"))
					} else {
						csrf = append(csrf, strings.Trim(string(matches[0][0]), "\r\n"))
					}
				} else {
					//looking in raw response w/o body (status, headers, cookies)
					rawresp, _ := httputil.DumpResponse(prehttpresp, false)
					matches = r.FindAllSubmatch(rawresp, -1)
					if len(matches) > 0 {
						if len(matches[0]) > 1 {
							csrf = append(csrf, strings.Trim(string(matches[0][1]), "\r\n"))
						} else {
							csrf = append(csrf, strings.Trim(string(matches[0][0]), "\r\n"))
						}
					} else {
						csrf = append(csrf, "")
					}
				}
			} else {
				csrf = append(csrf, "")
			}
		}

	}

	return csrf
}

func (r *SimpleRunner) Execute(req *ffuf.Request, newConn bool) (ffuf.Response, error) {
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

	if req.Auth != "" && len(strings.Split(req.Auth, ":")) > 1 {
		httpreq.SetBasicAuth(strings.SplitN(req.Auth, ":", 2)[0], strings.SplitN(req.Auth, ":", 2)[1])
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

	if len(r.config.OutputDirectory) > 0 || len(r.config.AuditLog) > 0 {
		rawreq, _ = httputil.DumpRequestOut(httpreq, true)
		req.Raw = string(rawreq)
	}

	if newConn {
		r.client = NewHttpClient(r.config, false)
	}

	httpresp, err := r.client.Do(httpreq)
	if err != nil {
		return ffuf.Response{}, err
	}

	req.Timestamp = start

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

	if len(r.config.OutputDirectory) > 0 || len(r.config.AuditLog) > 0 {
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
	resp.Duration = firstByteTime
	resp.Timestamp = start.Add(firstByteTime)

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
