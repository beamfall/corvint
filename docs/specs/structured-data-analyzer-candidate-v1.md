# Structured-data analyzer candidate V1

Owner: Russell Lewis
Intent status: rejected-as-specified
Delivery status: deferred
Disposition: a root-span-only fact has no ranking value. A future key/value-coordinate specification
would be new work, not a revival of this candidate.

## Agent digest
- Claim: Root-span-only structured-data coordinates add no ranking value, so this candidate is rejected as specified.
- Status: rejected-as-specified/deferred
- Exists: isolated root-coordinate extractor and immutable dogfood literal tests.
- Blocked on: a new key/value-coordinate specification with demonstrated ranking value.
- Read next: Intent and non-goals; Pinned Beamfall dogfood corpus; Acceptance evidence and rollback.

## Intent and non-goals

This isolated native-Go command parses immutable caller-supplied bytes under
one exact tuple `(corvint-structured-data/experimental-v1, structured-data,
format-profile)`. It emits only a `structured.document.coordinate` fact with a
root path, exact byte span, line/column origin, source witness digest, and the
selected format/profile. It does not state that a document is semantically
valid for an application schema, that an entity exists, or that any profile is
compatible. It is not Core, is not imported by Core, has no registry entry,
lock, package/install path, selector, launcher, admission, verifier, or
support claim.

The command receives no path, repository handle, environment input, network
handle, subprocess handle, schema URL, resolver, or writable destination. It
uses stdin only and writes exactly one LF-framed result to stdout. It does not
read its CWD, `HOME`, repository, or any caller path.

## Exact closed tuples

| Format tuple | Accepted closed form | Explicit denial boundary |
|---|---|---|
| `structured.json/rfc8259-v1` | one UTF-8 RFC 8259 value, depth/token/string bounded, duplicate object keys rejected | duplicate keys, malformed UTF-8/number/string, over-depth |
| `structured.jsonl/rfc8259-v1` | one nonempty UTF-8 JSON object per LF line | blank/CRLF/scalar line, duplicate keys |
| `structured.yaml/1.2-core-v1` | LF-terminated UTF-8 top-level scalar mapping subset | controls, tags, anchors, aliases, flow/nested/directive or malformed numeric forms; a key passes the same implicit-typing check as a value, so signed non-canonical numbers (`-0`, `-1.5`) and typed keys (`01`, `True`, `.inf`) reject (`TestClosedLexicalAndXMLBoundaryMatrix`) |
| `structured.toml/1.0.0-v1` | LF-terminated UTF-8 scalar key/value subset | controls, dotted keys, overflow/date/time, tables, arrays, inline/multiline forms |
| `structured.xml/1.0-v1` | UTF-8 strict XML with one root, whitespace-only outside text, and no namespace attributes/elements | DTD, entity reference, misplaced/non-exact declaration, comments, foreign namespace, duplicate attributes |
| `structured.plist/xml-v1` | exact UTF-8 XML declaration plus unnamespaced `plist version="1.0"` root and no DTD/entity | binary plist, DTD/Apple external entity, declaration/root/version/namespace mismatch |
| `structured.properties/java-17-v1` | UTF-8 ASCII key/value LF lines | continuation, escapes, comments, duplicate key |
| `structured.hcl/2.0-static-v1` | static top-level identifier assignments | blocks, traversals, template/interpolation, calls, arrays, comments, all expressions |
| `structured.webmanifest/whatwg-v1` | static closed Web Manifest metadata and icon projection | unknown members, non-string metadata, a non-array (including `null`) `icons` member, dynamic/resolved URL semantics |
| `structured.svg/1.1-static-v1` | static canonical SVG namespace root | non-SVG XML, DTD/entity/comment forms, foreign namespaces |

There is no extension, content-sniffing, generic-parser, version-nearness, or
fallback selection. Each profile is dispatched by literal request tuple only.

## Requirements

- `SDA-001`: Input is one canonical bounded LF JSON request; duplicate and
  unknown/noncanonical envelope forms receive a minimal fixed sentinel.
- `SDA-002`: After envelope validation, every failure binds the verbatim supplied profile,
  request/scope/unit/target identities and every supplied input digest; an unknown profile stays bound (`TestUnknownProfileRejectionStaysBound`).
  A foreign family or an invalid or duplicate echoed identifier, feature, handle, path, or digest keeps the sentinel (`TestInvalidEnvelopeRejectionStaysUnbound`).
- `SDA-003`: Base64 transport, decoded content, aggregate content, depth, token, ordinary string,
  input, fact, and output limits are checked before a result grows past its
  declared cap.
- `SDA-004`: Format/version modules are exact, independent dispatch cases;
  there is no cross-format acceptance or semantic validity fact.
- `SDA-005`: Facts are deterministic, typed structural coordinates with root
  path, byte span, line/column, selected profile, and domain-separated witness.
- `SDA-006`: Candidate code has no filesystem, process, network, PATH, schema,
  entity-resolution, or ambient-write capability.
- `SDA-007`: Core byte/dependency identity remains unchanged because no Core
  package imports the candidate.

## Pinned Beamfall dogfood corpus

The self-contained compressed literal corpus in
`internal/analyzerstructured/dogfood_literals_test.go` binds these immutable
Git blobs, not workspace paths at test time:

| Repository revision | Blob | Format/profile | Expected result |
|---|---|---|---|
| `beamfall@da38c59eb30b2121cbac37b912485b30b2e54841` | `703bfd80e42eccf0131519dc9c276a051bafcf1a` `docker-bake.hcl` | HCL 2.0 static | `REJECTED(UNSUPPORTED_SCHEMA)` because it has blocks/templates |
| same | `0a6acd1fb9fc8b4e000214a3cc2234a6c3b4e058` `contracts/export/v1/fixtures/full-state/records/library.jsonl` | JSONL RFC 8259 | coordinate candidate |
| same | `eb2689ebffa427fc87bafa325b171d2dad57d302` `packaging/unraid/beamfall-core.xml` | XML 1.0 | coordinate candidate |
| `beamfall-android-ui@6e379d7feec88128439d1753325bcfb22194fdfc` | `f37de70e9ddb470b6f286d0c6173e71be72cb9e8` `gradle.properties` | properties Java 17 | coordinate candidate |
| same | `3e9e4647b0ef36b19e9d7b639de7252b29aa7d54` `kit/src/androidTest/AndroidManifest.xml` | XML 1.0 | `REJECTED(UNSUPPORTED_SCHEMA)` because namespace attributes are outside V1 |
| same | `e0b4eabe37b8c8787ed1d51b1498da1889a9f4ad` `gradle/verification-metadata.xml` | XML 1.0 | boundary fixture; size exceeds the V1 aggregate cap |
| same | `442e4c36346c40995a7c88a3b5326ee671a68fb8` `gradle/libs.versions.toml` | TOML 1.0 | boundary fixture; size/grammar qualification remains closed |
| `beamfall-apple@8588cec3dedaef63bbff458e5e7c8bb6335de107` | `5c9d018cb19babfd24cd74adcaef1b96e5812872` `Apps/BeamfallMobile/Info.plist` | plist XML | `REJECTED(UNSUPPORTED_SCHEMA)` because its DTD is forbidden |
| `beamfall@da38c59eb30b2121cbac37b912485b30b2e54841` | `3e652e990e320eaeb850cc342a7d11133ea754a6` active Web Manifest | WHATWG Web Manifest | coordinate candidate after exact static `scope` tuple admission |
| same | `12cc99b3c3692462217eabf610638dfde58fb61a` active manifest SVG | SVG 1.1 static | coordinate candidate |

The XML/plist/property/HCL corpus is intentionally included even though it is
absent from the Corvint base checkout. Rejection of an outside profile is safety
evidence, not a claim that the Beamfall source is invalid.

## Acceptance evidence and rollback

Focused exact tuple and rejection tests are local implementation evidence only.
Fresh-process 1,001 built-CLI permutation, full rejection binding/reason/bound
matrices, descriptor byte/digest freeze, local filesystem/content/mode and
environment/CWD/process/network/syscall positive-control spies, dependency/AST
closure, source digest, allocation, and latency ceiling tests are local evidence
only. The allocation and latency ceilings are non-race measurements: a race build
skips `TestProductionCausalRatchets` and supplies no ceiling evidence. Race/vet/cross-platform builds, full exact review, CEM/OCM production
binding, admission, selection, launch, and exact verification remain separately
recorded and never inferred from these tests.
Rollback is one commit revert; no registry, lock, artifact, cache, or Core state
is created by this candidate.
