# Go authority-start query parity evidence

`manifest.json` is test evidence only. It is not a runtime authority source.

**Superseded 2026-09-12 by `docs/decisions/0088-*.md`:** `GOC-V0-002` forbids executing or
reconstructing the retired Python oracle, so `CORVINT_REQUIRE_PYTHON_ORACLE=1` below can no longer be
satisfied live; replay against already-captured frozen expectations remains the supported path.

Replay both pinned repositories and require the Python oracle:

```sh
CORVINT_QUERY_PARITY_CORVINT_ROOT=/path/to/corvint \
CORVINT_QUERY_PARITY_BEAMFALL_ROOT=/path/to/beamfall \
CORVINT_REQUIRE_PYTHON_ORACLE=1 \
go test -count=1 -run '^TestAuthorityStartParityManifestReplay$' ./cmd/corvint
```

A skip caused by either missing fixture-root variable is not parity evidence. Promotion requires a
passing replay whose fixture commits, trees, process bytes, and pre/post status digests match the
manifest exactly.

## Generated-header expectation correction (2026-09-07)

The original manifest is preserved in Git at
`33d926fe61710cc0cfb92b89232f932407a593a0:conformance/go-query-start-v0/manifest.json`;
its complete-file SHA-256 is
`5b13f8c9537a9e9b3fcbfc0a796cdc09a9c4a305d46a2934124c5bb542672d27`.
The original Corvint packet was 3,652 bytes with SHA-256
`bb28ce718a69de3c0cd7c4cbc805f40e39e07381f17a824554e00bb9e6866a4b`;
the original Beamfall packet was 5,137 bytes with SHA-256
`ab7456d5d3543154e1d0351580b02825e454cc0e226b3ead9441a612e93ecc69`.

The recorded generated-header correction landed in
`abcbcff24c1c8f0ba52f8bd85bdab880d07d896a`: a generated-file marker must start a
line after its permitted indentation/comment prefix. A marker quoted inside a
source string or appearing mid-line in prose is not a file header. The old
manifest incorrectly excluded Corvint's `internal/contextindex/impact_test.go`
and Beamfall's `script/fuzz_manifest_test.py` because their first 4,096 bytes
contain quoted marker text. The existing BUILD-LOG records this correction;
this expectation maintenance does not introduce new accepted intent.

The replacement expectations were independently derived from the original
Python packets and that recorded rule: remove the single false exclusion from
each packet, reduce the exclusion count by one, and recompute Corvint's canonical
packet-byte fixed point. Beamfall's bounded exclusion samples do not contain
its false exclusion, so only its count changes and its byte length is unchanged.
The original Python source reproduces the historical hashes; current Go and
Python independently match the derived replacement bytes. Only Corvint's stdout
length and the two stdout hashes change. Fixture commits, trees, input, status,
exit and stderr expectations, mandatory Python comparison, and all exact-byte
assertions are retained. The `GPK-V0-028 replay` test label supplies a literal
requirement witness without changing replay behavior. These two cases remain
bounded conformance evidence, not promotion or platform qualification.
