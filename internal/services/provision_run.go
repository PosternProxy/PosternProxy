package services

import (
	"errors"
	"sync"

	"github.com/posternproxy/posternproxy/internal/provisioner"
)

// ErrProvisioningInProgress is returned by ServerService.Create and Reprovision
// when a provisioning run for the same server is already active. Handlers
// should map this to HTTP 409 Conflict.
var ErrProvisioningInProgress = errors.New("provisioning already in progress for this server")

// maxRunLogLines caps the in-memory log buffer per provision run so a runaway
// install script cannot consume unbounded memory. The install scripts produce
// a few hundred lines in practice; this is generous.
const maxRunLogLines = 5000

// ProvisionRun tracks a single in-flight (or recently finished) provisioning
// attempt. It holds the accumulating log, the final result once available, and
// the list of SSE subscribers currently watching the run.
//
// All methods are safe for concurrent use.
type ProvisionRun struct {
	mu     sync.Mutex
	log    []string
	subs   []chan string
	done   bool
	result *provisioner.Result
}

// appendLine adds a line to the run's stored log and fans it out to active
// subscribers. Subscribers that are not draining their channel are dropped to
// prevent a slow client from blocking the producer.
func (r *ProvisionRun) appendLine(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.log) < maxRunLogLines {
		r.log = append(r.log, line)
	}

	alive := r.subs[:0]
	for _, sub := range r.subs {
		select {
		case sub <- line:
			alive = append(alive, sub)
		default:
			// Slow consumer — drop it from the broadcast list and close so
			// the client's SSE loop exits cleanly.
			close(sub)
		}
	}
	r.subs = alive
}

// finish marks the run complete, stores the result, and closes all subscriber
// channels so streaming handlers exit cleanly.
func (r *ProvisionRun) finish(res provisioner.Result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done = true
	r.result = &res
	for _, sub := range r.subs {
		close(sub)
	}
	r.subs = nil
}

// Subscribe returns a snapshot of the log so far plus a channel that receives
// any future lines. The channel is closed when the run finishes (or when the
// run is already done at subscribe time).
//
// Callers should range over the snapshot first, then range over the channel.
func (r *ProvisionRun) Subscribe() (history []string, live <-chan string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	history = append([]string(nil), r.log...)
	if r.done {
		ch := make(chan string)
		close(ch)
		return history, ch
	}
	ch := make(chan string, 64)
	r.subs = append(r.subs, ch)
	return history, ch
}

// Done returns true if the run has finished.
func (r *ProvisionRun) Done() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.done
}

// Result returns the run's final Result and true if finished, or zero+false.
func (r *ProvisionRun) Result() (provisioner.Result, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.result == nil {
		return provisioner.Result{}, false
	}
	return *r.result, true
}
