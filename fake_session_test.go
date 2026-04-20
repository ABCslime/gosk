package gosk

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/bh90210/soul"
	"github.com/bh90210/soul/peer"
)

// fakeSession is a scriptable session for unit tests. It doesn't touch
// the network. Tests build one, set the scripted responses/status, and
// pass it to newWithSession.
type fakeSession struct {
	mu sync.Mutex

	loginErr error
	loginHit bool

	// Search: scripted peer responses. Tests push into `searchFeed`; the
	// session forwards them verbatim to the caller's channel and closes
	// when the feed is drained + no more coming (sealed).
	searchErr   error
	searchFeed  []*peer.FileSearchResponse
	searchDelay time.Duration

	// Download: scripted status events and a terminal error (nil for success).
	downloadStatus []string
	downloadErr    error
	downloadDelay  time.Duration
	// downloadTermDelay holds off the terminal error until after this delay
	// from when the last status was sent. Useful for tests that want to
	// observe intermediate states before a failure.
	downloadTermDelay time.Duration

	closeHit bool
}

func (f *fakeSession) Login(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loginHit = true
	return f.loginErr
}

func (f *fakeSession) Search(ctx context.Context, _ string, _ soul.Token) (<-chan *peer.FileSearchResponse, error) {
	f.mu.Lock()
	if f.searchErr != nil {
		err := f.searchErr
		f.mu.Unlock()
		return nil, err
	}
	feed := append([]*peer.FileSearchResponse(nil), f.searchFeed...)
	delay := f.searchDelay
	f.mu.Unlock()

	out := make(chan *peer.FileSearchResponse)
	go func() {
		defer close(out)
		for _, r := range feed {
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					return
				}
			}
			select {
			case out <- r:
			case <-ctx.Done():
				return
			}
		}
		// Keep the channel open until ctx done, mirroring soul's real behavior
		// (search subscriptions stay alive until the caller cancels).
		<-ctx.Done()
	}()
	return out, nil
}

func (f *fakeSession) Download(ctx context.Context, _ DownloadJob) (<-chan string, <-chan error) {
	f.mu.Lock()
	statuses := append([]string(nil), f.downloadStatus...)
	delay := f.downloadDelay
	termDelay := f.downloadTermDelay
	termErr := f.downloadErr
	f.mu.Unlock()

	st := make(chan string, len(statuses))
	er := make(chan error, 1)

	go func() {
		defer close(st)
		defer close(er)
		for _, s := range statuses {
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-ctx.Done():
					er <- errors.New("context cancelled")
					return
				}
			}
			select {
			case st <- s:
			case <-ctx.Done():
				er <- errors.New("context cancelled")
				return
			}
		}
		if termDelay > 0 {
			select {
			case <-time.After(termDelay):
			case <-ctx.Done():
				er <- errors.New("context cancelled")
				return
			}
		}
		er <- termErr
	}()

	return st, er
}

func (f *fakeSession) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closeHit = true
	return nil
}

var _ session = (*fakeSession)(nil)
