# 2026-10-31 cmdcem-mac CEM and command-catalog batch

Lane cmdcem-mac took V1-2685, V1-2886, V1-1383, V1-2604, V1-2955, V1-3048, V1-3251, V1-1413,
V1-1451, V1-2209, V1-3404, V1-3415, V1-3443 and V1-3491 at origin/main
`8daec98ce4349c1583186692726d2c3a782ac647` on branch `claude/lane-cmdcem-mac-1031`. Requirement
IDs came from the lane block 930-939: `CEM-V1-930`..`CEM-V1-936` and `CAP-V0-930`..`CAP-V0-932`.
Go tests ran through `script/heavy.sh`, one package per command.

## Delivered

- V1-3415: `cemcode.CodeOf` walks `errors.Join` trees as well as single wraps, so a registered code
  inside a joined error is still found.
- V1-3491: a `release-artifact` `--revision` other than `HEAD` now refuses alone with the
  detached-worktree fix instead of a second, misleading refusal.
- V1-3251: the cli-parity `TestMain` pins the process umask as `main` does, so file modes in the
  parity transcripts no longer depend on the caller's umask. The ticket's provider sub-case did not
  reproduce on this host (darwin, go1.27.2); that case is left unproven, not refuted.
- V1-2886: STA-V0-001 re-pins OpenSpec 1.16.0 and Spec Kit 1.2.5 with regenerated locks. The ticket
  quoted older pins than origin/main held; the spec and locks were updated from the current
  upstream releases, and the first CI run is `NOT_RUN` until the batch PR runs.
- V1-3404 (CEM-V1-930..933): the native verifier accepts `old mode`/`new mode` headers beside
  textual hunks. Decision: a mode change is part of the same file entry, not a separate binary
  entry, so the hunk accounting is unchanged.
- V1-3443 (CEM-V1-934..936): the portable consumer in `interop/cem01-go` accepts the canonical
  binary section (fixture `interop/cem01-go/testdata/binary-section/`). Decisions: the stable packet
  moves to r2 and r1 is preserved byte for byte under `protocol/cem-1.0/stable/revisions/r1/`, so
  existing r1 consumers keep a pinned reference; an OID that disagrees with the binary body is
  `invalid-field`, not a new code; the candidate route stays text-only.

## V1-2685: command trees for the two CEM companions (slice)

CAP-V0-930..932 give `corvint-cem-candidate` and `corvint-cem-experiments` corvint-command-tree/0
snapshots under `internal/capcatalog/trees/`, linked from the catalog, `llms.txt` and
`docs/CAPABILITIES.md`, and drop the CLI ratchet from 52 to 50. `clitree.Answer` (CAP-V0-931) is
the shared help prelude: it answers only `help`, `help --json`, `help PATH` and `--help`/`-h`
before `--`, and leaves every other argument vector to the binary's own parser, so no existing
invocation changes. Decision: each binary keeps its usage text as the tree source, which keeps the
snapshot and the printed usage from drifting (TestCAPV0930TreeSnapshotIsCurrent).

The ticket asks for all ratchet binaries and the remaining MCP servers. The other 50 binaries each
need their own usage text and help prelude; they were not attempted in this lane, so V1-2685 stays
NOT DELIVERED with this slice landed.

## ALREADY-DELIVERED

- V1-3048 duplicates V1-2511, delivered by `f33bb4971` on origin/main. Its acceptance checks pass
  at the lane base without change.

## Not delivered

- V1-1383 (CCF-V1-007 N-1 replay covers 9 of 12 verbs): `coreN1Skips` in
  `cmd/corvint/core_freeze_test.go` still skips `cem status`, `cem verify`, `ocm status` and
  `ocm verify` under decision 0525 §6. Owner decision 2026-10-30 drops v0.8.1 and rc.1; rc.3 is the
  first release and becomes the N-1. `make core-n1-replay` reports `NOT_APPLICABLE FIRST_RELEASE`
  until then, and local tags stop at `v1.0.0-rc.2`. The skips can only be lifted against a
  published rc.3 that reads cem/1.0, so this is BLOCKED on the rc.3 publication, an owner action.
- V1-2604 (CEM 2.0 C3): the CEM 2.0 proposal is not accepted intent (invariant 8), and the v1-0
  part is a discovery spike plus an inventory-completeness check that needs the owner's scope. No
  implementation was started. BLOCKED on owner intent.
- V1-2955 (live host dispatch witness): needs live Claude Code and OpenCode sessions on exact host
  tuples driven by `conformance/host-lifecycle-v1`. A build lane cannot run a separate supervised
  host session. BLOCKED on a live-host run.
- V1-1413, V1-1451, V1-2209: each needs measurements under controlled load (N burners, three
  recorded pairs at up to 1200 s each, or comparable-load slot waits). The Mac stood at load 120-160
  during this lane, so the measurements would not be comparable. NOT DELIVERED within the lane
  budget; heavy-slot waits seen here were 0 s on the offload host and up to 120 s locally.

## Friction

1. `script/heavy.sh` waits up to 120 s for a slot at Mac load 160+, even for one small `-run` test.
2. V1-2511 and V1-3048 are duplicates; the dedupe screen missed them.
3. V1-2886's ticket body quoted stale pins; origin/main had already moved.
4. Dogfood hooks time out under host load.
5. The PreToolUse raw-`go test` hook silently drops the whole compound command, so file edits in the
   same command are lost; the nested module also needs the `../../script/heavy.sh` path.
6. `make spec-toolchain-conformance` picks a python3 older than 3.11 unless
   `SPEC_TOOLCHAIN_PYTHON=/opt/homebrew/bin/python3.12` is set.
7. heavy.sh refuses a whole-package run even for small packages without `HEAVY_WHOLE_REASON`.
8. heavy.sh offloads darwin-only files to the Linux host (`interop/cem01-go`
   `stable_process_darwin.go`, `cmd/corvint` `affected_identity_darwin.go`).
9. heavy.sh prints "copied back tools/capabilities-check/trees.go" for a file it did not change.
10. `make spec-requirements` regenerates `REQUIREMENTS.tsv` but leaves it unstaged, although the
    Makefile comment says it stages.
11. The frozen Intent-status header refuses new IDs only at check time, costing a round trip.
