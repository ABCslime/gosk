package state

import (
	"context"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestStore_UpsertAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	r := Record{
		HandleID: "h-1",
		Peer:     "peer1",
		Filename: "song.mp3",
		Size:     1000,
		State:    "queued",
	}
	if err := s.Upsert(ctx, r); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.Get(ctx, "h-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Peer != "peer1" || got.Filename != "song.mp3" || got.Size != 1000 {
		t.Errorf("mismatch: %+v", got)
	}
	if got.State != "queued" {
		t.Errorf("state %q, want queued", got.State)
	}
}

func TestStore_UpsertOverwrites(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Upsert(ctx, Record{HandleID: "h", Peer: "p", Filename: "f", Size: 1, State: "queued"})
	_ = s.Upsert(ctx, Record{HandleID: "h", Peer: "p", Filename: "f", Size: 1, State: "completed", Bytes: 1, FilePath: "/tmp/f"})

	got, _ := s.Get(ctx, "h")
	if got.State != "completed" {
		t.Errorf("state %q, want completed", got.State)
	}
	if got.FilePath != "/tmp/f" {
		t.Errorf("filepath %q", got.FilePath)
	}
}

func TestStore_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Upsert(ctx, Record{HandleID: "h", Peer: "p", Filename: "f", Size: 1, State: "queued"})
	if err := s.Delete(ctx, "h"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := s.Get(ctx, "h")
	if err != ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestStore_All(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i, id := range []string{"a", "b", "c"} {
		_ = s.Upsert(ctx, Record{
			HandleID: id, Peer: "p", Filename: "f", Size: int64(i),
			State: "queued",
		})
	}
	rows, err := s.All(ctx)
	if err != nil {
		t.Fatalf("All: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("got %d rows, want 3", len(rows))
	}
}

// TestStore_Persistence verifies that records survive Close+Reopen on the
// same file — the resume contract.
func TestStore_Persistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "persist.db")
	ctx := context.Background()

	s1, err := Open(path)
	if err != nil {
		t.Fatalf("Open 1: %v", err)
	}
	if err := s1.Upsert(ctx, Record{
		HandleID: "resume",
		Peer:     "peerX",
		Filename: "resume.mp3",
		Size:     9999,
		State:    "transferring",
		Bytes:    4096,
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	_ = s1.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("Open 2: %v", err)
	}
	defer s2.Close()

	got, err := s2.Get(ctx, "resume")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Bytes != 4096 {
		t.Errorf("bytes %d after reopen, want 4096", got.Bytes)
	}
	if got.State != "transferring" {
		t.Errorf("state %q after reopen, want transferring", got.State)
	}
}
