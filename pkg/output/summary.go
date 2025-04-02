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

func scale(unscaledNum int64, minAllowed int64, maxAllowed int64, min int64, max int64) int64 {
	return (maxAllowed-minAllowed)*(unscaledNum-min)/(max-min) + minAllowed
}

func (s *Stdoutput) printBoxPlot(width int64, q0 int64, q1 int64, q2 int64, q3 int64, q4 int64) {
	minimum := []string{"╷", "├", "╵"}
	line := []string{" ", "─", " "}
	left_box := []string{"┌", "┤", "└"}
	line_box := []string{"─", " ", "─"}
	median := []string{"┬", "│", "┴"}
	right_box := []string{"┐", "├", "┘"}
	maximum := []string{"╷", "┤", "╵"}

	out := make([]string, 3)

	if q0 > 0 {
		// rescale to zero
		q1 -= q0
		q2 -= q0
		q3 -= q0
		q4 -= q0
		q0 = 0
	}

	if q4 > width {
		q1 = scale(q1, 0, width, 0, q4)
		q2 = scale(q2, 0, width, 0, q4)
		q3 = scale(q3, 0, width, 0, q4)
		q4 = width
	}

	for i := range out {
		out[i] = out[i] + minimum[i]
		for j := int64(0); j < q1-2; j++ {
			out[i] = out[i] + line[i]
		}
		out[i] = out[i] + left_box[i]

		for j := int64(0); j < q2-q1-2; j++ {
			out[i] = out[i] + line_box[i]
		}

		out[i] = out[i] + median[i]

		for j := int64(0); j < q3-q2-2; j++ {
			out[i] = out[i] + line_box[i]
		}

		out[i] = out[i] + right_box[i]

		for j := int64(0); j < q4-q3-2; j++ {
			out[i] = out[i] + line[i]
		}

		out[i] = out[i] + maximum[i]
	}

	for i := range out {
		fmt.Println(out[i])
	}

}

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

		minLength = r[0].ContentLength
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

		fmt.Printf("== HTTP Status Codes ==\n")
		for k, v := range statusCodes {
			fmt.Printf("%d: %d, ", k, v)
		}
		fmt.Printf("\n")

		fmt.Printf("== Response Lengths (bytes) ==\n")
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
			s.printBoxPlot(40, minLength, contentLengths[len(r)/4], contentLengths[len(r)/2], contentLengths[(len(r)/4)*3], maxLength)
		}

		slices.Sort(durations)
		fmt.Printf("== Response Timing (milliseconds) ==\n")
		fmt.Printf("Median: %dms Mean: %.2fms Min/Max: %dms:%dms\n", durations[len(durations)/2],
			float64(sumDuration)/float64(len(durations)),
			durations[0],
			durations[len(durations)-1])
		s.printBoxPlot(40, durations[0], durations[len(r)/4], durations[len(r)/2], durations[(len(r)/4)*3], durations[len(durations)-1])

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
