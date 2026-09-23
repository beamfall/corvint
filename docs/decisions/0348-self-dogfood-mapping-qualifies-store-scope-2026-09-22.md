# Decision 0348 — Corvint's own `decision-0046-v0` mapping qualifies store scope

Date: 2026-09-22. Status: accepted (ticket V1-0082; amends decision 0321).

## Decision

Decision 0321 (WQO-V0-046) let a repository adopted through `corvint work init` reach
`VALIDATED_AT` by having Corvint independently recompute the `repository-worklist-v0` mapping
from the qualified committed tree and comparing byte-for-byte against the adapter's own output.
It explicitly left Corvint's own `decision-0046-v0` self-dogfood mapping (`docs/worklist.json`,
route `codex`) unqualified, so `corvint work observe` on this repository stays
`UNKNOWN/SOURCE_UNQUALIFIED` and `propose-wave` always abstains here, even on a clean, undrifted
capture.

The byte-reproduction argument that qualifies `repository-worklist-v0` holds identically for
`decision-0046-v0`: `worklistadapter.DocumentsFromSource` derives both mappings deterministically
from a committed worklist file read through the qualified source (`SourceFile`, the Git blob, not
the working-tree disk file) plus the policy, with no adapter-side state outside that tree. For
`decision-0046-v0` the committed file is `docs/worklist.json` instead of `.corvint/worklist.json`;
nothing else about the closure differs. `workMappingReproduced` (`cmd/corvint/work.go`) now accepts
either mapping version and recomputes the matching closed mapping before comparing canonical bytes.

This changes WQO-V0-046's own store-scope predicate (repository adoption, and Corvint's own
self-dogfood repository, both now reach complete store scope through the same requirement) and the
WQO-V0-017 paragraph's stated exception, which previously named `decision-0046-v0` as excluded. It
adds no new wire profile, policy field, store root, or mapping version; `decision-0046-v0` already
existed and was already the mapping this repository's policy declares.

The WQO-V0-021/025/032 final-check fixtures previously got their incomplete initial capture as a
side effect of the self-dogfood mapping staying unqualified. With that mapping now qualified, the
fixtures instead retain incomplete scope through mutation evidence they already construct for other
cases: `closing-inability` and `prior-mutation-and-closing-inability` now precede their untracked
caller-tree write with an empty commit (`git commit --allow-empty`), the same technique
`source-drift-and-mutation` already used. The commit changes an existing top-level entry (`HEAD`)
inside the Git-directory monitored root, which the filesystem manifest walk records via its
existing-entry content-diff branch (no root-completeness gate); the untracked file still dirties
`git status` and produces the closing qualification failure each case asserts. Each fixture's
assertion meaning (closing-inability, positive mutation, empty/stale proposal) is unchanged.

Three other tests built on the same production fixture (whose real `.corvint/work-queue-policy.json`
already declares `mappingVersion: "decision-0046-v0"`) asserted `State: StateUnknown` on a capture
with no deliberate mutation or drift; that was this same side effect, not their stated purpose
(isolation/materialization, network-unobserved qualification). `TestObserveWorkUsesOnlyTargetMaterialization`
is updated to assert the now-correct `VALIDATED_AT`/`UNCHANGED_OBSERVED` outcome; its isolation and
network-qualifier subtests are otherwise unchanged.

## Alternatives weighed

- *Leave `decision-0046-v0` unqualified permanently*: decision 0321 deferred this exact change
  ("Qualify both closed mappings: equally sound, but it changes the self-dogfood contract... It can
  be proposed separately"). Leaving it deferred keeps Corvint's own dogfood observation abstaining
  for no reason the byte-reproduction argument doesn't already cover for the adopted-repository case.
- *A third, narrower mapping version just for this repository*: no behavioral difference from
  reusing `decision-0046-v0` (already the live policy's mapping); it would only add an unused
  vocabulary entry.
- *Rework the final-check fixtures' incompleteness mechanism around `materialization.target`*:
  rejected — `materialization.target` is a freshly randomized directory created on every
  `observeWork`/`workFreshAtReturn` call, with no stable path to pre-seed a persistent marker before
  the first capture.
- *Place the incompleteness marker directly under the caller tree's `.git`*: this was the original
  approach and is broken — for a standard (non-external-git-dir) repository, `GitDir` is nested
  inside `Root` (`GitDir = Root + "/.git"`), and `.git` sorts before every other top-level entry, so
  an unsupported manifest entry anywhere inside it aborts the filesystem walk before any other
  top-level sibling (including a newly written caller-tree file) is ever visited — silently
  disabling new-file mutation detection at the caller tree root, not just store-scope completeness.

## Rollback

Revert `workMappingReproduced` to accept only `worklistadapter.RepositoryMapping`. Revert the
`closing-inability`/`prior-mutation-and-closing-inability` fixture scripts to their untracked-write-only
form and `TestObserveWorkUsesOnlyTargetMaterialization`'s assertion to `StateUnknown`/`"UNKNOWN"`.
Revert the WQO-V0-017 and WQO-V0-046 paragraph amendments. Corvint's own dogfood observation returns
to `UNKNOWN/SOURCE_UNQUALIFIED`; adopted repositories under `repository-worklist-v0` are unaffected.
