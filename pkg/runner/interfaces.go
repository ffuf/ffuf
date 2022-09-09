package runner

import (
	"github.com/ffuf/ffuf/pkg/http"
)

// RunnerProvider is an interface for request executors
type RunnerProvider interface {
	Prepare(input map[string][]byte, basereq *http.Request) (http.Request, error)
	Execute(req *http.Request) (http.Response, error)
}
