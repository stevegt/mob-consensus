# TODO 008 - Support peer-to-peer multi-remote collaboration

Context: `mob-consensus` was originally used where collaborators all
have write access to a single shared remote. In practice, many teams
work peer-to-peer: each collaborator pushes only to **their own**
remote (often their fork), and everyone else fetches from it.

Goal: make discovery/merge/push workflows work predictably with multiple
remotes (ex: your remote = `origin`, plus collaborator remotes like
`jj`, `bob`, etc). There should be no special “fork mode”: multi-remote
is the normal case.

## Can mob-consensus do this already?

Partially:
- Merge mode can merge from any ref that `git merge` can see, including
  remote-tracking refs like `jj/alice/feature-x`, *if the remote has been
  fetched*.
- Discovery lists local + remote-tracking branches from `git branch -a`,
  so it can show peers across multiple remotes *once those remotes are
  present and up to date locally*.

Gaps:
- Fetch policy: the tool must fetch *all relevant remotes* (or error and
  require explicit configuration) so discovery/merge targets are up to
  date in a multi-remote setup.
- Push policy: mob-consensus must never push to a collaborator remote.
  In a peer-to-peer model, you always push only to **your** remote.
  Using an upstream remote as a push target is unsafe because upstream
  can be set to someone else’s remote.
- UX/docs: help/examples should explain the peer-to-peer model and stop
  implying there is a single shared writable remote.

## Proposed behavior (high level)

- Separate concerns:
  - **Fetch remotes**: which remotes to update for discovery/merge inputs
    (usually “all collaborators”).
  - **Push remote**: where *your* branch should be pushed (your remote).

- Prefer deterministic behavior and clear errors over guessing.

## Decisions (locked in)

- Peer-to-peer is the default: users fetch from many remotes but push
  only to their own remote.
- Push remote must be explicitly configured (prefer `branch.<name>.pushRemote`
  or `remote.pushDefault`). Do not push to an upstream remote.

## Repo-tracked collaborator configuration

Motivation: with multiple collaborator forks, new group members need a
painless way to discover all collaborator remotes, and existing members
need a canonical place to keep the list up to date.

### Config file vs config directory

Single config file (ex: `mob-consensus.toml` or `.mob-consensus.toml`):
- Pros:
  - Easy to discover/review and easy to point users at (“edit this file”).
  - Simple to parse and to keep in sync with `mob-consensus init`.
  - Works well for small groups.
- Cons:
  - Higher chance of merge conflicts if multiple people edit it at once.
  - Can become a “junk drawer” as settings accumulate.

Config directory (ex: `.mob-consensus/`):
- Common layouts:
  - “One collaborator per file”: `.mob-consensus/remotes.d/jj.conf`
  - “One collaborator per directory”: `.mob-consensus/u/alice/config` (or `.mob-consensus/users/alice/config`)
- Pros:
  - Scales better: separate files for remotes/policy/templates reduces conflicts.
  - “One collaborator per file/dir” avoids everyone editing the same file.
  - Easier to extend without turning one file into a wall of settings.
- Cons:
  - More moving parts; less obvious for first-time users.
  - Slightly more work to document and implement.

#### Per-collaborator subdirectory (`.mob-consensus/u/<collaborator>/…`)

Idea: instead of a flat `remotes.d/`, make a per-collaborator directory. Example:
- `.mob-consensus/u/alice/remote` (or `remote.url`)
- `.mob-consensus/u/alice/defaults.conf` (defaults/policy that apply *when interacting with alice*)
- `.mob-consensus/u/alice/notes.md` (optional human notes)

Pros:
- Natural place to store more than “remote URL” (ex: multiple forks, preferred merge target naming, trust/policy).
- Less likely to conflict: each collaborator mostly edits their own directory.
- Easier to grow later (new files) without inventing new top-level conventions.

Cons:
- Needs clear terminology: “user” here means *collaborator identity*, not the local OS user.
- Git remote names are local state; storing “remote = jj” in a repo-tracked file can be wrong on someone else’s machine unless we standardize remote naming (ex: remote name == collaborator id) or store URLs and have `mob-consensus init` add/verify remotes.
- Risk of mixing in per-local-user defaults that should instead live in `.git/config` or a non-committed local file.

### Recommended layout (decision)

Use a repo-tracked config directory with one collaborator per directory:
- `.mob-consensus/u/<id>/remote.url`

Where `<id>` is the collaborator id used in branch names (the email local-part,
e.g. `alice` from `alice@example.com`). This avoids `@` in paths and keeps the
branch naming and collaborator naming aligned.

Possible future contents:
- `.mob-consensus/defaults` (team-wide defaults like preferred twig name)
- `.mob-consensus/u/<id>/notes.md` (human notes)
- `.mob-consensus/u/<id>/mode` / `review-policy` (non-secret preferences)
- `.mob-consensus/u/<id>/remote.urls` (multiple URLs for the same collaborator)

### Naming options

File:
- `mob-consensus.toml` (very discoverable)
- `.mob-consensus.toml` (keeps root clean)
- `mob-consensus.conf` (line-oriented, easy to parse without extra deps)

Directory:
- `.mob-consensus/config.toml` (clear “tool-owned” area)
- `.mob-consensus/remotes.d/` (one file per collaborator remote)
- `.mob-consensus/u/<collaborator>/config` (per-collaborator directory)

### Format considerations

- If we store “one collaborator per file/dir”, the exact format matters less because each unit is small.
- If we store remote URLs, we must avoid accidentally committing tokens (recommend: document “no embedded credentials”).

- TOML/YAML: friendlier for humans, but adds a parsing dependency in Go.
- JSON: no extra deps, but unpleasant to hand-edit.
- Line-oriented `.conf`: easiest to implement without deps; good fit if the
  first feature is “list collaborator remotes”.

### Minimal config contents (suggestion)

- Collaborator remotes (names + URLs or derivable usernames).
- Optional defaults:
  - `fetch` remote list (which collaborator remotes to fetch)
  - `push` remote (your fork remote name; often `origin`)
  - `twigSource` remote (where to fetch the shared twig from when joining)

Note: repo config is shared; per-user overrides should live in `.git/config`
or a non-committed “local” config file (ex: `.mob-consensus.local.toml`).

## Subtasks

- [ ] 008.1 Define CLI/config knobs (keep defaults safe).
  - [ ] 008.1.1 Add `--fetch <remote>` (repeatable) and/or `--fetch-all`
    (`git fetch --all`) for multi-remote discovery/merge.
  - [ ] 008.1.2 Define a push-remote selection policy that is safe for peer-to-peer:
    - prefer `branch.<name>.pushRemote`
    - else `remote.pushDefault`
    - else error (do not guess; do not use upstream)
  - [ ] 008.1.3 Clarify flag semantics for multi-remote:
    - `--remote` (if kept) should be fetch-only.
    - Push always uses the configured push-remote policy above.
  - [ ] 008.1.4 Add a repo-tracked collaborator config directory that lists collaborator remotes:
    - [ ] 008.1.4.1 Adopt `.mob-consensus/u/<id>/remote.url` (one collaborator per directory).
    - [ ] 008.1.4.2 Define minimal schema: collaborator ids + remote URLs + optional defaults.
- [ ] 008.2 Update fetch logic.
  - [ ] 008.2.1 Default behavior: fetch in a way that reliably updates
    collaborator remotes (do not rely on a single “upstream remote”).
    Options:
    - conservative default: require explicit `--fetch <remote>` when
      multiple remotes exist
    - simple default: `git fetch --all` (errors on any failing remote)
    - configurable default: `git config --local mob-consensus.fetchRemotes "<r1> <r2> ..."`
  - [ ] 008.2.2 If `--fetch-all`, fetch all remotes and error if any
    remote fetch fails (consistent with “fetch failures are errors”).
  - [ ] 008.2.3 If `--fetch <remote>`, fetch only those remotes.
- [ ] 008.3 Improve merge target ergonomics in multi-remote setups.
  - [ ] 008.3.1 Keep accepting explicit refs like `jj/bob/feature-x`.
  - [ ] 008.3.2 (Optional) If user passes `bob/feature-x`, resolve it to
    a unique `*/bob/feature-x` across remotes; if ambiguous, error and
    list candidates.
- [ ] 008.4 Make push behavior fork-friendly.
  - [ ] 008.4.1 Determine a “push remote” using only explicit configuration:
    - prefer `branch.<name>.pushRemote`
    - else `remote.pushDefault`
    - else error (do not guess; do not use upstream)
  - [ ] 008.4.2 Always push explicitly to the push remote (avoid bare
    `git push` which can target a collaborator remote via upstream).
  - [ ] 008.4.4 Ensure we never silently push to a guessed remote when
    multiple remotes exist.
- [ ] 008.5 Update help/docs to cover forks explicitly.
  - [ ] 008.5.1 Add a short “peer-to-peer” section to `README.md` explaining:
    fetch from collaborators, push only to your remote, converge by repeated merge cycles.
  - [ ] 008.5.2 Update `usage.tmpl` and examples to stop implying a single shared writable remote.
  - [ ] 008.5.3 Explain how to configure push remote (example: `git config --local remote.pushDefault <your-remote>`).
  - [ ] 008.5.4 Ensure examples don’t assume `origin`.
- [ ] 008.6 Testing
  - [ ] 008.6.1 Extend TODO 002 harness to add multiple remotes.
  - [ ] 008.6.3 Add system tests (TODO 003) for: merge from a
    collaborator remote and push to a configured push-remote.

## Notes / pitfalls

- In multi-remote setups, the same branch name may exist in multiple
  remotes; any shorthand resolution must handle ambiguity safely.
- Fork workflows may not have a truly “shared” twig unless the group
  designates a twig remote; TODO 006 onboarding should incorporate
  this concept.
