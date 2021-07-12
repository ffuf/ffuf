package ffuf

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

// CopyRequest performs a deep copy of a request and returns a new struct
func CopyRequest(basereq *Request) Request {
	var req Request
	req.Method = basereq.Method
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
