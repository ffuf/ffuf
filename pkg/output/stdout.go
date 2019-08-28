package output

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ffuf/ffuf/pkg/ffuf"
)

const (
	BANNER_HEADER = `
        /'___\  /'___\           /'___\       
       /\ \__/ /\ \__/  __  __  /\ \__/       
       \ \ ,__\\ \ ,__\/\ \/\ \ \ \ ,__\      
        \ \ \_/ \ \ \_/\ \ \_\ \ \ \ \_/      
         \ \_\   \ \_\  \ \____/  \ \_\       
          \/_/    \/_/   \/___/    \/_/       
`
	BANNER_SEP = "________________________________________________"
)

type Stdoutput struct {
	config  *ffuf.Config
	Results []Result
}

type Result struct {
	Input         string `json:"input"`
	Position      int    `json:"position"`
	StatusCode    int64  `json:"status"`
	ContentLength int64  `json:"length"`
	ContentWords  int64  `json:"words"`
}

func NewStdoutput(conf *ffuf.Config) *Stdoutput {
	var outp Stdoutput
	outp.config = conf
	outp.Results = []Result{}
	return &outp
}

func (s *Stdoutput) Banner() error {
	fmt.Printf("%s\n       v%s\n%s\n\n", BANNER_HEADER, ffuf.VERSION, BANNER_SEP)
	printOption([]byte("Method"), []byte(s.config.Method))
	printOption([]byte("URL"), []byte(s.config.Url))
	for _, f := range s.config.Matchers {
		printOption([]byte("Matcher"), []byte(f.Repr()))
	}
	for _, f := range s.config.Filters {
		printOption([]byte("Filter"), []byte(f.Repr()))
	}
	fmt.Printf("%s\n\n", BANNER_SEP)
	return nil
}

func (s *Stdoutput) Progress(status ffuf.Progress) {
	if s.config.Quiet {
		// No progress for quiet mode
		return
	}

	dur := time.Now().Sub(status.StartedAt)
	runningSecs := int(dur / time.Second)
	var reqRate int
	if runningSecs > 0 {
		reqRate = int(status.ReqCount / runningSecs)
	} else {
		reqRate = 0
	}

	hours := dur / time.Hour
	dur -= hours * time.Hour
	mins := dur / time.Minute
	dur -= mins * time.Minute
	secs := dur / time.Second

	fmt.Fprintf(os.Stderr, "%s:: Progress: [%d/%d] :: %d req/sec :: Duration: [%d:%02d:%02d] :: Errors: %d ::", TERMINAL_CLEAR_LINE, status.ReqCount, status.ReqTotal, reqRate, hours, mins, secs, status.ErrorCount)
}

func (s *Stdoutput) Error(errstring string) {
	if s.config.Quiet {
		fmt.Fprintf(os.Stderr, "%s", errstring)
	} else {
		if !s.config.Colors {
			fmt.Fprintf(os.Stderr, "%s[ERR] %s\n", TERMINAL_CLEAR_LINE, errstring)
		} else {
			fmt.Fprintf(os.Stderr, "%s[%sERR%s] %s\n", TERMINAL_CLEAR_LINE, ANSI_RED, ANSI_CLEAR, errstring)
		}
	}
}

func (s *Stdoutput) Warning(warnstring string) {
	if s.config.Quiet {
		fmt.Fprintf(os.Stderr, "%s", warnstring)
	} else {
		if !s.config.Colors {
			fmt.Fprintf(os.Stderr, "%s[WARN] %s", TERMINAL_CLEAR_LINE, warnstring)
		} else {
			fmt.Fprintf(os.Stderr, "%s[%sWARN%s] %s\n", TERMINAL_CLEAR_LINE, ANSI_RED, ANSI_CLEAR, warnstring)
		}
	}
}

func (s *Stdoutput) Finalize() error {
	var err error
	if s.config.OutputFile != "" {
		if s.config.OutputFormat == "json" {
			err = writeJSON(s.config, s.Results)
		} else if s.config.OutputFormat == "html" {
			err = writeHTML(s.config, s.Results)
		} else if s.config.OutputFormat == "csv" {
			err = writeCSV(s.config, s.Results, false)
		} else if s.config.OutputFormat == "ecsv" {
			err = writeCSV(s.config, s.Results, true)
		}
		if err != nil {
			s.Error(fmt.Sprintf("%s", err))
		}
	}
	fmt.Fprintf(os.Stderr, "\n")
	return nil
}

func (s *Stdoutput) Result(resp ffuf.Response) {
	// Output the result
	s.printResult(resp)
	// Check if we need the data later
	if s.config.OutputFile != "" {
		// No need to store results if we're not going to use them later
		sResult := Result{
			Input:         string(resp.Request.Input),
			Position:      resp.Request.Position,
			StatusCode:    resp.StatusCode,
			ContentLength: resp.ContentLength,
			ContentWords:  resp.ContentWords,
		}
		s.Results = append(s.Results, sResult)
	}
}

func (s *Stdoutput) printResult(resp ffuf.Response) {
	if s.config.Quiet {
		s.resultQuiet(resp)
	} else {
		s.resultNormal(resp)
	}
}

func (s *Stdoutput) resultQuiet(resp ffuf.Response) {
	if len(s.config.InputCommand) > 0 {
		// If we're using external command for input, display the position instead of input
		fmt.Println(strconv.Itoa(resp.Request.Position))
	} else {
		fmt.Println(string(resp.Request.Input))
	}
}

func (s *Stdoutput) resultNormal(resp ffuf.Response) {
	var res_str string
	if len(s.config.InputCommand) > 0 {
		// If we're using external command for input, display the position instead of input
		res_str = fmt.Sprintf("%s%-23s [Status: %s, Size: %d, Words: %d]", TERMINAL_CLEAR_LINE, strconv.Itoa(resp.Request.Position), s.colorizeStatus(resp.StatusCode), resp.ContentLength, resp.ContentWords)
	} else {
		res_str = fmt.Sprintf("%s%-23s [Status: %s, Size: %d, Words: %d]", TERMINAL_CLEAR_LINE, resp.Request.Input, s.colorizeStatus(resp.StatusCode), resp.ContentLength, resp.ContentWords)
	}
	fmt.Println(res_str)
}

func (s *Stdoutput) colorizeStatus(status int64) string {
	if !s.config.Colors {
		return fmt.Sprintf("%d", status)
	}
	colorCode := ANSI_CLEAR
	if status >= 200 && status < 300 {
		colorCode = ANSI_GREEN
	}
	if status >= 300 && status < 400 {
		colorCode = ANSI_BLUE
	}
	if status >= 400 && status < 500 {
		colorCode = ANSI_YELLOW
	}
	if status >= 500 && status < 600 {
		colorCode = ANSI_RED
	}
	return fmt.Sprintf("%s%d%s", colorCode, status, ANSI_CLEAR)
}

func printOption(name []byte, value []byte) {
	fmt.Printf(" :: %-12s : %s\n", name, value)
}
