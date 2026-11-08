# V1-4941 self-review repair: retain reader and suite obligations

Scope: AFP-V0-9310, with AFP-V0-9311/9312 regression coverage. The owner supplied
P1 (computed readers lost during reclassification) and P2 (Swift participation
without a suite obligation) as the only repair scope. The original experimental
status and frozen contracts remain unchanged.

## Decision and soundness classes

P1 is an obligation-routing gap: changing source classification must not hide the
path from other plugins' unresolved readers. Hosted unindexed Swift manifests now
retain Swift participation and also enter `participants`' unowned-path set. The
existing frontier-reach declarations are the only exclusion proofs: understood
source-bound or fixture-closed categories may close a frontier; undeclared forms
stay path-open. No Ruby-specific bad-form list or reader syntax special case was
added.

P2 is a missing suite proof: participation itself is not an input to the session
builder. The closed admission requires a Go-hosted `testdata` path ending exactly
in `/Package.swift` and the existing global
`swift:exclusion-evidence-unrepresentable` frontier. That known frontier carries
the Swift suite through the frozen consumers. Empty or unfamiliar frontier forms
retain `UNINDEXED_SOURCE_PATH` and FULL, even when the old fixture-only rule could
otherwise admit them. No invented frontier or wire member is needed. Additional
claimants and unobserved directories retain their previous uncertainty.

Rollback restores the pre-exception FULL fallback for hosted manifests; do not
restore metadata narrowing without both obligation checks. There is no migration.

## Evidence

`TestSwiftFixtureComputedReaders_AFPV09310` uses an actual RSpec reader with
`File.read(ENV.fetch("FIXTURE_PATH"))` and a declared Ruby suite. Its unknown-form
control supplies a plugin with an undeclared frontier category. Both retain their
frontier and declared suite alongside Swift. `TestSwiftFixtureSuiteProof_AFPV09310`
uses a real library-only manifest with an empty Swift frontier and an injected
unknown Swift frontier; both retain the source claim, FULL and the Swift suite.
Both tests exercise disk/immutable graph parity and the structural selector plus
session builder. Existing assertions and goldens are unchanged.

The pre-repair selector failed all four cases. The first repaired focused run
exposed a new-test fixture mistake: Ruby's frontier detail includes its source
site. The assertion now checks that complete existing detail; no product contract
or existing assertion changed. Final focused tests pass.

Raw evidence is under `/home/russell/lanes/swfix-evidence/`, outside Git. Final
whole-package and spec/citation results and raw evidence digests follow below.
Independent review and repository-wide verification belong to the host; this
entry does not claim either, ticket completion, or performance improvement.

Final whole-package command: `script/heavy.sh go test -count=1 -timeout 30m
./internal/liveverify/affected` (no `-run`), PASS: `ok   github.com/Beamfall/corvint/internal/liveverify/affected 106.249s`.
`make -s spec-requirements-check line-citations-check`: PASS; repinning found
no moved citations (180 pre-existing non-root basename citations skipped).
`corvint affected --base 8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734` selected the
touched affected package among 471 units with 71 unknowns. Its installed binary
warned that it predates this lane; this is selection evidence from that binary,
not a claim of current-selector output parity.

| External evidence | SHA-256 |
|---|---|
| `self-review-affected.json` | `8bd01b4d1d31549e194f3e01caae37301841779925063e6973512f05813420e2` |
| `self-review-before.log` | `d8c61d247e44d9bf836c7541e2ba2fcf75c36eb0e4b56b908929171152cfe0af` |
| `self-review-focused.log` | `f19678e6ae7d852a30d1ba77293e5f0a52d6bbb837d88062e0918aa20b94f956` |
| `self-review-focused-final.log` | `1dc4fc71461d33b53ce06b3bec2d96d1220551444585a6b95ed5b6435f6fa1ab` |
| `self-review-whole.log` | `e1343fb2d812673f3f5c56a91becee9355d49dab37f46e097b10d606f3a76962` |
| `self-review-doc-checks.log` | `faf9a3a9e452a181838dd865c6d3db75a00fa5da6eadbfd37dfb30080e0a166c` |
