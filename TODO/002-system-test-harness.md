# TODO 002 - System test harness + manual plan

Goal: system-test the Go `mob-consensus` CLI with **three simulated users** without creating multiple Linux accounts.

Approach: use **three separate clones/worktrees** plus per-clone Git identity (`user.name`/`user.email`).

- [ ] 002.1 Create a local bare “remote” and seed it with `main`.
- [ ] 002.2 Create 3 clones (`alice`, `bob`, `carol`) and set per-clone `user.name`/`user.email`.
- [ ] 002.3 Build a local `mob-consensus` binary and run it from each clone.
- [ ] 002.4 Run the manual test matrix below and record results (pass/fail + notes).

## Harness (copy/paste)

This creates a temporary workspace with a local bare repo acting as the remote.

```bash
set -euo pipefail
ROOT="$(mktemp -d)"
REMOTE="$ROOT/remote.git"
git init --bare "$REMOTE"

# Seed main
git clone "$REMOTE" "$ROOT/seed"
git -C "$ROOT/seed" config user.name Seed
git -C "$ROOT/seed" config user.email seed@example.com
git -C "$ROOT/seed" switch -c main
echo hello > "$ROOT/seed/README.md"
git -C "$ROOT/seed" add README.md
git -C "$ROOT/seed" commit -m "Seed"
git -C "$ROOT/seed" push -u origin main

# Build mob-consensus from this repo
go build -o "$ROOT/mob-consensus" .
MC="$ROOT/mob-consensus"
MC_INIT="$(pwd)/scripts/mc-init"

# Simulated users
for u in alice bob carol; do
  git clone "$REMOTE" "$ROOT/$u"
  "$MC_INIT" "$ROOT/$u" "$u"
done
```

## Manual test matrix

### Bootstrap (first member)
- In `alice/`, create a local twig branch from whatever base you want:
  - `git -C "$ROOT/alice" switch -c feature-x main`
- Create the personal branch from the local twig:
  - `(cd "$ROOT/alice" && "$MC" -b feature-x)`
- Verify:
  - Branch is now `alice/feature-x`.
  - Output prints a suggested `git push -u ... alice/feature-x`.
- Push it:
  - `git -C "$ROOT/alice" push -u origin alice/feature-x`

### Join (next members)
- In `bob/` and `carol/`, create `feature-x` locally (either from `main`, or by tracking the remote twig if you pushed it):
  - `git -C "$ROOT/bob" switch -c feature-x main`
  - `git -C "$ROOT/carol" switch -c feature-x main`
- Create personal branches:
  - `(cd "$ROOT/bob" && "$MC" -b feature-x)`
  - `(cd "$ROOT/carol" && "$MC" -b feature-x)`
- Push:
  - `git -C "$ROOT/bob" push -u origin bob/feature-x`
  - `git -C "$ROOT/carol" push -u origin carol/feature-x`

### Discovery output
Create commits and verify statuses from one user’s perspective (run in `alice/`):
- **Ahead**: make a commit on `bob/feature-x` and push; run `"$MC"` in `alice/` and confirm `bob/feature-x` reports “ahead”.
- **Behind**: make a commit on `alice/feature-x` only; confirm the same branch reports “behind” from Bob’s view (run `"$MC"` in `bob/`).
- **Diverged**: make commits on both `alice/feature-x` and `bob/feature-x` without merging; confirm it reports “has diverged”.
- **Synced**: after merging/pushing, confirm it reports “synced”.

### Merge mode (manual)
Run merges in `alice/`:
- Clean merge: `"$MC" origin/bob/feature-x` (resolve/review, commit, push).
- No-op merge: run the same command again; it should exit success and not try to commit.
- Conflict merge: create an intentional conflict between two branches; confirm mergetool launches and the flow continues to commit once resolved.

### Flags / edge cases
- `-c` dirty tree: create an uncommitted change; ensure `-b` or merge fails without `-c` and succeeds with `-c` (note: `git commit -a` may open an editor).
- `-n` no-push: in merge mode, confirm it commits but prints a reminder instead of pushing.
- Missing/failed fetch: temporarily break `git fetch` (e.g., remove remote) and confirm discovery/merge warns and continues with local refs.
- Multiple/no remotes: ensure `-b` prints reasonable push advice when `origin` is missing.

## Success criteria
- No remote name is assumed by `mob-consensus -b`.
- “Already up to date” merges are treated as success.
- Discovery clearly distinguishes ahead/behind/diverged/synced.
