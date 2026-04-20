package gosk

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
)

// downloadTracker is one in-flight download. A goroutine reads from the
// underlying session's status/errCh channels and updates state atomically;
// Client.DownloadStatus reads a snapshot without blocking the goroutine.
type downloadTracker struct {
	handleID string
	job      DownloadJob

	cancel context.CancelFunc

	mu       sync.RWMutex
	state    DownloadState
	finished bool // set when errCh fires; snapshot is terminal after.
}

func newDownloadTracker(handleID string, job DownloadJob) *downloadTracker {
	t := &downloadTracker{
		handleID: handleID,
		job:      job,
	}
	t.state = DownloadState{
		State: DownloadQueued,
		Size:  int64(job.File.Size),
	}
	return t
}

// snapshot returns the current state. Safe to call concurrently with updates.
func (t *downloadTracker) snapshot() DownloadState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.state
}

// setState is used by the watcher goroutine.
func (t *downloadTracker) setState(s DownloadState) {
	t.mu.Lock()
	t.state = s
	t.mu.Unlock()
}

// markFinished transitions into a terminal state (completed or failed).
func (t *downloadTracker) markFinished(kind DownloadStateKind, filePath string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.state.State = kind
	if filePath != "" {
		t.state.FilePath = filePath
	}
	if kind == DownloadCompleted {
		t.state.Bytes = t.state.Size
	}
	t.finished = true
	_ = err // errors are logged by the caller; state is the source of truth
}

// mapStatusString translates soul/client's status strings to our enum.
// soul doesn't export these as constants; the strings come from state.go
// and peer.go — we match on substrings for robustness.
func mapStatusString(s string) DownloadStateKind {
	ls := strings.ToLower(s)
	switch {
	case strings.Contains(ls, "complete"):
		return DownloadCompleted
	case strings.Contains(ls, "fail"), strings.Contains(ls, "denied"), strings.Contains(ls, "error"):
		return DownloadFailed
	case strings.Contains(ls, "transfer"), strings.Contains(ls, "downloading"), strings.Contains(ls, "writing"):
		return DownloadTransferring
	default:
		return DownloadQueued
	}
}

// atomic counter so tests are deterministic about handle uniqueness.
var handleCounter atomic.Uint64
