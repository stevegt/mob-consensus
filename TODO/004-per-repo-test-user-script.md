# TODO 004 - Per-repo test user init script

Goal: make “touch testing” `mob-consensus` with simulated users easy **without creating Linux users** and without repeatedly typing `git config --local ...` by hand.

`mob-consensus` derives the `<user>/` branch prefix from `git config user.email` (the part left of `@`). For testing, we can use emails like `alice@example.com`.

Idea: add a small init script that configures each test clone’s **repo-local Git identity** once:

- `x/mc-init <repo-path> <username>`
  - sets `user.name=<username>`
  - sets `user.email=<username>@example.com`

- [x] 004.1 Implement `x/mc-init` (usage: `mc-init REPO_PATH USERNAME`).
  - [x] 004.1.1 Validate args and that `REPO_PATH` is a Git worktree.
  - [x] 004.1.2 Reject usernames that can’t be used as a branch prefix (`git check-ref-format --branch "$user/probe"`).
  - [x] 004.1.3 Set `user.name` and `user.email` in repo-local config (`--local`).
- [x] 004.2 (Optional) Update TODO 002 harness to call `x/mc-init` instead of inline `git config`.

## Example usage (per clone)

```bash
x/mc-init /path/to/alice-clone alice
x/mc-init /path/to/bob-clone bob

cd /path/to/alice-clone
mob-consensus -b feature-x
mob-consensus
mob-consensus origin/bob/feature-x
```

## Comparison with TODO 002 and TODO 003

- **TODO 002 (3-clone harness + manual plan)**: simulates 3 users realistically via separate clones and per-clone git config.
  - `mc-init` **complements** TODO 002 by making per-clone identity setup reproducible and reducing “wrong user.email” mistakes.
  - It does **not** remove the need for multiple clones if you want realistic fetch/push behavior and independent working trees.

- **TODO 003 (automated system test plan)**: targets repeatable `go test -tags=system` style tests.
  - Automated tests can set repo-local config directly; calling a helper script is optional.
  - `mc-init` is mostly a developer ergonomics tool.

## Pros for touch testing

- **Low friction**: once initialized per clone, all subsequent commands “just work”.
- **Fewer footguns**: avoids accidentally running with the wrong `user.email` (and therefore the wrong `<user>/` prefix).
- **More faithful attribution**: correct `user.email` makes the tool’s “exclude self” co-author behavior predictable.

## Cons / limitations

- **Not a full simulation by itself**: if you only use one clone and keep re-initializing, you lose independent working trees and some remote-tracking realism.
- **Still need separate clones** for “three users” in the practical sense (parallel edits, independent fetch/push timing, conflict reproduction).
- **Identity coupling**: branch naming and attribution depend on `user.name`/`user.email`; `mc-init` must set those or results will be confusing.

## Will it work?

Yes, if each clone has its own repo-local `user.email` (and ideally `user.name`) set.

Things to watch for:
- `mob-consensus` uses the part left of `@` in `git config user.email` for branch naming/validation; it also uses `user.email` to exclude “self” from co-author trailers.
- If you’re testing merge flows non-interactively, you may still need `GIT_EDITOR=true` and difftool/mergetool config (see TODO 003).
