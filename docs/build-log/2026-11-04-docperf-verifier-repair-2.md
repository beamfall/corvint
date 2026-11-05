# Docperf verifier repair 2: deterministic activation fixtures

## Finding and decision

The host verifier at `316e5d8f1efc2cd85c9b461b31f1302ddadaf08a` failed the
init summary, adopt summary and init full receipt cases of
`TestCoreVerbsEmitTheFrozenProfiles`. The fixture revision was
`abf7cbec4e070de55f4487bd1f613fd66d3ebabd`, while the unchanged goldens
require `253a99a4567d96385daba31c2c1fd37acc622dbe`. Its receipt digests
changed with that revision. The lane's required exported Git identity overrides
the fixture's repository configuration, just as in the earlier context fixture
repair. No product behavior caused this difference.

Decision: in `newActivationFixture` in `cmd/corvint/init_adopt_test.go`, pin
both subprocess identities to the original `Corvint Test
<corvint@example.test>`, alongside the existing dates. This preserves
CCF-V1-002's existing frozen output and leaves the lane author/committer
identity intact. No assertion, golden, fixture file, wire shape, field, error,
exit code or stderr contract changes. No new requirement or spec amendment is
needed. Rollback removes this identity pin and restores ambient dependence.

The verifier also failed `TestDCRV1846GitFallbackWarmCallsWithinBudget`:
median freshness 107 ms against the unchanged strict bound of 100 ms; warm
total p95 898 ms against 1000 ms. The host load was 61 after admission.
`internal/mcp/corpusbridge/corpusbridge.go`, including its Git fallback
observation orchestration, is byte-identical to BASE (SHA-256
`9ae2553559f27603b3c2edb642906db7dfba0e064e7d32d1ad575dad39496a5c`).
The failing freshness interval excludes the changed checkout stamp capture.
DCR-V1-846's assertion and all fallback code remain unchanged pending the
whole-package rerun. A passing rerun is a new local observation, not a
controlled-host qualification or evidence that the original failure passed.

## Evidence routes and limits

Start `make dogfood-change` ran with the required author/committer identity,
retained context, and returned exit 2 for missing citation/intent/outcome
inputs. Final binding supplies these inputs. Guarding tests remain
NOT_PRODUCED (`guarding-tests-not-run`); prechange query/impact binding remains
NOT_OBSERVED (`agent-receipt-binding-mismatch`). The context route located
CCF-V1-002 and DCR-V1-846; their original clauses and existing tests were read.
The installed affected plan was retained with its stale-binary warning and
repeated with the current-tree dogfood binary before tests. It selects both
verifier packages and retains the two LANGUAGE_FRONTIER unknowns for
build-constraint variants and unmodelled shell invocation.

Raw evidence is outside the repository under
`/home/russell/lanes/docperf-evidence/verifier-repair-2/`.
Independent review belongs to the orchestrator; no model-backed reviewer runs
in the lane. Retrieval/learning evaluations and provider qualification do not
apply to this fixture repair. The original performance ticket qualifications
and native Tasks completion remain open. No push, PR, Tasks write, background
job or seal is authorized in this lane.

## Verification

With the four required Russell identity variables exported,
`script/heavy.sh go test -count=1 -run '^TestCoreVerbsEmitTheFrozenProfiles$'
./cmd/corvint/` passes: 2.073 s package time, wrapper rc 0. All existing
frozen assertions and committed goldens are unchanged.

`script/heavy.sh go test -v -count=1 -timeout 30m
./internal/mcp/corpusbridge/` passes the existing whole package: 45.682 s,
wrapper rc 0. The unchanged DCR-V1-846 test measured median freshness 28 ms
and warm total p95 96 ms; all 20 warm freshness samples were 24..38 ms and
all total samples 47..96 ms. Its incoming failed run is retained separately.
No fallback code, timing threshold or test assertion was changed. This run is
local Linux evidence, not quiet-host or Windows/Darwin qualification.

The required whole CLI rerun uses the mandated 0700 TMPDIR beneath
`/home/russell/lanes`, whose mode remains 0775 and which has an enclosing
`.git`. Prior round whole-package failures in those scratch conditions are
retained, with no change to the shared parent permissions, Git metadata or
product security checks. The exact current whole-CLI result, final CEM bind
and check are retained in `/home/russell/lanes/docperf.handback.md` and the
external evidence directory. A scoped pass does not imply a whole-package
pass or resolve those environmental limits. Local outcome remains `blocked`
while the original performance qualifications are open.

| Evidence (relative to external repair directory) | SHA-256 |
|---|---|
| `frozen.log` | `6f2ec9a1e60e5ca260782ccdab85687359a32896509a3c6a21af5058a2f38902` |
| `affected-current.json` | `7a904f56d52d83afea5693fef2e2f72227b18b67b3770a51fc99fa0bca6b5a1a` |
| `context.json` | `536a66821778340a4a5441d622bcdef1ad41f3e66649c2704cb69b9e1a470cc8` |
