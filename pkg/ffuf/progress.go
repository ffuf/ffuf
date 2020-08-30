package ffuf

import (
	"time"
)

type Progress struct {
	StartedAt  time.Time
	ReqCount   int
	ReqTotal   int
	ReqSec     int64
	QueuePos   int
	QueueTotal int
	ErrorCount int
}
