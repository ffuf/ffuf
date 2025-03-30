package output

import (
	"fmt"
	"github.com/NimbleMarkets/ntcharts/canvas"
	"github.com/NimbleMarkets/ntcharts/linechart"
	"github.com/ffuf/ffuf/v2/pkg/ffuf"
	"golang.org/x/term"
	"slices"
	"strconv"
)

func (s *Stdoutput) PrintSummary(r []ffuf.ResponseStatistics) {
	s.Raw("Total Requests: " + strconv.Itoa(len(r)) + "\n")

	statusCodes := make(map[int64]int)

	// find terminal width
	width := 40
	if term.IsTerminal(0) {
		var err error
		width, _, err = term.GetSize(0)
		if err != nil {
			width = 40
		}
	}

	if len(r) > 1 {
		var durations []int64
		lengths := make(map[int64]int)

		var minLength int64
		var maxLength int64
		var sumLength int64
		var minDuration int64
		var maxDuration int64
		var sumDuration int64

		minDuration = r[0].Duration.Milliseconds()
		maxDuration = 0
		sumDuration = 0

		for _, v := range r {
			statusCodes[v.StatusCode]++

			durations = append(durations, v.Duration.Milliseconds())
			sumDuration += v.Duration.Milliseconds()

			if v.Duration.Milliseconds() < minDuration {
				minDuration = v.Duration.Milliseconds()
			}

			if v.Duration.Milliseconds() > maxDuration {
				maxDuration = v.Duration.Milliseconds()
			}

			lengths[v.ContentLength]++
			sumLength += v.ContentLength

			if v.ContentLength < minLength {
				minLength = v.ContentLength
			}

			if v.ContentLength > maxLength {
				maxLength = v.ContentLength
			}
		}

		fmt.Printf("== Status Codes ==\n")
		for k, v := range statusCodes {
			fmt.Printf("%d: %d, ", k, v)
		}
		fmt.Printf("\n")

		fmt.Printf("== Response Lengths ==\n")
		if len(lengths) < 10 { // less than 10 distinct content lengths, just print the counts
			fmt.Printf("Len\tCount\n")
			for k, v := range lengths {
				fmt.Printf("%d\t%d\n", k, v)
			}
		} else {
			// calc median
			var contentLengths []int64
			for _, v := range r {
				contentLengths = append(contentLengths, v.ContentLength)
			}
			slices.Sort(contentLengths)
			fmt.Printf("Unique Content Lengths: %d\n", len(lengths))
			fmt.Printf("Median: %d Mean: %.2f Min/Max: %d:%d\n", contentLengths[len(r)/2], float64(sumLength)/float64(len(r)), minLength, maxLength)
		}

		slices.Sort(durations)
		fmt.Printf("== Response Timing ==\n")
		fmt.Printf("Median: %dms Mean: %.2fms Min/Max: %dms:%dms\n", durations[len(durations)/2],
			float64(sumDuration)/float64(len(durations)),
			durations[0],
			durations[len(durations)-1])

		lc := linechart.New(width, 10, 0, float64(len(r)), float64(minDuration), float64(maxDuration))
		p1 := canvas.Float64Point{X: 0, Y: float64(r[0].Duration.Milliseconds())}
		for i, v := range r {
			if i == 1 {
				continue
			}

			p2 := canvas.Float64Point{X: float64(i), Y: float64(v.Duration.Milliseconds())}
			lc.DrawBrailleLine(p1, p2)
			lc.DrawXYAxisAndLabel()
			p1 = p2
		}

		fmt.Println(lc.View())
	}
}
