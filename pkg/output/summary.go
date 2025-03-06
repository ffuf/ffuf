package output

import (
	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"strconv"
)

func (s *Stdoutput) PrintSummary(r []ffuf.ResponseStatistics) {
	s.Raw("Total Requests: " + strconv.Itoa(len(r)) + "\n")
}
