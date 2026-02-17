# TODO 013 - TUI automation and scraping (PTY-driven tests)

Context: to extend `mc-test` and `go test` coverage for merge/conflict flows
(TODO 010/011/012), we may want Playwright-like automation for **text-mode**
programs running in a PTY (e.g., `git mergetool`, `git difftool`, `vim`/`nvim`).

Goal: identify a safe, realistic approach to drive and scrape terminal UIs from
tests (Go preferred), without interfering with a developer’s live terminal
environment.

## Options

### A) `tmux` as an isolated “terminal emulator + automation API”

Approach:
- Start a **private tmux server** using a dedicated socket under the test
  harness root, so tests never touch the user’s normal tmux server/sessions.
- Drive input with `tmux send-keys`.
- Scrape the rendered screen with `tmux capture-pane -p`.

Key isolation mechanism (do this everywhere):
- `tmux -S "$ROOT/tmux.sock" ...`
- `tmux -S "$ROOT/tmux.sock" kill-server`

Pros:
- Scraping is easy (`capture-pane`) because tmux maintains a screen buffer.
- Works with “real” TUI programs (Vim, Git’s prompts, pagers).
- Minimal code: can be orchestrated from Go tests via `exec.Command`.

Cons:
- External dependency (`tmux`) must be installed.
- Still requires careful cleanup; stale sockets/sessions can accumulate if tests crash.

### B) `github.com/Netflix/go-expect` (expect-style PTY driving)

Approach:
- Spawn a process in a PTY and match output patterns, send input.

Pros:
- Great for interactive CLI prompts (line-oriented).
- Pure Go dependency; no tmux requirement.

Cons:
- Full-screen TUIs emit ANSI escapes; “scraping” the *rendered screen* is hard
  unless paired with a terminal emulator.

### C) `github.com/creack/pty` + a terminal emulator (e.g., `github.com/hinshun/vt10x`)

Note: a hybrid `go-expect` + `vt10x` approach is also viable:
- `go-expect` gives you a friendlier “expect/send” API over a PTY.
- `vt10x` gives you a rendered screen buffer you can assert against.

Using `creack/pty` directly is lower-level but keeps dependencies minimal and
gives full control. Using `go-expect` can simplify prompt-driven flows; for
full-screen apps, you still need a screen model (e.g., `vt10x`) if you want to
assert on what the user *sees* rather than raw ANSI output.

Approach:
- Spawn process under a PTY (`creack/pty`).
- Feed output bytes into a VT/xterm emulator (`vt10x`) to maintain a screen buffer.
  - Conceptually: read bytes from the PTY master as the program runs, and write
    those bytes into the emulator. The emulator applies ANSI/VT escape sequences
    and maintains a 2D grid of cells. Tests can then snapshot/assert on that grid.
- Send key sequences to the PTY.
  - Conceptually: write bytes to the PTY master that represent keystrokes. For
    example, `\x1b` is Escape, `\r` is Enter; arrow keys are escape sequences
    like `\x1b[A`. For robust input, consider using `go-expect`’s helpers or a
    small key-encoding helper.
- Assert on the emulator’s screen state (or snapshot it).

Pros:
- No external dependencies (all Go).
- Works with real TUIs and supports true “screen scraping”.
- Deterministic and self-contained inside the test process.

Cons:
- More implementation effort than tmux.
- Need to handle timing, screen resize, and terminal capabilities carefully.

### D) Neovim RPC (best for Neovim-specific workflows)

Two relevant modes:

1) `nvim --headless`:
   - Great for deterministic scripted edits and inspection of buffers/files.
   - Not a TUI: no screen to scrape.

2) Neovim UI attach via RPC (`nvim --listen ...`, `nvim_ui_attach`):
   - Tests can send `nvim_input()` and receive structured “grid redraw” events
     to build a screen model (similar in spirit to a browser automation API).

Pros:
- Very powerful and deterministic for Neovim sessions.
- UI attach gives “rendered screen” data without emulating a terminal.

Cons:
- Only helps for Neovim, not arbitrary TUIs.
- Does **not** solve Git’s own interactive prompts *before* the editor launches
  (e.g., mergetool/difftool confirmation prompts).

## Important constraint: Git prompts vs editor UI

`git mergetool`/`git difftool` may prompt on the terminal *before* launching the
editor (and Git may also invoke pagers). Therefore:

- `nvim --headless` cannot drive those prompts (it bypasses the terminal UI).
- For automated tests we likely want to set:
  - `mergetool.prompt=false`, `difftool.prompt=false`, or pass non-interactive flags
    where available (`-y`, `--no-prompt`).
- For “true end-to-end interactive” system tests, we need a PTY driver (A/B/C).

## Recommendation (tentative)

- For system tests that must drive *real* TUI flows (including Git prompts),
  prefer either:
  - **A (tmux + private socket)** for speed of implementation and reliable scraping, or
  - **C (pty + vt10x)** for a pure-Go, dependency-free approach.
- For Neovim-only scenarios (e.g., editor integration, commit-message editing),
  consider **D (RPC UI attach)** as a higher-level “Playwright-like” API.

## Plan

- [ ] 013.1 Decide which approach to prototype first (A vs C).
- [ ] 013.2 Prototype a minimal harness that:
  - [ ] 013.2.1 Launches `vim`/`nvim` in a PTY.
  - [ ] 013.2.2 Sends keys, waits deterministically, and captures the screen.
  - [ ] 013.2.3 Ensures cleanup even on failure (kill process / kill tmux server).
- [ ] 013.3 Integrate with `scripts/mc-test` as an optional scenario for conflict UX:
  - [ ] 013.3.1 Drive a conflict merge and verify the review/approval UI is reachable.
  - [ ] 013.3.2 Save captured “screenshots” (pane text / VT dump) under the harness root.
- [ ] 013.4 Document requirements and safety:
  - [ ] 013.4.1 If using tmux, always use `-S "$ROOT/tmux.sock"` (no interference with user sessions).
  - [ ] 013.4.2 Add guardrails to prevent running outside a harness root.
