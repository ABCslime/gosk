package gosk

import (
	"context"

	"github.com/bh90210/soul"
	"github.com/bh90210/soul/peer"
)

// session abstracts the Soulseek protocol plumbing so gosk's business logic
// (search accumulation, download tracking) can be unit-tested without real
// network I/O. The real implementation wraps github.com/bh90210/soul/client.
type session interface {
	// Login establishes the session with the server. Must succeed before
	// Search/Download.
	Login(ctx context.Context) error

	// Search issues a FileSearch and returns a channel that receives every
	// peer's FileSearchResponse for the given token. The channel remains
	// open until ctx is cancelled.
	Search(ctx context.Context, query string, token soul.Token) (<-chan *peer.FileSearchResponse, error)

	// Download enqueues a transfer from `username` for the given file.
	// Returns two channels:
	//   status — receives human-readable state labels ("queued", "transferring", ...)
	//   errCh  — terminal error (nil on success); closed after the final event
	Download(ctx context.Context, d DownloadJob) (status <-chan string, errCh <-chan error)

	// Close tears down the session.
	Close() error
}

// DownloadJob is what the session needs to start a download. Defined here
// (not in types.go) because it's an internal contract between Client and
// session, not a public API.
type DownloadJob struct {
	Username string
	Token    soul.Token
	File     *peer.File
}
