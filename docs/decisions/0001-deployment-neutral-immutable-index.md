# Decision 0001: one deployment-neutral immutable index data plane

- Status: accepted direction
- Owner: Russell Lewis
- Date: 2026-08-26
- Normative contract: [`../specs/deployment-neutral-index-platform-v0.md`](../specs/deployment-neutral-index-platform-v0.md)

## Context

Corvint must remain a local-first, proof-carrying context compiler while serving four materially
different execution environments: a one-shot developer process, an authenticated cloud MCP worker,
an ephemeral exact-commit review worker, and a documentation compiler. The current Go candidate
retains repository-sized source bodies, caps admission near 128 MiB, exposes only local stdio MCP,
and has no production-scale immutable-format evidence. Separate local and cloud engines would make
canonical parity, corruption recovery, review receipts, and rollback substantially harder to prove.

The term “database” in this decision names the complete retrieval and evidence-compilation system.
It does not select a conventional DBMS. Git commit, tree, and blob identities remain semantic
authority. Index roots, segments, vectors, claims, and rendered documentation are derived state.

## Decision

Build one transport-neutral Go indexing/query kernel and one portable immutable index format.
Deployment-specific adapters provide local files, authenticated object-range reads, exact review
deltas, and documentation compilation without changing canonical index bytes or receipt semantics.

The physical design has two levels: content-addressed extraction fragments keyed by Git object and
adapter profile, then tree-specific placement assembled into bounded, query-optimized packed
segments. Query workers use a bounded `ReaderAt` abstraction, verify every touched block against the
root manifest, and do not deserialize or retain the whole repository by default.

Local operation is the primary product: one Go binary, no account, network, hosted service, Python
runtime, or permanent daemon. Hosted MCP, provider synchronization, review snapshots, optional
vectors, and living documentation are separate profiles that reuse the same kernel and roots.
Repository-owned Python/Node/rendering environments may run only as isolated downstream renderers;
they are not part of the Corvint engine.

## Alternatives considered

- **Keep the current resident index.** Rejected as the target because repository-sized retention and
  the 128 MiB boundary do not meet the million-file/RSS goals.
- **Use one mutable server database or vector database.** Rejected as canonical core because it
  weakens offline parity, deterministic receipts, tenant isolation, and cache-disposal semantics.
- **Build separate local and cloud engines.** Rejected because byte identity, corruption recovery,
  and review/documentation parity would become cross-product problems.
- **Select a custom pack immediately.** Rejected. Immutable SQLite remains the safer P0 baseline;
  the custom pack is selected only after controlled corruption/parity/scale evidence shows a
  material advantage.
- **Require a resident local daemon.** Rejected. Client-owned acceleration may compete only with an
  idle/cleanup/latency proof and must terminate on EOF or bounded idle timeout.

## Consequences

- Existing local stdio MCP, Go migration, Pulse, and documentation-compiler V0 contracts are not
  silently broadened. Each new deployment profile needs an accepted slice contract and conformance.
- Cache loss or corruption may change latency only. It cannot change canonical answers.
- Clean, incremental, local, cloud, review, corrupt-cache rebuild, and no-cache modes must converge
  on byte-identical roots and canonical receipt cores for the same snapshot/profile; explicitly
  versioned delivery envelopes retain their mode-specific authority/observation fields.
- Optional semantic retrieval remains advisory and cannot displace authority or prove absence.
- Documentation source repositories remain read-only; normal documentation mutation occurs only in
  the documentation repository through reviewed pull requests and passing gates.
- Format implementation cannot begin until the benchmark/format plan passes Gate A with no HIGH
  concerns. All unmeasured targets remain `PROPOSED` or `NOT_RUN`.

## Rule ownership

This decision records rationale and alternatives. It is not a second home for build rules. The
normative requirements, limits, failure semantics, promotion gates, and rollback rules live only in
the linked capability spec and its accepted descendant profiles.
