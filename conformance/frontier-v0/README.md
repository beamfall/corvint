# Change Frontier V0 independent conformance vectors

Independent-consumer vectors for `frontier/0`, specified in
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
| `manifest.json` | the retired fixture ledger (`retired`: decision, requirement, reason, and every retired fixture with its matrix rows and cases) and the SHA-256 of every vector file (`artifactSha256`, `CF-V0-034`) |
| `refcodec.go` | an independent Go implementation of the `CF-V0-019` codec |
| `adapter.go`, `binding_impl.go` | the codec seam, bound to the shared WP3 codec |

## Retired fixture tier

`frontier/0` was deleted with the `cem/0.x` wire on 2026-10-12 (V1-1278 S2b, decision 0525 §1,
`CEM-V1-217`). Every adversarial fixture case declared a universe built from a verified `cem/0.2`
(or `cem/0.1`) map and invoked the `frontier/0` entry point; neither exists any more. The stable
successor is `frontier-stable-view/0` (`CEM-V1-126`, V1-1304), which never produces a frontier
document, so no case has a stable expectation to carry over. The fixture directories, their runner
(`runner_impl.go`, `tuple_impl.go`, `counts_impl.go`), the capability model and the identity seam
were deleted; `manifest.json` records all 30 retired fixtures and 84 retired cases with the matrix
rows they covered, and Git history keeps them. The stable route's refusals are tested in
`internal/frontier/stable_test.go` and `internal/frontierrepo/stable_test.go`.

## What runs today

```sh
go test ./conformance/frontier-v0
go run  ./conformance/frontier-v0 -check       # report and data validation
```

- every codec vector is re-derived by `refcodec.go` and must equal the hand-authored bytes, and is
  checked against the shared WP3 codec through the codec seam;
- every recorded digest must digest its own recorded bytes;
- every universe, item and frontier ID is re-derived from its recorded preimage;
- `manifest.json` must record the retirement and pin every vector file's digest.

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
