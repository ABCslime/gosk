package gosk

import (
	"context"
	"testing"
	"time"

	"github.com/bh90210/soul/peer"
)

func testConfig() *Config {
	c := DefaultConfig()
	c.Username = "tester"
	c.Password = "secret"
	return c
}

func mkFakeClient(t *testing.T, sess *fakeSession) *Client {
	t.Helper()
	c := newWithSession(testConfig(), sess)
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}
	return c
}

func TestSearch_AccumulatesResponses(t *testing.T) {
	sess := &fakeSession{
		searchFeed: []*peer.FileSearchResponse{
			{
				Username: "peer1",
				Queue:    0,
				Results: []peer.File{
					{Name: "song1.mp3", Size: 1000},
					{Name: "song2.mp3", Size: 2000},
				},
			},
			{
				Username: "peer2",
				Queue:    5,
				Results:  []peer.File{{Name: "song3.mp3", Size: 3000}},
			},
		},
	}
	c := mkFakeClient(t, sess)

	results, err := c.Search(context.Background(), "query", 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	// Check that per-peer metadata is attached to each result.
	var p1Count, p2Count int
	for _, r := range results {
		if r.Peer == "peer1" {
			p1Count++
			if r.QueueLen != 0 {
				t.Errorf("peer1 queue len %d, want 0", r.QueueLen)
			}
		}
		if r.Peer == "peer2" {
			p2Count++
			if r.QueueLen != 5 {
				t.Errorf("peer2 queue len %d, want 5", r.QueueLen)
			}
		}
	}
	if p1Count != 2 || p2Count != 1 {
		t.Errorf("peer1 count %d (want 2), peer2 count %d (want 1)", p1Count, p2Count)
	}
}

func TestSearch_RespectsWindow(t *testing.T) {
	// Feed delivers slowly; window cuts it off before the third item.
	sess := &fakeSession{
		searchFeed: []*peer.FileSearchResponse{
			{Username: "a", Results: []peer.File{{Name: "a.mp3", Size: 1}}},
			{Username: "b", Results: []peer.File{{Name: "b.mp3", Size: 1}}},
			{Username: "c", Results: []peer.File{{Name: "c.mp3", Size: 1}}},
		},
		searchDelay: 80 * time.Millisecond,
	}
	c := mkFakeClient(t, sess)

	start := time.Now()
	results, err := c.Search(context.Background(), "q", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 250*time.Millisecond {
		t.Errorf("search took %v, expected ≤ ~100ms window", elapsed)
	}
	// At 80ms delay with 100ms window, we should get 0 or 1 result, never 3.
	if len(results) >= 3 {
		t.Errorf("got %d results, expected window cutoff before 3", len(results))
	}
}

func TestSearch_NotLoggedIn(t *testing.T) {
	sess := &fakeSession{}
	c := newWithSession(testConfig(), sess)
	// intentionally skip Login
	_, err := c.Search(context.Background(), "q", time.Second)
	if err != ErrNotLoggedIn {
		t.Errorf("got %v, want ErrNotLoggedIn", err)
	}
}

func TestSearch_BitrateAttribute(t *testing.T) {
	sess := &fakeSession{
		searchFeed: []*peer.FileSearchResponse{
			{
				Username: "peer1",
				Results: []peer.File{
					{
						Name: "song.mp3",
						Size: 1000,
						Attributes: []peer.Attribute{
							{Code: 0, Value: 320}, // bitrate
							{Code: 1, Value: 180}, // duration
						},
					},
				},
			},
		},
	}
	c := mkFakeClient(t, sess)
	results, err := c.Search(context.Background(), "q", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results", len(results))
	}
	if results[0].Bitrate != 320 {
		t.Errorf("bitrate %d, want 320", results[0].Bitrate)
	}
}
