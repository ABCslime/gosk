package gosk

import (
	"context"
	"time"

	"github.com/bh90210/soul"
	"github.com/bh90210/soul/peer"
)

// Search issues a FileSearch and accumulates peer responses for `window`.
// If window is zero, Config.SearchWindow is used.
func (c *Client) Search(ctx context.Context, query string, window time.Duration) ([]SearchResult, error) {
	if err := c.requireLoggedIn(); err != nil {
		return nil, err
	}
	if window <= 0 {
		window = c.cfg.SearchWindow
	}
	if window <= 0 {
		window = 10 * time.Second
	}

	token := soul.NewToken()

	// Scope the search to its own cancellable context so the session can
	// clean up the subscription when we're done accumulating.
	searchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	respCh, err := c.sess.Search(searchCtx, query, token)
	if err != nil {
		return nil, err
	}

	results := accumulateResponses(searchCtx, respCh, window)
	c.log.Debug("search complete",
		"query", query, "token", token, "peers", len(results),
	)
	return flatten(results), nil
}

// accumulateResponses reads from respCh until either `window` elapses or the
// context is cancelled. Returns the list of responses we heard.
func accumulateResponses(
	ctx context.Context,
	respCh <-chan *peer.FileSearchResponse,
	window time.Duration,
) []*peer.FileSearchResponse {
	var out []*peer.FileSearchResponse
	deadline := time.NewTimer(window)
	defer deadline.Stop()
	for {
		select {
		case r, ok := <-respCh:
			if !ok {
				return out
			}
			if r == nil {
				continue
			}
			out = append(out, r)
		case <-deadline.C:
			return out
		case <-ctx.Done():
			return out
		}
	}
}

// flatten turns one response-per-peer into one record-per-file. We only
// surface public Results; PrivateResults are skipped (they require peer
// privileges we don't negotiate).
func flatten(responses []*peer.FileSearchResponse) []SearchResult {
	var out []SearchResult
	for _, r := range responses {
		for _, f := range r.Results {
			out = append(out, SearchResult{
				Peer:        r.Username,
				Filename:    f.Name,
				Size:        int64(f.Size),
				Bitrate:     extractBitrate(f),
				QueueLen:    r.Queue,
				FilesShared: 0, // soul doesn't expose a per-peer shared count; left zero.
			})
		}
	}
	return out
}

// extractBitrate reads the bitrate attribute if present. soul's File.Attribute
// uses an enum code — Code 0 is bitrate per the Soulseek wire protocol.
func extractBitrate(f peer.File) int {
	for _, a := range f.Attributes {
		if a.Code == 0 { // bitrate
			return int(a.Value)
		}
	}
	return 0
}
