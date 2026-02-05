# TODO 004 - Per-repo test user wrapper script

Goal: make “touch testing” `mob-consensus` with simulated users easy **without creating Linux users** and without constantly prefixing commands with `USER=...`.

Idea: store a test username in **repo-local git config** (inside `.git/config`), then run `mob-consensus` via a small wrapper that:
1) reads the configured username, and
2) sets `USER=<that username>` before exec’ing `mob-consensus "$@"`.

- [ ] 004.1 Pick the repo-local config key name (recommend `mob-consensus.testUser`).
- [ ] 004.2 Implement a wrapper script (suggest `x/mc-test` or `bin/mc-test`) with two modes:
  - [ ] 004.2.1 `mc-test init <user>`: write repo-local config for this clone.
  - [ ] 004.2.2 `mc-test [mob-consensus args...]`: read config, set `USER`, exec `mob-consensus`.
- [ ] 004.3 In `init`, also set git identity for correct attribution:
  - `git config --local user.name <user>`
  - `git config --local user.email <user>@example.com` (or allow `--email` override)
- [ ] 004.4 Error behavior:
  - not in a git repo → fail with a short message
  - test user not configured → print `mc-test init <user>` hint
- [ ] 004.5 Update TODO 002 harness examples to use `mc-test` (optional).

## Example usage (per clone)

```bash
cd /path/to/alice-clone
./x/mc-test init alice
./x/mc-test -b feature-x
./x/mc-test
./x/mc-test origin/bob/feature-x
```

## Comparison with TODO 002 and TODO 003

- **TODO 002 (3-clone harness + manual plan)**: simulates 3 users realistically via separate clones and per-clone git config, but requires `USER=alice ...` prefixes (or careful shell env management).
  - `mc-test` **complements** TODO 002 by removing the repeated `USER=...` prefixes and reducing “wrong user” mistakes.
  - It does **not** remove the need for multiple clones if you want realistic fetch/push behavior and independent working trees.

- **TODO 003 (automated system test plan)**: targets repeatable `go test -tags=system` style tests.
  - Automated tests can set env/config directly; calling a wrapper script is optional.
  - `mc-test` is mostly a developer ergonomics tool; it can still be used by system tests, but adds an extra moving part.

## Pros for touch testing

- **Low friction**: once `init` is run in each clone, all subsequent commands “just work”.
- **Fewer footguns**: avoids accidentally running `mob-consensus` with the wrong `USER` value.
- **More faithful attribution**: if `init` also sets `user.email`, the tool’s `Co-authored-by:` exclusion behaves as expected.

## Cons / limitations

- **Not a full simulation by itself**: if you only use one clone and flip the configured user, you lose independent working trees and some remote-tracking realism.
- **Still need separate clones** for “three users” in the practical sense (parallel edits, independent fetch/push timing, conflict reproduction).
- **Identity coupling**: the wrapper sets `USER`, but commits also depend on `user.name/user.email`; `init` must set those too or results will be confusing.

## Will it work?

Yes, if:
- `mob-consensus.testUser` is set per clone, and
- `init` also sets `user.email` (and ideally `user.name`) per clone.

Things to watch for:
- `mob-consensus` uses `USER` for branch naming/validation; it uses `git config user.email` to exclude “self” from co-author trailers.
- If you’re testing merge flows non-interactively, you may still need `GIT_EDITOR=true` and difftool/mergetool config (see TODO 003).

