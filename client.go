package gosk

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/macabc/gosk/state"
)

// Client is gosk's public surface. Zero value is not usable; construct with New.
type Client struct {
	cfg   *Config
	sess  session
	log   *slog.Logger
	store *state.Store // optional: nil when Config.StatePath is empty

	mu       sync.RWMutex
	loggedIn bool

	// downloads keyed by handle ID; updated by tracker goroutines.
	dlMu      sync.RWMutex
	downloads map[string]*downloadTracker
}

// New constructs a Client. cfg must have Username and Password set.
// Start a session with Login(ctx) before calling Search or Download.
func New(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.Username == "" || cfg.Password == "" {
		return nil, ErrMissingCredentials
	}
	sess, err := newSoulSession(cfg)
	if err != nil {
		return nil, fmt.Errorf("gosk: session init: %w", err)
	}
	c := newWithSession(cfg, sess)
	if cfg.StatePath != "" {
		store, err := state.Open(cfg.StatePath)
		if err != nil {
			return nil, fmt.Errorf("gosk: state store: %w", err)
		}
		c.store = store
	}
	return c, nil
}

// newWithSession is the test-friendly constructor — lets unit tests inject a
// fakeSession without touching the network.
func newWithSession(cfg *Config, sess session) *Client {
	return &Client{
		cfg:       cfg,
		sess:      sess,
		log:       slog.Default().With("mod", "gosk"),
		downloads: make(map[string]*downloadTracker),
	}
}

// WithStateStore attaches a state store to an already-constructed client.
// Useful in tests to bypass the Config.StatePath + New round trip.
func (c *Client) WithStateStore(s *state.Store) *Client {
	c.store = s
	return c
}

// Login establishes the Soulseek server session. Safe to call multiple times;
// each call re-establishes the connection.
func (c *Client) Login(ctx context.Context) error {
	if err := c.sess.Login(ctx); err != nil {
		return err
	}
	c.mu.Lock()
	c.loggedIn = true
	c.mu.Unlock()
	return nil
}

// Close shuts the session down and stops any in-flight tracker goroutines.
func (c *Client) Close() error {
	c.dlMu.Lock()
	for _, t := range c.downloads {
		t.cancel()
	}
	c.dlMu.Unlock()
	if c.store != nil {
		_ = c.store.Close()
	}
	return c.sess.Close()
}

// Resume reloads unfinished downloads from the state store. It returns
// the recovered records so callers can re-issue Download calls for each.
// gosk doesn't auto-resume because the session may refuse (peer offline,
// different token), so the decision is left to the caller.
func (c *Client) Resume(ctx context.Context) ([]state.Record, error) {
	if c.store == nil {
		return nil, nil
	}
	all, err := c.store.All(ctx)
	if err != nil {
		return nil, err
	}
	var pending []state.Record
	for _, r := range all {
		if r.State == string(DownloadCompleted) || r.State == string(DownloadFailed) {
			continue
		}
		pending = append(pending, r)
	}
	return pending, nil
}

func (c *Client) requireLoggedIn() error {
	c.mu.RLock()
	ok := c.loggedIn
	c.mu.RUnlock()
	if !ok {
		return ErrNotLoggedIn
	}
	return nil
}
