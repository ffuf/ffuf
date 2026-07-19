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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"

	"github.com/andybalholm/brotli"
)

// Download results < 5MB
const MAX_DOWNLOAD_SIZE = 5242880

type SimpleRunner struct {
	config *ffuf.Config
	client *http.Client
	lanes  *lanePool
}

// preflightLane holds the variables extracted by a per-thread preflight chain.
// A lane is checked out of the pool for the duration of one Execute call, so its
// map is only ever touched by a single goroutine at a time.
type preflightLane struct {
	vars        map[string]string
	initialized bool
}

// lanePool hands out preflightLanes for per-thread preflight mode. ffuf runs a
// fresh goroutine per request (bounded to -t concurrent), so there is no
// persistent worker to pin per-thread state to; instead a request borrows a lane
// for the length of its Execute call and returns it. Lanes are reused, so a
// per-thread preflight chain runs once per lane and its vars are amortized across
// every request that later borrows that lane. The free list grows on demand, so a
// checkout never blocks and cannot deadlock.
type lanePool struct {
	mu   sync.Mutex
	free []*preflightLane
}

func (p *lanePool) get() *preflightLane {
	p.mu.Lock()
	defer p.mu.Unlock()
	if n := len(p.free); n > 0 {
		l := p.free[n-1]
		p.free = p.free[:n-1]
		return l
	}
	return &preflightLane{}
}

func (p *lanePool) put(l *preflightLane) {
	p.mu.Lock()
	p.free = append(p.free, l)
	p.mu.Unlock()
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
	simplerunner.lanes = &lanePool{}
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

func (r *SimpleRunner) Execute(req *ffuf.Request) (resp ffuf.Response, err error) {
	// Pin the target host: a value captured from an (untrusted) preflight response
	// must not change which host this authenticated request is sent to.
	targetHost := hostOf(req.Url)
	// Run the preflight chain first: it may inject extracted variables into this
	// request's URL, headers and body before it is built.
	appliedVars, releaseLane, pferr := r.runPreflights(req)
	defer releaseLane()
	// Postflight runs only when the main request produced a response (err == nil).
	// Deferred so it also covers the oversized / -ignore-body early returns, but is
	// skipped when the request itself errors.
	defer func() {
		if err == nil {
			r.runPostflights(appliedVars)
		}
	}()
	if pferr != nil {
		return ffuf.Response{}, pferr
	}
	if len(appliedVars) > 0 {
		if h := hostOf(req.Url); h != targetHost {
			return ffuf.Response{}, fmt.Errorf("preflight variable changed the request host from %q to %q; refusing to send (a captured value must not alter the target host)", targetHost, h)
		}
	}

	var httpreq *http.Request
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

	if len(r.config.OutputDirectory) > 0 || len(r.config.AuditLog) > 0 {
		rawreq, _ = httputil.DumpRequestOut(httpreq, true)
		req.Raw = string(rawreq)
	}

	httpresp, err := r.client.Do(httpreq)
	if err != nil {
		return ffuf.Response{}, err
	}

	req.Timestamp = start

	resp = ffuf.NewResponse(httpresp, req)
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
	var bodyReader io.Reader = httpresp.Body
	switch httpresp.Header.Get("Content-Encoding") {
	case "gzip":
		if zr, zerr := gzip.NewReader(httpresp.Body); zerr == nil {
			bodyReader = zr
		} // else: fall back to the raw (undecoded) body
	case "br":
		bodyReader = brotli.NewReader(httpresp.Body)
	case "deflate":
		bodyReader = flate.NewReader(httpresp.Body)
	}

	// Bound the DECOMPRESSED read. The Content-Length guard above is not
	// sufficient on its own: a malicious server can send a small compressed body
	// (or a chunked response with no Content-Length, or one the transport
	// transparently decoded and stripped the headers from) that expands to
	// gigabytes. LimitReader caps the bytes we ever pull from the possibly-
	// decompressing reader, so a decompression bomb only materialises
	// MAX_DOWNLOAD_SIZE+1 bytes in memory instead of OOM-killing the process.
	limited := io.LimitReader(bodyReader, int64(MAX_DOWNLOAD_SIZE)+1)
	if respbody, rerr := io.ReadAll(limited); rerr == nil {
		if len(respbody) > MAX_DOWNLOAD_SIZE {
			resp.Cancelled = true
			return resp, nil
		}
		resp.ContentLength = int64(len(respbody))
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

// parsePreflightRequest reads a Burp-style raw HTTP request file and builds an
// *http.Request. Main-config headers are inherited (so auth carries over) and any
// already-known vars are substituted into the request before it is built.
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
	method, path := parts[0], parts[1]

	// Headers, seeded with the main config headers so auth etc. is inherited.
	headers := make(map[string]string)
	for k, v := range r.config.Headers {
		headers[k] = v
	}
	var host string
	for {
		line, rerr := reader.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break // blank line: end of the header section
		}
		// Parse the line BEFORE honoring rerr, so a final header with no trailing
		// newline (as many editors save) is not dropped.
		if p := strings.SplitN(line, ":", 2); len(p) == 2 {
			k, v := strings.TrimSpace(p[0]), strings.TrimSpace(p[1])
			switch {
			case strings.EqualFold(k, "content-length"): // let net/http compute it
			case strings.EqualFold(k, "host"):
				host = v // set via Request.Host, not a header
			default:
				headers[textproto.CanonicalMIMEHeaderKey(k)] = v
			}
		}
		if rerr != nil {
			break
		}
	}

	body, _ := io.ReadAll(reader)
	body = bytes.TrimRight(body, "\r\n") // trim trailing newline editors add

	// Substitute known variables into method, path, headers, host and body, in a
	// single deterministic pass.
	if len(vars) > 0 {
		rep := varsReplacer(vars)
		method = rep.Replace(method)
		path = rep.Replace(path)
		body = []byte(rep.Replace(string(body)))
		host = rep.Replace(host)
		for h, v := range headers {
			headers[h] = rep.Replace(v)
		}
	}

	// Build the full URL, deriving scheme/host from the main target when the raw
	// request uses a relative path.
	var rawURL string
	if strings.HasPrefix(path, "http") {
		rawURL = path
	} else {
		scheme := "https"
		if u, uerr := url.Parse(r.config.Url); uerr == nil && u.Scheme != "" {
			scheme = u.Scheme
			if host == "" {
				host = u.Host
			}
		}
		// A relative-path preflight whose host was derived from the target template
		// during host/vhost fuzzing would carry an unsubstituted keyword; fail loudly
		// rather than send the request to a literal "FUZZ.example.com".
		if kw, ok := r.hostHasKeyword(host); ok {
			return nil, fmt.Errorf("preflight %q: derived target host %q still contains fuzz keyword %q; give the preflight file an absolute URL or a Host header", filename, host, kw)
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
	if _, ok := headers["User-Agent"]; !ok {
		headers["User-Agent"] = fmt.Sprintf("%s v%s", "Fuzz Faster U Fool", ffuf.Version())
	}
	for k, v := range headers {
		httpreq.Header.Set(k, v)
	}
	return httpreq, nil
}

// runPreflightChain executes an ordered chain of requests, accumulating extracted
// variables. inheritVars seeds the map (e.g. with vars from an earlier step). The
// result is returned; no shared state is touched. On error it honours
// -preflight-error: "ignore" returns the vars gathered so far, "abort" errors.
func (r *SimpleRunner) runPreflightChain(chain []ffuf.PreflightConfig, inheritVars map[string]string) (map[string]string, error) {
	vars := make(map[string]string, len(inheritVars))
	for k, v := range inheritVars {
		vars[k] = v
	}
	ignore := r.config.PreflightError == "ignore"

	for _, pf := range chain {
		httpreq, err := r.parsePreflightRequest(pf.RequestFile, vars)
		if err != nil {
			if ignore {
				log.Printf("preflight ignored error building request from %q: %s", pf.RequestFile, err)
				return vars, nil
			}
			return nil, err
		}
		// Count this request against the shared rate limiter so preflight/postflight
		// traffic honors -rate/-p rather than firing unmetered.
		if r.config.RateLimitFunc != nil {
			r.config.RateLimitFunc()
		}
		resp, err := r.client.Do(httpreq)
		if err != nil {
			if ignore {
				log.Printf("preflight ignored error executing %q: %s", pf.RequestFile, err)
				return vars, nil
			}
			return nil, fmt.Errorf("preflight: request %q failed: %s", pf.RequestFile, err)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, int64(MAX_DOWNLOAD_SIZE)+1))
		resp.Body.Close()
		if err != nil {
			if ignore {
				log.Printf("preflight ignored error reading response from %q: %s", pf.RequestFile, err)
				return vars, nil
			}
			return nil, fmt.Errorf("preflight: reading response body from %q failed: %s", pf.RequestFile, err)
		}

		for _, ve := range pf.Vars {
			// Reuse the regex compiled once at config time; fall back to compiling
			// here only when a VarExtract was built outside ConfigFromOptions (tests).
			re := ve.Compiled
			if re == nil {
				var cerr error
				re, cerr = regexp.Compile(ve.Regex)
				if cerr != nil {
					if ignore {
						log.Printf("preflight ignored invalid regex %q for var %s: %s", ve.Regex, ve.Name, cerr)
						continue
					}
					return nil, fmt.Errorf("preflight: invalid regex %q for var %s: %s", ve.Regex, ve.Name, cerr)
				}
			}
			matches := re.FindSubmatch(body)
			if len(matches) < 2 {
				if ignore {
					log.Printf("preflight: regex %q did not match var %s in response from %q", ve.Regex, ve.Name, pf.RequestFile)
					continue
				}
				return nil, fmt.Errorf("preflight: regex %q did not capture var %s from response of %q", ve.Regex, ve.Name, pf.RequestFile)
			}
			val := string(matches[1])
			// The captured value comes from the scanned target (untrusted). Reject
			// control characters: they can't legally go into a URL/header and would
			// otherwise corrupt or fail the outgoing request (defense in depth).
			if strings.ContainsAny(val, "\r\n\x00") {
				if ignore {
					log.Printf("preflight: var %s from %q contains a control character, skipping", ve.Name, pf.RequestFile)
					continue
				}
				return nil, fmt.Errorf("preflight: var %s captured from %q contains a control character; refusing to use it", ve.Name, pf.RequestFile)
			}
			vars[ve.Name] = val
			log.Printf("preflight: extracted %s from %q", ve.Name, pf.RequestFile)
		}
	}
	return vars, nil
}

// varsReplacer builds one deterministic replacer for the extracted vars. Names
// are applied longest-first (then lexicographically) so a shorter name that is a
// prefix of a longer one can't win, and strings.Replacer does a single
// simultaneous pass, so a replaced value is never rescanned. That makes
// substitution independent of map iteration order and free of value-contains-name
// cascades.
func varsReplacer(vars map[string]string) *strings.Replacer {
	names := make([]string, 0, len(vars))
	for k := range vars {
		names = append(names, k)
	}
	sort.Slice(names, func(i, j int) bool {
		if len(names[i]) != len(names[j]) {
			return len(names[i]) > len(names[j])
		}
		return names[i] < names[j]
	})
	pairs := make([]string, 0, len(vars)*2)
	for _, n := range names {
		pairs = append(pairs, n, vars[n])
	}
	return strings.NewReplacer(pairs...)
}

// hostOf returns the authority (host[:port]) of a URL, or "" if it cannot be parsed.
func hostOf(rawurl string) string {
	u, err := url.Parse(rawurl)
	if err != nil {
		return ""
	}
	return u.Host
}

// hostHasKeyword reports whether host still contains an input fuzz keyword, which
// means it was derived from an unsubstituted target template (e.g. vhost fuzzing
// with a relative-path preflight).
func (r *SimpleRunner) hostHasKeyword(host string) (string, bool) {
	for _, ip := range r.config.InputProviders {
		if ip.Keyword != "" && strings.Contains(host, ip.Keyword) {
			return ip.Keyword, true
		}
	}
	return "", false
}

// applyVars substitutes extracted variable keywords into a single request's URL,
// headers and body. It mutates only the request passed in.
func (r *SimpleRunner) applyVars(req *ffuf.Request, vars map[string]string) {
	if len(vars) == 0 {
		return
	}
	rep := varsReplacer(vars)
	req.Url = rep.Replace(req.Url)
	req.Data = []byte(rep.Replace(string(req.Data)))
	for h, v := range req.Headers {
		req.Headers[h] = rep.Replace(v)
	}
}

// runPreflights resolves the variables for this request per -preflight-mode
// (fresh per request, or once per lane and amortized in per-thread mode) and
// applies them. It returns the applied vars so postflight can chain off them, and
// a cleanup func the caller must defer (it returns a borrowed lane, if any).
func (r *SimpleRunner) runPreflights(req *ffuf.Request) (applied map[string]string, cleanup func(), err error) {
	cleanup = func() {}
	if len(r.config.Preflights) == 0 {
		return nil, cleanup, nil
	}
	if r.config.PreflightMode == "per-thread" {
		lane := r.lanes.get()
		cleanup = func() { r.lanes.put(lane) }
		if !lane.initialized {
			v, ferr := r.runPreflightChain(r.config.Preflights, nil)
			if ferr != nil {
				cleanup()
				return nil, func() {}, ferr
			}
			lane.vars = v
			lane.initialized = true
		}
		applied = lane.vars
	} else {
		v, ferr := r.runPreflightChain(r.config.Preflights, nil)
		if ferr != nil {
			return nil, cleanup, ferr
		}
		applied = v
	}
	if len(applied) > 0 {
		r.applyVars(req, applied)
	}
	return applied, cleanup, nil
}

// runPostflights executes the postflight chain after the main request, seeded
// with the vars applied to it so chained extractions work. Postflight failure is
// non-fatal: the main result is always kept, the error is only logged.
func (r *SimpleRunner) runPostflights(seedVars map[string]string) {
	if len(r.config.Postflights) == 0 {
		return
	}
	if _, ferr := r.runPreflightChain(r.config.Postflights, seedVars); ferr != nil {
		log.Printf("postflight error (result still recorded): %s", ferr)
	}
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
