# Experimental corpus republish companion

This optional local command builds from a separately approved documentation revision, records provenance and whole-inventory parity, and renders one draft connector request. It performs no publication. Approval authenticity and host credential isolation remain unqualified; `--trusted-local` acknowledges a host prerequisite and does not authenticate an author-supplied policy.

The host supplies a closed request and a separately owned policy, with the policy's original-byte SHA256 configured outside the author environment. Both use `corvint-corpus-republish/0`; their full fields are defined in `internal/corpusrepublish/types.go` and [the proposed contract](../../docs/specs/corpus-republish-v0.md). The request contains a pinned `/2` manifest, source and documentation commits, target, fixed timestamp, evidence URL and explicit sorted reuse eligibility. It has no approval field. Policy additionally selects `APPROVED`, previous corpus/Result/sidecar pins, allowed shard retirements, expected pending generation/digest, decision attribution and URL origins. A first publication uses explicit `NONE` prior pins and generation zero. A cold replacement uses an empty eligibility array and explicit `NONE` sidecar pins.

Use existing private output directories outside the source/documentation checkouts and their Git metadata. Policy, previous artifacts and caches also stay outside those author roots. The source Git repository must contain both immutable commits. The host owns a separate environment for authoring, building and deterministic queue writing; the command cannot prove that isolation.

```sh
corvint-corpus-republish build --trusted-local \
  --root /absolute/source-repo --documentation-root /absolute/docs-checkout \
  --request /private/request.json --policy /private/policy.json \
  --expected-policy-sha256 HOST_CONFIGURED_SHA256 \
  --output-dir /private/fresh-output
```

Build emits fresh `corpus.json`, `index.json`, `result.json`, `request.json`, `consumer.json` and, for admitted reuse units, `incremental.json`. Canonical Result pins the original consumer receipt by digest and retains its typed trust envelope. Corpus/index bytes have their existing bounds; request, policy, provenance, parity and Result are each bounded at 4 MiB. An output failure can leave complete fresh files already emitted; it never overwrites previous artifacts. Remove only owned incomplete output bundles before retrying in a fresh directory.

A replacement supplies `--previous-result FILE --previous-artifact FILE`; admitted reuse additionally supplies `--previous-sidecar FILE`. Policy pins all original bytes. Ordinary adoption record shards without observation or behavior contributions can reuse complete pre-global import contributions. Other explicitly eligible profiles take a named cold fallback. Incremental counters describe import work only: fresh restriction screening, global joins, full source validation and independent indexed production still run. No total-cost or speedup claim is made.

To queue, use the same host input flags with `queue --pending /private/pending.json` in place of `build --output-dir ...`. Queue rederives the corpus, Result and generated request before any state write. The canonical pending file contains a versioned typed plan (rendering input plus exactly one existing connector Request) and its unsigned generation; this retains the repository/target, policy, revisions, digests and counts needed to validate the fixed body. Exact retries write nothing. A new approved grant must match the prior generation and original-byte digest and replaces one stable key. Old or corrupt state refuses. The private single-writer profile provides atomic file visibility; concurrent CAS, hostile-directory races and power-loss durability are not qualified.

The companion is not registered in Core or a release archive. Stop invoking it to roll back; retain prior artifacts and pending state. The proposed requirements remain unaccepted and delivery remains unqualified until review and the terminal evidence/native completion path succeed.
