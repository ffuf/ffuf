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

// Job ties together Config, Runner, Input and Output
type Job struct {
	AuditLogger          AuditLogger
	Config               *Config
	ErrorMutex           sync.Mutex
	Input                InputProvider
	Runner               RunnerProvider
	ReplayRunner         RunnerProvider
	Scraper              Scraper
	Output               OutputProvider
	Jobhash              string
	Counter              int
	ErrorCounter         int
	SpuriousErrorCounter int
	Total                int
	Running              bool
	RunningJob           bool
	Paused               bool
	Count403             int
	Count429             int
	Error                string
	Rate                 *RateThrottle
	startTime            time.Time
	startTimeJob         time.Time
	queuejobs            []QueueJob
	queuepos             int
	skipQueue            bool
	currentDepth         int
	calibMutex           sync.Mutex
	pauseWg              sync.WaitGroup
}

type QueueJob struct {
	Url   string
	depth int
	req   Request
}

func NewJob(conf *Config) *Job {
	var j Job
	j.Config = conf
	j.Counter = 0
	j.ErrorCounter = 0
	j.SpuriousErrorCounter = 0
	j.Running = false
	j.RunningJob = false
	j.Paused = false
	j.queuepos = 0
	j.queuejobs = make([]QueueJob, 0)
	j.currentDepth = 0
	j.Rate = NewRateThrottle(conf)
	j.skipQueue = false
	return &j
}

// The per-run counters and run-state flags below are mutated by the execution
// loop and worker goroutines while the progress goroutine and the interrupt
// handler read them concurrently. ErrorMutex serializes every access. All access
// goes through these leaf helpers, which never call one another, so they cannot
// deadlock (including CheckStop, which reads via getters and then calls Stop).

func (j *Job) incCounter()      { j.ErrorMutex.Lock(); j.Counter++; j.ErrorMutex.Unlock() }
func (j *Job) setCounter(n int) { j.ErrorMutex.Lock(); j.Counter = n; j.ErrorMutex.Unlock() }
func (j *Job) getCounter() int  { j.ErrorMutex.Lock(); defer j.ErrorMutex.Unlock(); return j.Counter }

func (j *Job) getErrorCounter() int {
	j.ErrorMutex.Lock()
	defer j.ErrorMutex.Unlock()
	return j.ErrorCounter
}
func (j *Job) getSpuriousErrorCounter() int {
	j.ErrorMutex.Lock()
	defer j.ErrorMutex.Unlock()
	return j.SpuriousErrorCounter
}
func (j *Job) getCount403() int { j.ErrorMutex.Lock(); defer j.ErrorMutex.Unlock(); return j.Count403 }
func (j *Job) getCount429() int { j.ErrorMutex.Lock(); defer j.ErrorMutex.Unlock(); return j.Count429 }

func (j *Job) setRunning(v bool)    { j.ErrorMutex.Lock(); j.Running = v; j.ErrorMutex.Unlock() }
func (j *Job) isRunning() bool      { j.ErrorMutex.Lock(); defer j.ErrorMutex.Unlock(); return j.Running }
func (j *Job) setRunningJob(v bool) { j.ErrorMutex.Lock(); j.RunningJob = v; j.ErrorMutex.Unlock() }
func (j *Job) isRunningJob() bool {
	j.ErrorMutex.Lock()
	defer j.ErrorMutex.Unlock()
	return j.RunningJob
}
func (j *Job) setPaused(v bool) { j.ErrorMutex.Lock(); j.Paused = v; j.ErrorMutex.Unlock() }
func (j *Job) isPaused() bool   { j.ErrorMutex.Lock(); defer j.ErrorMutex.Unlock(); return j.Paused }
func (j *Job) setSkipQueue(v bool) {
	j.ErrorMutex.Lock()
	j.skipQueue = v
	j.ErrorMutex.Unlock()
}
func (j *Job) isSkipQueue() bool {
	j.ErrorMutex.Lock()
	defer j.ErrorMutex.Unlock()
	return j.skipQueue
}
func (j *Job) setError(s string) { j.ErrorMutex.Lock(); j.Error = s; j.ErrorMutex.Unlock() }
func (j *Job) getError() string  { j.ErrorMutex.Lock(); defer j.ErrorMutex.Unlock(); return j.Error }

// incError increments the error counter
func (j *Job) incError() {
	j.ErrorMutex.Lock()
	defer j.ErrorMutex.Unlock()
	j.ErrorCounter++
	j.SpuriousErrorCounter++
}

// inc403 increments the 403 response counter
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

// resetSpuriousErrors resets the spurious error counter
func (j *Job) resetSpuriousErrors() {
	j.ErrorMutex.Lock()
	defer j.ErrorMutex.Unlock()
	j.SpuriousErrorCounter = 0
}

// DeleteQueueItem deletes a recursion job from the queue by its index in the slice
func (j *Job) DeleteQueueItem(index int) {
	index = j.queuepos + index - 1
	j.queuejobs = append(j.queuejobs[:index], j.queuejobs[index+1:]...)
}

// QueuedJobs returns the slice of queued recursive jobs
func (j *Job) QueuedJobs() []QueueJob {
	return j.queuejobs[j.queuepos-1:]
}

// Start the execution of the Job
func (j *Job) Start() {
	if j.startTime.IsZero() {
		j.startTime = time.Now()
	}

	basereq := BaseRequest(j.Config)

	if j.Config.InputMode == "sniper" {
		// process multiple payload locations and create a queue job for each location
		reqs := SniperRequests(&basereq, j.Config.InputProviders[0].Template)
		for _, r := range reqs {
			j.queuejobs = append(j.queuejobs, QueueJob{Url: j.Config.Url, depth: 0, req: r})
		}
		j.Total = j.Input.Total() * len(reqs)
	} else {
		// Add the default job to job queue
		j.queuejobs = append(j.queuejobs, QueueJob{Url: j.Config.Url, depth: 0, req: BaseRequest(j.Config)})
		j.Total = j.Input.Total()
	}

	rand.Seed(time.Now().UnixNano())
	defer j.Stop()

	j.setRunning(true)
	j.setRunningJob(true)
	//Show banner if not running in silent mode
	if !j.Config.Quiet {
		j.Output.Banner()
	}
	// Monitor for SIGTERM and do cleanup properly (writing the output files etc)
	j.interruptMonitor()
	for j.jobsInQueue() {
		j.prepareQueueJob()
		j.Reset(true)
		j.setRunningJob(true)
		j.startExecution()
	}

	err := j.Output.Finalize()
	if err != nil {
		j.Output.Error(err.Error())
	}
}

// Reset resets the counters and wordlist position for a job
func (j *Job) Reset(cycle bool) {
	j.Input.Reset()
	j.setCounter(0)
	j.setSkipQueue(false)
	j.startTimeJob = time.Now()
	if cycle {
		j.Output.Cycle()
	} else {
		j.Output.Reset()
	}
}

func (j *Job) jobsInQueue() bool {
	return j.queuepos < len(j.queuejobs)
}

func (j *Job) prepareQueueJob() {
	j.Config.Url = j.queuejobs[j.queuepos].Url
	j.currentDepth = j.queuejobs[j.queuepos].depth

	//Find all keywords present in new queued job
	kws := j.Input.Keywords()
	found_kws := make([]string, 0)
	for _, k := range kws {
		if RequestContainsKeyword(j.queuejobs[j.queuepos].req, k) {
			found_kws = append(found_kws, k)
		}
	}
	//And activate / disable inputproviders as needed
	j.Input.ActivateKeywords(found_kws)
	j.queuepos += 1
	j.Jobhash, _ = WriteHistoryEntry(j.Config)
}

// SkipQueue allows to skip the current job and advance to the next queued recursion job
func (j *Job) SkipQueue() {
	j.setSkipQueue(true)
}

func (j *Job) sleepIfNeeded() {
	var sleepDuration time.Duration
	if j.Config.Delay.HasDelay {
		if j.Config.Delay.IsRange {
			sTime := j.Config.Delay.Min + rand.Float64()*(j.Config.Delay.Max-j.Config.Delay.Min)
			sleepDuration = time.Duration(sTime * 1000)
		} else {
			sleepDuration = time.Duration(j.Config.Delay.Min * 1000)
		}
		sleepDuration = sleepDuration * time.Millisecond
	}
	// makes the sleep cancellable by context
	select {
	case <-j.Config.Context.Done(): // cancelled
	case <-time.After(sleepDuration): // sleep
	}
}

// Pause pauses the job process
func (j *Job) Pause() {
	if !j.isPaused() {
		j.setPaused(true)
		j.pauseWg.Add(1)
		j.Output.Info("------ PAUSING ------")
	}
}

// Resume resumes the job process
func (j *Job) Resume() {
	if j.isPaused() {
		j.setPaused(false)
		j.Output.Info("------ RESUMING -----")
		j.pauseWg.Done()
	}
}

func (j *Job) startExecution() {
	var wg sync.WaitGroup
	wg.Add(1)
	go j.runBackgroundTasks(&wg)

	// Print the base URL when starting a new recursion or sniper queue job
	if j.queuepos > 1 {
		if j.Config.InputMode == "sniper" {
			j.Output.Info(fmt.Sprintf("Starting queued sniper job (%d of %d) on target: %s", j.queuepos, len(j.queuejobs), j.Config.Url))
		} else {
			j.Output.Info(fmt.Sprintf("Starting queued job on target: %s", j.Config.Url))
		}
	}

	//Limiter blocks after reaching the buffer, ensuring limited concurrency
	threadlimiter := make(chan bool, j.Config.Threads)

	for j.Input.Next() && !j.isSkipQueue() {
		// Check if we should stop the process
		j.CheckStop()

		if !j.isRunning() {
			defer j.Output.Warning(j.getError())
			break
		}
		j.pauseWg.Wait()
		// Handle the rate & thread limiting
		threadlimiter <- true
		// Ratelimiter handles the rate ticker
		<-j.Rate.RateLimiter.C
		nextInput := j.Input.Value()
		nextPosition := j.Input.Position()
		// Add FFUFHASH and its value
		nextInput["FFUFHASH"] = j.ffufHash(nextPosition)

		wg.Add(1)
		j.incCounter()

		go func() {
			defer func() { <-threadlimiter }()
			defer wg.Done()
			threadStart := time.Now()
			j.runTask(nextInput, nextPosition, false)
			j.sleepIfNeeded()
			threadEnd := time.Now()
			j.Rate.Tick(threadStart, threadEnd)
		}()
		if !j.isRunningJob() {
			defer j.Output.Warning(j.getError())
			return
		}
	}
	wg.Wait()
	j.updateProgress()
}

func (j *Job) interruptMonitor() {
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range sigChan {
			j.setError("Caught keyboard interrupt (Ctrl-C)\n")
			// resume if paused
			if j.isPaused() {
				j.pauseWg.Done()
			}
			// Stop the job
			j.Stop()
		}
	}()
}

func (j *Job) runBackgroundTasks(wg *sync.WaitGroup) {
	defer wg.Done()
	totalProgress := j.Input.Total()
	for j.getCounter() <= totalProgress && !j.isSkipQueue() {
		j.pauseWg.Wait()
		if !j.isRunning() {
			break
		}
		j.updateProgress()
		if j.getCounter() == totalProgress {
			return
		}
		if !j.isRunningJob() {
			return
		}
		time.Sleep(time.Millisecond * time.Duration(j.Config.ProgressFrequency))
	}
}

func (j *Job) updateProgress() {
	prog := Progress{
		StartedAt:  j.startTimeJob,
		ReqCount:   j.getCounter(),
		ReqTotal:   j.Input.Total(),
		ReqSec:     j.Rate.CurrentRate(),
		QueuePos:   j.queuepos,
		QueueTotal: len(j.queuejobs),
		ErrorCount: j.getErrorCounter(),
	}
	j.Output.Progress(prog)
}

func (j *Job) isMatch(resp Response) bool {
	matched := false
	var matchers map[string]FilterProvider
	var filters map[string]FilterProvider
	if j.Config.AutoCalibrationPerHost {
		filters = j.Config.MatcherManager.FiltersForDomain(HostURLFromRequest(*resp.Request))
	} else {
		filters = j.Config.MatcherManager.GetFilters()
	}
	matchers = j.Config.MatcherManager.GetMatchers()
	for _, m := range matchers {
		match, err := m.Filter(&resp)
		if err != nil {
			continue
		}
		if match {
			matched = true
		} else if j.Config.MatcherMode == "and" {
			// we already know this isn't "and" match
			return false

		}
	}
	// The response was not matched, return before running filters
	if !matched {
		return false
	}
	for _, f := range filters {
		fv, err := f.Filter(&resp)
		if err != nil {
			continue
		}
		if fv {
			//	return false
			if j.Config.FilterMode == "or" {
				// return early, as filter matched
				return false
			}
		} else {
			if j.Config.FilterMode == "and" {
				// return early as not all filters matched in "and" mode
				return true
			}
		}
	}
	if len(filters) > 0 && j.Config.FilterMode == "and" {
		// we did not return early, so all filters were matched
		return false
	}
	return true
}

func (j *Job) ffufHash(pos int) []byte {
	hashstring := ""
	r := []rune(j.Jobhash)
	if len(r) > 5 {
		hashstring = string(r[:5])
	}
	hashstring += fmt.Sprintf("%x", pos)
	return []byte(hashstring)
}

func (j *Job) runTask(input map[string][]byte, position int, retried bool) {
	basereq := j.queuejobs[j.queuepos-1].req
	req, err := j.Runner.Prepare(input, &basereq)
	req.Timestamp = time.Now()

	req.Position = position
	if err != nil {
		j.Output.Error(fmt.Sprintf("Encountered an error while preparing request: %s\n", err))
		j.incError()
		log.Printf("%s", err)
		return
	}

	resp, err := j.Runner.Execute(&req)
	if err != nil {
		req.Error = err.Error()
	}

	// Audit the request after sending to the runner so we get any changes
	if j.AuditLogger != nil {
		e := j.AuditLogger.Write(&req)
		if e != nil {
			j.Output.Error(fmt.Sprintf("Encountered error while writing request audit log: %s\n", e))
		}
	}

	if err != nil {
		if retried {
			j.incError()
			log.Printf("%s", err)
		} else {
			j.runTask(input, position, true)
		}
		if os.IsTimeout(err) {
			for name := range j.Config.MatcherManager.GetMatchers() {
				if name == "time" {
					inputmsg := ""
					for k, v := range input {
						inputmsg = inputmsg + fmt.Sprintf("%s : %s  // ", k, v)
					}
					j.Output.Info("Timeout while 'time' matcher is active: " + inputmsg)
					return
				}
			}
			for name := range j.Config.MatcherManager.GetFilters() {
				if name == "time" {
					inputmsg := ""
					for k, v := range input {
						inputmsg = inputmsg + fmt.Sprintf("%s : %s  // ", k, v)
					}
					j.Output.Info("Timeout while 'time' filter is active: " + inputmsg)
					return
				}
			}
		}
		return
	}

	// audit the response after the error handling
	if j.AuditLogger != nil {
		err = j.AuditLogger.Write(&resp)
		if err != nil {
			j.Output.Error(fmt.Sprintf("Encountered error while writing response audit log: %s\n", err))
		}
	}

	if j.getSpuriousErrorCounter() > 0 {
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
		if resp.StatusCode == 429 {
			j.inc429()
		}
	}
	j.pauseWg.Wait()

	// Handle autocalibration, must be done after the actual request to ensure sane value in req.Host
	_ = j.CalibrateIfNeeded(HostURLFromRequest(req), input)

	// Handle scraper actions
	if j.Scraper != nil {
		for _, sres := range j.Scraper.Execute(&resp, j.isMatch(resp)) {
			resp.ScraperData[sres.Name] = sres.Results
			j.handleScraperResult(&resp, sres)
		}
	}

	if j.isMatch(resp) {
		// Re-send request through replay-proxy if needed
		if j.ReplayRunner != nil {
			replayreq, err := j.ReplayRunner.Prepare(input, &basereq)
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
		if j.Config.Recursion && j.Config.RecursionStrategy == "greedy" {
			j.handleGreedyRecursionJob(resp)
		}
	} else {
		if len(resp.ScraperData) > 0 {
			// print the result anyway, as scraper found something
			j.Output.Result(resp)
		}
	}

	if j.Config.Recursion && j.Config.RecursionStrategy == "default" && len(resp.GetRedirectLocation(false)) > 0 {
		j.handleDefaultRecursionJob(resp)
	}
}

func (j *Job) handleScraperResult(resp *Response, sres ScraperResult) {
	for _, a := range sres.Action {
		switch a {
		case "output":
			resp.ScraperData[sres.Name] = sres.Results
		}
	}
}

// handleGreedyRecursionJob adds a recursion job to the queue if the maximum depth has not been reached
func (j *Job) handleGreedyRecursionJob(resp Response) {
	// Handle greedy recursion strategy. Match has been determined before calling handleRecursionJob
	if j.Config.RecursionDepth == 0 || j.currentDepth < j.Config.RecursionDepth {
		recUrl := resp.Request.Url + "/" + "FUZZ"
		newJob := QueueJob{Url: recUrl, depth: j.currentDepth + 1, req: RecursionRequest(j.Config, recUrl)}
		j.queuejobs = append(j.queuejobs, newJob)
		j.Output.Info(fmt.Sprintf("Adding a new job to the queue: %s", recUrl))
	} else {
		j.Output.Warning(fmt.Sprintf("Maximum recursion depth reached. Ignoring: %s", resp.Request.Url))
	}
}

// handleDefaultRecursionJob adds a new recursion job to the job queue if a new directory is found and maximum depth has
// not been reached
func (j *Job) handleDefaultRecursionJob(resp Response) {
	recUrl := resp.Request.Url + "/" + "FUZZ"
	if (resp.Request.Url + "/") != resp.GetRedirectLocation(true) {
		// Not a directory, return early
		return
	}
	if j.Config.RecursionDepth == 0 || j.currentDepth < j.Config.RecursionDepth {
		// We have yet to reach the maximum recursion depth
		newJob := QueueJob{Url: recUrl, depth: j.currentDepth + 1, req: RecursionRequest(j.Config, recUrl)}
		j.queuejobs = append(j.queuejobs, newJob)
		j.Output.Info(fmt.Sprintf("Adding a new job to the queue: %s", recUrl))
	} else {
		j.Output.Warning(fmt.Sprintf("Directory found, but recursion depth exceeded. Ignoring: %s", resp.GetRedirectLocation(true)))
	}
}

// CheckStop stops the job if stopping conditions are met
func (j *Job) CheckStop() {
	counter := j.getCounter()
	if counter > 50 {
		// We have enough samples
		if j.Config.StopOn403 || j.Config.StopOnAll {
			if float64(j.getCount403())/float64(counter) > 0.95 {
				// Over 95% of requests are 403
				j.setError("Getting an unusual amount of 403 responses, exiting.")
				j.Stop()
			}
		}
		if j.Config.StopOnErrors || j.Config.StopOnAll {
			if j.getSpuriousErrorCounter() > j.Config.Threads*2 {
				// Most of the requests are erroring
				j.setError("Receiving spurious errors, exiting.")
				j.Stop()
			}

		}
		if j.Config.StopOnAll && (float64(j.getCount429())/float64(counter) > 0.2) {
			// Over 20% of responses are 429
			j.setError("Getting an unusual amount of 429 responses, exiting.")
			j.Stop()
		}
	}

	// Check for runtime of entire process
	if j.Config.MaxTime > 0 {
		dur := time.Since(j.startTime)
		runningSecs := int(dur / time.Second)
		if runningSecs >= j.Config.MaxTime {
			j.setError("Maximum running time for entire process reached, exiting.")
			j.Stop()
		}
	}

	// Check for runtime of current job
	if j.Config.MaxTimeJob > 0 {
		dur := time.Since(j.startTimeJob)
		runningSecs := int(dur / time.Second)
		if runningSecs >= j.Config.MaxTimeJob {
			j.setError("Maximum running time for this job reached, continuing with next job if one exists.")
			j.Next()

		}
	}
}

// Stop the execution of the Job
func (j *Job) Stop() {
	j.setRunning(false)
	j.Config.Cancel()
}

// Stop current, resume to next
func (j *Job) Next() {
	j.setRunningJob(false)
}
