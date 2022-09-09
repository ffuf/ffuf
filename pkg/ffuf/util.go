package ffuf

import (
	"net/url"
	"strings"

	"github.com/ffuf/ffuf/pkg/config"
	"github.com/ffuf/ffuf/pkg/http"
)

// RequestContainsKeyword checks if a keyword is present in any field of a request
func RequestContainsKeyword(req http.Request, kw string) bool {
	if strings.Contains(req.Host, kw) {
		return true
	}
	if strings.Contains(req.Url, kw) {
		return true
	}
	if strings.Contains(req.Method, kw) {
		return true
	}
	if strings.Contains(string(req.Data), kw) {
		return true
	}
	for k, v := range req.Headers {
		if strings.Contains(k, kw) || strings.Contains(v, kw) {
			return true
		}
	}
	return false
}

// HostURLFromRequest gets a host + path without the filename or last part of the URL path
func HostURLFromRequest(req http.Request) string {
	u, _ := url.Parse(req.Url)
	u.Host = req.Host
	pathparts := strings.Split(u.Path, "/")
	trimpath := strings.TrimSpace(strings.Join(pathparts[:len(pathparts)-1], "/"))
	return u.Host + trimpath
}

func NewRequest(conf *config.Config) http.Request {
	var req http.Request
	req.Method = conf.Method
	req.Url = conf.Url
	req.Headers = make(map[string]string)
	return req
}

// BaseRequest returns a base request struct populated from the main config
func BaseRequest(conf *config.Config) http.Request {
	req := NewRequest(conf)
	req.Headers = conf.Headers
	req.Data = []byte(conf.Data)
	return req
}

// RecursionRequest returns a base request for a recursion target
func RecursionRequest(conf *config.Config, path string) http.Request {
	r := BaseRequest(conf)
	r.Url = path
	return r
}
