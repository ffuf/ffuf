package ffuf

import (
	"sync"
	"testing"
)

// TestPauseGate_ConcurrentIdempotent regresses C8: the pause gate used a
// sync.WaitGroup as a resettable barrier, which called Add concurrently with
// Wait (a documented misuse the race detector flags) and had a double-Done
// negative-counter panic path. The replacement is an RWMutex gate made
// idempotent by an atomic CAS. Run under -race.
func TestPauseGate_ConcurrentIdempotent(t *testing.T) {
	j := &Job{Output: NewNullOutput()}

	// Duplicate Pause/Resume must not panic or unbalance the gate.
	j.Pause()
	j.Pause()
	j.Resume()
	j.Resume()
	// A stray Resume with no matching Pause must be a no-op, not an unlock panic.
	j.Resume()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Workers hammer the pause checkpoint while another goroutine toggles pause.
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					j.pauseCheckpoint()
				}
			}
		}()
	}
	for k := 0; k < 500; k++ {
		j.Pause()
		j.Resume()
	}
	close(stop)
	wg.Wait()
}

// TestPauseGate_ConcurrentPauseResume drives Pause and Resume from two separate
// goroutines. That is the real interleaving (an interactive pause while the
// SIGINT handler calls Resume), and a gate that flips its flag and takes the lock
// in two steps turns it into a fatal "Unlock of unlocked RWMutex". The flag flip
// and the Lock/Unlock must be one atomic step. Run under -race.
func TestPauseGate_ConcurrentPauseResume(t *testing.T) {
	j := &Job{Output: NewNullOutput()}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100000; i++ {
			j.Pause()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 100000; i++ {
			j.Resume()
		}
	}()
	wg.Wait()

	// Leave the gate unlocked regardless of which side won the last iteration.
	j.Resume()
}
