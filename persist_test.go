package gosk

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/macabc/gosk/state"
)

func TestPersist_WritesToStoreOnStateChange(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "gosk.db")
	store, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	sess := &fakeSession{
		downloadStatus:    []string{"transferring"},
		downloadTermDelay: 150 * time.Millisecond,
		downloadErr:       nil,
	}
	c := mkFakeClient(t, sess)
	c.WithStateStore(store)

	h, err := c.Download(context.Background(), "peer1", "song.mp3", 4321)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	// Wait for at least the transferring state to land in the store.
	eventuallyState(t, c, h, DownloadTransferring, 500*time.Millisecond)

	rec, err := store.Get(context.Background(), h.ID)
	if err != nil {
		t.Fatalf("store.Get: %v", err)
	}
	if rec.Peer != "peer1" || rec.Filename != "song.mp3" || rec.Size != 4321 {
		t.Errorf("record mismatch: %+v", rec)
	}
	if rec.State != string(DownloadTransferring) {
		t.Errorf("state %q, want transferring", rec.State)
	}

	// Wait for completion — store should reflect it.
	eventuallyState(t, c, h, DownloadCompleted, 500*time.Millisecond)
	rec, _ = store.Get(context.Background(), h.ID)
	if rec.State != string(DownloadCompleted) {
		t.Errorf("post-completion state %q", rec.State)
	}
}

func TestResume_ReturnsUnfinishedOnly(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "gosk.db")
	store, err := state.Open(dbPath)
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Seed the store directly with mixed states.
	_ = store.Upsert(ctx, state.Record{
		HandleID: "pending",
		Peer:     "p", Filename: "f", Size: 1, State: string(DownloadTransferring), Bytes: 100,
	})
	_ = store.Upsert(ctx, state.Record{
		HandleID: "done",
		Peer:     "p", Filename: "f", Size: 1, State: string(DownloadCompleted), Bytes: 1,
	})
	_ = store.Upsert(ctx, state.Record{
		HandleID: "failed",
		Peer:     "p", Filename: "f", Size: 1, State: string(DownloadFailed),
	})

	sess := &fakeSession{}
	c := mkFakeClient(t, sess)
	c.WithStateStore(store)

	pending, err := c.Resume(ctx)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("got %d pending, want 1", len(pending))
	}
	if pending[0].HandleID != "pending" {
		t.Errorf("wrong handle: %q", pending[0].HandleID)
	}
}

func TestResume_NoStoreIsNoop(t *testing.T) {
	sess := &fakeSession{}
	c := mkFakeClient(t, sess)
	pending, err := c.Resume(context.Background())
	if err != nil {
		t.Errorf("Resume: %v", err)
	}
	if pending != nil {
		t.Errorf("got %v, want nil", pending)
	}
}
