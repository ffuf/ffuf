package output

import (
	"time"

	"github.com/ffuf/ffuf/pkg/http"
)

// OutputProvider is responsible of providing output from the RunnerProvider
type OutputProvider interface {
	Banner()
	Finalize() error
	Progress(status Progress)
	Info(infostring string)
	Error(errstring string)
	Raw(output string)
	Warning(warnstring string)
	Result(resp http.Response)
	PrintResult(res Result)
	SaveFile(filename, format string) error
	GetCurrentResults() []Result
	SetCurrentResults(results []Result)
	Reset()
	Cycle()
}

type Progress struct {
	StartedAt  time.Time
	ReqCount   int
	ReqTotal   int
	ReqSec     int64
	QueuePos   int
	QueueTotal int
	ErrorCount int
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
	Duration         time.Duration     `json:"duration"`
	ResultFile       string            `json:"resultfile"`
	Host             string            `json:"host"`
	HTMLColor        string            `json:"-"`
}
