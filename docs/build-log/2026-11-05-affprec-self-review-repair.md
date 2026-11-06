# affprec self-review repair: explicit requirement prefixes

Date: 2026-11-05
Owner direction: repair only the supplied P2 index/header mismatch.

## Finding and decision

The Affected Reader Precision V0 spec declares `AFP-V0` and defines
AFP-V0-9000 through AFP-V0-9005, but its INDEX.json entry retained `ARP-V0`.
Correct that entry to `AFP-V0`. Extend the existing root-based spec consistency
check with an assertion comparing reqPrefix against an explicit Requirement
prefix header, using the same backtick normalization as the generator. Keep
specs without an explicit prefix eligible for their existing inferred metadata.
Fixture cases cover plain, backticked and bold matching headers, mismatches,
and absence of an explicit header. This supports SRG-V0-045's contract check;
no new capability, requirement or wire shape is introduced. Rollback removes
only this added assertion and its fixtures; the corrected metadata remains
consistent with the governing spec.

## Evidence and limits

Reuse the existing pre-change capture; no pre-change producer ran. The affected
route selects internal/specindex directly through index_test.go. The installed
observer's self-binary-stale warning remains visible in the retained stderr.
The new assertion was run against the unchanged bad entry: the full package
failed only TestIndexCoversSpecsAndHeaders, reporting ARP-V0 versus AFP-V0.
The requirement generator's existing --check passed despite the mismatch;
it retains an existing INDEX entry's prefix, so it cannot replace this assertion.
Raw logs stay in /home/russell/lanes/affprec-evidence/self-review/.

After correcting the entry, the complete internal/specindex package passes
(1.392 test seconds; heavy.sh queue 130.0 seconds / run 1.9 seconds), without a
-run filter. All Go tests use script/heavy.sh and the existing shared semaphore.
The initial reproduction queued 175.7 seconds and failed in 2.625 test seconds.
The first invocation was refused for missing HEAVY_WHOLE_REASON before running
tests; the corrected invocation supplies the affected-selection reason.

script/check-line-citations.sh --repin found no moved citations. The staged
make line-citations-check, make spec-readme-check, generator --check and
spec-requirements-worktree-check all pass. The complete spec-requirements-check
target was not invoked: its additional lrfrepo enumeration run is outside this
metadata/assertion repair; no requirements changed. CEM binding/check results
and artifact digests are retained in the handback. The local outcome continues
to retain the earlier failed whole-command-package qualification; this passing
scoped repair does not erase it.

Independent review belongs to orchestration. Earlier whole cmd/corvint fixture
environment failures remain retained in the prior repair entry and handback;
this scoped metadata repair does not qualify that package. No golden, existing
assertion, command behavior or frozen output changed. No Tasks writes,
publication, seal, broad gate or new performance claim is part of this repair.
