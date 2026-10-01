# Corpus parity companion command boundary

Date: 2026-10-01
Intent: GH #399 / V1-0546, part of umbrella #388.
Owning contract: `docs/specs/documentation-corpus-v1.md`, new `DCP-V1-043`.

## Observed defect

PR #418 delivered the switch-over harness (`DCP-V1-041`) and indexed artifact (`DCP-V1-040`)
with package-level proof, but `cmd/corvint-corpus-parity` itself had no test. Exercising it found
that `build` mode declared its producer error inside a nested scope: a readable file that the
producer refused (for example a provider record named as `--artifact`) exited 0 with empty stdout
and no diagnostic. The read modes shadowed the same variable for encoding failures. A CI step that
pinned the empty output's SHA256 would have advanced with no artifact. The new
`TestCorpusParityCommandFailuresExitNonzero` failed with `producer-refusal: exit 0 stdout 0 bytes`
before the repair and passes after it.

## Decision

`DCP-V1-043` qualifies the companion at its own command boundary: every failure exits nonzero with
empty stdout and a stderr diagnostic; two command builds from identical inputs are byte-identical;
query returns the trust-enveloped native receipt; parity over a recorded question set binds the index
and recording digests, reports per-tool agreement (including an operation-mismatched question) and
repeats byte-identically; a wrong operator pin produces no report. The repair assigns the outer error
and renames inner read errors; it changes no wire profile, index byte or package API.

## Evidence and limits

`TestCorpusParityCommandEndToEnd` and `TestCorpusParityCommandFailuresExitNonzero` run the real
`run` entry point over a committed two-commit Git fixture. Pre-change Corvint query and impact
receipts were captured outside the worktree Git directory because this session's worktree isolation
refused commands naming Git paths outside the worktree; they are retained privately, not committed.

Issue #399 remains open. External adopter utility, a real bespoke-server recording, hosted
TLS/authentication/security and MCP Streamable HTTP qualification remain `NOT_OBSERVED`. The
supplemental symbol-as-path, multiple-path selector, altered-posting-only and multi-file coverage
proofs named in `2026-10-01-corpus-final-integration.md` remain `NOT_OBSERVED`. Owner acceptance of
the experimental profile is unchanged. Repository-wide `make gate` is `NOT_RUN` under the scoped
issue policy.

## Rollback

Revert this change. The previous command remains usable for successful runs; reverting restores the
zero-exit producer failure.
