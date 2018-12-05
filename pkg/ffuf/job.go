package ffuf

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

//Job ties together Config, Runner, Input and Output
type Job struct {
	Config    *Config
	Input     InputProvider
	Runner    RunnerProvider
	Output    OutputProvider
	Counter   int
	Total     int
	Running   bool
	startTime time.Time
}

func NewJob(conf *Config) Job {
	var j Job
	j.Counter = 0
	j.Running = false
	return j
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
	var wg sync.WaitGroup
	wg.Add(1)
	go j.runProgress(&wg)
	//Limiter blocks after reaching the buffer, ensuring limited concurrency
	limiter := make(chan bool, j.Config.Threads)
	for j.Input.Next() {
		limiter <- true
		nextInput := j.Input.Value()
		wg.Add(1)
		j.Counter++
		go func() {
			defer func() { <-limiter }()
			defer wg.Done()
			j.runTask([]byte(nextInput))
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

func (j *Job) runProgress(wg *sync.WaitGroup) {
	defer wg.Done()
	j.startTime = time.Now()
	totalProgress := j.Input.Total()
	for j.Counter <= totalProgress {
		j.updateProgress()
		if j.Counter == totalProgress {
			return
		}
		time.Sleep(time.Millisecond * 100)
	}
}

func (j *Job) updateProgress() {
	//TODO: refactor to use a defined progress struct for future output modules
	if j.Config.Quiet {
		// Do not print progress status in silent mode
		return
	}
	runningSecs := int((time.Now().Sub(j.startTime)) / time.Second)
	var reqRate int
	if runningSecs > 0 {
		reqRate = int(j.Counter / runningSecs)
	} else {
		reqRate = 0
	}
	dur := time.Now().Sub(j.startTime)
	hours := dur / time.Hour
	dur -= hours * time.Hour
	mins := dur / time.Minute
	dur -= mins * time.Minute
	secs := dur / time.Second
	progString := fmt.Sprintf(":: Progress: [%d/%d] :: %d req/sec :: Duration: [%d:%02d:%02d] ::", j.Counter, j.Total, int(reqRate), hours, mins, secs)
	j.Output.Error(progString)
}

func (j *Job) runTask(input []byte) {
	req, err := j.Runner.Prepare(input)
	if err != nil {
		j.Output.Error(fmt.Sprintf("Encountered error while preparing request: %s\n", err))
		return
	}
	resp, err := j.Runner.Execute(&req)
	if err != nil {
		j.Output.Error(fmt.Sprintf("Error in runner: %s\n", err))
		return
	}
	if j.Output.Result(resp) {
		// Refresh the progress indicator as we printed something out
		j.updateProgress()
	}
	return
}

//Stop the execution of the Job
func (j *Job) Stop() {
	j.Running = false
	return
}
