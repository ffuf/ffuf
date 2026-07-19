package engine

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// Job ties together Config, Runner, Input and Output
type Job struct {
	// 64-bit atomic counters. These MUST stay first in the struct so they are
	// 8-byte aligned on 32-bit architectures, which sync/atomic requires. Access
	// only through the helper methods below, never directly.
	counter              int64
	errorCounter         int64
	spuriousErrorCounter int64
	count403             int64
	count429             int64

	// 32-bit atomic flags (0 = false, 1 = true). Access only through the helpers.
	running    int32
	runningJob int32
	skipQueue  int32

	AuditLogger  ffuf.AuditLogger
	Config       *ffuf.Config
	Input        ffuf.InputProvider
	Runner       ffuf.RunnerProvider
	ReplayRunner ffuf.RunnerProvider
	Scraper      ffuf.Scraper
	Output       ffuf.OutputProvider
	Jobhash      string
	Total        int
	Rate         *RateThrottle

	queue     *jobQueue
	recursion *recursionManager

	startTime    time.Time
	startTimeJob time.Time
	errorMsg     string
	paused       bool // guarded by pauseStateMutex

	timeMutex       sync.Mutex   // guards startTime and startTimeJob
	errMutex        sync.Mutex   // guards errorMsg
	inputMutex      sync.Mutex   // serializes main-loop input iteration vs interactive restart Reset
	calibMutex      sync.Mutex   // serializes autocalibration
	pauseStateMutex sync.Mutex   // makes the pause-flag flip and the pauseMutex Lock/Unlock one atomic step
	pauseMutex      sync.RWMutex // pause gate: workers RLock as a speed bump, Pause takes the write Lock
}

type QueueJob struct {
	Url   string
	depth int
	req   ffuf.Request
}

// jobContext carries the per-queue-job values a worker needs, passed BY VALUE so
// no worker reads mutable Job/Config state during execution. basereq is the
// immutable base request for the job; depth is its recursion depth. This is what
// makes the recursion-depth and base-request reads correct by construction rather
// than by drain timing.
type jobContext struct {
	basereq ffuf.Request
	depth   int
}

func NewJob(conf *ffuf.Config) *Job {
	var j Job
	j.Config = conf
	j.queue = newJobQueue()
	j.Rate = NewRateThrottle(conf)
	// Let the runner meter preflight/postflight requests against the same rate
	// limiter as the main dispatch loop, so -rate/-p bound total outgoing volume
	// rather than only the fuzzing requests.
	conf.RateLimitFunc = func() { <-j.Rate.RateLimiter.C }
	return &j
}

// The per-run counters and run-state flags are mutated by the execution loop and
// the worker goroutines while the progress goroutine and the interrupt handler
// read them concurrently. Counters use sync/atomic (int64), flags use atomic
// int32, so there is no mutex to forget and no lock-ordering to get wrong. The
// fields are private: the only way to touch them is through these helpers.

func (j *Job) incCounter()      { atomic.AddInt64(&j.counter, 1) }
func (j *Job) setCounter(n int) { atomic.StoreInt64(&j.counter, int64(n)) }
func (j *Job) getCounter() int  { return int(atomic.LoadInt64(&j.counter)) }

func (j *Job) getErrorCounter() int { return int(atomic.LoadInt64(&j.errorCounter)) }
func (j *Job) getSpuriousErrorCounter() int {
	return int(atomic.LoadInt64(&j.spuriousErrorCounter))
}
func (j *Job) getCount403() int { return int(atomic.LoadInt64(&j.count403)) }
func (j *Job) getCount429() int { return int(atomic.LoadInt64(&j.count429)) }

func boolToInt32(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

func (j *Job) setRunning(v bool)    { atomic.StoreInt32(&j.running, boolToInt32(v)) }
func (j *Job) isRunning() bool      { return atomic.LoadInt32(&j.running) == 1 }
func (j *Job) setRunningJob(v bool) { atomic.StoreInt32(&j.runningJob, boolToInt32(v)) }
func (j *Job) isRunningJob() bool   { return atomic.LoadInt32(&j.runningJob) == 1 }
func (j *Job) setSkipQueue(v bool)  { atomic.StoreInt32(&j.skipQueue, boolToInt32(v)) }
func (j *Job) isSkipQueue() bool    { return atomic.LoadInt32(&j.skipQueue) == 1 }

func (j *Job) setError(s string) { j.errMutex.Lock(); j.errorMsg = s; j.errMutex.Unlock() }
func (j *Job) getError() string  { j.errMutex.Lock(); defer j.errMutex.Unlock(); return j.errorMsg }

func (j *Job) setStartTime(t time.Time) { j.timeMutex.Lock(); j.startTime = t; j.timeMutex.Unlock() }
func (j *Job) getStartTime() time.Time {
	j.timeMutex.Lock()
	defer j.timeMutex.Unlock()
	return j.startTime
}
func (j *Job) setStartTimeJob(t time.Time) {
	j.timeMutex.Lock()
	j.startTimeJob = t
	j.timeMutex.Unlock()
}
func (j *Job) getStartTimeJob() time.Time {
	j.timeMutex.Lock()
	defer j.timeMutex.Unlock()
	return j.startTimeJob
}

// incError increments the error counter
func (j *Job) incError() {
	atomic.AddInt64(&j.errorCounter, 1)
	atomic.AddInt64(&j.spuriousErrorCounter, 1)
}

// inc403 increments the 403 response counter
func (j *Job) inc403() { atomic.AddInt64(&j.count403, 1) }

// inc429 increments the 429 response counter
func (j *Job) inc429() { atomic.AddInt64(&j.count429, 1) }

// resetSpuriousErrors resets the spurious error counter
func (j *Job) resetSpuriousErrors() { atomic.StoreInt64(&j.spuriousErrorCounter, 0) }

// DeleteQueueItem deletes a recursion job from the queue by its index in the slice
func (j *Job) DeleteQueueItem(index int) {
	j.queue.removeAt(index)
}

// QueuedJobs returns the slice of queued recursive jobs
func (j *Job) QueuedJobs() []QueueJob {
	return j.queue.remaining()
}

// Start the execution of the Job
func (j *Job) Start() {
	if j.getStartTime().IsZero() {
		j.setStartTime(time.Now())
	}
	// Output is wired by the assembly before Start, so the recursion manager can
	// capture it here.
	j.recursion = newRecursionManager(j.Config, j.queue, j.Output)

	basereq := ffuf.BaseRequest(j.Config)

	if j.Config.InputMode == "sniper" {
		// process multiple payload locations and create a queue job for each location
		reqs := ffuf.SniperRequests(&basereq, j.Config.InputProviders[0].Template)
		for _, r := range reqs {
			j.queue.push(QueueJob{Url: j.Config.Url, depth: 0, req: r})
		}
		j.Total = j.Input.Total() * len(reqs)
	} else {
		// Add the default job to job queue
		j.queue.push(QueueJob{Url: j.Config.Url, depth: 0, req: ffuf.BaseRequest(j.Config)})
		j.Total = j.Input.Total()
	}

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
		ctx := j.prepareQueueJob()
		j.Reset(true)
		j.setRunningJob(true)
		j.startExecution(ctx)
	}

	err := j.Output.Finalize()
	if err != nil {
		j.Output.Error(err.Error())
	}
}

// Reset resets the counters and wordlist position for a job
func (j *Job) Reset(cycle bool) {
	// inputMutex serializes this against the main loop's Input iteration, so an
	// interactive "restart" that calls Reset while workers are live cannot race
	// the input provider's internal position.
	j.inputMutex.Lock()
	j.Input.Reset()
	j.inputMutex.Unlock()
	j.setCounter(0)
	j.setSkipQueue(false)
	j.setStartTimeJob(time.Now())
	if cycle {
		j.Output.Cycle()
	} else {
		j.Output.Reset()
	}
}

func (j *Job) jobsInQueue() bool {
	return j.queue.hasNext()
}

func (j *Job) prepareQueueJob() jobContext {
	job, _ := j.queue.advance()
	// Config.Url is set here on the MAIN goroutine only, so WriteHistoryEntry below
	// (FFUFHASH) and the queued-job banner serialize the current target. Workers
	// never read it; they use the immutable jobContext returned from here.
	j.Config.Url = job.Url

	//Find all keywords present in new queued job
	kws := j.Input.Keywords()
	found_kws := make([]string, 0)
	for _, k := range kws {
		if ffuf.RequestContainsKeyword(job.req, k) {
			found_kws = append(found_kws, k)
		}
	}
	//And activate / disable inputproviders as needed
	j.Input.ActivateKeywords(found_kws)
	j.Jobhash, _ = WriteHistoryEntry(j.Config)
	return jobContext{basereq: job.req, depth: job.depth}
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

// Pause pauses the job process. The pause gate is a sync.RWMutex: while Pause
// holds the write lock, every worker blocks at its pauseCheckpoint (an RLock).
//
// pauseStateMutex makes the paused-flag flip and the pauseMutex Lock/Unlock a
// single atomic step, so there is exactly one Lock per Unlock and no goroutine
// ever unlocks an unlocked mutex (a fatal panic), however Pause, Resume, and the
// SIGINT handler interleave.
//
// Pausing is refused once the job is stopping (isRunning is false). Together with
// Stop() calling Resume(), that guarantees the gate always ends OPEN after a Stop
// regardless of ordering, so a Pause racing a Stop can never strand a worker at
// the checkpoint with no Resume left to wake it.
func (j *Job) Pause() {
	j.pauseStateMutex.Lock()
	defer j.pauseStateMutex.Unlock()
	if !j.isRunning() {
		return
	}
	if !j.paused {
		j.paused = true
		j.pauseMutex.Lock()
		j.Output.Info("------ PAUSING ------")
	}
}

// Resume resumes the job process
func (j *Job) Resume() {
	j.pauseStateMutex.Lock()
	defer j.pauseStateMutex.Unlock()
	if j.paused {
		j.paused = false
		j.pauseMutex.Unlock()
		j.Output.Info("------ RESUMING -----")
	}
}

// pauseCheckpoint blocks while the job is paused and returns immediately
// otherwise. Acquiring and releasing the read lock is a cheap speed bump when no
// pause is in effect.
func (j *Job) pauseCheckpoint() {
	j.pauseMutex.RLock()
	//nolint:staticcheck // intentional: RLock/RUnlock is a pause speed bump, not a critical section
	j.pauseMutex.RUnlock()
}

func (j *Job) startExecution(ctx jobContext) {
	var wg sync.WaitGroup
	wg.Add(1)
	go j.runBackgroundTasks(&wg)

	// Print the base URL when starting a new recursion or sniper queue job
	if j.queue.position() > 1 {
		if j.Config.InputMode == "sniper" {
			j.Output.Info(fmt.Sprintf("Starting queued sniper job (%d of %d) on target: %s", j.queue.position(), j.queue.total(), j.Config.Url))
		} else {
			j.Output.Info(fmt.Sprintf("Starting queued job on target: %s", j.Config.Url))
		}
	}

	//Limiter blocks after reaching the buffer, ensuring limited concurrency
	threadlimiter := make(chan bool, j.Config.Threads)

	for {
		// Advancing the input cursor is done under inputMutex so an interactive
		// restart (which calls Input.Reset) cannot race it.
		j.inputMutex.Lock()
		hasNext := j.Input.Next()
		j.inputMutex.Unlock()
		if !hasNext || j.isSkipQueue() {
			break
		}

		// Check if we should stop the process
		j.CheckStop()

		if !j.isRunning() {
			defer j.Output.Warning(j.getError())
			break
		}
		j.pauseCheckpoint()
		// Handle the rate & thread limiting
		threadlimiter <- true
		// Ratelimiter handles the rate ticker
		<-j.Rate.RateLimiter.C

		j.inputMutex.Lock()
		nextInput := j.Input.Value()
		nextPosition := j.Input.Position()
		j.inputMutex.Unlock()
		// Add FFUFHASH and its value
		nextInput["FFUFHASH"] = j.ffufHash(nextPosition)

		wg.Add(1)
		j.incCounter()

		go func() {
			defer func() { <-threadlimiter }()
			defer wg.Done()
			threadStart := time.Now()
			j.runTask(ctx, nextInput, nextPosition, false)
			j.sleepIfNeeded()
			threadEnd := time.Now()
			j.Rate.Tick(threadStart, threadEnd)
		}()
		if !j.isRunningJob() {
			defer j.Output.Warning(j.getError())
			// break, not return: fall through to wg.Wait() so the in-flight
			// workers finish before Start() advances to the next queue job, which
			// rewrites Config.Url / currentDepth / keyword-active state that those
			// workers still read (the -maxtime-job drain race).
			break
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
			// Stop reopens the pause gate itself, so a worker blocked at a
			// checkpoint wakes and observes the stop.
			j.Stop()
		}
	}()
}

func (j *Job) runBackgroundTasks(wg *sync.WaitGroup) {
	defer wg.Done()
	totalProgress := j.Input.Total()
	for j.getCounter() <= totalProgress && !j.isSkipQueue() {
		j.pauseCheckpoint()
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
	prog := ffuf.Progress{
		StartedAt:  j.getStartTimeJob(),
		ReqCount:   j.getCounter(),
		ReqTotal:   j.Input.Total(),
		ReqSec:     j.Rate.CurrentRate(),
		QueuePos:   j.queue.position(),
		QueueTotal: j.queue.total(),
		ErrorCount: j.getErrorCounter(),
	}
	j.Output.Progress(prog)
}

// isMatch delegates the match/filter decision to the MatcherManager seam, adapting
// the Config fields it needs. The logic itself now lives in filter.MatcherManager.
func (j *Job) isMatch(resp ffuf.Response) bool {
	return j.Config.MatcherManager.Matches(&resp, j.Config.AutoCalibrationPerHost, j.Config.MatcherMode, j.Config.FilterMode)
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

func (j *Job) runTask(ctx jobContext, input map[string][]byte, position int, retried bool) {
	basereq := ctx.basereq
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
		if !retried {
			// Retry once. The timeout messaging below runs only on the final
			// failure, so a request that recovers on retry does not also print a
			// spurious timeout notice.
			j.runTask(ctx, input, position, true)
			return
		}
		j.incError()
		log.Printf("%s", err)
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
	j.pauseCheckpoint()

	// Handle autocalibration, must be done after the actual request to ensure sane value in req.Host
	_ = j.CalibrateIfNeeded(ffuf.HostURLFromRequest(req), input)

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
			j.recursion.handleGreedy(ctx, resp)
		}
	} else {
		if len(resp.ScraperData) > 0 {
			// print the result anyway, as scraper found something
			j.Output.Result(resp)
		}
	}

	if j.Config.Recursion && j.Config.RecursionStrategy == "default" && len(resp.GetRedirectLocation(false)) > 0 {
		j.recursion.handleDefault(ctx, resp)
	}
}

func (j *Job) handleScraperResult(resp *ffuf.Response, sres ffuf.ScraperResult) {
	for _, a := range sres.Action {
		switch a {
		case "output":
			resp.ScraperData[sres.Name] = sres.Results
		}
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
		dur := time.Since(j.getStartTime())
		runningSecs := int(dur / time.Second)
		if runningSecs >= j.Config.MaxTime {
			j.setError("Maximum running time for entire process reached, exiting.")
			j.Stop()
		}
	}

	// Check for runtime of current job
	if j.Config.MaxTimeJob > 0 {
		dur := time.Since(j.getStartTimeJob())
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
	// Reopen the pause gate so no worker is left blocked at a checkpoint after a
	// stop. Resume is a no-op when the job is not paused, and Pause refuses to
	// close the gate once isRunning is false, so the gate always ends open.
	j.Resume()
}

// Stop current, resume to next
func (j *Job) Next() {
	j.setRunningJob(false)
}
