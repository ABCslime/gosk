# gosk — plan

Native Go Soulseek client, implementing the `soulseek.Client` contract
that [muzika](https://github.com/macabc/muzika) defines. Separate Go
module so it can evolve independently and be pinned/swapped without
affecting the muzika surface.

## Scope (v1, explicit)

**In:**
- Server login + persistent session (reconnect with backoff on drop)
- Time-bounded search (collect peer responses for N seconds, then return)
- Peer selection heuristics (file count threshold, queue length limit)
- Single-file download with resume across process restarts
- In-flight download state persistence (SQLite)
- Clean public API matching `soulseek.Client` shape

**Out (v1 non-goals):**
- Uploads
- Chat / room search / private messages
- Distributed parent/child participation
- Wishlists
- Web UI / metrics surface
- Multi-peer swarm download of a single file
- TLS / port obfuscation beyond what upstream provides

## Architecture

Built on top of [`github.com/bh90210/soul`](https://github.com/bh90210/soul)
v1.1.0, which gives us:
- Message framing + serialization for the server and peer protocols
- A high-level `client` subpackage with `Dial`, `Login`, `Search`, `Download`
- Goroutine plumbing for incoming messages

What we add:
1. **Typed public API** matching muzika's contract (`SearchResult`,
   `DownloadHandle`, `DownloadState`, `DownloadStateKind`).
2. **Time-bounded search orchestration**: soul's `state.Search` returns a
   channel that runs until the caller cancels. We accumulate for
   `window`, then close the subscription and flatten the responses.
3. **Download handle registry**: soul's `state.Download` returns
   `(statusCh, errCh)` pairs; gosk wraps each download in a tracker
   keyed by a handle ID, updating a state snapshot asynchronously so
   that `DownloadStatus(handle)` is a cheap lookup.
4. **State persistence**: SQLite table `gosk_downloads` tracking every
   started download. On process start, reconcile against files on disk
   so crashes don't leave dangling handles.
5. **Session abstraction**: internal `session` interface with a real
   impl wrapping `soul/client`, plus a fake impl for tests. Lets gosk
   unit-test its orchestration without real network.

## Module layout

```
gosk/
├── go.mod                  # module github.com/macabc/gosk
├── go.sum
├── PLAN.md                 # this file
├── README.md               # usage-facing doc
├── types.go                # SearchResult, DownloadHandle, DownloadState
├── config.go               # Config struct + defaults
├── errors.go               # exported errors
├── session.go              # internal session interface + soul impl
├── client.go               # public Client: New, Search, Download, DownloadStatus
├── downloads.go            # download handle registry + tracker goroutines
├── state/
│   ├── store.go            # SQLite-backed download state
│   └── store_test.go
└── *_test.go               # unit tests with fake session
```

## Phases

- **G1 — Scaffold.** go.mod, public types, Config, stub Client. Verify
  build + vet.
- **G2 — Session abstraction.** Define `session` interface; write
  `soulSession` wrapping `soul/client`; write `fakeSession` for tests.
- **G3 — Search.** Implement time-bounded collection. Test with fake
  session that emits scripted `FileSearchResponse` messages.
- **G4 — Downloads + handle registry.** Implement tracker goroutine,
  handle map, state transitions. Test with fake session that scripts
  status/error channel messages.
- **G5 — State persistence.** SQLite-backed store. Test persistence
  across "restarts" (new client pointing at same DB).
- **G6 — Integration assembly.** Wire all pieces; full unit-test run;
  ensure `go build ./...`, `go vet ./...`, `go test ./...` all pass.

Real-network validation against `server.slsknet.org` is a **deploy-time**
activity — requires a real Soulseek account and is not in the CI loop.
Tests use the fake session for deterministic coverage.

## Performance target

Under sustained load (one search + one download in flight): 40–80 MB
RSS. If the measured footprint exceeds 80 MB, gosk remains an
experiment; muzika sticks with slskd.

## Wire-up with muzika

gosk is imported into muzika at `cmd/muzika/main.go`. Because muzika's
`internal/soulseek.Client` is internal, we can't implement it directly
from gosk. Instead:

- gosk exposes its own `Client` type with `Search`, `Download`,
  `DownloadStatus` methods returning gosk types.
- muzika adds a thin adapter in `internal/soulseek/native.go` that
  wraps a `*gosk.Client` and translates field names (1-to-1; both
  packages share the same shape). The adapter is a few dozen lines.

Switching backends is flipping `MUZIKA_SOULSEEK_BACKEND=native` — no
other config changes required.
