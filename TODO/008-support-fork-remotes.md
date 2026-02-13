# TODO 008 - Support fork-based collaboration (multi-remote)

Context: `mob-consensus` was originally used where collaborators all
have write access to a single shared remote. We also need to support
the case where each collaborator only has write access to **their own
fork**, and other collaborators can only fetch from it.

Goal: make discovery/merge/push workflows work predictably with multiple
remotes (ex: `origin` = your fork, `upstream` = canonical, plus `jj`,
`bob`, etc).

## Can mob-consensus do this already?

Partially:
- Merge mode can merge from any ref that `git merge` can see, including
  remote-tracking refs like `jj/alice/feature-x`, *if the remote has been
  fetched*.
- Discovery lists local + remote-tracking branches from `git branch -a`,
  so it can show peers across multiple remotes *once those remotes are
  present and up to date locally*.

Gaps:
- The tool currently runs `git fetch` with no remote specified; this does
  not necessarily update all remotes in a multi-fork setup.
- Auto-push behavior can fail if the current branch’s upstream remote is
  not writable (common when tracking a shared twig on someone else’s
  fork, or tracking `upstream/*`).
  - Policy: mob-consensus should only ever push to a configured “push
    remote” (normally your fork, often named `origin`) and never push to
    `upstream` (canonical) or collaborator remotes.
- Help/examples currently assume a single “chosen remote” for peer refs;
  in fork workflows the peer remote varies by collaborator.

## Proposed behavior (high level)

- Separate concerns:
  - **Fetch remotes**: which remotes to update for discovery/merge inputs.
  - **Push remote**: where *your* branch should be pushed (your fork).
  - **Twig source remote** (optional): where a “shared twig” branch is
    fetched from for onboarding (in fork workflows this may be the first
    group member’s fork). This is fetch-only; pushes still go only to
    the push remote.

- Prefer deterministic behavior and clear errors over guessing.

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

Config directory (ex: `.mob-consensus/` with `config.toml` and `remotes.d/`):
- XXX use .mob-consensus/remotes/
- Pros:
  - Scales better: separate files for remotes/policy/templates reduces conflicts.
  - Can go “one collaborator per file” (ex: `.mob-consensus/remotes.d/jj.conf`), which avoids
    everyone editing the same file.
  - Easier to extend without turning one file into a wall of settings.
- Cons:
  - More moving parts; less obvious for first-time users.
  - Slightly more work to document and implement.

### Naming options

File:
- `mob-consensus.toml` (very discoverable)
- `.mob-consensus.toml` (keeps root clean)
- `mob-consensus.conf` (line-oriented, easy to parse without extra deps)

Directory:
- `.mob-consensus/config.toml` (clear “tool-owned” area)
- `.mob-consensus/remotes` (line-oriented remotes list)  XXX use this one

### Format considerations

- XXX does this matter if we're using .mob-consensus/remotes/ with one file per remote?

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
  - [ ] 008.1.2 Add `--push-remote <remote>` (or support using
    `git config remote.pushDefault`) for upstreamless pushes.
    - Push remote should be your fork (often `origin`); never push to
      `upstream` or collaborator remotes.
  - [ ] 008.1.3 (Optional) Add `--twig-remote <remote>` for onboarding:
    where the “shared twig” branch is fetched from. (In a `start` flow,
    the twig is created and pushed to the push remote; in a `join` flow,
    the twig is fetched from someone else’s fork.)
  - [ ] 008.1.4 Add a repo-tracked collaborator config (file or dir) that lists remotes.
    - [ ] 008.1.4.1 Decide: single file vs config dir; pick a name/path.
    - [ ] 008.1.4.2 Decide format (TOML vs line-oriented `.conf` vs JSON).
    - [ ] 008.1.4.3 Define minimal schema: collaborator remotes + optional defaults.
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
  - [ ] 008.4.1 Determine a “push remote” (never guess when ambiguous):
    - prefer `branch.<name>.pushRemote`
    - else `remote.pushDefault`
    - else `origin` if present
  - [ ] 008.4.2 Always push explicitly to the push remote (avoid bare
    `git push` which can target an unwritable upstream remote).
  - [ ] 008.4.4 Ensure we never silently push to a guessed remote when
    multiple remotes exist.
- [ ] 008.5 Update help/docs to cover forks explicitly.
  - [ ] 008.5.1 Add a short “fork workflow” section to `usage.tmpl`:
    add collaborator remotes and merge from `REMOTE/<user>/<twig>`.
  - [ ] 008.5.2 Explain how to configure push remote for fork workflows
    (example: `git config --local remote.pushDefault <your-fork-remote>`).
  - [ ] 008.5.3 Ensure examples don’t assume `origin`.
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
