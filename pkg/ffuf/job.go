package ffuf

import (
	"fmt"
	"sync"
)

//Job ties together Config, Runner, Input and Output
type Job struct {
	Config  *Config
	Input   InputProvider
	Runner  RunnerProvider
	Output  OutputProvider
	Counter int
	Running bool
}

func NewJob(conf *Config) Job {
	var j Job
	j.Counter = 0
	j.Running = false
	return j
}

//Start the execution of the Job
func (j *Job) Start() {
	defer j.Stop()
	if !j.Config.Quiet {
		j.Output.Banner()
	}
	j.Running = true
	var wg sync.WaitGroup
	//Limiter blocks after reaching the buffer, ensuring limited concurrency
	limiter := make(chan bool, j.Config.Threads)
	for j.Input.Next() {
		limiter <- true
		wg.Add(1)
		nextInput := j.Input.Value()
		go func() {
			defer func() { <-limiter }()
			defer wg.Done()
			j.runTask([]byte(nextInput))
		}()
	}
	wg.Wait()
	return
}

func (j *Job) runTask(input []byte) {
	req, err := j.Runner.Prepare(input)
	if err != nil {
		fmt.Printf("Encountered error while preparing request: %s\n", err)
		return
	}
	resp, err := j.Runner.Execute(&req)
	if err != nil {
		fmt.Printf("Error in runner: %s\n", err)
		return
	}
	j.Output.Result(resp)
	return
}

//Stop the execution of the Job
func (j *Job) Stop() {
	j.Running = false
	return
}
