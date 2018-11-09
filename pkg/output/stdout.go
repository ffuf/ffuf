package output

import (
	"fmt"
	"os"

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
	config *ffuf.Config
}

func NewStdoutput(conf *ffuf.Config) *Stdoutput {
	var outp Stdoutput
	outp.config = conf
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

func (s *Stdoutput) Error(errstring string) {
	if s.config.Quiet {
		fmt.Fprintf(os.Stderr, "%s", errstring)
	} else {
		fmt.Fprintf(os.Stderr, "%s%s", TERMINAL_CLEAR_LINE, errstring)
	}
}

func (s *Stdoutput) Finalize() error {
	fmt.Fprintf(os.Stderr, "\n")
	return nil
}

func (s *Stdoutput) Result(resp ffuf.Response) bool {
	matched := false
	for _, m := range s.config.Matchers {
		match, err := m.Filter(&resp)
		if err != nil {
			continue
		}
		if match {
			matched = true
		}
	}
	// The response was not matched, return before running filters
	if !matched {
		return false
	}
	for _, f := range s.config.Filters {
		fv, err := f.Filter(&resp)
		if err != nil {
			continue
		}
		if fv {
			return false
		}
	}
	// Response survived the filtering, output the result
	s.printResult(resp)
	return true
}

func (s *Stdoutput) printResult(resp ffuf.Response) {
	if s.config.Quiet {
		s.resultQuiet(resp)
	} else {
		s.resultNormal(resp)
	}
}

func (s *Stdoutput) resultQuiet(resp ffuf.Response) {
	fmt.Println(string(resp.Request.Input))
}

func (s *Stdoutput) resultNormal(resp ffuf.Response) {
	res_str := fmt.Sprintf("%s%-23s [Status: %s, Size: %d]", TERMINAL_CLEAR_LINE, resp.Request.Input, s.colorizeStatus(resp.StatusCode), resp.ContentLength)
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
