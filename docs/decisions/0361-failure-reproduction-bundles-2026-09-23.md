# Decision 0361 — Explicit failure-reproduction bundles under `prove`, replayed on a second checkout as history

Date: 2026-09-23. Status: proposed (experimental delivery; ticket V1-0024). Amends
`docs/specs/falsifiable-packet-v0.md` revision 22 (FPK-V0-041 to FPK-V0-049). It adds no root
verb and changes no existing `prove` output.

## Context

A refused or surprising `prove` result is hard to report. The reporter has to describe the
request, the commit, the binary, and any checkpoint by hand, and the maintainer cannot tell
whether a second run differs because the input, the code, or the engine changed. A reproduction
aid must not become a second route to fresh evidence. If an old receipt can be replayed into
something a verifier, promotion, or learning path accepts, invariants 1, 2, and 5 are broken.

## Decision

1. `prove --export-bundle` is an explicit opt-in on the existing verb, for query and checkpoint
   results only. It requires a clean worktree. It writes one canonical JSON document to stdout
   and nothing else. The document records:
   - the exact arguments;
   - the commit, tree, and cited blob identities;
   - the Corvint version, build, and `prove` profile;
   - the checkpoint bytes;
   - the original receipt (without the local ledger) or refusal.

   The bundle is at most 8 MiB. Every string value is screened by `internal/secretscreen`, and a
   match refuses the export. A self-digest detects edits. It is not a signature.
2. `prove --replay-bundle FILE` checks the bundle and this checkout, then reruns the frozen
   request through the same parser and `compileProof`. It fetches nothing, because object reads
   set `GIT_NO_LAZY_FETCH`, and it executes no recorded command. It either reports `reproduced`
   (exit 0) or `diverged` (exit 1) with the differing member paths, or refuses with one named
   code: `invalid-bundle`, `unsupported-version`, `tampered`, `missing-input`,
   `incompatible-engine`, `missing-git-object`, `drift`, or `mixed-worktree`.
3. The comparison covers `exit`, `error.code`, and the whole receipt. The only exclusions are
   declared in advance and printed in every report: `error.message`, which can name local paths,
   and `proof.ledger`, which is never bundled. `engine.build` is recorded and not compared, so the
   same release built twice can replay.
4. Bundles and replay reports carry `historical: true` and no `falsifiable-packet/0` profile.
   `prove-observe`, the only reader of `prove` output, refuses any document with a `historical`
   member. No bundle member is read by ranking, learning, authority, or promotion.

## Consequences

- A bundle contains source snippets that the receipt cites. The operator inspects it before
  sharing it. The secret screen refuses secret-shaped strings conservatively, including a
  committed test fixture that looks like a key.
- The query packet's `learning` members depend on the local trace store, which is not Git
  content. A divergence limited to them points at that store.
- A dirty worktree cannot be exported or replayed. Reproducing uncommitted state is a non-goal.

## Rollback

Delete `cmd/corvint/prove_bundle.go` and its test. Restore `parseProveInvocation`/`runProve` at
the `main.go` dispatch. Restore `readCheckpointDocument` in `compileCheckpointProof` and fold
`decodeCheckpointDocument` back into it. Drop the `historical` condition in `proofCounts`. Remove
the help lines, FPK-V0-041 to FPK-V0-049, and this decision. Bundles already exported become inert
files.
