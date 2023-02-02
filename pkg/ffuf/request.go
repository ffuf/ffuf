package ffuf

import (
	"strings"
)

// Request holds the meaningful data that is passed for runner for making the query
type Request struct {
	Method   string
	Host     string
	Url      string
	Headers  map[string]string
	Data     []byte
	Input    map[string][]byte
	Position int
	Raw      string
}

func NewRequest(conf *Config) Request {
	var req Request
	req.Method = conf.Method
	req.Url = conf.Url
	req.Headers = make(map[string]string)
	return req
}

// BaseRequest returns a base request struct populated from the main config
func BaseRequest(conf *Config) Request {
	req := NewRequest(conf)
	req.Headers = conf.Headers
	req.Data = []byte(conf.Data)
	return req
}

// RecursionRequest returns a base request for a recursion target
func RecursionRequest(conf *Config, path string) Request {
	r := BaseRequest(conf)
	r.Url = path
	return r
}

// CopyRequest performs a deep copy of a request and returns a new struct
func CopyRequest(basereq *Request) Request {
	var req Request
	req.Method = basereq.Method
	req.Host = basereq.Host
	req.Url = basereq.Url

	req.Headers = make(map[string]string, len(basereq.Headers))
	for h, v := range basereq.Headers {
		req.Headers[h] = v
	}

	req.Data = make([]byte, len(basereq.Data))
	copy(req.Data, basereq.Data)

	if len(basereq.Input) > 0 {
		req.Input = make(map[string][]byte, len(basereq.Input))
		for k, v := range basereq.Input {
			req.Input[k] = v
		}
	}

	req.Position = basereq.Position
	req.Raw = basereq.Raw

	return req
}

// SniperRequests returns an array of requests, each with one of the templated locations replaced by a keyword
func SniperRequests(basereq *Request, template string) []Request {
	var reqs []Request
	keyword := "FUZZ"

	// Search for input location identifiers, these must exist in pairs
	if c := strings.Count(basereq.Method, template); c > 0 {
		if c%2 == 0 {
			tokens := templateLocations(template, basereq.Method)

			for i := 0; i < len(tokens); i = i + 2 {
				newreq := CopyRequest(basereq)
				newreq.Method = injectKeyword(basereq.Method, keyword, tokens[i], tokens[i+1])
				scrubTemplates(&newreq, template)
				reqs = append(reqs, newreq)
			}
		}
	}

	if c := strings.Count(basereq.Url, template); c > 0 {
		if c%2 == 0 {
			tokens := templateLocations(template, basereq.Url)

			for i := 0; i < len(tokens); i = i + 2 {
				newreq := CopyRequest(basereq)
				newreq.Url = injectKeyword(basereq.Url, keyword, tokens[i], tokens[i+1])
				scrubTemplates(&newreq, template)
				reqs = append(reqs, newreq)
			}
		}
	}

	data := string(basereq.Data)
	if c := strings.Count(data, template); c > 0 {
		if c%2 == 0 {
			tokens := templateLocations(template, data)

			for i := 0; i < len(tokens); i = i + 2 {
				newreq := CopyRequest(basereq)
				newreq.Data = []byte(injectKeyword(data, keyword, tokens[i], tokens[i+1]))
				scrubTemplates(&newreq, template)
				reqs = append(reqs, newreq)
			}
		}
	}

	for k, v := range basereq.Headers {
		if c := strings.Count(k, template); c > 0 {
			if c%2 == 0 {
				tokens := templateLocations(template, k)

				for i := 0; i < len(tokens); i = i + 2 {
					newreq := CopyRequest(basereq)
					newreq.Headers[injectKeyword(k, keyword, tokens[i], tokens[i+1])] = v
					delete(newreq.Headers, k)
					scrubTemplates(&newreq, template)
					reqs = append(reqs, newreq)
				}
			}
		}
		if c := strings.Count(v, template); c > 0 {
			if c%2 == 0 {
				tokens := templateLocations(template, v)

				for i := 0; i < len(tokens); i = i + 2 {
					newreq := CopyRequest(basereq)
					newreq.Headers[k] = injectKeyword(v, keyword, tokens[i], tokens[i+1])
					scrubTemplates(&newreq, template)
					reqs = append(reqs, newreq)
				}
			}
		}
	}

	return reqs
}

// templateLocations returns an array of template character locations in input
func templateLocations(template string, input string) []int {
	var tokens []int

	for k, i := range []rune(input) {
		if i == []rune(template)[0] {
			tokens = append(tokens, k)
		}
	}

	return tokens
}

// injectKeyword takes a string, a keyword, and a start/end offset. The data between
// the start/end offset in string is removed, and replaced by keyword
func injectKeyword(input string, keyword string, startOffset int, endOffset int) string {

	// some basic sanity checking, return the original string unchanged if offsets didnt make sense
	if startOffset > len(input) || endOffset > len(input) || startOffset > endOffset {
		return input
	}

	inputslice := []rune(input)
	keywordslice := []rune(keyword)

	prefix := inputslice[:startOffset]
	suffix := inputslice[endOffset+1:]

	var outputslice []rune
	outputslice = append(outputslice, prefix...)
	outputslice = append(outputslice, keywordslice...)
	outputslice = append(outputslice, suffix...)

	return string(outputslice)
}

// scrubTemplates removes all template (§) strings from the request struct
func scrubTemplates(req *Request, template string) {
	req.Method = strings.Join(strings.Split(req.Method, template), "")
	req.Url = strings.Join(strings.Split(req.Url, template), "")
	req.Data = []byte(strings.Join(strings.Split(string(req.Data), template), ""))

	for k, v := range req.Headers {
		if c := strings.Count(k, template); c > 0 {
			if c%2 == 0 {
				delete(req.Headers, k)
				req.Headers[strings.Join(strings.Split(k, template), "")] = v
			}
		}
		if c := strings.Count(v, template); c > 0 {
			if c%2 == 0 {
				req.Headers[k] = strings.Join(strings.Split(v, template), "")
			}
		}
	}
}
