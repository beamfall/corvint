# Change Frontier V0 independent conformance vectors

Independent-consumer vectors and adversarial fixtures for `frontier/0`, specified in
[`docs/specs/change-frontier-v0.md`](../../docs/specs/change-frontier-v0.md).

`CF-V0-028` is the reason this directory exists:

> an independent consumer MUST reproduce every reference result byte-for-byte from the same raw
> inputs and Git objects. Corvint producer-to-Corvint verifier tests alone cannot satisfy this
> interoperability requirement.

**Every expectation here was authored from the clause text.** Nothing in this directory was read
from, captured from, or verified against `internal/frontier`. That independence is the whole value
of the suite: a vector regenerated from the implementation asserts only that the implementation
agrees with itself. If a vector and the implementation ever disagree, the vector is not
automatically wrong — re-read the clause and decide which one is.

## Layout

| Path | What it is |
|---|---|
| `vectors/codec.json` | 45 hand-derived `CF-V0-019` commitment-codec vectors |
| `vectors/identity.json` | 4 `CF-V0-006` universe IDs, 12 `CF-V0-007` item IDs, 1 `CF-V0-019` frontier ID |
| `fixtures/<row>/case.json` | one directory per row of the spec's conformance and adversarial matrix |
| `manifest.json` | matrix row → fixture ledger, with each row's required result verbatim |
| `refcodec.go` | independent native derivation checked against the hand-authored bytes |
| `refcodec.go` | an independent Go implementation of the `CF-V0-019` codec |
| `adapter.go` | the only seam to the implementation, deliberately tiny |

## What runs today

```sh
go test ./conformance/frontier-v0/...          # vectors + fixture self-consistency
go run  ./conformance/frontier-v0 -check       # coverage report and data validation
go test ./conformance/frontier-v0
```

All three seams are bound (`binding_impl.go`, `runner_impl.go`). The fixture runner builds a REAL
temporary Git repository per case — real commits, a real `cem/0.2` sidecar from the real CEM
two-phase workflow, a real OCM artifact bound to that map and patch — and invokes `frontier.Compute`
over it through `internal/frontierrepo`.

Not every declared universe can be built, and the suite says so per case rather than per suite. Each
case derives its **required capabilities** from its declared universe (`capabilities.go`); a case
whose requirements the bound runner does not have skips with the SPECIFIC reason that universe is out
of reach — "a timeout cannot be injected through the public entry point", "no verified universe can
carry zero obligations" — never a blanket one. Run with `-v` to read them.

Two kinds of universe are SYNTHESIZED rather than read off the fixture:

- a **complete CF-V0-003 dynamic tuple** (`tuple_impl.go`) — the command and the observation come
  from TCQ's own producers, and the JUnit report names the execution identity TCQ derives from the
  universe's committed claims blob, so the row really MATCHES instead of merely existing;
- a **CF-V0-023 counts universe** (`counts_impl.go`) — a limit case declares a size, not a hunk
  list, so the universe that realizes it is derived from the bound being driven. Deriving it also
  settles satisfiability: five of the seven bounds are fed by an artifact whose OWN ceiling equals
  the Frontier one (a CEM map holds 2,048 hunks; the OCM verifier admits 256 obligations and 64
  references per array), so those at-N+1 cases cannot be materialized at all, and each skips with
  the ceiling that blocks it. `TestCountBoundsAreDefenceInDepth` fails if one of those coincidences
  stops holding, so a case that becomes materializable stops being skipped.

Everything else is green now:

- every codec vector is re-derived by `refcodec.go` and must equal the hand-authored bytes;
- every recorded digest must digest its own recorded bytes;
- every universe, item and frontier ID is re-derived from its recorded preimage;
- every fixture is validated against the closed `CF-V0-017`/`CF-V0-022`/`CF-V0-023` vocabularies;
- `manifest.json` is checked against the **live** spec table, so a new matrix row fails the suite
  until a fixture covers it.

## Independent derivation of one clause

`CF-V0-019` is specified in full and can be implemented from the text alone. The Go
`refcodec.go` derivation must agree with every hand-authored vector. The redundant Python
derivation generator is retired under GOC-V0-002; its original source remains in Git at
`9ca27f9a`. Frozen vectors must never be regenerated from the candidate implementation.
`refcodec.go` deliberately does not use `encoding/json`. That decoder replaces invalid UTF-8 and
unpaired surrogates with U+FFFD and accepts duplicate object keys last-wins — three inputs
`CF-V0-019` names as **invalid**. A lenient parser would silently convert three required rejections
into acceptances, which is exactly the class of bug an independent consumer exists to catch.

## Codec edge cases the clause names

Each has at least one vector, and `TestCodecVectorCoverage` fails if one disappears.

| Clause phrase | Vector |
|---|---|
| duplicate object key is invalid | `invalid/duplicate-object-key`, `invalid/duplicate-key-nested` |
| invalid UTF-8 is invalid | three vectors, in a value, in a key, and a truncated sequence |
| a BOM is invalid | `invalid/bom` (hex-encoded, since no JSON string can carry one) |
| an unpaired surrogate is invalid | high, low, and a reversed pair |
| JSON numbers are forbidden | integer, float, zero, and inside an array |
| control characters escape as lowercase `\u00xx` | `serialize/escape-controls-lowercase-u00xx` — a tab becomes `\u0009`, never a short `\t` escape |
| keys sort by **raw UTF-8 bytes** | `serialize/key-sort-raw-utf8-not-utf16` |
| no whitespace | `serialize/whitespace-stripped` |
| no terminal LF inside `codec()` | `verify/trailing-lf-inside-codec` |
| exactly one LF on a complete document | `document/*`: none, one, two, CRLF, leading |
| no optional escaping, no normalization | `serialize/no-optional-escaping`, `serialize/nfc-nfd-not-normalized` |

The raw-UTF-8 key sort is the one that separates implementations. Keys `U+FF3A` (`ef bc ba`) and
`U+10000` (`f0 90 80 80`) sort with `U+FF3A` first by raw bytes; a UTF-16 code-unit sort — the
natural default in several languages — puts the surrogate pair `d800 dc00` first and produces
different bytes and therefore a different ID.

`CF-V0-018` prints an all-zero placeholder for the document `id`. `vectors/identity.json` records
the **real** identity of that exact document,
`frontier:sha256:141826f8f017a9b2471252b1e32c75c3ea37c1c1e26de8e49cf80fe971f3fb7f`, so an
implementation cannot pass by reproducing the placeholder.

## Adversarial fixtures

One directory per matrix row; 28 fixtures, 81 cases. Each case declares its universe in a **closed
vocabulary** — a typo fails the suite today — and the exact required outcome. Every operational
failure pins the byte-exact `CF-V0-022` envelope and its SHA-256, so a leak cannot hide in an added
field:

```json
{"code":"noncanonical-frontier","profile":"frontier-error/0"}
```

The negative rows are the ones worth reading first, because a permissive implementation passes a
naive test on all of them:

- **`noncanonical-frontier-refusals`** — `OPEN` with empty items, `EMPTY` with items, an extra
  stop-decision field, and a retrieval-packet term in the wire are each `noncanonical-frontier`,
  exit 2, **no stdout**. Not a document emitted with a warning.
- **`aggregate-lrf-bound-is-resource-exhaustion`** — an aggregate LRF bound document is
  `frontier-resource-exhausted`, and the string `relevance-bound-exceeded` is asserted absent from
  the output. `CF-V0-022` is explicit that it is a result issue, not an operational code. The
  contrasting case in the same fixture shows an explicit `unsupported-lrf-context` passing through
  as the exact upstream code, which is what distinguishes the two LRF rows.
- **`forged-tcq-authority-rejected-upstream`** — a forged authority class or an unknown relation is
  an upstream canonical rejection. The fixture asserts the emitted code is **not** Frontier-owned,
  so translating the rejection into a Frontier code fails.
- **`passing-caller-report-stays-open`** — a clean target, passing rows, exit zero and a signature
  over caller-controlled bytes still leave the frontier `OPEN`.
- **`error-code-translation/uncatchable-termination`** — the one row that guarantees nothing. It
  asserts only that no partial document is left behind, and must not be quietly upgraded into an
  exit-code assertion.

`TestNoFixtureClosesAnIntentTestItem` enforces the spec's own release blocker across the whole tree:
every linked obligation must carry both intent items, and no OCM-unknown obligation may carry a
fabricated test item.

## Privacy canaries (`CF-V0-024`)

Distinctive strings are planted in all six bodies the clause names — source, diff, command, report,
sibling path, exception text — and asserted absent from **valid and error** output.
`TestCanaryDeclarationsAreSelfConsistent` fails if a canary is planted but never asserted absent, if
a canary appears in the expectation itself, or if one of the six bodies is never planted at all. A
planted canary nobody checks is worse than no canary.

## Limits (`CF-V0-023`)

Every one of the seven bounds appears at N and at N+1: complete result versus fail-closed
exhaustion with no partial output. The three per-kind bounds are driven **independently, with the
other kinds empty**, because 2048 + 256 + 256 happens to equal the 2,560 total bound — an
implementation that checks only the total, or only the three kinds, would otherwise pass.

## Binding the implementation

Add exactly one file to this directory. Nothing else changes.

```go
// binding_impl.go
package main

func init() {
	RegisterCodecer(...)    // CodecCanonicalize(raw []byte) ([]byte, error)
	RegisterIdentifier(...) // UniverseID / ItemID over the recorded preimage codec bytes
	RegisterRunner(...)     // materialize Case.Declared, invoke, return Outcome
}
```

A `Runner` also declares `Supports(Capability)`. That is what lets a runner execute every case it
can materialize and skip the rest by name: the alternative — one seam that is either bound or dark —
turns a handful of unbuildable rows into a wall of harness failures that hide the real results.
`Outcome.Symbols` reports what the runner bound each of the fixture's symbolic identifiers to, so a
suite that could not name a content-addressed hunk or claim ID still compares the full item
projection.

`adapter.go` names no `internal/frontier` symbol on purpose: these vectors were authored before the
entry point existed, and guessing its signature would have made them hostage to the guess. The
three seams are separable — an implementation that exposes only its codec can still be checked
against all 45 codec vectors.

`Outcome` is expressed in wire terms (exact bytes, exit code) rather than implementation types, so
the binding survives a refactor of the library API.
