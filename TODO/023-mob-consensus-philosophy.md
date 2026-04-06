# TODO 023 - Mob-Consensus Philosophy

## Decision Intent Log

ID: DI-023-20260406-120000  
Date: 2026-04-06 12:00:00  
Status: active  
Decision: Publish a plain-English philosophy statement for mob-consensus, including required DF protocol and clear DF/DI/DR definitions.  
Intent: Keep the collaboration model explicit and stable as humans and AI agents use mob-consensus to converge work across branches and remotes.  
Constraints: No PR dependency; peer-to-peer pull-based workflow; ownership boundaries stay hard safety invariants; DI/DR protocol semantics are not hardcoded in mob-consensus runtime.  
Affects: `TODO/023-mob-consensus-philosophy.md`, `TODO/TODO.md`

ID: DI-023-20260406-190000  
Date: 2026-04-06 19:00:00  
Status: active  
Decision: Execute phase 1 philosophy alignment by updating TODO 008, TODO 015, TODO 022, README, and AGENTS with consistent peer-to-peer and DF/DI/DR facilitator language.  
Intent: Remove drift and contradictions before recovery/stabilization work so implementation decisions follow a single collaboration model.  
Constraints: Documentation/TODO alignment only in this phase; no runtime behavior changes; retain append-only DI history.  
Affects: `TODO/023-mob-consensus-philosophy.md`, `TODO/008-support-fork-remotes.md`, `TODO/015-cli-tui-api-interfaces.md`, `TODO/022-git-consensus-subcommand.md`, `README.md`, `AGENTS.md`, `TODO/TODO.md`

## Purpose

mob-consensus is an intermediate collaboration tool on the path to PromiseGrid implementation. Its job is to help people and agents converge code safely and frequently while building grid-era systems and related tooling.

## Core Philosophy

1. Peer-to-peer collaboration, not centralized control.
2. Pull-based coordination: collaborators fetch from peers and publish their own work.
3. Ownership boundaries are safety invariants: never push peer branches.
4. Decision-First (DF) protocol is required before implementation.
5. Human-in-the-loop review is required for integration quality.
6. mob-consensus is a facilitator for DI/DR workflows, not the DI/DR protocol engine.
7. This tool stays focused on merge and convergence workflow, not general orchestration.

## What DF, DI, and DR Mean

- DF (Decision-First) is the required process: gather and lock decisions before making changes, and stop when key decisions are missing or ambiguous.
- DI (Decision Intent) is the durable record of a settled decision and its rationale.
- DR (Decision Request) is the durable record of an unresolved question or blocker that needs a decision.

## Scope Boundaries

mob-consensus supports DI/DR process work by providing useful context to LLM-driven flows and by integrating approved workflow actions. It does not implement repository-specific DI/DR semantics as runtime logic.

## Architecture Direction

Git is the current substrate. Keep behavior deterministic and safety-first now, while keeping algorithm boundaries clean enough to import into Storm later.

## Implementation Checklist

- [x] 023.1 Align `TODO/008-support-fork-remotes.md` language with this philosophy.
- [x] 023.2 Align `TODO/015-cli-tui-api-interfaces.md` with facilitator boundaries and reusable engine direction.
- [x] 023.3 Align `TODO/022-git-consensus-subcommand.md` wording with no-PR, peer-to-peer workflow.
- [x] 023.4 Add a short philosophy section to `README.md`.
- [x] 023.5 Add a short philosophy protocol note to `AGENTS.md`.
- [x] 023.6 Clarify DI/DR facilitation boundaries in related TODO and docs where ambiguous.
