package ffuf

//FilterProvider is a generic interface for both Matchers and Filters
type FilterProvider interface {
	Filter(response *Response) (bool, error)
	Repr() string
}

//RunnerProvider is an interface for request executors
type RunnerProvider interface {
	Prepare(input []byte) (Request, error)
	Execute(req *Request) (Response, error)
}

//InputProvider interface handles the input data for RunnerProvider
type InputProvider interface {
	Next() bool
	Value() []byte
	Total() int
}

//OutputProvider is responsible of providing output from the RunnerProvider
type OutputProvider interface {
	Banner() error
	Finalize() error
	Error(errstring string)
	Result(resp Response) bool
}
