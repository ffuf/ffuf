package ffuf

//FilterProvider is a generic interface for both Matchers and Filters
type FilterProvider interface {
	Filter(response *Response) (bool, error)
	Repr() string
	ReprVerbose() string
}

//RunnerProvider is an interface for request executors
type RunnerProvider interface {
	Prepare(input map[string][]byte) (Request, error)
	Execute(req *Request) (Response, error)
}

//InputProvider interface handles the input data for RunnerProvider
type InputProvider interface {
	AddProvider(InputProviderConfig) error
	Next() bool
	Position() int
	Reset()
	Value() map[string][]byte
	Total() int
}

//InternalInputProvider interface handles providing input data to InputProvider
type InternalInputProvider interface {
	Keyword() string
	Next() bool
	Position() int
	ResetPosition()
	IncrementPosition()
	Value() []byte
	Total() int
}

//OutputProvider is responsible of providing output from the RunnerProvider
type OutputProvider interface {
	Banner()
	Finalize() error
	Progress(status Progress)
	Info(infostring string)
	Error(errstring string)
	Raw(output string)
	Warning(warnstring string)
	Result(resp Response)
	PrintResult(res Result)
	SaveFile(filename, format string) error
	GetResults() []Result
	SetResults(results []Result)
	Reset()
}

type Result struct {
	Input            map[string][]byte `json:"input"`
	Position         int               `json:"position"`
	StatusCode       int64             `json:"status"`
	ContentLength    int64             `json:"length"`
	ContentWords     int64             `json:"words"`
	ContentLines     int64             `json:"lines"`
	ContentType      string            `json:"content-type"`
	RedirectLocation string            `json:"redirectlocation"`
	Url              string            `json:"url"`
	ResultFile       string            `json:"resultfile"`
	Host             string            `json:"host"`
	HTMLColor        string            `json:"-"`
}
