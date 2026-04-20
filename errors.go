package gosk

import "errors"

// Exported errors.
var (
	// ErrNotLoggedIn is returned when a method requires a live session and
	// the client hasn't successfully logged in yet.
	ErrNotLoggedIn = errors.New("gosk: not logged in")
	// ErrNoResults is returned by Search when the accumulation window
	// elapsed without any peer responses.
	ErrNoResults = errors.New("gosk: no results")
	// ErrUnknownHandle is returned by DownloadStatus when the handle is
	// not tracked (typically because it's stale or from a different client).
	ErrUnknownHandle = errors.New("gosk: unknown download handle")
	// ErrMissingCredentials is returned by New when Username/Password are empty.
	ErrMissingCredentials = errors.New("gosk: username and password are required")
)
