# Conflict hotspots: frozen spec headers, per-target .PHONY (V1-3458)

Date: 2026-10-31. Lane `hotspots-w1`, base `85d5ad4b758193ef702a83632f4cb8c573b91933`.
Requirements: `SRG-V0-980` to `SRG-V0-983` in `docs/specs/spec-requirement-index-generator-v0.md`,
standing under the owner's auto-accept until read back.

## Measured cause

Re-merging the parents of the last 150 merges on `origin/main` with `git merge-tree` gave 23
conflicted merges. The repository-wide counts the ticket cites (INDEX.json 10, REQUIREMENTS.tsv 8,
README.md 7, two long specs 6 and 5) trace to the one-line spec `Intent status:` header: every
amendment appended a segment, and the generator mirrors the header into the digest `Status:`,
INDEX.json `intent` and the README row, so two lanes amending one spec conflicted in four files.
The Makefile's three conflicts each hit the shared `.PHONY:` line, the `GATE_STEPS =` line and the
`shell-test:` prerequisite line together.

## Decisions

1. **Freeze by baseline, not by hash.** `docs/specs/INTENT-STATUS-FROZEN.tsv` holds one row per spec
   whose header at the freeze named a requirement ID or exceeded 160 characters (84 rows): its
   character length and the IDs its header or digest `Status:` names. It is written once and never
   appended to, so it is not a new hotspot; a header may shrink or drop IDs without editing it. A
   per-spec hash was rejected because it would refuse every legitimate status change. The length
   bound catches a new segment that reuses a frozen ID; 160 matches the claim bound (SRG-V0-040),
   and 32 of 239 headers were over it. IDs are read as the SRG-V0-003 token plus each `..NNN` or
   `/NNN` suffix (so `X-V0-001..003` names X-V0-003), and the frozen rows were built with that reader.
   The guard is lexical: a segment that names no ID passes while the header stays within 160
   characters or its frozen length. A renamed spec moves its frozen row in the rename's commit.
2. **Refuse in the generator.** The refusal runs in every `gen-spec-requirements.sh` mode, before
   any output, as SRG-V0-040 and SRG-V0-547 do, so `make spec-requirements` reports it at the first
   spec step and `spec-requirements-check` (and so `make doc-gates`) enforces it. The message names
   the spec, the field and the ID or the two lengths, and gives the requirement-line repair.
3. **Amendments go on the requirement line (SRG-V0-981).** No new column or reader: the SRG-V0-009
   reader already extracts the leading note into `acceptance` and `acceptedBy`. An amendment adds
   `amended under the owner's standing auto-accept (YYYY-MM-DD); V1-NNNN` to the note, so an `owner`
   requirement reads `standing`, with every anchor, until the owner reads the amendment back. This
   is the intended demotion, not a loss.
4. **One `.PHONY:` line per target, next to its rule (SRG-V0-982).** All 11 multi-target lines were
   split (122 targets; `ai-coauthor-trailers-check` was declared twice and now once). `make -pn`
   lists the same phony set before and after. The check lives in
   `script/spec-requirements-conflict_test.sh` with the replay, so no new gate step was registered
   (registering one would itself append to `GATE_STEPS`, `shell-test` and the unbounded-readers list).
5. **Replay test (SRG-V0-983).** The conflict test replays two lanes that amend SEC-V0-001 and
   SEC-V0-011 on their lines and add `beta` and `psi` targets: they merge in both orders with no
   conflict and regenerate to the merged bytes. The controls (header segments, one shared
   `.PHONY:` line) conflict, and the generator refuses the header segment. A mutation in which both
   lanes amend the same requirement fails the replay, and case 13 of
   `script/gen-spec-requirements_test.sh` fails on the pre-change generator.

6. **Independent review (one subagent pass).** It found no correctness bug in the refusal, the frozen
   file (rebuilt independently, exact match) or the Makefile split (same 122 phony targets, none in a
   conditional). It found that the docs claimed more than the replay proved: amendments to
   adjacent definition lines, and two targets added at the same spot, still conflict. That limit is
   now in SRG-V0-983 and the Status model, and the replay asserts it (adjacent SEC-V0-001/002
   amendments conflict only in the spec and REQUIREMENTS.tsv). Also fixed from the review: range and
   slash tokens, the rename repair, a case-13 assertion that could not fail (now ID-free), the
   message's SRG-V0-009/981 pointer and its no-row length wording, and `.PHONY:a b` or a trailing
   comment in the `.PHONY` check.

## Rollout

An open lane PR that still adds an ID segment to a spec header or digest `Status:` merges cleanly
on text but turns `spec-requirements-check` red after this lands. Its repair is the refusal's: move
the segment into the requirement's leading note and regenerate.

## Documented, not fixed

- **`docs/AUTOMATION.md` (7 of 23 merges).** Its conflicts are line citations such as
  `cmd/corvint/affected.go:666-705@af892533` re-pinned by `make line-citations-check --repin` (or
  `make batch-regen`) in each lane that edits a cited hot file (`cmd/corvint/main.go`, `help.go`,
  `affected.go`). Two such lanes re-pin the same citation to different lines. Both sides are
  derived; take either and rerun `make batch-regen`. Preventing it would mean citing without line
  ranges, which weakens invariant 1.
- **`conformance/use-cases-v0/ledger.json` (6 of 23).** The ledger pins the sha256 of each use-case
  receipt, and `UC-CHANGE-CONSEQUENCE/implementation.json` pins the sha256 of `cmd/corvint/main.go`
  and `affected.go`. Any two lanes that edit those files both re-pin (`script/repin-use-case-receipts.sh`),
  so the digests conflict. Resolution is the same: take either side and run `make batch-regen`.
  Dropping the content pin is not an option under invariant 1.
- **`GATE_STEPS` and `shell-test:` in the Makefile.** Both are still single lines and conflicted in
  all three Makefile merges. They are not split here: `tools/test-commands-check` requires one
  plain `GATE_STEPS = ...` line and refuses `+=`, `TestSTRV0004ShellTestIsTheGateShellSteps` reads
  both literally, and `docs/SHELL-TEST-REGISTRATION.md`, `AFP-V0-025` and `CEM-V1-223` name the
  single line. Splitting them is a separate change to the session-test-runner contract. Until
  then a Makefile conflict that remains is confined to those two lines.

## Evidence

- `script/gen-spec-requirements_test.sh`: ok (case 13 fails on the pre-change generator).
- `script/spec-requirements-conflict_test.sh`: ok; the same-requirement mutation fails.
- `make spec-requirements-check`, `test-commands-check`, `unbounded-readers-check`,
  `requirement-definitions-check`: pass. The committed frozen file regenerates REQUIREMENTS.tsv,
  INDEX.json and README.md byte for byte.
- Go, via heavy.sh: `internal/specindex`, `internal/console`, `internal/lrfrepo`,
  `conformance/use-cases-v0`, `conformance/release-artifact-v0`, `tools/test-commands-check`,
  `tools/capabilities-check`, and the Makefile readers in `cmd/corvint` (AFP-V0-054 windows-vet,
  STR-V0-011): ok. `corvint affected` was UNKNOWN (the Makefile has no owning plugin), so the
  exhaustive gate is `NOT_RUN`.
- `make no-python-runtime-dependency-test` fails on the base too (`script/qualify-tasks-basic.sh:53`
  names `python3`); not this change.

## Merge with main 030efaed1

The freeze baseline is the header state at the commit where the freeze lands on main. PR #21
(ASD-V0-940..942) and the new `affected-plan-python-playwright-v0.md` changed headers on main after
85d5ad4b7. Those two rows were rebuilt from 030efaed1 with the same reader, which reproduces the
base rows exactly. No other spec header changed between those two commits. The new main targets
in the Makefile each got their own `.PHONY` line (SRG-V0-982).
