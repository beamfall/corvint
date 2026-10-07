# 2026-10-07: evicted snapshot mappings are never unmapped (V1-0947)

## Intent

Ticket V1-0947 suspected that `retainPack` and `retainSectioned` in `internal/contextindex` drop an
evicted mapping from the four-slot ring without unmapping it, so a long-lived process (such as
`corvint-mcp` serving several repositories) keeps every mapping it ever made. It asked for unmapping
once no in-flight Index uses the bytes, a bounded live-mapping count, and the contract in
`docs/specs/index-snapshot-v0.md`.

## Finding: confirmed, and the requested fix is unsafe

- **Confirmed.** A scratch probe read twelve distinct copies of a fixture pack and of a fixture
  sectioned file, dropped every Index, deleted each file, and ran the collector. `lsof` on the test
  process still listed twelve mappings for each format: the cache held four and eight were orphaned.
  The deleted files stay allocated while mapped. The growth is per cache miss, not per distinct
  file: twenty reads cycling five unchanged files left twenty mapped regions per format (`vmmap`;
  `lsof` lists one row per file, so it showed five). Both formats are opt-in
  (`CORVINT_SNAPSHOT_FORMAT=pack|sectioned`); the default gob path does not map.
- **Aliases escape the Index.** In the same probe `Symbol.Path`, `Symbol.Name` and a `Source.Text`
  result of a pack load all pointed into the mapping. Consumers copy these out: for example
  `internal/doccorpus` keeps `Source` values in its own map. The collector does not trace pointers
  into a mapping, so reachability of the Index, the `packFile` or `sectionedFile`, or any other
  owner says nothing about whether those bytes are still in use.
- **Unmapping faults.** Two mutants were run against the new guard test: release on ring eviction,
  and `runtime.AddCleanup` on the retained `packFile`. Both died with SIGSEGV reading an escaped
  `Source.Data`. In a long-lived server that is a crash; if the address were reused by a later
  mapping it would read another file's bytes, which is wrong evidence rather than a refusal.

## Decision

No production change. The lane added the guard test and a spec amendment that states the real
contract: the bound applies to retained mappings, not live ones, and an evicted mapping is never
unmapped. Bounding live mappings safely needs an owner choice between:

- an explicit lifetime (lease or `Close`) for every value that aliases a mapping, with copy-out at
  every boundary that can outlive it; or
- heap-backed bytes, which the collector can track. The pack's measured 8 MB heap row excludes this.

## Evidence

- `TestEvictedMappingsStayValidForEscapedAliases` (pack and sectioned) keeps whole `Source` values
  and bare `Source.Text` and symbol strings after dropping the Index. It passes under `-race` on the
  unchanged readers and fails with SIGSEGV under both mutants. It skips where no mapping was made.
  Its catch of a cleanup-driven unmap is best effort, because cleanups run asynchronously after a
  collection. Deferred-body loads are not covered.
- Logs are in the lane's private evidence directory, not committed.
