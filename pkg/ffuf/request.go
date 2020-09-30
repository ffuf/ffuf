package ffuf

import "time"

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
	Start    time.Time
}

func NewRequest(conf *Config) Request {
	var req Request
	req.Method = conf.Method
	req.Url = conf.Url
	req.Headers = make(map[string]string)
	req.Start = time.Now()
	return req
}
