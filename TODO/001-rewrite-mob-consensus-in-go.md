# TODO 001 - Rewrite mob-consensus in Go

Goal: replace `x/mob-consensus` (Bash) with a small, testable Go CLI while keeping the workflow fast and familiar.

Migrated from the coordination repo (formerly “TODO 004 - Rewrite mob-consensus in Go”).

- [x] 001.1 Decide how it’s installed and upgraded (e.g., `go install ...@latest`) and where the Go entrypoint lives (e.g., `cmd/mob-consensus`).
- [x] 001.2 Specify the CLI contract and compatibility with the current script: list related branches, `-b BASE_BRANCH`, `-c` commit dirty tree, `-n` no-push, `-F` force, `[OTHER_BRANCH]` merge mode.
- [x] 001.3 Implement “related branches” discovery (branches ending in `/$twig`) and ahead/behind shortstat reporting.
- [x] 001.4 Implement branch creation from local or remote bases (including setting upstream).
- [x] 001.5 Implement merge flow: generate commit message with `Co-authored-by:` lines, run `git merge --no-commit --no-ff`, then launch mergetool/difftool.
- [ ] 001.6 Add config overrides for tools (`difftool`, `mergetool`, editor) and ensure non-interactive failure modes are clear.
- [x] 001.7 Add deterministic tests around parsing and branch selection logic (shelling out can be integration-tested later).
- [ ] 001.8 Plan the migration: keep the Bash script as a thin wrapper (or deprecate) once the Go tool is proven.
- [ ] 001.9 Define a reusable library boundary (so Storm can import the “engine” bits without adopting the CLI UX).

Decisions:
- Go entrypoint: module root (`main.go`)
- Install/upgrade: `go install github.com/stevegt/mob-consensus@latest`

## Library vs CLI Layout (for Storm reuse)
We may want to reuse parts of `mob-consensus` inside `~/lab/grokker/x/storm` (notably for the “consensus-merge (mob-consensus style)” and `Co-authored-by:` attribution mentioned in Storm TODOs like `TODO/025-promisegrid-node.md` and `TODO/026-planning-group-workspace-mvp.md`, and to align with the diff/approve/apply/commit workflow in `TODO/014-change-review-gate.md`).

### Pros of “library + cmd/”
- **Reuse without UX coupling**: Storm can call the same core logic while presenting it via web/daemon flows.
- **Smaller drift surface**: avoid copy/paste reimplementations of twig/branch discovery and `Co-authored-by:` generation.
- **Better testing**: keep deterministic logic in a package with unit tests; keep the CLI as a thin wrapper.

### Cons / tradeoffs
- **API stability pressure**: without `internal/`/`pkg/`, exported identifiers become de-facto public API.
- **Refactor overhead**: interactive steps (difftool/mergetool/editor) need to be pushed out of the library into the CLI.
- **Versioning/deps**: Storm will depend on this module; local dev may want `go.work` or a `replace` during iteration.

### Suggested minimal package boundary
Create a small package (e.g., `consensus/`) that is **non-interactive** and returns **structured data**.

Library responsibilities (reusable by Storm):
- Parse/derive `twig` and filter “related branches” (branches ending in `/<twig>`).
- Compute ahead/behind status using Git primitives (or by shelling out behind an interface).
- Generate `Co-authored-by:` trailers from a ref-range (`..OTHER_BRANCH`) with optional exclusions (current user email).
- Format a merge commit message header + trailers (but do not write `MERGE_MSG`).

CLI-only responsibilities (stay out of the library):
- Running `git mergetool` / `git difftool` and other interactive tooling.
- Writing `.git/MERGE_MSG` and doing `git commit -e ...`.
- Any “print-oriented” UX (blank lines, headers, alignment) except where it’s purely formatting output for the CLI.

Implementation sketch:
- `consensus` exports a narrow API like:
  - `Twig(branch string) string`
  - `RelatedBranches(branchAOutput, twig string) []string`
  - `CoAuthorLines(gitLogOutput, excludeEmail string) []string`
  - Optionally a `Runner` that takes a `Git` interface (`Output(ctx, args...)`, `Run(ctx, args...)`) so Storm can swap execution/mocking.

Constraints:
- Do not modify `x/mob-consensus` (keep as legacy reference/behavior oracle).

## How `mob-consensus` Works (Today)
`mob-consensus` is a Git workflow helper optimized for mob/pair sessions where collaborators each work on their own branch, but want rapid, repeated convergence without the overhead of PRs.

### Branch Convention (“twig”)
- Each collaborator uses a namespaced branch: `alice/<twig>`, `bob/<twig>`, etc.
- The shared suffix `<twig>` is the coordination key; the tool looks for other branches ending in `/<twig>`.

### Modes
- **Status / discovery mode (no args)**: `git fetch`, then lists related branches and prints whether each is ahead/behind/synced relative to the current branch.
- **Branch bootstrap (`-b BASE_BRANCH`)**: creates a new `$USER/<twig>` branch based on `BASE_BRANCH` and pushes it with an upstream.
- **Merge mode (`OTHER_BRANCH`)**: performs an explicit, manual merge of `OTHER_BRANCH` into the current branch, including conflict resolution and review, then commits and (optionally) pushes.

## How It Builds Multilateral Consensus
“Consensus” here is not a distributed-systems quorum protocol; it’s a repeatable social/technical workflow that creates a shared, auditable integration point:
- **Common comparison set**: everyone can run discovery mode and see the same set of related branches and deltas (ahead/behind).
- **Explicit integration events**: merges are done with `--no-commit --no-ff`, so the combined result is reviewed before a merge commit is created.
- **Shared review in the moment**: conflicts are resolved while the mob is present; a difftool review step makes the “what changed” discussion concrete.
- **Attribution as a forcing function**: the merge commit message includes `Co-authored-by:` lines derived from the merged branch’s history, increasing clarity and accountability.
- **Convergence is observable**: when all related branches report “synced” (or the same HEAD), the group has a simple, verifiable signal of agreement on the current state.

## Key Techniques Used for Reliable Agreement
- **Git’s three-way merge**: relies on `git merge` and the commit DAG (merge-base) for deterministic structure; conflicts are surfaced explicitly.
- **No fast-forward merges**: `--no-ff` preserves an explicit merge commit as an audit trail of the “we agreed on this integrated state” moment.
- **Gated commit creation**: `--no-commit` ensures the merge result is reviewed/resolved before it becomes history.
- **Symmetric-diff based status**: uses Git’s symmetric comparison (`...`) to detect whether branches have meaningful changes relative to each other.
- **Clean working tree requirement (optional `-c`)**: reduces accidental mixing of unrelated local edits into a merge.
- **Co-author extraction**: harvests unique author identities from commits reachable on the other branch but not on `HEAD`, excluding the current user, to populate commit trailers.

## Usage Flow (Typical Mob Session)
1. Each collaborator creates their own branch for the same twig (from a shared base):
   - Example: `mob-consensus -b feature-x` → creates `$USER/feature-x` and prints a suggested `git push -u ...`.
2. Collaborators work normally (edit/commit/push on their own `$USER/<twig>` branches).
3. Periodically, anyone runs `mob-consensus` (no args) to see which sibling branches are ahead/behind/diverged.
4. When it’s time to converge, pick a sibling branch and merge it into the current branch:
   - Example: `mob-consensus jj/feature-x`
   - Resolve conflicts (mergetool), review changes (difftool), commit (with co-authors), push (unless `-n`).
5. Repeat until the relevant sibling branches are “synced”, or until the group agrees the session’s integrated state is complete.

## How It Differs From PRs / Code Review
- **Multilateral vs bilateral**: PRs are typically one-to-one (contributor → reviewer); `mob-consensus` is many-to-many (multiple contributors converge together).
- **Integration location**: PRs usually merge into a branch owned by a primary maintainer (e.g., `main`); `mob-consensus` merges into each participant’s `$USER/<twig>` branch, with convergence achieved over time.
- **Review artifact**: PR discussion threads live outside Git history; `mob-consensus` records the consensus point as a merge commit plus co-author attribution.
- **Lower ceremony, higher frequency**: encourages frequent, small convergence steps rather than infrequent “big PR” events.
