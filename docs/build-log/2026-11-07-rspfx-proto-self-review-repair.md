# Runner protocol self-review repair

Requirements: RSP-V0-027, RSP-V0-028, RSP-V0-029.
Repair parent: c82b9cd1bd4c81d959c70ccbb2c4881cac9f1418.
Lane evidence base: 9761ea1f459c0d3b3c963953e24a76eaeda16370.

## Findings and decisions

The mutation gap is a metadata-location class: a linked checkout and its common
repository have distinct Git directories. Reuse gitstatus's bounded resolver to
return both; hash common/config and common/hooks, and checkout/config.worktree.
Do not introduce a second pointer parser or guess that metadata is always common.
The closed location grammar accepts a directory .git marker or a regular bounded
file containing the literal `gitdir: ` prefix and a nonempty path, optionally
followed by a bounded commondir path. Existing symlink-marker, malformed-pointer,
NUL, embedded-newline and unreadable-input refusals remain unchanged. Unknown
marker syntax cannot certify a snapshot and Close discards staged evidence.

TestRSPV0029GitMetadataMutation now covers existing config.worktree content in
both ordinary and linked checkouts with extensions.worktreeConfig enabled, and
asserts evidence discard. TestRSPV0029UnknownGitPointerFailsClosed covers an
unrecognized pointer form, resolver/snapshot refusal and session evidence discard.
This is a closed pointer grammar, not an expanding list of forbidden forms.

NUL environment values already fail Schema.validate with schema-violation:string.
TestRSPV0027NulEnvironmentRefused now asserts that existing refusal at Command and
Close, retaining the evidence-discard assertion; production errors are unchanged.

The RSP-V0-028 discovery example now claims reference-go-test.
TestRSPV0028DocumentedDiscovery extracts that exact fenced example, decodes it
with DecodeDiscovery and checks the reference claim. No wire shapes, goldens,
core-freeze output, or production error/exit/stderr contracts change.

## Verification and limits

The mandated affected plan selects both touched Go packages. Its installed binary
reports self-binary-stale, and the aggregate plan remains UNKNOWN (135 unknowns).
The whole internal/gitstatus package passed (2.748 s). The whole runnerprotocol
package was attempted after final test edits but fails before protocol sessions:
plugin-store-inside-repository, because sandbox /home/russell/lanes/.git makes
fixtures under the required TMPDIR repository-owned. No fixture assertion or
containment check was weakened. Runtime qualification of the session regressions
is blocked pending the host verifier. The documentation test reported no failure.
Spec requirements and line citations passed; staged repin found no moved citations.
Independent review, broad gates and frozen evaluations are orchestrator-owned and
NOT_RUN here. The explicit repair instruction reuses existing pre-change capture.
Binding/check outcome is retained in the external lane handback; no seal is allowed.

Raw evidence lives outside the repository in /home/russell/lanes/rspfx-proto-evidence/.
SHA-256 digests:

- self-review-affected.json: `cd36716b1f1b3abab146e5ecb309aa692c565cb790d62d6ad82cf6d3b9a7a035`
- self-review-gitstatus.log: `71d7a52aabad6088ce2c55d515bb636cbe073a421796f019acd9f2cf7b556f36`
- self-review-runnerprotocol-final.log: `e5583a94d7551dfebeaff38e7d772a9158a7929709234efde707a951deb1d1dd`
- self-review-spec-checks.log: `ff2ba6d5459da613c88f4dfea32c44e60d9974bd6a0430561ba7172ef770ee0d`

Rollback: revert this repair commit; no migration or release manifest change.
