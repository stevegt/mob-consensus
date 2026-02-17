# TUI testing experiments

This directory is an **experiment** playground for driving and scraping real text-mode (PTY) programs from Go.

It explores a few approaches:

- `github.com/creack/pty`: spawn a child process in a PTY (read/write bytes directly).
- `github.com/hinshun/vt10x`: parse ANSI/VT output and scrape a “screen” snapshot.
- `github.com/Netflix/go-expect`: an expect-like API on top of a PTY.
- `tmux` (optional): run a TUI inside a detached tmux server and `capture-pane`.

Everything here is **Unix-only** (Linux/macOS); Windows isn’t supported by these PTY-based approaches.

## Run

From this directory:

```bash
go mod tidy
go run ./cmd/vt10x-bytes
go run ./cmd/pty-raw
go run ./cmd/pty-vt10x
go run ./cmd/expect-raw
go run ./cmd/expect-vt10x
go run ./cmd/tmux-capture
```

Each command drives an embedded mini TUI (`tuidemo`) and prints what it captured (raw bytes and/or a `vt10x` screen dump).

Notes:
- The `tmux` experiment uses an isolated server socket (`tmux -S <sock>`), so it should not interfere with your existing tmux sessions.

