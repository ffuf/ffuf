package runner

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
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

func NewSimpleRunner(conf *ffuf.Config) ffuf.RunnerProvider {
	var simplerunner SimpleRunner
	simplerunner.config = conf
	simplerunner.client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       time.Duration(time.Duration(conf.Timeout) * time.Second),
		Transport: &http.Transport{
			Proxy:               conf.ProxyURL,
			MaxIdleConns:        1000,
			MaxIdleConnsPerHost: 500,
			MaxConnsPerHost:     500,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: !conf.TLSVerify,
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
		req.Method = strings.Replace(req.Method, keyword, string(inputitem), -1)
		headers := make(map[string]string, 0)
		for h, v := range req.Headers {
			headers[strings.Replace(h, keyword, string(inputitem), -1)] = strings.Replace(v, keyword, string(inputitem), -1)
		}
		req.Headers = headers
		req.Url = strings.Replace(req.Url, keyword, string(inputitem), -1)
		req.Data = []byte(strings.Replace(string(req.Data), keyword, string(inputitem), -1))
	}

	req.Input = input
	return req, nil
}

func (r *SimpleRunner) Execute(req *ffuf.Request) (ffuf.Response, error) {
	var httpreq *http.Request
	var err error
	data := bytes.NewReader(req.Data)
	httpreq, err = http.NewRequest(req.Method, req.Url, data)
	if err != nil {
		return ffuf.Response{}, err
	}
	// Add user agent string if not defined
	if _, ok := req.Headers["User-Agent"]; !ok {
		req.Headers["User-Agent"] = fmt.Sprintf("%s v%s", "Fuzz Faster U Fool", ffuf.VERSION)
	}
	// Handle Go http.Request special cases
	if _, ok := req.Headers["Host"]; ok {
		httpreq.Host = req.Headers["Host"]
	}
	httpreq = httpreq.WithContext(r.config.Context)
	for k, v := range req.Headers {
		httpreq.Header.Set(k, v)
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
		if size > MAX_DOWNLOAD_SIZE {
			resp.Cancelled = true
			return resp, nil
		}
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
