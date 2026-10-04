# H1 — finite semantic link resolution and exact preimages

PRIVATE successor contract. semantic-links.json is the exhaustive12branch source→target table; all other raw digests remain exact or use the already named native-seal facts after native verification. This is a switch over four fixed target kinds and their canonical nested locators, not a general graph system. Existing source schemas remain unchanged.

The new SemanticLinkV2 has exactly kind, locator, sha256. Four closed shapes exist:

- provider: `/providers/P/receipt`, payload is exactly that sem.jstestprovider.Receipt subtree.
- hook: `/hooks/H/receipt`, payload is exactly that sem.behaviorfalsify.HookReceipt subtree.
- control-evidence: `/assessment/controls/C/evidence`, payload is exactly the nested sem.behaviorfalsify.EvidenceReceipt subtree, excluding the sibling Control.receipt_digest link.
- attestation: `/providers/P/receipt/applicationAttestation/Q/attestation`, Q before or after, payload is exactly the sem.jstestprovider.ApplicationAttestation subtree. It excludes enclosing ApplicationAttestationObservation.outputDigest and provider wrapper, avoiding a backlink/cycle.

P/H/C indexes use canonical unsigned decimal (zero or nonzero digit followed by digits), no leading zero, signs, URI escapes, `~` escapes, relative path, registry id alias or alternate spelling. Provider.id and Hook.id are the corresponding receipt locator and cannot introduce aliases. One provider entry per Report.Run in original native order, then one per control/result/attempt native tuple in original nested order. One hook per complete tuple. Baselines may contain multiple requested tests, so provider wrapper uses ordered `test_ids` derived from request inventory filtered by actual native requested file/isolation identity, preserving request order; control-native wrappers have exactly one target test ID. Every wrapper's native run/control/test fields must agree with the locator/index-derived association. Baseline and control-native entries cannot alias even with equal raw bytes. There is one nested control evidence per request test/control, and one separate attestation target per present before/after observation.

Every target has one producer-owned preimage object, with exactly:

```
{kind, locator, association, payload}
```

Association schema is frozen in semantic-links.json: native request digest; product/test/draft commits; run index (null only for control-native), kind/ordinal; ordered test IDs; control index, planned_control_id/ordinal/attempt (null for baseline and for the whole control-evidence payload where no single planned control is selected); attestation_phase (before/after only for attestation, null otherwise). All values derive from verified raw/native identities and exact request/control bijections, not caller JSON. Provider/hook/control associations exclude executionID/raw hashes; attachment binds freshness separately. Control-evidence association has null run_index, run_kind control, run_ordinal0, test_ids=[request.Tests[C].ID], control_index C, and null planned_control_id/control_ordinal/attempt_ordinal because payload covers the complete control set. Hook/control-native association binds the exact planned ID, ordinal and attempt; provider/hook wrapper control_id is that planned control ID, while outer Report.Control.ID and test_ids remain the request test ID. These identities are never conflated. Attestation association is its containing provider's association with phase filled. Before/after remain different identities even for equal payload bytes.

Digest is SHA256(ASCII `corvint-new-e2e-semantic-node/2` +NUL+ existing wire canonical JSON of that full preimage). No LF, no second canonicalizer, no floating stable fields. Associations and locator participate, preventing same-shaped wrong-target substitution. Payload is the full closed typed subtree and already contains child link digests; own wrapper/digest does not. Native raw hash recipes/preimages are additionally verified separately before this mapping: exact decoded provider/hook/attestation/stdout/artifact bytes, and exact native EvidenceReceipt aggregate recipe with own Digest emptied. Native marshalJSON removes LF and disables HTML escaping; do not substitute sorted-key canonical bytes into a native digest recipe.

Allowed dependencies, topologically:

1. Verified logical process facts (embedded exact values, not semantic links); parent topology must be acyclic. Birth-distinct relations are facts, not payload-hash edges.
2. Each attestation payload, containing logical instance and exact source/build/config/health facts.
3. Each provider payload, with outputDigest links to its own nested before/after attestation leaves.
4. Each hook payload, with native receipt/artifact links to its corresponding control-native provider.
5. Each control evidence payload, with RawAttempt hook/provider links, nested hook receipt links, HookProcess stdout link, retained native artifact links, and verified aggregate seals.
6. Whole decision, with Report.Run provider links, sibling Control.receipt_digest→nested evidence, complete provider/hook registries and exact logical process registry.

A target referenced multiple times is hashed once from its unique locator; all repeated references must carry the same recomputed kind/locator/digest. Native raw duplicate artifacts are retained in their actual original arrays; they cannot create multiple semantic targets for a tuple. Every registry entry must be used by its expected source association; extra targets, missing targets or unresolved dependencies block. A native artifact link requires exact path and byte-preimage correspondence to that tuple's decoded hook native artifact, not merely digest equality. Only Hook.artifacts and its corresponding AttemptResult.retained_artifacts may use the native provider link; any other Artifact use stays exact or unsupported. CleanupProcess stdout and all stderr are log-facts only, never hook targets. Native success logs preserve absent/empty/present and actual bytes in attachments; consumed/error logs remain exact/refuse.

Complete topology/hash example is semantic-example.json plus semantic-example-preimages.json. It includes two baseline providers, one control-native provider, six before/after attestation leaves, one hook and one nested control evidence target, all12link branches (including repeated hash/byte/artifact joins). Exact full typed target preimages and their canonical UTF8 bytes/base64 and SHA256 values are retained. This is a synthetic closed semantic-wire/link example: it does not contain verified kernel/native execution preimages and must never qualify a run or mint process tokens. It demonstrates finite resolution and absence of a containing-node cycle, not native acceptance.

Regression plan: wrong target kind, same payload under wrong test/control/ordinal/attempt/phase, alias locator, duplicate provider/hook/control target, optional missing attestation, containing-provider attestation cycle, unrecognized Artifact context, HookProcess/CleanupProcess stdout confusion, reordered native registry/context, digest computed without association or over containing wrapper, altered preimage and unresolved extra target. Every such case refuses before connector Build/Record. Existing native test/run/worker schedule order remains exact.
