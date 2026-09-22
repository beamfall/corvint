# Decision 0039 — the Go declaration set is stated, and DR-0018 becomes a Python defect

Date: 2026-09-03. Status: accepted. Authority: repository owner, verbatim instruction "accept the
DR-0018 clause" (2026-09-03), answering the question carried in `docs/agent-memory/questions.md`
since 2026-09-03.

## The clause

`GPK-V0-048` of `docs/specs/go-production-kernel-migration-v0.md`: a Go source contributes exactly
one symbol per name declared at file scope as `go/ast` reports it — every `FuncDecl`, and every name
of every `ValueSpec` and `TypeSpec` of a `GenDecl`, whether or not the declaration is written inside
a parenthesized group. A parenthesized group is a grouping of declarations, not a scope, and does
not change the symbol set. Where the file does not parse, the lossy scanner's symbols are recorded
and the source additionally carries its `Unparsed` row.

Two things were tightened against the amendment as the register proposed it. `type (...)` is named
alongside `const (...)` and `var (...)`, because the same AST node carries all three and stating two
of them would have left the third in the gap the clause exists to close. And the `Unparsed` row on a
parse error is a MUST rather than a description, so a parse failure narrows the symbol set visibly
instead of silently.

## Why the oracle is not repaired

`GPK-V0-033` forbids resolving a divergence toward the candidate, but this one is not resolved
toward it: the clause was decided on its own terms, and the candidate happens to satisfy it. The
oracle's per-line `_go_symbols` (`src/context_corvint_index.py`) emits nothing for a name
declared inside a group, which contradicts the clause; the oracle is frozen and scheduled for
deletion, so `src/` is not changed and `DR-0018` is re-adjudicated `python-defect` /
known-divergent. `internal/betarung/admissions.json` no longer carries it as open.

The rejected alternative was making Go drop group members to match the oracle. That is a measured
fidelity regression — over cobra's `active_help.go` the oracle sees none of `activeHelpMarker`,
`activeHelpEnvVarSuffix`, `activeHelpGlobalDisable`, and over Beamfall's `internal/pairing/
pairing.go` none of `enrollAttemptAction`, `enrollAttemptWindow` — adopted to preserve parity with
a runtime being removed.

## Evidence

No code changed. `goGroupNames` (`internal/contextindex/parse.go:343@d0011009`) already flattens every group,
and `TestGoSymbolsNamesEveryGroupMember` already pins it against a fixture whose own assertion
proves it discriminates: the lossy scanner reaches one of the six members. The clause is therefore
stated over an implementation that satisfies it, which is the only honest order — the spec was
missing, not the behaviour.

Contract metrics are unchanged and were unchanged before this record: 0/31 critical misses and
recall 1.0 under both engines, byte-weighted precision 0.780332 Go / 0.804952 Python. Only
non-critical ranked selectors on the cobra active-help and Beamfall exact-feature-pairing cases
differ, which is what the register recorded on 2026-09-03.

## What this record does not do

It does not make any `conformance/perf-v0` task valid. That harness absorbs a divergence only
through `ratifiedAcceptedDivergences` (`conformance/perf-v0/manifest.go`), a closed set in Go
source holding exactly `DR-0004`, bound to one task and one exact argv; decision 0007 item D9
requires a further entry to carry its own ratification and a source change. A known-divergent
register entry is not a perf grant, and no perf grant is created here. It does not repair, port, or
schedule work on `src/`, and it does not touch the still-open `DR-0014`.
