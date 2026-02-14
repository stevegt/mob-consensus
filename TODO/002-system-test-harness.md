# TODO 002 - System test harness + manual plan

Goal: system-test the Go `mob-consensus` CLI with **three simulated users** without creating multiple Linux accounts.

Approach: use **three separate clones/worktrees** plus per-clone Git identity (`user.name`/`user.email`), driven by `scripts/mc-test`.

- [ ] 002.1 Create a local bare “remote” and seed it with `main`.
- [ ] 002.2 Create 3 clones (`alice`, `bob`, `carol`) and set per-clone `user.name`/`user.email`.
- [ ] 002.3 Build a local `mob-consensus` binary and run it from each clone.
- [ ] 002.4 Run the scenarios below via `scripts/mc-test` and record results (pass/fail + notes).

## Harness script

Use `scripts/mc-test` to create a temporary workspace with a local bare repo acting as the remote, plus N clones configured with repo-local identity (`<user>@example.com`).

Quick start (runs setup + scenarios):

```bash
scripts/mc-test all
```

Create a harness only (prints the harness root directory):

```bash
ROOT="$(scripts/mc-test harness)"
echo "$ROOT"
```

Run one scenario in an existing harness:

```bash
scripts/mc-test run --root "$ROOT" --scenario merge
```

Default `scripts/mc-test` mode is `--noninteractive` so it can be used as a fast smoke test without launching editors or `vimdiff`. If you want to observe the real interactive UX, rerun with `--interactive`.

## Scenarios covered by `scripts/mc-test`

The scenario runner performs these checks:

- `bootstrap`: leader creates/pushes the shared twig and creates/pushes `leader/<twig>`.
- `join`: other users create/push `user/<twig>` branches based on the shared twig.
- `discovery`: creates commits and asserts the discovery output includes “ahead”, “behind”, and “has diverged”.
- `merge`: merges `other/<twig>` onto `leader/<twig>` using shorthand resolution + confirmation; asserts the merge commit contains a `Co-authored-by:` for the other user; then reruns the same merge to ensure a no-op merge succeeds.

## Success criteria
- No remote name is assumed by `mob-consensus -b`.
- “Already up to date” merges are treated as success.
- Discovery clearly distinguishes ahead/behind/diverged/synced.

## Manual follow-ups (still worth doing occasionally)

- Conflict merge UX: run `scripts/mc-test all --interactive`, create a real conflict, and confirm `mergetool` launches and you can complete the merge.
- `-c` dirty-tree flow: verify `mob-consensus -c` works as intended (commits dirty state, then pushes via `smartPush`).
- Fetch failures are errors: break `git fetch` (remove remote or set bogus URL) and confirm `mob-consensus` exits non-zero with a human-readable error.
