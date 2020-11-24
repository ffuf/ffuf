package output

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"path"
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
	Input            map[string][]byte `json:"input"`
	Position         int               `json:"position"`
	StatusCode       int64             `json:"status"`
	ContentLength    int64             `json:"length"`
	ContentWords     int64             `json:"words"`
	ContentLines     int64             `json:"lines"`
	RedirectLocation string            `json:"redirectlocation"`
	Url              string            `json:"url"`
	ResultFile       string            `json:"resultfile"`
	Host             string            `json:"host"`
	HTMLColor        string            `json:"-"`
}

func NewStdoutput(conf *ffuf.Config) *Stdoutput {
	var outp Stdoutput
	outp.config = conf
	outp.Results = []Result{}
	return &outp
}

func (s *Stdoutput) Banner() {
	fmt.Fprintf(os.Stderr, "%s\n       v%s\n%s\n\n", BANNER_HEADER, ffuf.VERSION, BANNER_SEP)
	printOption([]byte("Method"), []byte(s.config.Method))
	printOption([]byte("URL"), []byte(s.config.Url))

	// Print wordlists
	for _, provider := range s.config.InputProviders {
		if provider.Name == "wordlist" {
			printOption([]byte("Wordlist"), []byte(provider.Keyword+": "+provider.Value))
		}
	}

	// Print headers
	if len(s.config.Headers) > 0 {
		for k, v := range s.config.Headers {
			printOption([]byte("Header"), []byte(fmt.Sprintf("%s: %s", k, v)))
		}
	}
	// Print POST data
	if len(s.config.Data) > 0 {
		printOption([]byte("Data"), []byte(s.config.Data))
	}

	// Print extensions
	if len(s.config.Extensions) > 0 {
		exts := ""
		for _, ext := range s.config.Extensions {
			exts = fmt.Sprintf("%s%s ", exts, ext)
		}
		printOption([]byte("Extensions"), []byte(exts))
	}

	// Output file info
	if len(s.config.OutputFile) > 0 {

		// Use filename as specified by user
		OutputFile := s.config.OutputFile

		if s.config.OutputFormat == "all" {
			// Actually... append all extensions
			OutputFile += ".{json,ejson,html,md,csv,ecsv}"
		}

		printOption([]byte("Output file"), []byte(OutputFile))
		printOption([]byte("File format"), []byte(s.config.OutputFormat))
	}

	// Follow redirects?
	follow := fmt.Sprintf("%t", s.config.FollowRedirects)
	printOption([]byte("Follow redirects"), []byte(follow))

	// Autocalibration
	autocalib := fmt.Sprintf("%t", s.config.AutoCalibration)
	printOption([]byte("Calibration"), []byte(autocalib))

	// Proxies
	if len(s.config.ProxyURL) > 0 {
		printOption([]byte("Proxy"), []byte(s.config.ProxyURL))
	}
	if len(s.config.ReplayProxyURL) > 0 {
		printOption([]byte("ReplayProxy"), []byte(s.config.ReplayProxyURL))
	}

	// Timeout
	timeout := fmt.Sprintf("%d", s.config.Timeout)
	printOption([]byte("Timeout"), []byte(timeout))

	// Threads
	threads := fmt.Sprintf("%d", s.config.Threads)
	printOption([]byte("Threads"), []byte(threads))

	// Delay?
	if s.config.Delay.HasDelay {
		delay := ""
		if s.config.Delay.IsRange {
			delay = fmt.Sprintf("%.2f - %.2f seconds", s.config.Delay.Min, s.config.Delay.Max)
		} else {
			delay = fmt.Sprintf("%.2f seconds", s.config.Delay.Min)
		}
		printOption([]byte("Delay"), []byte(delay))
	}

	// Print matchers
	for _, f := range s.config.Matchers {
		printOption([]byte("Matcher"), []byte(f.Repr()))
	}
	// Print filters
	for _, f := range s.config.Filters {
		printOption([]byte("Filter"), []byte(f.Repr()))
	}
	fmt.Fprintf(os.Stderr, "%s\n\n", BANNER_SEP)
}

func (s *Stdoutput) Progress(status ffuf.Progress) {
	if s.config.Quiet {
		// No progress for quiet mode
		return
	}

	dur := time.Since(status.StartedAt)
	runningSecs := int(dur / time.Second)
	var reqRate int64
	if runningSecs > 0 {
		reqRate = status.ReqSec
	} else {
		reqRate = 0
	}

	hours := dur / time.Hour
	dur -= hours * time.Hour
	mins := dur / time.Minute
	dur -= mins * time.Minute
	secs := dur / time.Second

	fmt.Fprintf(os.Stderr, "%s:: Progress: [%d/%d] :: Job [%d/%d] :: %d req/sec :: Duration: [%d:%02d:%02d] :: Errors: %d ::", TERMINAL_CLEAR_LINE, status.ReqCount, status.ReqTotal, status.QueuePos, status.QueueTotal, reqRate, hours, mins, secs, status.ErrorCount)
}

func (s *Stdoutput) Info(infostring string) {
	if s.config.Quiet {
		fmt.Fprintf(os.Stderr, "%s", infostring)
	} else {
		if !s.config.Colors {
			fmt.Fprintf(os.Stderr, "%s[INFO] %s\n", TERMINAL_CLEAR_LINE, infostring)
		} else {
			fmt.Fprintf(os.Stderr, "%s[%sINFO%s] %s\n", TERMINAL_CLEAR_LINE, ANSI_BLUE, ANSI_CLEAR, infostring)
		}
	}
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

func (s *Stdoutput) writeToAll(config *ffuf.Config, res []Result) error {
	var err error
	var BaseFilename string = s.config.OutputFile

	// Go through each type of write, adding
	// the suffix to each output file.

	if(config.OutputCreateEmptyFile && (len(res) == 0)){
		return nil
  	}

	s.config.OutputFile = BaseFilename + ".json"
	err = writeJSON(s.config, s.Results)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".ejson"
	err = writeEJSON(s.config, s.Results)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".html"
	err = writeHTML(s.config, s.Results)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".md"
	err = writeMarkdown(s.config, s.Results)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".csv"
	err = writeCSV(s.config, s.Results, false)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".ecsv"
	err = writeCSV(s.config, s.Results, true)
	if err != nil {
		s.Error(err.Error())
	}

	return nil

}

func (s *Stdoutput) Finalize() error {
	var err error
	if s.config.OutputFile != "" {
		if s.config.OutputFormat == "all" {
			err = s.writeToAll(s.config, s.Results)
		} else if s.config.OutputFormat == "json" {
			err = writeJSON(s.config, s.Results)
		} else if s.config.OutputFormat == "ejson" {
			err = writeEJSON(s.config, s.Results)
		} else if s.config.OutputFormat == "html" {
			err = writeHTML(s.config, s.Results)
		} else if s.config.OutputFormat == "md" {
			err = writeMarkdown(s.config, s.Results)
		} else if s.config.OutputFormat == "csv" {
			err = writeCSV(s.config, s.Results, false)
		} else if s.config.OutputFormat == "ecsv" {
			err = writeCSV(s.config, s.Results, true)
		}
		if err != nil {
			s.Error(err.Error())
		}
	}
	fmt.Fprintf(os.Stderr, "\n")
	return nil
}

func (s *Stdoutput) Result(resp ffuf.Response) {
	// Do we want to write request and response to a file
	if len(s.config.OutputDirectory) > 0 {
		resp.ResultFile = s.writeResultToFile(resp)
	}
	// Output the result
	s.printResult(resp)
	// Check if we need the data later
	if s.config.OutputFile != "" {
		// No need to store results if we're not going to use them later
		inputs := make(map[string][]byte, len(resp.Request.Input))
		for k, v := range resp.Request.Input {
			inputs[k] = v
		}
		sResult := Result{
			Input:            inputs,
			Position:         resp.Request.Position,
			StatusCode:       resp.StatusCode,
			ContentLength:    resp.ContentLength,
			ContentWords:     resp.ContentWords,
			ContentLines:     resp.ContentLines,
			RedirectLocation: resp.GetRedirectLocation(false),
			Url:              resp.Request.Url,
			ResultFile:       resp.ResultFile,
			Host:             resp.Request.Host,
		}
		s.Results = append(s.Results, sResult)
	}
}

func (s *Stdoutput) writeResultToFile(resp ffuf.Response) string {
	var fileContent, fileName, filePath string
	// Create directory if needed
	if s.config.OutputDirectory != "" {
		err := os.Mkdir(s.config.OutputDirectory, 0750)
		if err != nil {
			if !os.IsExist(err) {
				s.Error(err.Error())
				return ""
			}
		}
	}
	fileContent = fmt.Sprintf("%s\n---- ↑ Request ---- Response ↓ ----\n\n%s", resp.Request.Raw, resp.Raw)

	// Create file name
	fileName = fmt.Sprintf("%x", md5.Sum([]byte(fileContent)))

	filePath = path.Join(s.config.OutputDirectory, fileName)
	err := ioutil.WriteFile(filePath, []byte(fileContent), 0640)
	if err != nil {
		s.Error(err.Error())
	}
	return fileName
}

func (s *Stdoutput) printResult(resp ffuf.Response) {
	if s.config.Quiet {
		s.resultQuiet(resp)
	} else {
		if len(resp.Request.Input) > 1 || s.config.Verbose || len(s.config.OutputDirectory) > 0 {
			// Print a multi-line result (when using multiple input keywords and wordlists)
			s.resultMultiline(resp)
		} else {
			s.resultNormal(resp)
		}
	}
}

func (s *Stdoutput) prepareInputsOneLine(resp ffuf.Response) string {
	inputs := ""
	if len(resp.Request.Input) > 1 {
		for k, v := range resp.Request.Input {
			if inSlice(k, s.config.CommandKeywords) {
				// If we're using external command for input, display the position instead of input
				inputs = fmt.Sprintf("%s%s : %s ", inputs, k, strconv.Itoa(resp.Request.Position))
			} else {
				inputs = fmt.Sprintf("%s%s : %s ", inputs, k, v)
			}
		}
	} else {
		for k, v := range resp.Request.Input {
			if inSlice(k, s.config.CommandKeywords) {
				// If we're using external command for input, display the position instead of input
				inputs = strconv.Itoa(resp.Request.Position)
			} else {
				inputs = string(v)
			}
		}
	}
	return inputs
}

func (s *Stdoutput) resultQuiet(resp ffuf.Response) {
	fmt.Println(s.prepareInputsOneLine(resp))
}

func (s *Stdoutput) resultMultiline(resp ffuf.Response) {
	var res_hdr, res_str string
	res_str = "%s%s    * %s: %s\n"
	res_hdr = fmt.Sprintf("%s[Status: %d, Size: %d, Words: %d, Lines: %d]", TERMINAL_CLEAR_LINE, resp.StatusCode, resp.ContentLength, resp.ContentWords, resp.ContentLines)
	res_hdr = s.colorize(res_hdr, resp.StatusCode)
	reslines := ""
	if s.config.Verbose {
		reslines = fmt.Sprintf("%s%s| URL | %s\n", reslines, TERMINAL_CLEAR_LINE, resp.Request.Url)
		redirectLocation := resp.GetRedirectLocation(false)
		if redirectLocation != "" {
			reslines = fmt.Sprintf("%s%s| --> | %s\n", reslines, TERMINAL_CLEAR_LINE, redirectLocation)
		}
	}
	if resp.ResultFile != "" {
		reslines = fmt.Sprintf("%s%s| RES | %s\n", reslines, TERMINAL_CLEAR_LINE, resp.ResultFile)
	}
	for k, v := range resp.Request.Input {
		if inSlice(k, s.config.CommandKeywords) {
			// If we're using external command for input, display the position instead of input
			reslines = fmt.Sprintf(res_str, reslines, TERMINAL_CLEAR_LINE, k, strconv.Itoa(resp.Request.Position))
		} else {
			// Wordlist input
			reslines = fmt.Sprintf(res_str, reslines, TERMINAL_CLEAR_LINE, k, v)
		}
	}
	fmt.Printf("%s\n%s\n", res_hdr, reslines)
}

func (s *Stdoutput) resultNormal(resp ffuf.Response) {
	res := fmt.Sprintf("%s%-23s [Status: %s, Size: %d, Words: %d, Lines: %d]", TERMINAL_CLEAR_LINE, s.prepareInputsOneLine(resp), s.colorize(fmt.Sprintf("%d", resp.StatusCode), resp.StatusCode), resp.ContentLength, resp.ContentWords, resp.ContentLines)
	fmt.Println(res)
}

func (s *Stdoutput) colorize(input string, status int64) string {
	if !s.config.Colors {
		return input
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
	return fmt.Sprintf("%s%s%s", colorCode, input, ANSI_CLEAR)
}

func printOption(name []byte, value []byte) {
	fmt.Fprintf(os.Stderr, " :: %-16s : %s\n", name, value)
}

func inSlice(key string, slice []string) bool {
	for _, v := range slice {
		if v == key {
			return true
		}
	}
	return false
}
