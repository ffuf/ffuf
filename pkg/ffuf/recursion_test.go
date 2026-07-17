package ffuf

import "testing"

func TestRecursionManager_GreedyQueuesWithinDepth(t *testing.T) {
	q := newJobQueue()
	rm := newRecursionManager(&Config{Method: "GET", RecursionDepth: 2}, q, NewNullOutput())
	resp := Response{Request: &Request{Url: "http://x/admin"}}

	rm.handleGreedy(jobContext{depth: 0}, resp)

	if !q.hasNext() {
		t.Fatal("expected a recursion job to be queued")
	}
	job, _ := q.advance()
	if job.Url != "http://x/admin/FUZZ" {
		t.Errorf("queued url = %q, want http://x/admin/FUZZ", job.Url)
	}
	if job.depth != 1 {
		t.Errorf("queued depth = %d, want 1 (parent depth + 1)", job.depth)
	}
}

func TestRecursionManager_GreedyRespectsMaxDepth(t *testing.T) {
	q := newJobQueue()
	rm := newRecursionManager(&Config{Method: "GET", RecursionDepth: 1}, q, NewNullOutput())
	resp := Response{Request: &Request{Url: "http://x/admin"}}

	// depth == RecursionDepth: at the limit, nothing more is queued.
	rm.handleGreedy(jobContext{depth: 1}, resp)

	if q.total() != 0 {
		t.Errorf("expected no queued job at max depth, got %d", q.total())
	}
}

func TestRecursionManager_DefaultSkipsNonDirectory(t *testing.T) {
	q := newJobQueue()
	rm := newRecursionManager(&Config{Method: "GET", RecursionDepth: 0}, q, NewNullOutput())

	// A 200 with no redirect is not a directory, so the default strategy queues
	// nothing.
	resp := Response{Request: &Request{Url: "http://x/file"}, StatusCode: 200}
	rm.handleDefault(jobContext{depth: 0}, resp)

	if q.total() != 0 {
		t.Errorf("non-directory response should not queue a job, got %d", q.total())
	}
}

func TestRecursionManager_DefaultQueuesDirectory(t *testing.T) {
	q := newJobQueue()
	rm := newRecursionManager(&Config{Method: "GET", RecursionDepth: 0}, q, NewNullOutput())

	// A 301 redirecting to url + "/" is a directory; the default strategy descends.
	resp := Response{
		Request:    &Request{Url: "http://x/dir"},
		StatusCode: 301,
		Headers:    map[string][]string{"Location": {"http://x/dir/"}},
	}
	rm.handleDefault(jobContext{depth: 0}, resp)

	if q.total() != 1 {
		t.Fatalf("directory response should queue a recursion job, got %d", q.total())
	}
	job, _ := q.advance()
	if job.Url != "http://x/dir/FUZZ" {
		t.Errorf("queued url = %q, want http://x/dir/FUZZ", job.Url)
	}
}
