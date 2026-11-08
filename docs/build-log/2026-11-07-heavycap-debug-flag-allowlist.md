# Heavycap self-review repair: closed flag admission

Date: 2026-11-07. Repair starts at `3cbb6429c04686f11c5d3b1262f0e314206064fa`.
Affected and CEM base: `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`.
Owner intent: supplied P1 self-review finding, V1-5007 / ASD-V0-2478.

## Finding, decision and rollback

The class is unrecognized flag arity masquerading as a later selection. Both
scanners previously certified `-run=TestOther -debug-actiongraph -run=.` whole,
although Go consumes the second run-looking token as a filename. The same defect
applied to debug-runtime-trace, debug-trace and unknown future value-taking flags.
The pre-repair function extraction reproduces that success for all three debug
flags, a synthetic future flag and malformed triple-dash syntax.

Automatic admission now requires a closed syntax and arity allowlist shared by
both scanners. Value flags are exactly `go_test_value_flag`, with all three debug
flags added; boolean forms are exactly `go_test_known_flag`, including Go boolean
spellings, buildvcs=auto and v=test2json. Only Go's forwarded test flags accept the
test. alias. Unknown names, malformed syntax and unsupported aliases fail closed:
whole-source authority is refused and the classifier retains the wider claim.
GOFLAGS value flags require inline operands. Existing exclusion validation still
applies after syntax validation. The existing -args boundary and its alias and
terminator guards preserve DTL-V0-007's accepted narrow binary-argument commands.

Go 1.27.3's local cmd/go/internal/work/build.go registers the three debug flags as
strings and describes them as unstable. The bounded design choice is to understand
their operands but leave instrumented narrow commands outside automatic narrow
admission. The reported command therefore refuses with the existing status and
text; a whole-source run whose debug filename only resembles a narrow selection
executes the broken existing test. No wire, diagnostic, golden or old assertion
was edited. Revert this entry's scanner, regression and ASD-V0-2478 clarification
changes together to roll back; that reopens the reported soundness gap.

## Verification and retained limits

`corvint affected --base c3b29cbb54d31c8526d3cce4ad74356b6fb9b943` selected
tools/test-commands-check via DIRECT_TEST_CHANGE; scope UNKNOWN retains 69
LANGUAGE_FRONTIER unknowns. The installed binary reports a stale build and 30
changed Go source paths. Existing pre-change evidence is reused, not recaptured.
Other optional feature routes are inapplicable to this bounded scanner repair.

The first whole-package run failed the existing TestDTLV0007ListAliasAndArgsRefused
assertion for `-args -update` (60.902 s). Restore the established argument boundary;
do not weaken its assertion. The final whole tools/test-commands-check package
passes (50.583 s; wrapper 51.1 s; queue 0.0 s), through heavy.sh with no -run filter
and no commit identity exported. TestASDV02478SourceEditRunsExistingTests covers
all three debug operands, exact reported command refusals, unknown separate and
inline flags, malformed boolean/triple-dash syntax, unsupported test.tags aliases,
unknown inherited GOFLAGS and real execution of the broken TestExisting.

Pinned Go version go1.27.3, bash syntax and whitespace checks pass. Citation repin
finds no moves. `make -s spec-requirements-check line-citations-check` passes
(internal/specindex whole 1.001 s; internal/lrfrepo required selection 0.076 s),
using heavy.sh. The initial package failure remains retained as negative evidence.
Final CEM bind/check results and missing rows belong to the external handback.

V1-5007's local P1 is repaired; its previously missing external pre-commit entry
point still prevents full ticket completion. V1-5040, V1-4722 and V1-4944 are
unchanged. Independent review belongs to the orchestrator. Broad gates, Windows
qualification, Tasks writes, publication and seal are NOT_RUN by the charter.
External outcomes and absent telemetry remain NOT_OBSERVED. The runner's /tmp
2 GiB warning remains visible; required private TMPDIR and configured lock paths
are used without overrides.

## Evidence digests

Raw evidence is outside the repository in `/home/russell/lanes/heavycap-evidence/`.

| Artifact | SHA-256 |
| --- | --- |
| self-review-affected.json | 7063a4a4bccc018a0641d894ec9cd7b547319c9fc6bf83513fcdb466ee2c6163 |
| self-review-affected.stderr | 0ee4ade61bddb0b8c027b78fa8737c82aace500a40af0cd6dfa11e8abed5f83e |
| self-review-scanners.log | c732de9ee1dd475ad76c93f769b712873dcb4caad7bcc559ed7cdd212c1c2ede |
| self-review-package.log | c29976f3c058dc4e324c01b4ee25351fdea6a4a19b9f14d2637a9ac96fc977f6 |
| self-review-package-final.log | 0dde1afc11fe69757d00e3c7d56e85bf72deebfbe46567333d336a607d1eff82 |
| self-review-final-repin.log | 85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133 |
| self-review-spec-lines.log | f7563e38559edbcd16a32b55873a80d5d05f85fdcb12eb844ffbe41f1f682524 |
