package gosk

import (
	"context"
	"errors"
	"testing"
	"time"
)

func eventuallyState(
	t *testing.T,
	c *Client,
	h DownloadHandle,
	want DownloadStateKind,
	timeout time.Duration,
) DownloadState {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		st, err := c.DownloadStatus(context.Background(), h)
		if err != nil {
			t.Fatalf("DownloadStatus: %v", err)
		}
		if st.State == want {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	st, _ := c.DownloadStatus(context.Background(), h)
	t.Fatalf("state never reached %q; last snapshot = %+v", want, st)
	return st
}

func TestDownload_HappyPath(t *testing.T) {
	sess := &fakeSession{
		downloadStatus: []string{"queued", "transferring", "writing"},
		downloadErr:    nil, // clean finish
	}
	c := mkFakeClient(t, sess)

	h, err := c.Download(context.Background(), "peer1", "song.mp3", 1234)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if h.ID == "" {
		t.Fatal("empty handle ID")
	}

	st := eventuallyState(t, c, h, DownloadCompleted, 500*time.Millisecond)
	if st.Size != 1234 {
		t.Errorf("size = %d, want 1234", st.Size)
	}
	if st.FilePath == "" {
		t.Error("FilePath is empty on completed state")
	}
	if !hasBasename(st.FilePath, "song.mp3") {
		t.Errorf("FilePath %q doesn't end in song.mp3", st.FilePath)
	}
}

func TestDownload_PropagatesTransferringState(t *testing.T) {
	// Hold off the terminal error so the tracker lingers in Transferring
	// long enough for the test's polling loop to observe it.
	sess := &fakeSession{
		downloadStatus:    []string{"transferring"},
		downloadTermDelay: 150 * time.Millisecond,
		downloadErr:       errors.New("simulated early end"),
	}
	c := mkFakeClient(t, sess)

	h, err := c.Download(context.Background(), "peer1", "x.mp3", 100)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	// Look for DownloadTransferring before it flips to Failed.
	found := false
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		st, _ := c.DownloadStatus(context.Background(), h)
		if st.State == DownloadTransferring {
			found = true
			break
		}
		if st.State == DownloadFailed {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !found {
		t.Error("never observed DownloadTransferring")
	}
}

func TestDownload_Failed(t *testing.T) {
	sess := &fakeSession{
		downloadStatus: []string{"queued"},
		downloadErr:    errors.New("peer refused upload"),
	}
	c := mkFakeClient(t, sess)

	h, err := c.Download(context.Background(), "peer1", "x.mp3", 100)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	eventuallyState(t, c, h, DownloadFailed, 500*time.Millisecond)
}

func TestDownload_UnknownHandle(t *testing.T) {
	sess := &fakeSession{}
	c := mkFakeClient(t, sess)
	_, err := c.DownloadStatus(context.Background(), DownloadHandle{ID: "nope"})
	if err != ErrUnknownHandle {
		t.Errorf("got %v, want ErrUnknownHandle", err)
	}
}

func TestDownload_NotLoggedIn(t *testing.T) {
	sess := &fakeSession{}
	c := newWithSession(testConfig(), sess)
	_, err := c.Download(context.Background(), "peer", "x", 1)
	if err != ErrNotLoggedIn {
		t.Errorf("got %v, want ErrNotLoggedIn", err)
	}
}

func TestDownload_HandlesUnique(t *testing.T) {
	sess := &fakeSession{
		downloadStatus: []string{"queued"},
		downloadErr:    errors.New("finish"),
	}
	c := mkFakeClient(t, sess)
	h1, _ := c.Download(context.Background(), "p", "a.mp3", 1)
	h2, _ := c.Download(context.Background(), "p", "a.mp3", 1)
	if h1.ID == h2.ID {
		t.Errorf("expected unique handle IDs, both got %q", h1.ID)
	}
}

func TestDownload_Basename(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{`@@peer\dir\song.mp3`, "song.mp3"},
		{`/abs/path/song.mp3`, "song.mp3"},
		{`song.mp3`, "song.mp3"},
	}
	for _, tc := range cases {
		if got := basename(tc.in); got != tc.want {
			t.Errorf("basename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func hasBasename(full, base string) bool {
	return len(full) >= len(base) && full[len(full)-len(base):] == base
}
