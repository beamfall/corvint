# pk-ci review corrections

Owner scope: fix the seven findings from the independent round-0 review, on top of
`4c395fbdf21c69f0f8764bfe88dbbc32568a1f08`, retaining base
`6c33df33bc5670131482f5263b410cb980580657`. The orchestrator owns the next independent
review, publication, hosted qualification and native ticket writes. This entry supersedes the
round-0 publication and selector-authority claims where stated below.

The conservative selector boundary (V1-4436, CIN-V0-7504) now requires web-flows for FULL
plans, Incomplete dry-runs (ResultIncomplete, exit 3), and explicit still-required binary
consumers. Other nonzero statuses fail planning. The workflow deliberately diffs the raw event
base: a stale base over-selects. The selector and confine binaries come from the trusted
checkout; consumer/read-scope declarations remain tested-tree data reviewed as code. No trusted
declaration profile was added. `TestCINV07504WebFlowsConsumerJob` executes the actual workflow
status handling and jq expression for reached/unreached consumers, FULL with empty checks,
Incomplete with empty checks, and nonzero failures 1, 2 and 4. The real CLI parser regression
retains its flag check with the new status-capture suffix. Hosted PR/MQ run and skip evidence
remain NOT_OBSERVED; V1-4436 is locally implemented, not fully qualified.

The same-spec merge contract (V1-4550) needed correction rather than fixture narrowing alone.
SRG-V0-013 now allows the generated README range-cell conflict when both lanes append a new
highest ID, consistent with SRG-V0-7500. Keep the fixed-endpoint isolation and other existing
fixtures. The new highest-ID fixture requires exactly the spec, REQUIREMENTS.tsv and README.md
as conflicted paths, resolves only the spec by hand, and proves `spec-requirements-resolve`
regenerates all three generated outputs, retains both IDs and leaves a clean committed tree.
Copied fixture scripts are tracked so the clean-tree assertion includes all fixture inputs.
The full conflict shell suite passes. The next hosted nightly remains NOT_RUN; V1-4550's local
contract and regression are implemented, with hosted acceptance still pending.

V1-4573 is SUPERSEDED-BY v1fix-a (DWC-V0-929). Remove this lane's duplicate report publisher,
its tests, temporary-file ignore and DWC-V0-7501. `internal/dogfoodflow/check.go` and `.gitignore`
are byte-identical to BASE. The full internal/dogfoodflow package passes after removal. An EXDEV
failure at dogfood-report-write remains an external integration blocker until v1fix-a lands;
this lane must not duplicate that repair.

CSD-V0-7517 (V1-4553) now specifies at least two seconds for each TERM/KILL grace, including
probe cost. The `SECONDS + 3` deadline compensates for whole-second truncation. The existing
slow-ps, TERM-ignoring runner fixture passes with bounded join and descendant cleanup intact.

Focused execution evidence is in `/home/russell/lanes/pk-ci-fix-evidence/`: cmd/corvint's
TestCINV0750 prefix, tools/ci-forward-row's TestCINV0750 prefix, the full internal/dogfoodflow
package, conflict shell suite and companion shell suite. All Go tests use script/heavy.sh,
one package per command, after pinned-base affected selection. Both shell suites use isolated
HOME and GIT_CONFIG_NOSYSTEM=1. The initial CLI-parser regex and untracked fixture-script
failures are retained alongside their passing repairs. The old affected binary emitted
self-binary-stale; the lane-built selector also selected the requested units. Its UNKNOWN
frontier is not exhaustive coverage.

The requested `/var/tmp/pk-ci-fix` cannot be created in this sandbox (read-only filesystem).
Scratch is `/home/russell/.cache/pk-ci-fix/tmp`, verified outside a Git repository, with shared
locks at `/home/russell/lanes/.heavy-locks`; /tmp is not used for these tests. A new explicit
prechange receipt refused `prechange-after-change` because this is a repair of an already
committed range. The required start dogfood-change ran and retained its missing-input reasons;
round-0 immutable prechange evidence remains visible. Native guarding tests, independent review,
exhaustive gates, native Mac execution, hosted CI and ticket mutation are not claimed here.
Final generation, static checks, binding and check outcomes belong in the external handback.
Rollback reverts this repair commit; it does not restore authority to tested-tree declarations
or promote unobserved hosted outcomes.
