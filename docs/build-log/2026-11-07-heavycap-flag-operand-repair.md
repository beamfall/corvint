# Heavycap self-review repair: flag operands and final run selection

Date: 2026-11-07. Repair starts at `e0f5c9e55302a156fdda1c52b5fbf784134f0770`.
Affected and CEM base: `c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`.
Owner intent: supplied self-review P1/P2, V1-5007 / ASD-V0-2478.

## Findings and decision

P1: `source_tests_unfiltered` treated the operand of `-tags` as a separate flag.
For `-run=TestOther -tags -run=.`, Go consumes `-run=.` as the tag list and
executes only TestOther, while the guard granted whole-source authority.
P2: `run_selects_all` returned at the first broad pattern. A subsequent narrow
`-run` therefore received whole-package classification and a missing-reason refusal,
contradicting ASD-V0-2478's final-value rule.

Both scanners now consume separate operands of Go's value-taking flags before
interpreting selections. A shared flag-name predicate keeps them aligned; boolean
flags consume no operand. Classification uses the last effective `-run`; without
one, selection remains whole. Existing whole-pattern definitions, inherited-filter
restrictions, manifest authority and diagnostics stay unchanged. No wire shape,
golden, existing assertion or frozen output changes.

Extend `TestASDV02478SourceEditRunsExistingTests` to check the guard independently
and execute real Go with the reported operand and reverse-order cases. The selected
test must pass, TestExisting must not execute, and the log must record selected
rather than whole. An operand-only `-run=TestOther` must still execute the whole
package and expose TestExisting's source regression. Separate-word repeated flags
and a boolean build flag also exercise the same rules.

Rollback: revert this repair's two scanners, helper, new regression cases and spec
evidence-table update together. V1-5007 remains PARTIAL only for the previously
recorded absent external pre-commit entry point; this repair closes both supplied
local findings. V1-5040, V1-4722 and V1-4944 receive no implementation changes.

## Verification and limits

Used: `corvint affected --base c3b29cbb54d31c8526d3cce4ad74356b6fb9b943`
selects tools/test-commands-check with DIRECT_TEST_CHANGE. The receipt retains 69
LANGUAGE_FRONTIER unknowns and the installed binary's stale warning. The original
pre-change evidence is reused as instructed; no new pre-change capture is claimed.
Other optional feature routes are not applicable to this bounded shell repair.

The negative control fails against the pre-repair script after 21.261 s: the guard
incorrectly grants whole-source authority to the reported `-tags` operand case.
After repair, the whole tools/test-commands-check package passes in 50.704 s
(wrapper 51.3 s, queue 0.0 s), including every existing test without a run filter.
All Go tests run through heavy.sh, one package per command, with no commit identity
exported. The pinned toolchain reports go1.27.3. Shell syntax and staged whitespace
checks pass. Repin finds no moved citations. The required
`make -s spec-requirements-check line-citations-check` passes, including its whole
internal/specindex and selected internal/lrfrepo tests, through heavy.sh.

No broad repository gate, selection-baseline-check, corvint test, Windows vet,
publication, Tasks write or seal runs under this repair charter. Independent review
belongs to the orchestrator. External interoperability, platform qualification and
unavailable telemetry remain NOT_OBSERVED. Final binding/check results and all
NOT_PRODUCED rows are retained in the external handback and dogfood report.

Friction: the configured runner warns that /tmp has 2 GiB free below its 3 GiB floor,
then proceeds under existing policy. TMPDIR is the required private lane directory;
no lock-directory override or policy change is used. Affected still reports the
installed binary was built at cb403735903b, with 30 Go source paths differing.

## Raw evidence digests

Artifacts are outside the repository under `/home/russell/lanes/heavycap-evidence/`.

| Artifact | SHA-256 |
| --- | --- |
| flag-repair-affected.json | a434743e9c7bb3f381b72783d381a5a1dbbfabc80fd396c5eb9b501520ec4222 |
| flag-repair-affected.stderr | 0ee4ade61bddb0b8c027b78fa8737c82aace500a40af0cd6dfa11e8abed5f83e |
| flag-repair-red.log | 90d611c22fc5747b56e4bd0666e204bc9b70ca5f88e8ec56225d09ce0c602a5b |
| flag-repair-package.log | ab5c38d519ac68ccf0de8e39433e85376a05ed71da613f45d74d3ac4f0285cba |
| flag-repair-repin.log | 85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133 |
| flag-repair-spec-lines.log | fb091f3fdcbb6008dea029feebf67d7f716d254919f560d9983228dda2d48fd1 |
