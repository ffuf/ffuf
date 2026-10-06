package engine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

type retryRunner struct {
	mu           sync.Mutex
	failures     int
	prepareCalls int
	executeCalls int
	executeCh    chan int
}

func (r *retryRunner) Prepare(input map[string][]byte, base *ffuf.Request) (ffuf.Request, error) {
	r.mu.Lock()
	r.prepareCalls++
	r.mu.Unlock()
	req := ffuf.CopyRequest(base)
	req.Input = input
	return req, nil
}

func (r *retryRunner) Execute(req *ffuf.Request) (ffuf.Response, error) {
	r.mu.Lock()
	r.executeCalls++
	call := r.executeCalls
	executeCh := r.executeCh
	r.mu.Unlock()
	if executeCh != nil {
		executeCh <- call
	}
	if call <= r.failures {
		return ffuf.Response{}, errors.New("synthetic transport error")
	}
	return ffuf.Response{Request: req, StatusCode: 500}, nil
}

func (*retryRunner) Dump(*ffuf.Request) ([]byte, error) { return nil, nil }

func (r *retryRunner) calls() (prepare, execute int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.prepareCalls, r.executeCalls
}

type retryAudit struct {
	mu        sync.Mutex
	requests  int
	responses int
}

func (*retryAudit) Close() {}

func (a *retryAudit) Write(value interface{}) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch value.(type) {
	case *ffuf.Request:
		a.requests++
	case *ffuf.Response:
		a.responses++
	}
	return nil
}

func (a *retryAudit) counts() (requests, responses int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.requests, a.responses
}

func retryTestJob(retries, failures int) (*Job, *retryRunner, *retryAudit, *int) {
	runner := &retryRunner{failures: failures}
	audit := &retryAudit{}
	rateWaits := 0
	conf := &ffuf.Config{
		Context:        context.Background(),
		Retries:        retries,
		MatcherManager: &fakeMatcherManager{},
		RateLimitFunc:  func() { rateWaits++ },
	}
	job := &Job{
		Config:      conf,
		Runner:      runner,
		AuditLogger: audit,
		Output:      NewNullOutput(),
	}
	return job, runner, audit, &rateWaits
}

func TestRunTask_RetryLimitAndRateAccounting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		retries int
		want    int
	}{
		{name: "disabled", retries: 0, want: 1},
		{name: "current default", retries: 1, want: 2},
		{name: "configured", retries: 3, want: 4},
		{name: "defensive negative", retries: -1, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			job, runner, audit, rateWaits := retryTestJob(tc.retries, tc.want+1)
			job.runTask(jobContext{basereq: ffuf.Request{Url: "http://example.test/FUZZ"}}, map[string][]byte{"FUZZ": []byte("value")}, 1)

			prepare, execute := runner.calls()
			if prepare != tc.want || execute != tc.want {
				t.Fatalf("calls = prepare:%d execute:%d, want %d each", prepare, execute, tc.want)
			}
			if *rateWaits != tc.want-1 {
				t.Errorf("retry rate waits = %d, want %d", *rateWaits, tc.want-1)
			}
			requests, responses := audit.counts()
			if requests != tc.want || responses != 0 {
				t.Errorf("audit writes = requests:%d responses:%d, want %d and 0", requests, responses, tc.want)
			}
			if job.getErrorCounter() != 1 {
				t.Errorf("final error count = %d, want 1", job.getErrorCounter())
			}
		})
	}
}

func TestRunTask_StopsRetryingAfterSuccess(t *testing.T) {
	job, runner, audit, rateWaits := retryTestJob(3, 2)
	job.runTask(jobContext{basereq: ffuf.Request{Url: "http://example.test/FUZZ"}}, map[string][]byte{"FUZZ": []byte("value")}, 1)

	prepare, execute := runner.calls()
	if prepare != 3 || execute != 3 {
		t.Fatalf("calls = prepare:%d execute:%d, want 3 each", prepare, execute)
	}
	if *rateWaits != 2 {
		t.Errorf("retry rate waits = %d, want 2", *rateWaits)
	}
	requests, responses := audit.counts()
	if requests != 3 || responses != 1 {
		t.Errorf("audit writes = requests:%d responses:%d, want 3 and 1", requests, responses)
	}
	if job.getErrorCounter() != 0 {
		t.Errorf("final error count = %d, want 0", job.getErrorCounter())
	}
}

func TestRunTask_DoesNotRetryHTTPResponse(t *testing.T) {
	job, runner, _, rateWaits := retryTestJob(3, 0)
	job.runTask(jobContext{basereq: ffuf.Request{Url: "http://example.test/FUZZ"}}, map[string][]byte{"FUZZ": []byte("value")}, 1)

	_, execute := runner.calls()
	if execute != 1 {
		t.Fatalf("HTTP 500 execute calls = %d, want 1", execute)
	}
	if *rateWaits != 0 {
		t.Errorf("retry rate waits = %d, want 0", *rateWaits)
	}
}

func TestRunTask_DoesNotRetryAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	job, runner, _, rateWaits := retryTestJob(3, 4)
	job.Config.Context = ctx
	job.runTask(jobContext{basereq: ffuf.Request{Url: "http://example.test/FUZZ"}}, map[string][]byte{"FUZZ": []byte("value")}, 1)

	_, execute := runner.calls()
	if execute != 1 {
		t.Fatalf("execute calls after cancellation = %d, want 1", execute)
	}
	if *rateWaits != 0 {
		t.Errorf("retry rate waits = %d, want 0", *rateWaits)
	}
}

func TestRunTask_DoesNotExecuteRetryWhenCancelledDuringRateWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	job, runner, _, _ := retryTestJob(3, 4)
	job.Config.Context = ctx
	rateWaitStarted := make(chan struct{})
	releaseRateWait := make(chan struct{})
	release := func() {
		select {
		case <-releaseRateWait:
		default:
			close(releaseRateWait)
		}
	}
	defer release()
	job.Config.RateLimitFunc = func() {
		close(rateWaitStarted)
		<-releaseRateWait
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		job.runTask(jobContext{basereq: ffuf.Request{Url: "http://example.test/FUZZ"}}, map[string][]byte{"FUZZ": []byte("value")}, 1)
	}()

	select {
	case <-rateWaitStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("retry did not reach rate wait")
	}
	cancel()
	release()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runTask did not stop after cancellation")
	}

	_, execute := runner.calls()
	if execute != 1 {
		t.Fatalf("execute calls after cancellation during rate wait = %d, want 1", execute)
	}
}

func TestRunTask_RetryWaitsWhilePaused(t *testing.T) {
	job, runner, _, _ := retryTestJob(1, 2)
	runner.executeCh = make(chan int, 2)
	job.setRunning(true)
	job.Pause()
	defer job.Resume()

	done := make(chan struct{})
	go func() {
		defer close(done)
		job.runTask(jobContext{basereq: ffuf.Request{Url: "http://example.test/FUZZ"}}, map[string][]byte{"FUZZ": []byte("value")}, 1)
	}()

	select {
	case call := <-runner.executeCh:
		if call != 1 {
			t.Fatalf("first execute call = %d, want 1", call)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("initial request did not execute")
	}
	select {
	case call := <-runner.executeCh:
		t.Fatalf("retry executed while paused (call %d)", call)
	case <-time.After(50 * time.Millisecond):
	}

	job.Resume()
	select {
	case call := <-runner.executeCh:
		if call != 2 {
			t.Fatalf("execute call after resume = %d, want 2", call)
		}
	case <-time.After(time.Second):
		t.Fatal("retry did not execute after resume")
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runTask did not finish after resume")
	}
}

func TestNewJob_RateWaitStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	conf := &ffuf.Config{Context: ctx, Rate: 1, Threads: 1}
	job := NewJob(conf)
	job.Rate.RateLimiter.Stop()
	cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conf.RateLimitFunc()
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("rate wait did not stop promptly after cancellation")
	}
}
