# 2026-10-22 cmdmisc-1022d: source-view deadline, corpus --repository, trailing Status notes, delivery-screen verdicts, gate-verdict help

Lane `cmdmisc-1022d`, started on `f29ddc0db09c01c370cd3c9bcc5022a296ae3876`. Tickets V1-2320,
V1-2345, V1-2309, V1-2303 and V1-2324. ESV-V0-014, DCP-V1-312, SRG-V0-044, TCD-V0-167..168 and
the TCB-V0-045, SRG-V0-042, TCD-V0-160 and TCD-V0-161 amendments are accepted under the owner's
standing auto-accept (2026-10-11). Every new test was run against the code without its fix and
failed there.

## V1-2320: source-view deadline expiry names itself (ESV-V0-014)

Problem. A Git read cut short by the source view's 10-second bound or by caller cancellation
surfaced as `repository-root`, `stale-commit`, `missing-path` or `git-failed`, so an agent could
not tell a slow host from a broken pin.

Decisions. `readPinnedSourceView` (cmd/corvint/source_handoff.go) is the single read path; when
its context is done the refusal is `deadline-exceeded` whatever Git step was interrupted. The
native source digest frozen in `benchmarks/selfuse-batch/source-views-manifest.json` is re-frozen
for the changed file.

Evidence. `TestSourceViewDeadlineExpiryRefusesAsDeadlineExceeded` (14 cases: expiry and
cancellation at each Git step) fails on the base code.

## V1-2345: corpus CLI `--repository` parity (DCP-V1-312)

Problem. The corpus MCP tools accept the DCP-V1-310 repository qualifier; `corvint docs corpus
locate` and `recommend-tests` did not.

Decisions. Both commands take `--repository ID` with the MCP receipts and refusals; help and the
option matrix name it.

Evidence. `TestDCPV1312CorpusCLIRepositoryQualifierParity` compares CLI and MCP outcomes for
the declared, an undeclared and an empty repository ID; the matrix and help tests are updated.

## V1-2309: trailing `Status:` acceptance notes are refused (SRG-V0-044)

Problem. 31 TQD-V0 definitions carried their acceptance as a trailing `Status: (accepted ...)`
note, which the generator never reads (SRG-V0-009 reads only the leading parenthetical), so they
classified `unrecorded` silently.

Decisions. `acceptance_near_miss` refuses a definition line with a trailing `Status: (accepted`
or `(approved` note, naming why. The 31 TQD-V0 notes are moved to the leading position without
changing their text: 12 owner and 19 standing rows, none unrecorded.

Evidence. `script/gen-spec-requirements_test.sh` case 11 gains NEAR-007 and fails on the base
generator; `spec-requirements-check` passes over the repository.

## V1-2303: delivery-screen CITED and TRUNCATED verdicts (TCD-V0-167..168)

Problem. `corvint-tasks ticket delivery-screen` reported a ticket ID mentioned anywhere in a
commit message as delivered evidence, and a ticket older than a truncated scan window as `CLEAR`.

Decisions. A commit line delivers only when it is not vetoed and is a lead/`DELIVERED` line, or is
the subject and names the ticket as its first word, its conventional-commit scope or its final
parenthetical. Any other mention is `CITED`. With a truncated scan, a ticket created before the
oldest scanned commit (`store.CommitTime`, whole-second precision) is `TRUNCATED`. Verdict order:
IN_FLIGHT, DELIVERED, TRUNCATED, CITED, NOT_OBSERVED, MENTIONED, CLEAR. TCD-V0-165/166 were taken
by open PRs #1517 and #1525, so the new IDs are 167..168.

Evidence. `TestTCDV0167_DeliveryScreenCitedAndTruncated` (prose mention, `Refs:` trailer, leading
ID, `feat(ID):`, final parenthetical, `--limit 1`, backdated commit) fails on the base code; the
TCD-V0-159 test now expects `CITED` for a vetoed line.

## V1-2324: gate-verdict help names verifier-build-mismatch (TCB-V0-045 amendment)

Problem. `corvint help cem gate-verdict` named only `verifier-disagreement`.

Decisions. The help names `verifier-build-mismatch` (TCB-V0-045) and the consumer rebuild
command `(cd interop/cem01-go && GOTOOLCHAIN=local go build -o "$CONSUMER" .)`.

Evidence. `TestTCBV0045GateVerdictHelpNamesBuildMismatchRecovery` fails without the help text.

## Not delivered

- V1-2310 (README ranges) was built and withdrawn: generating a range in every row whose spec
  defines ids made `script/spec-requirements-conflict_test.sh` fail its SRG-V0-016 case, because
  two lanes adding requirements to specs whose README rows are adjacent then conflict in
  README.md. That trade against SRG-V0-016 needs a decision; the 57 rows that already carry a
  range have the same exposure, untested.
- V1-1302: AC1/AC2 delivered by #914 (`liftedSourceDeadline`); AC3 needs a full loaded `-p 1`
  `cmd/corvint` run, longer than this lane's 600-second command bound.
- V1-1003, V1-2350, V1-2347: design, a recreated 8,001-input fixture or a second host needed.
- V1-2219, V1-2348, V1-2036: in flight in open PRs. V1-2353, V1-1895, V1-2107, V1-2249,
  V1-2341, V1-2346: dependency, owner decision, sealed-schema collision or network.

## Tests run

Focused tests per ticket (each proven to fail without its fix); full `internal/tasks/cli` (500 s,
includes the CAL-V0-131 trap), `internal/tasks/service`, `ticket`, `transaction`, `mutation`,
`scale`, `benchmarks`, `benchmarks/selfuse-batch`, `TestImportDirection`; `cmd/corvint` with
`-run 'SourceView|SourceHandoff|Corpus|DCPV1|Help|GateVerdict|TCB|ESV'`; `internal/tasks/store` in
eight `-run` shards (all 552 tests pass), because one serial run exceeded the 30-minute hang
detector under host load and the J-prefixed journal tests alone took over 9 minutes.

## NOT_RUN

- `cmd/corvint` outside the `-run` subset; `corvint affected` tier 2 (94 units) for batch CI.
- `make gate` (not required for scoped lane work).

## Residuals

- 26 trailing `Status: (accepted` notes sit on wrapped continuation lines (12 in
  corvint-tasks-obligation-ledger-v0.md, 13 in documentation-corpus-v1.md, 1 in
  live-proof-carrying-verification-v0.md) and are not diagnosed, because the reader keeps only
  the definition line.
- README rows with a generated range conflict when adjacent specs change in two lanes.
- `internal/tasks/store` runs longer than 8 minutes serially (JSGV0013_UnsealStopsSettle 89 s,
  JSGV0012_CompactionOnASealedStore 52 s) and hit the 30-minute detector under load.

## Rollback

Revert the lane's commits; each ticket is one commit. The source-view manifest digest returns with
the source_handoff.go revert.
