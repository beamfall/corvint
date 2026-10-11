# V1-0660: remaining flow-maintenance mutation checks

Scope: test evidence for existing FDM-V0-003, FDM-V0-004 and FDM-V0-005 in
`docs/specs/flow-document-maintenance-v0.md`. No behavior or requirement changes; no new owner
acceptance is claimed. The receipt-integrity condition lives in
`internal/appflows/docs_maintain.go`, so its fixture belongs in that package in addition to the
requested `internal/doccorpus/apply_pair_test.go`.

Baseline: `686fb1ac9c067046c1b7b3dc6776ac41e5c61ee6`, initially clean. Environment:
`GOTOOLCHAIN=local`, Go `go1.27.1`, `TMPDIR=/private/tmp/claude-501/gotmp`, `GOMAXPROCS=3`.
The builder retained its session model; independent review used `gpt-6.1-sol`/medium for bounded
fixture/oracle review. Account usage, billed tokens and cost are `NOT_OBSERVED` (no available
account-usage tool); this record makes no efficiency claim.

## Fixtures and preservation

- `TestMaintenancePairPostPublishParentRecheck/{missing,replaced}` moves the first parent during
  the second file's `before-publish` checkpoint, after both pre-capture checks. Both outputs must
  report published before the final parent error. The replacement contains identical output bytes,
  isolating parent identity from output integrity. Assertions account for the renamed directory,
  preserve both originals through recovery paths, retain published and competing output bytes,
  and require stage cleanup through the pinned parents.
- `TestFlowDocMaintenanceReceiptIntegrity` proves a generated, canonical, committed receipt works
  before altering it. Each field case is resealed, so its invalidity is not masked by an unrelated
  digest mismatch. Malformed digest and noncanonical-byte cases preserve their separate purposes.
  The final fixture decodes and is committed before preview/apply reach the named integrity error.
  Both permitted provenance values remain positive controls; complete filesystem snapshots detect
  publication or hidden writes on refusal and on no-op apply.
- Production code and all existing tests remain unchanged in the delivered diff, preserving the
  fifteen previously protected checks, bounded local publication and recovery behavior. Rollback
  is removal of the new test/evidence additions; there is no product migration or runtime rollback.

## Isolated mutation results

Each trial changes only the named condition, executes its named test with `go test -count=1
-timeout 30m`, and restores the original source in a `finally` block. A disjunct is neutralized as
`false && (condition)`; each inner provenance conjunct is neutralized as `true || condition`.
No mutation is retained. A build failure or fixture failure is not counted as a kill.

The restored-code control run passed both new test functions (`doccorpus` 0.356s, `appflows`
101.714s). Final trial results: **16 mutations, 13 killed, 3 redundant survivors**. In the table,
`Parent/CASE` names `TestMaintenancePairPostPublishParentRecheck/CASE` in `internal/doccorpus`;
`Receipt/CASE` names `TestFlowDocMaintenanceReceiptIntegrity/CASE` in `internal/appflows`.

| Only neutralized guard / component | Named test case(s) | Result |
|---|---|---|
| Final parent-open `e != nil` | Parent/missing | KILLED: nil-parent panic replaces the required refusal |
| Final parent stat/identity `e != nil || !os.SameFile(info, s.parentInfo)` | Parent/replaced | KILLED: erroneous success |
| `canonicalErr != nil` | Receipt/digest-format, Receipt/canonical-bytes | SURVIVED; redundant, argument below |
| `digestErr != nil` | Receipt/digest-format, Receipt/canonical-bytes | SURVIVED; redundant, argument below |
| `r.Schema != DocMaintenanceSchema` | Receipt/schema | KILLED: invalid receipt accepted |
| `r.Profile != docMaintenanceProfile` | Receipt/profile | KILLED: invalid receipt accepted |
| `!sha256Pattern.MatchString(digest)` | Receipt/digest-format | SURVIVED; redundant, argument below |
| `digest != expectedDigest` | Receipt/digest-mismatch | KILLED: invalid receipt accepted |
| `!bytes.Equal(raw, canonical)` | Receipt/canonical-bytes | KILLED: noncanonical receipt accepted |
| `!oidPattern.MatchString(r.Revision)` | Receipt/revision | KILLED: invalid receipt accepted |
| `!oidPattern.MatchString(r.Tree)` | Receipt/tree | KILLED: invalid receipt accepted |
| `!sha256Pattern.MatchString(r.Proposal)` | Receipt/proposal | KILLED: invalid receipt accepted |
| Ownership-string inequality | Receipt/ownership | KILLED: invalid receipt accepted |
| Entire historical-provenance disjunct | Receipt/historical-provenance | KILLED: invalid receipt accepted |
| Historical-provenance `!= "RECORDED"` conjunct | Receipt/recorded | KILLED: valid provenance refused |
| Historical-provenance `!= "UNKNOWN"` conjunct | Receipt/unknown | KILLED: valid provenance refused |

All kills exited 1 with the named subtest failing; survivors exited 0. No build failures count.
The error-clause trials do not pretend to isolate an encoding error from the other rejecting
clauses: their refutation rests on the return-value implications below. After the trials, both
production files were byte-compared with `git show HEAD:PATH` and matched.

The three expected survivors are redundant as independent rejection gates, not missing behavior:

1. `canonicalErr != nil`: `doccorpus.Encode` returns nil bytes on error; the successfully decoded
   receipt is nonempty, so `!bytes.Equal(raw, canonical)` already rejects it.
2. `digestErr != nil`: `maintenanceDigest` returns an empty expected digest on error. Any nonempty
   supplied digest mismatches it; an empty supplied digest fails SHA-256 syntax. There is no input
   for which only this disjunct rejects.
3. `!sha256Pattern.MatchString(digest)`: when digest calculation succeeds, equality with its
   lowercase 64-hex SHA-256 already implies valid syntax. On failure, `digestErr` rejects. An
   independently killing fixture for just this syntax clause would require neutralizing another
   guard, which this task explicitly forbids.

These guards remain in production. Their retained redundancy does not authorize removing or
weakening any check. This is bounded mutation evidence, not exhaustive interleaving or integrity
proof.

## Verification and independent review

- Restored-source baseline of both new test functions: PASS.
- Requested documentation gates: PASS after the temporary-directory correction below; six
  pre-existing planned traceability tests remain reported, all eighteen use-case receipts current,
  and all five receipt-script regression cases passed.
- `sh script/repin-use-case-receipts.sh --check`: PASS.
- `make -s spec-requirements` after staging the spec: PASS; generated TSV unchanged.
- Final restored-source `go test -count=1 -timeout 30m ./internal/doccorpus ./internal/appflows
  -run 'TestMaintenancePair|TestFlowDocMaintenance'`: PASS. This includes the existing maintenance
  tests and the new fixtures (`doccorpus` 0.461s, `appflows` 61.343s).
- Final staged documentation recheck and receipt repin check: PASS. The exact gate command was
  `make -s spec-requirements-check requirement-definitions-check traceability-tests-check
  decision-numbers-check line-citations-check error-code-ownership-check unbounded-readers-check
  use-case-receipts-check use-case-receipts-test`, followed by
  `sh script/repin-use-case-receipts.sh --check`. This result annotation is the only subsequent
  evidence-text change; it does not alter requirements, test references or production inputs.
- An additional unfiltered package baseline was interrupted before producing a package result;
  it is INCOMPLETE, not a pass. The execution-session interrupt retired its Go driver and
  `doccorpus.test` child (verified absent afterward). No broader package-coverage claim is made.

The first receipt baseline exposed a fixture error: the `RECORDED` positive control already
matched the generated receipt, so an unconditional `commitAll` failed on an empty diff. The fixture
now commits only changed canonical bytes. Any trial compiled with that failed fixture is excluded
from mutation conclusions and rerun after repair.

Independent read-only review found no correctness issues in the parent injection, resealed receipt
fixtures, side-effect assertions or unchanged spec intent. A follow-up review accepted the fixture
repair and the three redundancy arguments, and confirmed all twelve top-level receipt clauses
were accounted for. The reviewer explicitly acknowledged missing the empty-commit issue on its
initial static pass. Final independent evidence review checked all sixteen logs against the JSON and this table,
confirmed all outcomes, and reported no findings.

## Corvint evidence and limits

Installed Core: `1.0.0-rc.3 (build 407)`; Tasks: `0.0.0-tcp01-unverified+build.407`.
Both official `corvint-update check --allow-network --component core|tasks` checks refused during
version smoke with `process-after-start-failed: descendant snapshot unavailable`. Available release
versions and freshness remain `UNKNOWN`; no install or verification bypass occurred.

The authority query succeeded, retaining four omitted ranked results, eighteen withheld test-symbol
candidates and source exclusions. Path impact retained mixed-worktree limitations. The empty-diff
`affected` result selected no tests; after additions it selected the owning packages among a wider
static dependency frontier, retaining 22 unknowns. The requested focused checks govern this task;
`make gate` and repository-wide `./...` commands are `NOT_RUN` by explicit instruction.

The requested snapshot refresh failed writing the shared main-checkout Git index snapshot outside
this sandbox. Initial `make dogfood-change BASE=686fb1ac9c067046c1b7b3dc6776ac41e5c61ee6` likewise
failed creating private Git evidence there. Post-commit CEM binding/sealing and native completion
are `NOT_PRODUCED`: commits and ticket-store writes are explicitly excluded from this task.
No decision, plugin metadata, ticket-store write, commit or push is part of this change.

The first documentation-gate attempt could not allocate the system `mktemp` default outside the
sandbox. A temporary PATH wrapper redirects only implicit allocations to the requested writable
TMPDIR and preserves explicit templates; repository scripts are unchanged. An initial overly broad
wrapper broke explicit absolute templates and was corrected before the final gate run. Operational
filing is deferred because the task explicitly prohibits ticket-store writes.
