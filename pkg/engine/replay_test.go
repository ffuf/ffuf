package engine

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

type timingReplayRunner struct {
	prepared     bool
	executed     int
	executedAt   time.Time
	lastPosition int
	prepareErr   error
}

func (r *timingReplayRunner) Prepare(input map[string][]byte, base *ffuf.Request) (ffuf.Request, error) {
	r.prepared = true
	if r.prepareErr != nil {
		return ffuf.Request{}, r.prepareErr
	}
	return ffuf.CopyRequest(base), nil
}

func (r *timingReplayRunner) Execute(req *ffuf.Request) (ffuf.Response, error) {
	r.executed++
	r.executedAt = time.Now()
	r.lastPosition = req.Position
	return ffuf.Response{}, nil
}

func (r *timingReplayRunner) Dump(req *ffuf.Request) ([]byte, error) {
	return nil, nil
}

func TestReplayMatchedHonorsConfiguredDelay(t *testing.T) {
	conf := &ffuf.Config{Context: context.Background()}
	if err := conf.Delay.Initialize("0.06"); err != nil {
		t.Fatal(err)
	}
	runner := &timingReplayRunner{}
	job := &Job{Config: conf, ReplayRunner: runner, Output: NewNullOutput()}
	base := &ffuf.Request{Url: "http://example.test/FUZZ"}
	start := time.Now()

	job.replayMatched(map[string][]byte{"FUZZ": []byte("admin")}, base, 7)

	if !runner.prepared || runner.executed != 1 {
		t.Fatalf("replay prepare/execute calls: prepared=%t, executed=%d", runner.prepared, runner.executed)
	}
	if runner.lastPosition != 7 {
		t.Errorf("replay position = %d, want 7", runner.lastPosition)
	}
	if elapsed := runner.executedAt.Sub(start); elapsed < 45*time.Millisecond {
		t.Fatalf("replay happened after only %s; -p should delay it ~60ms", elapsed)
	}
}

func TestReplayMatchedWithoutDelayStillExecutes(t *testing.T) {
	conf := &ffuf.Config{Context: context.Background()}
	runner := &timingReplayRunner{}
	job := &Job{Config: conf, ReplayRunner: runner, Output: NewNullOutput()}
	job.replayMatched(nil, &ffuf.Request{Url: "http://example.test/"}, 1)
	if runner.executed != 1 {
		t.Fatalf("replay executed %d times, want 1", runner.executed)
	}
}

func TestReplayMatchedPrepareErrorSkipsReplay(t *testing.T) {
	conf := &ffuf.Config{Context: context.Background()}
	runner := &timingReplayRunner{prepareErr: errors.New("bad request")}
	job := &Job{Config: conf, ReplayRunner: runner, Output: NewNullOutput()}
	job.replayMatched(nil, &ffuf.Request{}, 1)
	if runner.executed != 0 {
		t.Fatalf("replay executed %d times after a prepare error", runner.executed)
	}
	if got := job.getErrorCounter(); got != 1 {
		t.Fatalf("error counter = %d, want 1", got)
	}
}
