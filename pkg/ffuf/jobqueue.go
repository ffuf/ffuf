package ffuf

import "sync"

// jobQueue owns the recursion/sniper job list and the active position, together
// with the mutex that guards them. Worker goroutines append to the queue (from
// the recursion handlers) while the main loop advances through it and the
// progress goroutine and the interactive handler read it, so the slice and the
// position must never be touched outside these methods. The fields are private:
// there is no way to append without the lock.
type jobQueue struct {
	mu   sync.Mutex
	jobs []QueueJob
	pos  int
}

func newJobQueue() *jobQueue {
	return &jobQueue{jobs: make([]QueueJob, 0)}
}

// push appends a job to the queue.
func (q *jobQueue) push(j QueueJob) {
	q.mu.Lock()
	q.jobs = append(q.jobs, j)
	q.mu.Unlock()
}

// hasNext reports whether an unprocessed job remains.
func (q *jobQueue) hasNext() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pos < len(q.jobs)
}

// advance returns the job at the active position and moves the cursor past it,
// returning the new position (1-based marker matching the previous queuepos).
func (q *jobQueue) advance() (QueueJob, int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	job := q.jobs[q.pos]
	q.pos++
	return job, q.pos
}

// currentReq returns the request of the job at the active position (pos-1).
func (q *jobQueue) currentReq() Request {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.jobs[q.pos-1].req
}

// position returns the active position marker.
func (q *jobQueue) position() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pos
}

// total returns the number of jobs currently in the queue.
func (q *jobQueue) total() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}

// remaining returns a copy of the jobs from the active position onward.
func (q *jobQueue) remaining() []QueueJob {
	q.mu.Lock()
	defer q.mu.Unlock()
	start := q.pos - 1
	if start < 0 || start > len(q.jobs) {
		return nil
	}
	out := make([]QueueJob, len(q.jobs[start:]))
	copy(out, q.jobs[start:])
	return out
}

// removeAt deletes the queued job at the given offset from the active position
// (the 1-based offset the interactive queuedel command uses). It returns false
// when the offset is out of range instead of panicking.
func (q *jobQueue) removeAt(offset int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	idx := q.pos + offset - 1
	if idx < 0 || idx >= len(q.jobs) {
		return false
	}
	q.jobs = append(q.jobs[:idx], q.jobs[idx+1:]...)
	return true
}
