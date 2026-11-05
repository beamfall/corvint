<!-- SPDX-License-Identifier: Apache-2.0 -->
# mixbugs V1-2651 revert round

The owner requires reverting V1-2651 after the fix-only review found a P1 in
Navigate fragment handling. `to="/#/target"` denotes pathname `/` with fragment
`#/target`; interpreting it as pathname `/target` misattributes tests and can
omit them when only the root screen's router file changes.

## Decision and requirement disposition

V1-2651 is NOT MET. Restore every appmap change to lane base
`d436dff76de1b8e75c427069ae7f80003ae99bc2`, including the original
`TestAMAPV0194NavigateRedirect` assertion that redirects do not match URLs.
Withdraw AMAP-V0-8206 and its spec text; AMAP-V0-194 governs again. Regenerate
the requirement register and README. The two earlier mixed-ticket build-log
entries remain historical evidence; their V1-2651 delivery claims are superseded
by this disposition. No build-log file was solely for V1-2651.

Follow-up needed: preserve incoming URL prefixes without reinterpreting Navigate
fragments, plus a root-screen selection regression. No replacement redirect
implementation or changed frozen output is authorized in this round. Every other
ticket's implementation, tests, contracts and goldens remain unchanged.

## Evidence and limits

Reuse the existing pre-change context and query/impact receipts retained in
`/home/russell/lanes/mixbugs.tmp/fix-round-1/`; do not recapture pre-change evidence.
Current command logs and return codes are retained outside the repository in
`/home/russell/lanes/mixbugs-evidence/revert-round/`. The handback at
`/home/russell/lanes/mixbugs.handback.md` records the final revision, validation
results, explicit omissions and numbered friction.

The required validation is the existing whole appmap package and focused spec/doc
gates, followed by CEM rebinding, report inspection and dogfood-check. Independent
review remains orchestrator-owned. No model-backed reviewer, Tasks write, push,
PR, full gate or seal is authorized. Self-use proves structural binding only;
semantic adequacy, external interoperability and downstream outcomes remain
NOT_OBSERVED unless separately evidenced.
