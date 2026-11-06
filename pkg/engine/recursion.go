package engine

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ffuf/ffuf/v2/pkg/ffuf"
)

// recursionManager owns the recursion policy: deciding whether a matched response
// should spawn a deeper queue job and enqueuing it. It was lifted out of Job so the
// greedy/default recursion logic lives in one place and is testable without a full
// engine. It holds only what it needs: the config (for the depth limit and to build
// recursion requests), the queue to push onto, and the output for its messages.
type recursionManager struct {
	config *ffuf.Config
	queue  *jobQueue
	output ffuf.OutputProvider
}

func newRecursionManager(conf *ffuf.Config, queue *jobQueue, output ffuf.OutputProvider) *recursionManager {
	return &recursionManager{config: conf, queue: queue, output: output}
}

// handleGreedy queues a recursion job for a matched response (greedy strategy: a
// match is enough), if the recursion depth allows. The match has been determined
// by the caller.
func (r *recursionManager) handleGreedy(ctx jobContext, resp ffuf.Response) {
	if r.config.RecursionDepth == 0 || ctx.depth < r.config.RecursionDepth {
		recUrl := resp.Request.Url + "/" + "FUZZ"
		newJob := QueueJob{Url: recUrl, depth: ctx.depth + 1, req: ffuf.RecursionRequest(r.config, recUrl)}
		r.queue.push(newJob)
		r.output.Info(fmt.Sprintf("Adding a new job to the queue: %s", recUrl))
	} else {
		r.output.Warning(fmt.Sprintf("Maximum recursion depth reached. Ignoring: %s", resp.Request.Url))
	}
}

// handleDefault queues a recursion job when a matched response is a directory
// (default strategy: the response redirects to its own path with a trailing
// slash), if the recursion depth allows.
func (r *recursionManager) handleDefault(ctx jobContext, resp ffuf.Response) {
	if !isDirectoryRedirect(resp) {
		// Not a directory, return early
		return
	}
	recUrl := resp.Request.Url + "/" + "FUZZ"
	if r.config.RecursionDepth == 0 || ctx.depth < r.config.RecursionDepth {
		// We have yet to reach the maximum recursion depth
		newJob := QueueJob{Url: recUrl, depth: ctx.depth + 1, req: ffuf.RecursionRequest(r.config, recUrl)}
		r.queue.push(newJob)
		r.output.Info(fmt.Sprintf("Adding a new job to the queue: %s", recUrl))
	} else {
		r.output.Warning(fmt.Sprintf("Directory found, but recursion depth exceeded. Ignoring: %s", resp.GetRedirectLocation(true)))
	}
}

// isDirectoryRedirect recognizes a directory slash redirect even when a scan
// connects to an IP address with an overridden Host header (-H "Host: ...").
// Servers often use the virtual host in an absolute Location header, while the
// queued recursion URL must still use the original IP address. Only equivalent
// paths on the requested virtual host count; unrelated redirect hosts do not.
func isDirectoryRedirect(resp ffuf.Response) bool {
	if resp.Request == nil {
		return false
	}
	expected := resp.Request.Url + "/"
	actual := resp.GetRedirectLocation(true)
	if actual == expected {
		return true
	}
	if resp.Request.Host == "" || actual == "" {
		return false
	}

	expectedURL, err := url.Parse(expected)
	if err != nil {
		return false
	}
	redirectURL, err := url.Parse(actual)
	if err != nil {
		return false
	}
	virtualHost := &url.URL{Host: resp.Request.Host}
	if !strings.EqualFold(redirectURL.Hostname(), virtualHost.Hostname()) {
		return false
	}

	// Honor an explicitly selected virtual-host port. When the Host override
	// omits a port, an absolute redirect may omit it too or use the target port.
	if port := virtualHost.Port(); port != "" {
		if redirectURL.Port() != port {
			return false
		}
	} else if port := redirectURL.Port(); port != "" && port != expectedURL.Port() {
		return false
	}

	redirectURL.Host = expectedURL.Host
	return redirectURL.String() == expectedURL.String()
}
