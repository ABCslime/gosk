# gosk

Native Go Soulseek client. Built to plug into
[muzika](https://github.com/macabc/muzika) as an alternative to the
`slskd` daemon, but usable on its own.

**Status: v1, not yet production-validated.** The unit-test suite covers
the orchestration layer end-to-end with a fake session. Real-network
validation against `server.slsknet.org` requires a Soulseek account and
is a deploy-time step.

See [PLAN.md](./PLAN.md) for scope and non-goals.

## Install

```go
import "github.com/macabc/gosk"
```

```sh
go get github.com/macabc/gosk@latest
```

## Use

```go
cfg := gosk.DefaultConfig()
cfg.Username = os.Getenv("SOULSEEK_USERNAME")
cfg.Password = os.Getenv("SOULSEEK_PASSWORD")
cfg.DownloadFolder = "/data/music"
cfg.StatePath = "/var/lib/gosk/state.db" // optional; enables resume

c, err := gosk.New(cfg)
if err != nil { log.Fatal(err) }
defer c.Close()

if err := c.Login(ctx); err != nil { log.Fatal(err) }

// Search with a 10-second accumulation window.
results, err := c.Search(ctx, "some artist some song", 10*time.Second)
if err != nil { log.Fatal(err) }

// Pick a peer and start a download.
r := results[0]
h, err := c.Download(ctx, r.Peer, r.Filename, r.Size)
if err != nil { log.Fatal(err) }

// Poll status.
for {
    st, err := c.DownloadStatus(ctx, h)
    if err != nil { log.Fatal(err) }
    log.Printf("state=%s bytes=%d/%d", st.State, st.Bytes, st.Size)
    if st.State == gosk.DownloadCompleted || st.State == gosk.DownloadFailed {
        break
    }
    time.Sleep(time.Second)
}
```

## Resume across restarts

If `Config.StatePath` is set, every state transition is written to a
SQLite file. On the next process start, call `c.Resume(ctx)` to get the
list of unfinished downloads; re-issue `Download` for each one you want
to continue.

```go
pending, _ := c.Resume(ctx)
for _, rec := range pending {
    h, _ := c.Download(ctx, rec.Peer, rec.Filename, rec.Size)
    _ = h // watch it like any other
}
```

## Wire-up with muzika

muzika's `internal/soulseek.Client` is internal. gosk provides its own
types with matching shapes; a small adapter in muzika's
`internal/soulseek/native.go` bridges the two:

```go
// In muzika's internal/soulseek/native.go (NOT in gosk).
type NativeClient struct{ g *gosk.Client }

func (n *NativeClient) Search(ctx context.Context, q string, w time.Duration) ([]SearchResult, error) {
    res, err := n.g.Search(ctx, q, w)
    if err != nil { return nil, err }
    out := make([]SearchResult, len(res))
    for i, r := range res {
        out[i] = SearchResult{
            Peer: r.Peer, Filename: r.Filename, Size: r.Size,
            Bitrate: r.Bitrate, QueueLen: r.QueueLen, FilesShared: r.FilesShared,
        }
    }
    return out, nil
}
// ... Download and DownloadStatus similarly trivial.
```

Flip `MUZIKA_SOULSEEK_BACKEND=native` to activate.

## Layout

- `types.go` — public types (`SearchResult`, `DownloadHandle`, `DownloadState`).
- `config.go` — `Config` + `DefaultConfig()`.
- `errors.go` — exported sentinel errors.
- `client.go` — `Client`, `New`, `Login`, `Close`, `Resume`.
- `session.go` — internal session interface (seam for tests).
- `soul_session.go` — production impl, wraps `github.com/bh90210/soul/client`.
- `fake_session_test.go` — test-only scripted session.
- `search.go` — `Search`; accumulates peer responses for `window`.
- `download.go` — `Download`, `DownloadStatus`, handle registry, tracker goroutines.
- `downloads.go` — `downloadTracker` + status-string mapping.
- `state/store.go` — SQLite-backed persistence.

## Dependencies

- [`github.com/bh90210/soul`](https://github.com/bh90210/soul) v1.1.0 —
  Soulseek wire protocol
- [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) — pure-Go
  SQLite for state persistence (same choice as muzika; no CGO, clean
  `linux/arm64` cross-compile)

## Testing

```sh
go test ./...
```

All tests use the fake session. No network I/O, no real Soulseek credentials required.
