package runner

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

//Download results < 5MB
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
				Renegotiation:      tls.RenegotiateOnceAsClient,
			},
		}}

	if conf.FollowRedirects {
		simplerunner.client.CheckRedirect = nil
	}
	return &simplerunner
}

func (r *SimpleRunner) Prepare(input map[string][]byte) (ffuf.Request, error) {
	req := ffuf.NewRequest(r.config)

	req.Headers = r.config.Headers
	req.Url = r.config.Url
	req.Method = r.config.Method
	req.Data = []byte(r.config.Data)

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
	httpreq, err = http.NewRequestWithContext(r.config.Context, req.Method, req.Url, data)
	if err != nil {
		return ffuf.Response{}, err
	}

	// set default User-Agent header if not present
	if _, ok := req.Headers["User-Agent"]; !ok {
		req.Headers["User-Agent"] = fmt.Sprintf("%s v%s", "Fuzz Faster U Fool", ffuf.VERSION)
	}

	// Handle Go http.Request special cases
	if _, ok := req.Headers["Host"]; ok {
		httpreq.Host = req.Headers["Host"]
	}

	req.Host = httpreq.Host
	httpreq = httpreq.WithContext(r.config.Context)
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

	if respbody, err := ioutil.ReadAll(httpresp.Body); err == nil {
		resp.ContentLength = int64(utf8.RuneCountInString(string(respbody)))
		resp.Data = respbody
	}

	wordsSize := len(strings.Split(string(resp.Data), " "))
	linesSize := len(strings.Split(string(resp.Data), "\n"))
	resp.ContentWords = int64(wordsSize)
	resp.ContentLines = int64(linesSize)

	return resp, nil
}
