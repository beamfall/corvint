# Decision 0052 — owner accepts the first-wave proposals and authorises the build

Date: 2026-09-04. Status: accepted. Authority: repository owner, verbatim instruction
"everyting is accepted and approved. build", given after a status table listing every open
ticket, the five documents under Gate A review, the open owner inputs in
`docs/agent-memory/questions.md`, and the uncommitted state of the tree.

## What is accepted

1. The AT-06 proposal `FPK-V0-020..026` in `docs/specs/falsifiable-packet-v0.md`, the AT-07
   proposal `SBQ-V0-007..010` in `docs/specs/snapshot-batch-v0.md`, the AT-10 spec
   `docs/specs/compat-replay-runner-v0.md` (`CRR-V0-001..007`), and the AT-14 proposal
   `GLTP-V0-038..043` in `docs/specs/go-live-test-provider-v0.md`, each at the revision that lands
   from the round in progress (AT-06 rev 11, AT-07 rev 10, AT-10 rev 12, AT-14 rev 12). Their
   labelled "Proposed amendment (AT-nn, unaccepted)" blocks become accepted text, including the
   amendments they carry into `task-context-packet-v0.md` (TCP-V0-006),
   `go-production-kernel-migration-v0.md`, and `compat-trial-v0.md` (CTR-V0-001, CTR-V0-003).
2. The agent task manager plan `docs/plans/AGENT-TASK-MANAGER-2026-09-04.md` at revision 6 and
   its `ATM-V0-` requirement set; S0's registry entry is created experimental and this decision is
   the owner acceptance line that plan §10 (S0) and Codex round-6 finding 12 required.
3. The five first-wave contracts listed in `questions.md` on 2026-09-04: `SBQ-V0-001..006`,
   `GPK-V0-052`, `AFP-V0-009`, `CTR-V0-001..010`, and `DR-0022`. `index-snapshot-v0.md` admits
   `impact` as a snapshot reader.
4. The three 2026-09-01 audit questions close on the current reading: `authoritative_results`
   means non-advisory as coded; the SOL-V0-001 ledger carve-out stays as written; the ten
   workflows stay committed in `docs/PRODUCT.md` with the roadmap's demand gating unchanged.
5. The owner-owned CTR-V0-004/005/009 labels, budgets, rubric and baseline are delegated to the
   coordinator under the standing 2026-09-04 instruction to make owner calls and record them; they
   are recorded as coordinator calls when AT-09/AT-10 scoring runs.

## What is authorised

- Building the four code slices (AT-06, AT-07, AT-10, AT-14) and the task manager S0b/S1, with
  parallel builders under disjoint file ownership, fresh reviewers, and repair.
- The `docs/DOGFOOD.md` committed-change sequence with local commits on `main` only; nothing is
  pushed. Base for rollback: `01aa66ad071756f7308bb04b0ec379b051a231e3`.

## Coordinator calls recorded here

- The task manager is a standalone Go module at `~/projects/corvint-taskman`,
  binary `atm`, consuming Corvint as its evidence engine (plan §3).
- Reviews run on Codex `gpt-5.6-sol` plus Claude expert reviewers; Astra is reserved for problems
  that need it (owner rule 2026-09-04).

## Rollback

`git reset --hard 01aa66ad` on `main` removes every local commit of this wave; the task-manager
module is a separate directory and is removed by deleting it.
