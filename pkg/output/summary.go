package output

import (
	"fmt"
	"github.com/NimbleMarkets/ntcharts/linechart/streamlinechart"
	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"strconv"
)

func (s *Stdoutput) PrintSummary(r []ffuf.ResponseStatistics) {
	s.Raw("Total Requests: " + strconv.Itoa(len(r)) + "\n")

	var slc streamlinechart.Model
	if len(r) < 40 {
		slc = streamlinechart.New(len(r), 10)
	} else {
		slc = streamlinechart.New(40, 10)
	}
	for _, v := range r {
		slc.Push(float64(v.Duration.Milliseconds()))
	}

	slc.Draw()
	fmt.Println(slc.View())
}
