# OCM V0 conformance vectors

Frozen wire vectors and adversarial fixtures for `ocm/0.1-experimental`, specified in
[`docs/specs/ocm-v0-dogfood.md`](../../docs/specs/ocm-v0-dogfood.md) (`OCM-V0-014`).

The suite freezes the minimum portable proof wire (ticket V1-0013): the exact bytes the real
producers emit, the canonical encoding, every disposition and `unknown` reason, and the refusal
code for each hostile input. Unlike `conformance/frontier-v0/`, the valid vectors here are **derived
from the implementation**, not hand-authored from the clause text: the purpose is to notice when
producer bytes or refusal codes drift, so that every drift is recorded in the spec before the data
is refrozen. A vector that disagrees with the implementation is therefore a wire change to record,
not automatically a defect.

## Layout

| Path | What it is |
|---|---|
| `vectors/structural.json` | 25 byte-level vectors: 5 valid maps and 20 hostile inputs, each with its refusal code |
| `fixtures/<id>/case.json` | 5 fixtures, 24 cases: one seed universe each, one perturbation per case, exact verifier verdict |
| `manifest.json` | vector and fixture ledger, the wire-profile clause each case covers, the six hostile evidence states (`states`) and the SHA-256 of every vector and fixture file (`artifactSha256`) |
| `universe.go` | the deterministic seed repository (pinned Git identity and dates) and the real producer calls |
| `adapter.go` | the only seams to the implementation: the structural parser (`mark`) and the full verifier (`status`) |
| `vectors.go`, `fixtures.go`, `manifest.go` | loaders, self-validation, and the perturbation operators |
| `main.go` | `go run ./conformance/ocm-v0 -check` validates the data without touching the implementation |

## What runs

```sh
GOTOOLCHAIN=local go test -count=1 ./conformance/ocm-v0/
GOTOOLCHAIN=local go run ./conformance/ocm-v0 -check
```

- `TestFrozenVectorsMatchTheRealProducer` rebuilds each valid vector's universe with the real
  `prepare`, `link`, and `mark` producers and requires the frozen bytes to match byte-for-byte.
- `TestStructuralVectorsAgainstRealParser` offers every vector to the real structural parser and
  asserts the declared outcome: round-trip identity for valid maps, the exact code otherwise.
- `TestValidVectorsAreCanonical` proves each valid map is its own canonical encoding using only
  the CEM wire codec.
- `TestStructuralVectorsCoverEveryDisposition` proves both dispositions and all four `unknown`
  reasons appear in the frozen valid maps.
- `TestFixturesAgainstRealVerifier` builds each fixture's universe, applies each case's
  perturbation, and asserts the real `status` verdict (refusal, state, code, counts).
- `TestSuiteDataIsSelfConsistent` runs the `-check` validation, which also requires every
  stable/relocated/stale/ambiguous/deleted/unknown state to name existing cases and every file
  under `vectors/` and `fixtures/` to match its pinned digest (`OCM-V0-015`).
- `TestArtifactDigestDriftFails` proves a one-byte edit of a frozen vector fails that check.

## Derivation

Every valid vector is the byte output of the real producers over `universe.go`: one base commit
(intent doc with two requirements, a claims test file, a source file), one target commit (source
change), and one empty successor commit, all under a pinned Git environment so OIDs, hunk IDs,
claim IDs, and CEM digests are stable across hosts. Hostile vectors are one minimal edit of a
produced map. Fixture cases are one structural perturbation of a produced map (`set`, `swap`,
`drop`, `append`, `top`, `cem`, `intent`, `intentShift`), applied through the CEM wire codec so the perturbed map stays
canonical and only the intended defect is present.

Two outcomes were confirmed while freezing and are asserted as-is: an extra top-level member is
refused with `unknown-field` (the wire is a closed object; there is no preservation path), and a
map binding a full OID the repository does not hold is refused with
`repository-object-unavailable`, which the spec says takes precedence over `target-mismatch`.
The `intent-scope-drift` fixture (added 2026-09-22) confirmed that a relocated or stale-digest
intent span refuses with `intent-scope-mismatch`, while an absent intent blob or path refuses with
`repository-object-unavailable` (`OCM-V0-012`).
