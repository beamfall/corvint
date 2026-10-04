# Immutable delta V0 conformance

SPDX-License-Identifier: Apache-2.0

`decision-vectors.json` contains closed wire/decision vectors. Their repeated fixture OIDs and
digests are shape fixtures, not claims that those objects exist or that Core produced those bindings.
The executable immutable merge fixture is `internal/delta/compile_test.go`; its two actual runs must
be byte-identical and preserve refs/index/state. `regression_test.go` retains independent-review
counterexamples. `cmd/corvint/delta_test.go` includes entrypoint, public dispatcher and help tests.
Their source presence does not qualify the current native binary; terminal native replay remains
pending. No benchmark, execution attestation or runtime coverage is claimed by these vectors.

The schema is `protocol/delta/schema.json`. All fields are closed; unknowns and gaps determine
`findings` before suite obligations determine `tests-needed`. `docs-only` requires exclusively
classified documentation paths with no unresolved evidence or test obligation. `no-op` describes a
complete empty immutable change set. Optional unconfigured work keys and the permanently unknown
runtime denominator are informational; missing lexical/provider/baseline evidence remains findings.
The current conservative adapters may leave many changes in findings: they do not infer a narrow
suite or complete lexical universe to make another decision reachable.
