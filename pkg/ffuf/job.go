package ffuf

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

//Job ties together Config, Runner, Input and Output
type Job struct {
	Config               *Config
	ErrorMutex           sync.Mutex
	Input                InputProvider
	Runner               RunnerProvider
	Output               OutputProvider
	Counter              int
	ErrorCounter         int
	SpuriousErrorCounter int
	Total                int
	Running              bool
	Count403             int
	Error                string
	startTime            time.Time
}

func NewJob(conf *Config) Job {
	var j Job
	j.Counter = 0
	j.ErrorCounter = 0
	j.SpuriousErrorCounter = 0
	j.Running = false
	return j
}

//incError increments the error counter
func (j *Job) incError() {
	j.ErrorMutex.Lock()
	defer j.ErrorMutex.Unlock()
	j.ErrorCounter++
	j.SpuriousErrorCounter++
}

//inc403 increments the 403 response counter
func (j *Job) inc403() {
	j.ErrorMutex.Lock()
	defer j.ErrorMutex.Unlock()
	j.Count403++
}

//resetSpuriousErrors resets the spurious error counter
func (j *Job) resetSpuriousErrors() {
	j.ErrorMutex.Lock()
	defer j.ErrorMutex.Unlock()
	j.SpuriousErrorCounter = 0
}

//Start the execution of the Job
func (j *Job) Start() {
	rand.Seed(time.Now().UnixNano())
	j.Total = j.Input.Total()
	defer j.Stop()
	//Show banner if not running in silent mode
	if !j.Config.Quiet {
		j.Output.Banner()
	}
	j.Running = true
	// Monitor for SIGTERM and do cleanup properly (writing the output files etc)
	j.interruptMonitor()
	var wg sync.WaitGroup
	wg.Add(1)
	go j.runProgress(&wg)
	//Limiter blocks after reaching the buffer, ensuring limited concurrency
	limiter := make(chan bool, j.Config.Threads)
	for j.Input.Next() {
		// Check if we should stop the process
		j.CheckStop()
		if !j.Running {
			defer j.Output.Warning(j.Error)
			break
		}
		limiter <- true
		nextInput := j.Input.Value()
		nextPosition := j.Input.Position()
		wg.Add(1)
		j.Counter++
		go func() {
			defer func() { <-limiter }()
			defer wg.Done()
			j.runTask(nextInput, nextPosition, false)
			if j.Config.Delay.HasDelay {
				var sleepDurationMS time.Duration
				if j.Config.Delay.IsRange {
					sTime := j.Config.Delay.Min + rand.Float64()*(j.Config.Delay.Max-j.Config.Delay.Min)
					sleepDurationMS = time.Duration(sTime * 1000)
				} else {
					sleepDurationMS = time.Duration(j.Config.Delay.Min * 1000)
				}
				time.Sleep(sleepDurationMS * time.Millisecond)
			}
		}()
	}
	wg.Wait()
	j.updateProgress()
	j.Output.Finalize()
	return
}

func (j *Job) interruptMonitor() {
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		for _ = range sigChan {
			j.Error = "Caught keyboard interrupt (Ctrl-C)\n"
			j.Stop()
		}
	}()
}

func (j *Job) runProgress(wg *sync.WaitGroup) {
	defer wg.Done()
	j.startTime = time.Now()
	totalProgress := j.Input.Total()
	for j.Counter <= totalProgress {
		if !j.Running {
			break
		}
		j.updateProgress()
		if j.Counter == totalProgress {
			return
		}
		time.Sleep(time.Millisecond * time.Duration(j.Config.ProgressFrequency))
	}
}

func (j *Job) updateProgress() {
	prog := Progress{
		StartedAt:  j.startTime,
		ReqCount:   j.Counter,
		ReqTotal:   j.Input.Total(),
		ErrorCount: j.ErrorCounter,
	}
	j.Output.Progress(prog)
}

func (j *Job) isMatch(resp Response) bool {
	matched := false
	for _, m := range j.Config.Matchers {
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
	for _, f := range j.Config.Filters {
		fv, err := f.Filter(&resp)
		if err != nil {
			continue
		}
		if fv {
			return false
		}
	}
	return true
}

func (j *Job) runTask(input map[string][]byte, position int, retried bool) {
	req, err := j.Runner.Prepare(input)
	req.Position = position
	if err != nil {
		j.Output.Error(fmt.Sprintf("Encountered an error while preparing request: %s\n", err))
		j.incError()
		log.Printf("%s", err)
		return
	}
	resp, err := j.Runner.Execute(&req)
	if err != nil {
		if retried {
			j.incError()
			log.Printf("%s", err)
		} else {
			j.runTask(input, position, true)
		}
		return
	}
	if j.SpuriousErrorCounter > 0 {
		j.resetSpuriousErrors()
	}
	if j.Config.StopOn403 || j.Config.StopOnAll {
		// Incremnt Forbidden counter if we encountered one
		if resp.StatusCode == 403 {
			j.inc403()
		}
	}
	if j.isMatch(resp) {
		j.Output.Result(resp)
		// Refresh the progress indicator as we printed something out
		j.updateProgress()
	}
	return
}

//CalibrateResponses returns slice of Responses for randomly generated filter autocalibration requests
func (j *Job) CalibrateResponses() ([]Response, error) {
	cInputs := make([]string, 0)
	if len(j.Config.AutoCalibrationStrings) < 1 {
		cInputs = append(cInputs, "admin"+RandomString(16)+"/")
		cInputs = append(cInputs, ".htaccess"+RandomString(16))
		cInputs = append(cInputs, RandomString(16)+"/")
		cInputs = append(cInputs, RandomString(16))
	} else {
		cInputs = append(cInputs, j.Config.AutoCalibrationStrings...)
	}

	results := make([]Response, 0)
	for _, input := range cInputs {
		inputs := make(map[string][]byte, 0)
		for _, v := range j.Config.InputProviders {
			inputs[v.Keyword] = []byte(input)
		}

		req, err := j.Runner.Prepare(inputs)
		if err != nil {
			j.Output.Error(fmt.Sprintf("Encountered an error while preparing request: %s\n", err))
			j.incError()
			log.Printf("%s", err)
			return results, err
		}
		resp, err := j.Runner.Execute(&req)
		if err != nil {
			return results, err
		}

		// Only calibrate on responses that would be matched otherwise
		if j.isMatch(resp) {
			results = append(results, resp)
		}
	}
	return results, nil
}

func (j *Job) CheckStop() {
	if j.Counter > 50 {
		// We have enough samples
		if j.Config.StopOn403 || j.Config.StopOnAll {
			if float64(j.Count403)/float64(j.Counter) > 0.95 {
				// Over 95% of requests are 403
				j.Error = "Getting an unusual amount of 403 responses, exiting."
				j.Stop()
			}
		}
		if j.Config.StopOnErrors || j.Config.StopOnAll {
			if j.SpuriousErrorCounter > j.Config.Threads*2 {
				// Most of the requests are erroring
				j.Error = "Receiving spurious errors, exiting."
				j.Stop()
			}

		}
	}
}

//Stop the execution of the Job
func (j *Job) Stop() {
	j.Running = false
	return
}
