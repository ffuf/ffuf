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
	ReplayRunner         RunnerProvider
	Output               OutputProvider
	Counter              int
	ErrorCounter         int
	SpuriousErrorCounter int
	Total                int
	Running              bool
	RunningJob           bool
	Count403             int
	Count429             int
	Error                string
	startTime            time.Time
	startTimeJob         time.Time
	queuejobs            []QueueJob
	queuepos             int
	currentDepth         int
}

type QueueJob struct {
	Url   string
	depth int
}

func NewJob(conf *Config) Job {
	var j Job
	j.Counter = 0
	j.ErrorCounter = 0
	j.SpuriousErrorCounter = 0
	j.Running = false
	j.RunningJob = false
	j.queuepos = 0
	j.queuejobs = make([]QueueJob, 0)
	j.currentDepth = 0
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

// inc429 increments the 429 response counter
func (j *Job) inc429() {
	j.ErrorMutex.Lock()
	defer j.ErrorMutex.Unlock()
	j.Count429++
}

//resetSpuriousErrors resets the spurious error counter
func (j *Job) resetSpuriousErrors() {
	j.ErrorMutex.Lock()
	defer j.ErrorMutex.Unlock()
	j.SpuriousErrorCounter = 0
}

//Start the execution of the Job
func (j *Job) Start() {
	if j.startTime.IsZero() {
		j.startTime = time.Now()
	}

	// Add the default job to job queue
	j.queuejobs = append(j.queuejobs, QueueJob{Url: j.Config.Url, depth: 0})
	rand.Seed(time.Now().UnixNano())
	j.Total = j.Input.Total()
	defer j.Stop()

	j.Running = true
	j.RunningJob = true
	//Show banner if not running in silent mode
	if !j.Config.Quiet {
		j.Output.Banner()
	}
	// Monitor for SIGTERM and do cleanup properly (writing the output files etc)
	j.interruptMonitor()
	for j.jobsInQueue() {
		j.prepareQueueJob()

		if j.queuepos > 1 && !j.RunningJob {
			// Print info for queued recursive jobs
			j.Output.Info(fmt.Sprintf("Scanning: %s", j.Config.Url))
		}
		j.Input.Reset()
		j.startTimeJob = time.Now()
		j.RunningJob = true
		j.Counter = 0
		j.startExecution()
	}

	j.Output.Finalize()
}

func (j *Job) jobsInQueue() bool {
	if j.queuepos < len(j.queuejobs) {
		return true
	}
	return false
}

func (j *Job) prepareQueueJob() {
	j.Config.Url = j.queuejobs[j.queuepos].Url
	j.currentDepth = j.queuejobs[j.queuepos].depth
	j.queuepos += 1
}

func (j *Job) startExecution() {
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

		if !j.RunningJob {
			defer j.Output.Warning(j.Error)
			return
		}
	}
	wg.Wait()
	j.updateProgress()
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
	totalProgress := j.Input.Total()
	for j.Counter <= totalProgress {

		if !j.Running {
			break
		}

		j.updateProgress()
		if j.Counter == totalProgress {
			return
		}

		if !j.RunningJob {
			return
		}

		time.Sleep(time.Millisecond * time.Duration(j.Config.ProgressFrequency))
	}
}

func (j *Job) updateProgress() {
	prog := Progress{
		StartedAt:  j.startTimeJob,
		ReqCount:   j.Counter,
		ReqTotal:   j.Input.Total(),
		QueuePos:   j.queuepos,
		QueueTotal: len(j.queuejobs),
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
		// Increment Forbidden counter if we encountered one
		if resp.StatusCode == 403 {
			j.inc403()
		}
	}
	if j.Config.StopOnAll {
		// increment 429 counter if the response code is 429
		if j.Config.StopOnAll {
			if resp.StatusCode == 429 {
				j.inc429()
			}
		}
	}
	if j.isMatch(resp) {
		// Re-send request through replay-proxy if needed
		if j.ReplayRunner != nil {
			replayreq, err := j.ReplayRunner.Prepare(input)
			replayreq.Position = position
			if err != nil {
				j.Output.Error(fmt.Sprintf("Encountered an error while preparing replayproxy request: %s\n", err))
				j.incError()
				log.Printf("%s", err)
			} else {
				_, _ = j.ReplayRunner.Execute(&replayreq)
			}
		}
		j.Output.Result(resp)
		// Refresh the progress indicator as we printed something out
		j.updateProgress()
	}

	if j.Config.Recursion && len(resp.GetRedirectLocation(false)) > 0 {
		j.handleRecursionJob(resp)
	}
	return
}

//handleRecursionJob adds a new recursion job to the job queue if a new directory is found
func (j *Job) handleRecursionJob(resp Response) {
	if (resp.Request.Url + "/") != resp.GetRedirectLocation(true) {
		// Not a directory, return early
		return
	}
	if j.Config.RecursionDepth == 0 || j.currentDepth < j.Config.RecursionDepth {
		// We have yet to reach the maximum recursion depth
		recUrl := resp.Request.Url + "/" + "FUZZ"
		newJob := QueueJob{Url: recUrl, depth: j.currentDepth + 1}
		j.queuejobs = append(j.queuejobs, newJob)
		j.Output.Info(fmt.Sprintf("Adding a new job to the queue: %s", recUrl))
	} else {
		j.Output.Warning(fmt.Sprintf("Directory found, but recursion depth exceeded. Ignoring: %s", resp.GetRedirectLocation(true)))
	}
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

// CheckStop stops the job if stopping conditions are met
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
		if j.Config.StopOnAll && (float64(j.Count429)/float64(j.Counter) > 0.2) {
			// Over 20% of responses are 429
			j.Error = "Getting an unusual amount of 429 responses, exiting."
			j.Stop()
		}
	}

	// Check for runtime of entire process
	if j.Config.MaxTime > 0 {
		dur := time.Now().Sub(j.startTime)
		runningSecs := int(dur / time.Second)
		if runningSecs >= j.Config.MaxTime {
			j.Error = "Maximum running time for entire process reached, exiting."
			j.Stop()
		}
	}

	// Check for runtime of current job
	if j.Config.MaxTimeJob > 0 {
		dur := time.Now().Sub(j.startTimeJob)
		runningSecs := int(dur / time.Second)
		if runningSecs >= j.Config.MaxTimeJob {
			j.Error = "Maximum running time for this job reached, continuing with next job if one exists."
			j.Next()

		}
	}
}

//Stop the execution of the Job
func (j *Job) Stop() {
	j.Running = false
	return
}

//Stop current, resume to next
func (j *Job) Next() {
	j.RunningJob = false
	return
}
