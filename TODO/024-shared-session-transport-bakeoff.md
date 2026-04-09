# TODO 024 - Shared cross-machine session fabric (Git vs PromiseGrid bakeoff)

## Decision Intent Log

ID: DI-024-20260409-120000  
Date: 2026-04-09 12:00:00  
Status: active  
Decision: Define a single canonical, append-only session event model first, then run a transport bakeoff between Git-based replication and PromiseGrid relay patterns.  
Intent: Replace machine-local, non-shared session history with a multi-user session fabric that supports human and agent collaboration, early conflict detection, and consensus tracking across machines.  
Constraints: Security-first; no mixing session history into the code repo; per-user write ownership enforced cryptographically; support freeform chat and structured DF/DI/DR workflow events in v1; ranking tie-break is security > simplicity > latency.  
Affects: `TODO/024-shared-session-transport-bakeoff.md`, `TODO/TODO.md`

## Problem statement

Codex session context is local to one machine. mob-consensus needs a shared session fabric so collaborators and agents can:

- communicate in-tool (freeform + structured),
- publish and consume DF/DI/DR state,
- share pre-commit diff previews to detect emerging conflicts early,
- detect consensus drift before code-level merge conflicts accumulate.

## Scope and non-goals

### In scope

- Canonical transport-agnostic event model.
- Per-user session streams with append-only semantics.
- Encryption, signatures, and key lifecycle model.
- Git and PromiseGrid transport candidates compared against one interface.
- Security and privacy threat model.
- Bakeoff matrix and winner selection criteria.

### Out of scope (this TODO)

- Production implementation in `main.go` or CLI commands.
- Automatic migration of existing local Codex session stores.
- Final UX polish.

## Locked decisions

1. Use a canonical event model and then compare transport adapters.
2. Propagate full pre-commit diff previews and session consensus signals.
3. Store shared session history outside the code repo.
4. Partition shared history by per-user stream.
5. Allow reading peer streams, but only stream owners can publish to their own streams.
6. Enforce ownership with cryptographic verification.
7. Use per-recipient envelope encryption.
8. Sign each event payload.
9. Include freeform in-tool chat in v1.
10. Bakeoff tie-break order is security > simplicity > latency.

## Architecture baseline (to be evaluated, not yet implemented)

### Canonical event envelope

Define a transport-agnostic envelope containing at least:

- event identity: `event_id`, `project_id`, `stream_id`, `seq`, `ts`
- classification: `event_type`
- content indirection: `payload_cid`, `payload_encoding`
- causality: `parents[]`
- provenance: `signing_key_id`, `signature`
- confidentiality metadata: `cipher_meta`

### Event types (minimum v1 set)

- `chat.message`
- `df.request`, `df.lock`
- `dr.open`, `dr.resolve`
- `di.record`
- `worktree.diff.preview`
- `conflict.risk.signal`
- `approval.request`, `approval.grant`, `approval.reject`

### Security model

- Dedicated mob-consensus keys (not host SSH login keys).
- Per-recipient encryption for payload confidentiality.
- Per-event signatures for stream ownership and tamper evidence.
- Project collaborator registry with pseudonymous `actor_id` + public keys.

## Bakeoff candidates

### Git transport family

- G1: one shared state repo per project, per-user stream paths.
- G2: one state repo per session.
- G3: one state repo with branch-per-user stream mapping.

### PromiseGrid transport family

- P1: relay-style pCID publish/subscribe adapter.
- P2: fuller node workflow candidate.
- P3: protocol-only baseline if implementation cost blocks P2.

## Evaluation matrix

Score each candidate (0-5), then apply tie-break order:

1. Security
   - stream spoof resistance
   - confidentiality boundary quality
   - key rotation and revocation practicality
2. Simplicity
   - setup/onboarding burden
   - operator recovery/debug effort
   - day-2 operational complexity
3. Latency
   - time to peer awareness for pre-commit overlap
   - resilience under delayed/dropped transport delivery
4. Determinism and reproducibility
5. Observability and auditability

## Test scenarios for bakeoff

1. Reject forged write to `user/<other>` stream.
2. Verify non-recipient cannot decrypt encrypted payload.
3. Emit overlap warning on concurrent pre-commit edits to same path.
4. Emit consensus-drift warning when DR/DI lines diverge.
5. Catch up correctly after offline window.
6. Enforce key rotation/revocation behavior.
7. Verify semantic equivalence across transports.
8. Test replay/out-of-order/drop conditions.
9. Exercise mixed human/agent participation in the same session.

## Implementation checklist

- [ ] 024.1 Define canonical event envelope and canonicalization rules.
- [ ] 024.2 Define event payload schemas and versioning policy.
- [ ] 024.3 Define collaborator identity/key registry schema.
- [ ] 024.4 Define signature verification and stream ownership rules.
- [ ] 024.5 Define per-recipient encryption envelope and metadata.
- [ ] 024.6 Define Git transport adapter contracts (G1/G2/G3).
- [ ] 024.7 Define PromiseGrid adapter contracts (P1/P2/P3).
- [ ] 024.8 Write threat model and trust assumptions.
- [ ] 024.9 Build scored bakeoff matrix template and scoring rubric.
- [ ] 024.10 Specify acceptance tests and deterministic fixtures.
- [ ] 024.11 Run bakeoff and record winner + runner-up.
- [ ] 024.12 Publish migration plan from local session workflows.

## Acceptance criteria

1. One canonical event model is used by all candidates.
2. Ownership and confidentiality rules are explicit and testable.
3. Pre-commit conflict and consensus-drift signals are first-class events.
4. Matrix scoring is complete enough to pick a winner without ad hoc decisions.
5. Winner selection includes explicit rationale and fallback path.
