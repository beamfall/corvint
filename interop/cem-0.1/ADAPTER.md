# CEM 0.1 conformance adapter

The kit tests implementations through a process boundary. A consumer adapter must not import,
link, translate, or subprocess Corvint. Invocation is:

```text
IMPLEMENTATION verify --repository DIR --map FILE --patch FILE [--target FULL_OID]
```

The harness supplies absolute paths. The adapter reads exact bytes and writes exactly one UTF-8 JSON
object to stdout on protocol results:

```json
{"accept":true,"spec":"cem/0.1","drift":[]}
```

With `--target`, `drift` contains one item per evidence ID sorted lexically:

```json
{"evidenceId":"...","path":"...","status":"stable|relocated|stale|ambiguous|deleted","targetBlobOid":"OID or null","targetSpan":{"start":0,"end":1}}
```

The current frozen public suite contains one evidence item in each drift case. It does not test the
multi-evidence lexical-order rule above. A public-suite PASS is exact agreement with its six drift
vectors, not strict multi-evidence conformance; a versioned next suite must close that gap.

`targetSpan` is null for stale/ambiguous/deleted. Exit 0 means accepted and `accept:true`. Exit 1
means protocol-invalid or unsafe drift and `accept:false`; diagnostic fields are allowed but ignored.
Exit 2 means invocation, repository I/O, unavailable SHA-256 support, or a documented lower resource
limit prevented a decision. Exit 2 is `UNSUPPORTED/ERROR`, never a conformance acceptance or
rejection. Do not execute map, patch, repository, hook, filter, textconv, credential helper, or
network-derived content.

For a manifest case with `patchRecipe` instead of `patch`, the harness materializes the one closed
recipe defined in `ALGORITHMS.md`, verifies its fixed byte length, LF count, and SHA-256, writes
those exact bytes to a private temporary regular file, and supplies that file as `--patch`. The
adapter receives no recipe metadata. The harness rejects unknown, malformed, oversized, dual, or
missing patch selectors before adapter invocation; recipes cannot run code or accept parameters.

Producer adapters consume a single entry from `manifest.json.producerJobs` plus the reconstructed
repository and exact patch, and write a CEM map. `hunkIndex` is zero-based patch order. A supported
assignment selects the raw-byte evidence span and relation; unknown/mechanical assignments supply
their reason. Output JSON formatting and array order are free, but the decoded structure and all
derived IDs must equal `expectedMap`. Every produced map is then verified by all registered
consumers. A producer may retrieve evidence however it likes outside these fixed jobs.

The standalone external runner currently executes **consumer adapters only**. A process ABI for
producer jobs is `NOT_BUILT`: prose describing a producer job is not an executable interoperability
contract. Until a separately specified ABI and non-public anti-copying qualification exist, the
runner MUST NOT fill or imply the independent `P1` matrix cell.

`runner.py` is retained, changed only by erratum 1's manifest pins (`IMPLEMENTATIONS.md`), as the historical executable adjacent to the raw kit and as a
packet-identity member. The active non-normative convenience runner is the native Go command in
`../../tools/cem-interop-runner`. Historical Python receipts are not current native receipts. It is
harness: implementations conform to this document, `ALGORITHMS.md`, and `manifest.json`, never to
runner implementation details. The runner imports and invokes no Corvint implementation. It invokes a
fresh private copy of the initially read implementation entry-point bytes for every case; adapters
must therefore be self-contained at that entry point or resolve declared runtime dependencies
independently of the entry-point file's original directory. The manifest-derived executable in
Corvint's harness tests is fixture plumbing only and is not independent semantic/conformance evidence.

Full conformance is a matrix, not this public self-test: every required valid case exits 0; every
invalid case exits 1; every drift case has the declared acceptance, status, target OID, and span;
every producer output matches its expected semantics and is accepted by every consumer. SHA-1 and
SHA-256 are required. The current runner can report public-suite agreement only.
