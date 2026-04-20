package gosk

import (
	"os"
	"time"
)

// Config holds runtime knobs. Fields are safe to leave at zero values where
// DefaultConfig() supplies a reasonable fallback.
type Config struct {
	// Username on the Soulseek network. Required.
	Username string
	// Password. Required.
	Password string

	// SoulSeekAddress is the server host. Default "server.slsknet.org".
	SoulSeekAddress string
	// SoulSeekPort is the server port. Default 2242.
	SoulSeekPort int
	// OwnPort is the TCP port we listen on for incoming peer connections.
	OwnPort int

	// DialTimeout bounds the initial TCP connect. Default 10s.
	DialTimeout time.Duration
	// LoginTimeout bounds the login handshake. Default 3s.
	LoginTimeout time.Duration
	// SearchWindow is how long Search accumulates peer responses. Default 10s
	// (overridable per call).
	SearchWindow time.Duration
	// PollInterval is the cadence for download-status internal polls. Default 2s.
	PollInterval time.Duration

	// DownloadFolder is where completed files land. Default: os.TempDir().
	DownloadFolder string

	// StatePath is the SQLite file for download-state persistence. If empty,
	// state is kept in memory only (no resume across restarts).
	StatePath string

	// SharedFolders / SharedFiles are reported to the server at login. The
	// Soulseek etiquette norm is to share at least something; default 1/1
	// matches the slskd upstream default.
	SharedFolders int
	SharedFiles   int
}

// DefaultConfig returns a Config populated with safe defaults. Username and
// Password must still be set by the caller before New is useful.
func DefaultConfig() *Config {
	return &Config{
		SoulSeekAddress: "server.slsknet.org",
		SoulSeekPort:    2242,
		OwnPort:         2234,
		DialTimeout:     10 * time.Second,
		LoginTimeout:    3 * time.Second,
		SearchWindow:    10 * time.Second,
		PollInterval:    2 * time.Second,
		DownloadFolder:  os.TempDir(),
		SharedFolders:   1,
		SharedFiles:     1,
	}
}
