# Heavycap self-review repair

Date: 2026-11-07. Base: `cb403735903b1e54a8469373489960c0b2b89d99`.
Repair input: P1 source-change admission finding at
`344e904ae5f855dfbcbf05727af22ba78d3c5140`.

Under V1-5007 / ASD-V0-2478, a source edit justifies running its package whole; it cannot
justify excluding the existing test that the edit breaks. The prior admission treated `-skip`
as a neutral argument and logged the resulting filtered run as whole-package. Self-review's
read-only replay found that the base refused the invocation while the candidate admitted it.

Automatic small-package, new-package and source-change admission now require an unfiltered
command. Guarding all three prevents an earlier shortcut from bypassing the source-change
repair. The check reads effective GOFLAGS with the command's GOENV/GOFLAGS assignments and
rejects inherited filters, test flag aliases, zero repetition and execution substitutes.
Existing narrow commands and explicit manual reasons retain their existing policy.

Decision: inherited filters conservatively require spelling the selection explicitly instead
of inferring combined flag precedence. The manifest path still validates a direct `-skip`
against its exact HEAD-bound slice. A whole-unit manifest cannot authorize an exclusion, and
cannot hide an inherited filter. This narrows automatic admission without adding a wire field,
error code, exit code, diagnostic, golden or core-freeze change.

`TestASDV02478SourceEditRunsExistingTests` retains every existing assertion and adds a broken
source edit combined with `-skip=TestExisting`, separated and aliased skip forms, environment
and argv GOFLAGS, multiline GOFLAGS, GOENV defaults, zero count, and both earlier shortcuts.
It also checks that an exact manifest exclusion of TestOther still reaches the broken
TestExisting. Whole-package results and raw log digests are retained in the lane handback;
raw logs remain outside the repository under `/home/russell/lanes/heavycap-evidence/`.

All three affected Go packages passed whole through heavy.sh, without a run filter or exported
commit identity: tools/test-commands-check (23.486 s), tools/selection-baseline (0.677 s), and
internal/linecov (19.210 s). The new exclusion cases passed in the first of those runs.
The required spec-requirements and line-citations checks passed; repin found no unchanged-content
citation moves. The affected plan retains scope UNKNOWN and 69 LANGUAGE_FRONTIER unknowns.

Raw log SHA-256:

- test-commands-check: `1868bf1e36cc69464c39fa00f4ff67656e994e13c4f4b334965c2754d306fc71`
- selection-baseline: `d682a89cfbd336bc8c93402cd3214715574d81e2788f7a9ac29390d9060e0b33`
- linecov: `b2e463bc77f9a17b9f9db1cc87524b485006b93fe8384f7f923ffccc1609494a`
- spec/citations: `a117d9ace2426369d4f6020ca45045f77e4ab785c94a2aefc5247427ccdedb47`

Rollback is a scoped revert of this guard, its assertions and the ASD-V0-2478 clarification;
it would restore the reported exclusion bypass. The original absent external pre-commit
integration remains PARTIAL. Independent review and broad verification belong to the
orchestrator. No new pre-change capture, Tasks write, publication or seal is part of this repair.
