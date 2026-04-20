package gosk

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/bh90210/soul"
	"github.com/bh90210/soul/client"
	"github.com/bh90210/soul/peer"
	"github.com/rs/zerolog"
)

// soulSession wraps github.com/bh90210/soul/client to satisfy the session
// interface. This is the production implementation.
type soulSession struct {
	cfg *Config
	c   *client.Client
	s   *client.State
}

func newSoulSession(cfg *Config) (session, error) {
	conf := &client.Config{
		Username:        cfg.Username,
		Password:        cfg.Password,
		SoulSeekAddress: cfg.SoulSeekAddress,
		SoulSeekPort:    cfg.SoulSeekPort,
		OwnPort:         cfg.OwnPort,
		SharedFolders:   cfg.SharedFolders,
		SharedFiles:     cfg.SharedFiles,
		LogLevel:        zerolog.Disabled, // route logs through muzika's slog instead
		Timeout:         cfg.DialTimeout,
		LoginTimeout:    cfg.LoginTimeout,
		DownloadFolder:  cfg.DownloadFolder,
		MaxPeers:        100,
		MaxUploads:      0, // we don't upload
	}
	if conf.DownloadFolder == "" {
		conf.DownloadFolder = os.TempDir()
	}
	c, err := client.New(conf)
	if err != nil {
		return nil, fmt.Errorf("new soul client: %w", err)
	}
	return &soulSession{
		cfg: cfg,
		c:   c,
		s:   client.NewState(c),
	}, nil
}

func (s *soulSession) Login(ctx context.Context) error {
	cancelCtx, cancel := context.WithCancel(ctx)
	if err := s.c.Dial(cancelCtx, cancel); err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	if err := s.s.Login(ctx); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	return nil
}

func (s *soulSession) Search(ctx context.Context, query string, token soul.Token) (<-chan *peer.FileSearchResponse, error) {
	ch, err := s.s.Search(ctx, query, token)
	if err != nil {
		return nil, err
	}
	// soul's Search returns a bidirectional chan; narrow it to receive-only.
	return ch, nil
}

func (s *soulSession) Download(ctx context.Context, d DownloadJob) (<-chan string, <-chan error) {
	st, er := s.s.Download(ctx, client.Download{
		Username: d.Username,
		Token:    d.Token,
		File:     d.File,
	})
	return st, er
}

func (s *soulSession) Close() error {
	// soul/client doesn't expose a public Close; we rely on the context
	// passed to Dial being cancelled by the caller. Nothing to do here
	// beyond signalling that we're done.
	return nil
}

// Compile-time check.
var _ session = (*soulSession)(nil)

// silence "imported and not used" when soul is only referenced transitively.
var _ = errors.New
