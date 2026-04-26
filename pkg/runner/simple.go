package runner

import (
	"bufio"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/http/httputil"
	"net/textproto"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"

	"github.com/andybalholm/brotli"
)

// Download results < 5MB
const MAX_DOWNLOAD_SIZE = 5242880

type SimpleRunner struct {
	config        *ffuf.Config
	client        *http.Client
	threadVars    map[string]string // per-thread variable cache (per-thread mode only)
	threadVarsInit bool              // whether threadVars have been populated
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

// parsePreflightRequest reads a Burp-style raw HTTP request file and returns
// a *http.Request ready to execute. Main config headers are inherited and
// any already-known vars are substituted into the request before it is built.
func (r *SimpleRunner) parsePreflightRequest(filename string, vars map[string]string) (*http.Request, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("preflight: could not open %q: %s", filename, err)
	}
	defer f.Close()

	reader := bufio.NewReader(f)

	// First line: METHOD path HTTP/version
	firstLine, err := reader.ReadString('\n')
	if err != nil && firstLine == "" {
		return nil, fmt.Errorf("preflight: could not read request line from %q: %s", filename, err)
	}
	parts := strings.SplitN(strings.TrimSpace(firstLine), " ", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("preflight: malformed request line in %q", filename)
	}
	method := parts[0]
	path := parts[1]

	// Parse headers
	headers := make(map[string]string)
	// Start with main config headers so auth etc. is inherited
	for k, v := range r.config.Headers {
		headers[k] = v
	}
	var host string
	for {
		line, err := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line == "" || err != nil {
			break
		}
		p := strings.SplitN(line, ":", 2)
		if len(p) != 2 {
			continue
		}
		k := strings.TrimSpace(p[0])
		v := strings.TrimSpace(p[1])
		if strings.EqualFold(k, "content-length") {
			continue // let net/http compute it
		}
		if strings.EqualFold(k, "host") {
			host = v
			continue // set via Request.Host, not header
		}
		headers[textproto.CanonicalMIMEHeaderKey(k)] = v
	}

	// Body
	body, _ := io.ReadAll(reader)
	// Trim trailing newline added by editors
	body = bytes.TrimRight(body, "\r\n")

	// Substitute known variables into method, path, headers and body
	for name, val := range vars {
		method = strings.ReplaceAll(method, name, val)
		path = strings.ReplaceAll(path, name, val)
		body = []byte(strings.ReplaceAll(string(body), name, val))
		for h, v := range headers {
			headers[h] = strings.ReplaceAll(v, name, val)
		}
		if host != "" {
			host = strings.ReplaceAll(host, name, val)
		}
	}

	// Build the full URL
	var rawURL string
	if strings.HasPrefix(path, "http") {
		rawURL = path
	} else {
		// Derive scheme from main config URL if available
		scheme := "https"
		if r.config.Url != "" {
			if u, err2 := url.Parse(r.config.Url); err2 == nil {
				scheme = u.Scheme
			}
		}
		if host == "" {
			if u, err2 := url.Parse(r.config.Url); err2 == nil {
				host = u.Host
			}
		}
		rawURL = scheme + "://" + host + path
	}

	httpreq, err := http.NewRequestWithContext(r.config.Context, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("preflight: could not build request from %q: %s", filename, err)
	}
	if host != "" {
		httpreq.Host = host
	}
	// Apply headers (User-Agent fallback)
	if _, ok := headers["User-Agent"]; !ok {
		headers["User-Agent"] = fmt.Sprintf("%s v%s", "Fuzz Faster U Fool", ffuf.Version())
	}
	for k, v := range headers {
		httpreq.Header.Set(k, v)
	}
	return httpreq, nil
}

// runPreflightChain executes an ordered slice of PreflightConfig entries,
// accumulates extracted variables, and returns the resulting map.
// inheritVars seeds the map with already-known variables (e.g. from earlier chain steps).
// Variables are stored only in the returned map — never in shared state.
func (r *SimpleRunner) runPreflightChain(chain []ffuf.PreflightConfig, inheritVars map[string]string) (map[string]string, error) {
	vars := make(map[string]string, len(inheritVars))
	for k, v := range inheritVars {
		vars[k] = v
	}

	for _, pf := range chain {
		httpreq, err := r.parsePreflightRequest(pf.RequestFile, vars)
		if err != nil {
			if r.config.PreflightError == "ignore" {
				if r.config.Debuglog != "" {
					log.Printf("preflight ignored error building request from %q: %s", pf.RequestFile, err)
				}
				return vars, nil
			}
			return nil, err
		}

		resp, err := r.client.Do(httpreq)
		if err != nil {
			if r.config.PreflightError == "ignore" {
				if r.config.Debuglog != "" {
					log.Printf("preflight ignored error executing request from %q: %s", pf.RequestFile, err)
				}
				return vars, nil
			}
			return nil, fmt.Errorf("preflight: request %q failed: %s", pf.RequestFile, err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			if r.config.PreflightError == "ignore" {
				if r.config.Debuglog != "" {
					log.Printf("preflight ignored error reading response from %q: %s", pf.RequestFile, err)
				}
				return vars, nil
			}
			return nil, fmt.Errorf("preflight: reading response body from %q failed: %s", pf.RequestFile, err)
		}

		// Apply variable extractions for this step
		for _, ve := range pf.Vars {
			re, err := regexp.Compile(ve.Regex)
			if err != nil {
				if r.config.PreflightError == "ignore" {
					if r.config.Debuglog != "" {
						log.Printf("preflight ignored invalid regex %q for var %s: %s", ve.Regex, ve.Name, err)
					}
					continue
				}
				return nil, fmt.Errorf("preflight: invalid regex %q for var %s: %s", ve.Regex, ve.Name, err)
			}
			matches := re.FindSubmatch(body)
			if len(matches) < 2 {
				if r.config.PreflightError == "ignore" {
					if r.config.Debuglog != "" {
						log.Printf("preflight: regex %q did not match for var %s in response from %q", ve.Regex, ve.Name, pf.RequestFile)
					}
					continue
				}
				return nil, fmt.Errorf("preflight: regex %q did not capture var %s from response of %q", ve.Regex, ve.Name, pf.RequestFile)
			}
			vars[ve.Name] = string(matches[1])
			if r.config.Debuglog != "" {
				log.Printf("preflight: extracted %s from %q", ve.Name, pf.RequestFile)
			}
		}
	}
	return vars, nil
}

// applyVars substitutes extracted variable keywords into a request's URL, headers, and body.
// Variables are applied only to this request — no shared state is modified.
func (r *SimpleRunner) applyVars(req *ffuf.Request, vars map[string]string) {
	for name, val := range vars {
		req.Url = strings.ReplaceAll(req.Url, name, val)
		req.Data = []byte(strings.ReplaceAll(string(req.Data), name, val))
		for h, v := range req.Headers {
			req.Headers[h] = strings.ReplaceAll(v, name, val)
		}
	}
}

func (r *SimpleRunner) Execute(req *ffuf.Request) (ffuf.Response, error) {
	var httpreq *http.Request
	var err error
	var rawreq []byte

	// Run preflight chain and apply extracted variables to the request.
	// Variables live only in local scope (per-request) or on r.threadVars (per-thread),
	// never in shared/global state — each SimpleRunner is owned by a single goroutine.
	if len(r.config.Preflights) > 0 {
		var vars map[string]string
		if r.config.PreflightMode == "per-thread" {
			if !r.threadVarsInit {
				v, ferr := r.runPreflightChain(r.config.Preflights, nil)
				if ferr != nil {
					if r.config.Debuglog != "" {
						log.Printf("preflight chain failed: %s", ferr)
					}
					return ffuf.Response{}, ferr
				}
				r.threadVars = v
				r.threadVarsInit = true
			}
			vars = r.threadVars
		} else {
			// per-request: fresh variable map every Execute call
			v, ferr := r.runPreflightChain(r.config.Preflights, nil)
			if ferr != nil {
				if r.config.Debuglog != "" {
					log.Printf("preflight chain failed: %s", ferr)
				}
				return ffuf.Response{}, ferr
			}
			vars = v
		}
		if len(vars) > 0 {
			r.applyVars(req, vars)
		}
	}

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

	if len(r.config.OutputDirectory) > 0 || len(r.config.AuditLog) > 0 {
		rawreq, _ = httputil.DumpRequestOut(httpreq, true)
		req.Raw = string(rawreq)
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

	// Run postflight chain. Errors are handled per -preflight-error policy.
	// Postflight runs per-request regardless of -preflight-mode; only variable
	// initialisation is "per-thread". We seed postflight with the same vars
	// that were applied to the main request so chained extractions work.
	if len(r.config.Postflights) > 0 {
		var seedVars map[string]string
		if r.config.PreflightMode == "per-thread" && r.threadVarsInit {
			seedVars = r.threadVars
		}
		_, ferr := r.runPreflightChain(r.config.Postflights, seedVars)
		if ferr != nil && r.config.PreflightError != "ignore" {
			if r.config.Debuglog != "" {
				log.Printf("postflight chain failed: %s", ferr)
			}
			// Return the response we already have; postflight failure is non-fatal
			// unless the user wants strict abort — log it but don't discard the result.
			log.Printf("postflight error (result still recorded): %s", ferr)
		}
	}

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
