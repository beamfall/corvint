# Decision 0015 — `impact` admits `.py` and web paths without a Go module

Date: 2026-09-01. Status: accepted. Authority: repository owner, verbatim instruction "then do
the impact language slice next" (2026-09-01), given after the Agent Retrieval Bench impact arm
showed 12 of 28 change-aware code2test samples refused as `unsupported-impact-repository`.

## Scope

The instruction accepts one amendment to `GPK-V0-027` in
`docs/specs/go-production-kernel-migration-v0.md`: the slash-qualified Go module is a
precondition only when a changed path is `.go`, because only rule (a) resolves through the module
path. Rule (b) `.py` and rule (c) web paths are admitted in any Git repository, which is what the
Python oracle already does (`_impact` never reads a Go module). Parity evidence: the oracle on a
fastapi snapshot without any `go.mod` answers `impact fastapi/_compat/v2.py` with the path and
two reverse importers; the new `impact-python-nomodule` fixture replays the module-free form of
the existing `impact-python` fixture against the oracle.

## Explicit exclusions

This decision does not add a reverse-import rule for `.rs`, `.java`, or any suffix without an
oracle branch: decision 0007 D2 excluded them under `GPK-V0-033`, and the bench's 8 Rust and 2
Java changed files stay refused as `unsupported-impact-path-suffix` until the owner overrides
that ruling. It does not change the non-root-package rule, the 128 MiB admission bound, or any
promotion, cutover, merging, tagging, signing, publication, or release gate.

## Consequences

`Impact` checks the module only when `needsGoModule` finds a `.go` path; the refusal message
names `.go` paths. The bench impact arm becomes eligible on the release's 12 Python samples.
