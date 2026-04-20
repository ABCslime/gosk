// Package gosk is a native Go Soulseek client. It implements the contract
// expected by muzika's internal/soulseek.Client surface, but as a separate
// public module so it can evolve independently.
//
// See PLAN.md for scope, non-goals, and architecture.
package gosk

// SearchResult is a single peer's advertised file for a query.
type SearchResult struct {
	// Peer is the Soulseek username that advertised this file.
	Peer string
	// Filename is the remote filename (backslash-separated on the wire).
	Filename string
	// Size is the file size in bytes.
	Size int64
	// Bitrate in kbps if the peer reported one; 0 otherwise.
	Bitrate int
	// QueueLen is the length of the peer's upload queue at response time.
	QueueLen int
	// FilesShared is how many files the peer has shared (reliability proxy).
	FilesShared int
}

// DownloadHandle is an opaque identifier returned by Download. Callers pass
// it to DownloadStatus to poll progress.
type DownloadHandle struct {
	ID string
}

// DownloadState captures progress and, on completion, the local file path.
type DownloadState struct {
	State    DownloadStateKind
	Bytes    int64
	Size     int64
	FilePath string
}

// DownloadStateKind is the lifecycle of a transfer from gosk's perspective.
type DownloadStateKind string

const (
	// DownloadQueued — transfer accepted by the peer, not yet transferring.
	DownloadQueued DownloadStateKind = "queued"
	// DownloadTransferring — bytes actively flowing.
	DownloadTransferring DownloadStateKind = "transferring"
	// DownloadCompleted — file fully received, FilePath is populated.
	DownloadCompleted DownloadStateKind = "completed"
	// DownloadFailed — transfer errored or was rejected.
	DownloadFailed DownloadStateKind = "failed"
)
