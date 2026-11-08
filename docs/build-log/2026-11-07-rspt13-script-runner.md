# Script command runner (V1-5115)

RSP-V0-050 is implemented as an internal, compiled-in script plugin factory in
`internal/testrunner/registry`. The lane starts at
`80274abee8da1afdf4e193751b6f1f71231e4581`; the requested affected/dogfood-start
base is `8b80e6020d8a78b65ca0b05d8f3d829a1ecb4734`.

## Implementation boundary

The declaration reuses `sessiontest.ParseDeclared` and the existing
`corvint-test-commands/0` schema without changing it. The default source is
`.corvint/test-commands.json`; the existing `.github/cishards/test-commands.json`
instance is also admitted explicitly. Reads use verified Git tree/blob objects
at the requested commit and refuse absent, linked, malformed or oversized files.
The Go flags/environment template describes one aggregate Go command. Each
non-`none` language entry describes one literal argv command; glob words are
preserved, with no shell expansion or per-file inventory invented.

Each entry has a stable `script:<entry>` identity and its own SPI instance and
inventory key. This fits the existing one-process Command/Parse SPI while giving
every aggregate command its own traced execution and reconciliation. The key
binds checkout, declaration path and bytes, command identity and target. The
inventory source path is the declaration: aggregate tasks need not have a single
native test source. Unknown commands produce an absent plugin and FULL.

The core executor accepts an explicitly admitted executable and trace request,
checks argv/cwd agreement, applies declared environment and invokes
`internal/proctrace`. Only its exit observation supplies a terminal SPI result;
stdout is never treated as a passing report. Unsupported tracing retains no
terminal result or complete evidence. Incomplete tracing may still describe a
successful FULL execution but cannot authorize exclusion.

The trace adapter uses T1's record/components and immutable Git identities for
reads and execs. It preserves listings, failed lookups, writes and external
contact, and marks unsupported repositories, paths, states or bounds incomplete.
It always replaces native `executed`, `regions` and `loaded` with absence reason
`script-kind`; execs are file dependencies. It neither consumes coverage nor
implements a second selector. Core admission, freshness and future qualification
remain responsible for production exclusions. The script qualification piece is
explicitly absent, so this slice cannot promote any runner.

No legacy runner registry entry, existing wire shape, command surface, frozen
output, golden or existing test assertion changes. Rollback removes these internal
factory/adapter files and their tests; no store migration is needed.

## Verification and retained limits

`TestRSPV0050DeclaredCommandInventory` covers the five requested fixture argv forms
and the Go aggregate template. `TestRSPV0050ExistingDeclarationSchema` checks the
existing project instance. Other RSP-V0-050 tests cover undeclared FULL,
malformed/linked declarations, traced SPI execution, file-only components,
failed-lookup retention, incomplete/capped traces and refused execution.

The touched package passes `GOTOOLCHAIN=local go vet` through the local heavy
wrapper. Formatting, static spec generation/checking and line citation checking
pass. The lane is build-only: all Go tests, live execution/qualification, independent
review and final dogfood binding/checking are NOT_RUN. The full spec Make targets
also execute Go tests, so only their static scripts ran. Generation produced no
index changes. Test success and qualified narrowing are not claimed.

The pre-change context receipts reported revision
`81e0254c1e31caa290a859ff73ae26c47e5a6efb`, different from this lane's HEAD,
including a later explicit-root read. Their revision binding remains uncertain;
local original source/spec reads govern this implementation. The affected receipt
uses the lane's actual HEAD but reports `self-binary-stale`, scope UNKNOWN,
511 selected units and 82 unknowns (77 language frontiers, one no-selectable-test,
four unowned dirty paths). Selection is advice, not passing test evidence.

The requested start dogfood pass produced an incomplete report (missing citation,
intent and outcome inputs) plus a merged-forward/wide-base warning. Its generated
shared CEM was restored to the pre-task bytes; no final bind or seal ran. Tool
update checks could not prove process quiescence under the host sandbox and
reported freshness UNKNOWN; the official releases endpoint returned no releases.
Build 127 remains installed. Raw receipts/logs live outside the repository in
`~/lanes/rspt13-evidence/`; the lane handback records command results and digests.

## Retained receipt digests

The external files above have the following SHA-256 identities. Empty formatting
output means no files needed formatting; an empty release list does not establish
that the installed developer build is the latest.

| Receipt | SHA-256 |
|---|---|
| `context.json` | `fe129f5d3b2d4d03c3027e7c5b5f3e5ffb77f03a77b4150969f8098259b20992` |
| `context-explicit-root.json` | `27aa9308b65a7c426d38e0d8d2695e1610d71380dd05db704bad39f45dbefad9` |
| `affected.json` | `135099ea5a5de3da6df420085d1db6f8c0d876ed87393a1ce34751fdfcd5ae1f` |
| `dogfood-start.log` | `32db87e6fa7b548a7b35d8b8795d63bf0edfbe0e722c37852a5dea0bbc9827ca` |
| `vet-final.log` | `46e5639902d66199a743ff0027890de84aba9f181d7cdc51d87f5a8a4fd1941a` |
| `spec-write.log` | `574000ff552503675798e941126791a2bd9028c1ae10acfa0799c446b1be1e19` |
| `spec-static.log` | `6bf14df520819e42dae8ff5a53f29686ba6a055d883c40c9a36293aaaf51f6b5` |
| `repin.log` | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
| `line-citations.log` | `855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba` |
| `gofmt.log` | `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855` |
| `releases.json` | `4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945` |
