package gosk

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bh90210/soul"
	"github.com/bh90210/soul/peer"

	"github.com/macabc/gosk/state"
)

// Download enqueues a transfer from `peer` for `filename`. The returned
// handle is used to poll progress via DownloadStatus. gosk spawns a
// background goroutine that watches the session and keeps the in-memory
// state snapshot fresh; callers do not need to poll the returned channels.
//
// The file ends up in cfg.DownloadFolder with the basename of `filename`
// (slashes and backslashes stripped).
func (c *Client) Download(ctx context.Context, peerName, filename string, size int64) (DownloadHandle, error) {
	if err := c.requireLoggedIn(); err != nil {
		return DownloadHandle{}, err
	}

	token := soul.NewToken()
	job := DownloadJob{
		Username: peerName,
		Token:    token,
		File: &peer.File{
			Name: filename,
			Size: uint64(size),
		},
	}

	handleID := fmt.Sprintf("%s-%d-%d", peerName, uint32(token), handleCounter.Add(1))

	trackerCtx, cancel := context.WithCancel(context.Background())
	tr := newDownloadTracker(handleID, job)
	tr.cancel = cancel

	c.dlMu.Lock()
	c.downloads[handleID] = tr
	c.dlMu.Unlock()

	// Persist the initial record so a crash before any status message
	// still leaves a breadcrumb for Resume.
	c.persist(tr)

	// Kick off the session's download. The returned channels are owned by
	// the tracker goroutine below.
	status, errCh := c.sess.Download(ctx, job)

	go c.track(trackerCtx, tr, status, errCh)

	return DownloadHandle{ID: handleID}, nil
}

// persist writes the tracker's current snapshot to the state store, if one
// is attached. Best-effort: errors are logged and ignored so a flaky store
// doesn't break live downloads.
func (c *Client) persist(tr *downloadTracker) {
	if c.store == nil {
		return
	}
	snap := tr.snapshot()
	err := c.store.Upsert(context.Background(), state.Record{
		HandleID: tr.handleID,
		Peer:     tr.job.Username,
		Filename: tr.job.File.Name,
		Size:     snap.Size,
		State:    string(snap.State),
		Bytes:    snap.Bytes,
		FilePath: snap.FilePath,
	})
	if err != nil {
		c.log.Warn("state persist failed", "handle", tr.handleID, "err", err)
	}
}

// DownloadStatus looks up an in-flight (or recently finished) download and
// returns a snapshot of its state.
func (c *Client) DownloadStatus(_ context.Context, h DownloadHandle) (DownloadState, error) {
	c.dlMu.RLock()
	tr, ok := c.downloads[h.ID]
	c.dlMu.RUnlock()
	if !ok {
		return DownloadState{}, ErrUnknownHandle
	}
	return tr.snapshot(), nil
}

// track consumes status and error channels from the session, updating the
// tracker's state snapshot. Exits when errCh produces a terminal value or
// the tracker's context is cancelled.
func (c *Client) track(
	ctx context.Context,
	tr *downloadTracker,
	status <-chan string,
	errCh <-chan error,
) {
	for {
		select {
		case <-ctx.Done():
			return

		case s, ok := <-status:
			if !ok {
				status = nil // don't re-select on a closed channel
				continue
			}
			c.log.Debug("download status",
				"handle", tr.handleID, "status", s,
			)
			kind := mapStatusString(s)
			// Preserve size + filepath as we transition through states.
			curr := tr.snapshot()
			curr.State = kind
			tr.setState(curr)
			c.persist(tr)

		case e, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			c.finishDownload(tr, e)
			return
		}

		// If both channels are nil/closed, we're done.
		if status == nil && errCh == nil {
			c.finishDownload(tr, nil)
			return
		}
	}
}

// finishDownload is the single transition point into a terminal state. The
// `err` is the session's final value — nil on clean success, peer.ErrComplete
// on a normal finish, or any other error on failure.
func (c *Client) finishDownload(tr *downloadTracker, err error) {
	path := filepath.Join(c.cfg.DownloadFolder, basename(tr.job.File.Name))

	if err == nil || isComplete(err) {
		tr.markFinished(DownloadCompleted, path, nil)
		c.log.Info("download complete", "handle", tr.handleID, "path", path)
		c.persist(tr)
		return
	}
	tr.markFinished(DownloadFailed, "", err)
	c.log.Warn("download failed", "handle", tr.handleID, "err", err)
	c.persist(tr)
}

// isComplete detects soul's "transfer completed" sentinel. Upstream uses
// peer.ErrComplete; we check by string to avoid a hard import dependency
// (and so future sentinels don't break us).
func isComplete(err error) bool {
	if err == nil {
		return false
	}
	// Direct identity check against peer.ErrComplete.
	if errors.Is(err, peer.ErrComplete) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "complete")
}

// basename strips directory components from both slash flavors. Soulseek
// filenames are often backslash-separated.
func basename(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	return filepath.Base(p)
}
