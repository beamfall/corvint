# OpenCode terminal context inspector

Owner intent: on 2026-09-28 Russell accepted the proposed context sidebar and clickable evidence
first slice with “looks good. build it”. AHI-033 records that scope. The broader impact/proof
views and desktop/web embedding remain deferred; this change makes no delivery claim for them.

The base is `decf36d6354a369c0afaa05b7c34ab1cf1e29c87` from `origin/main`. The original checkout's
uncommitted work was preserved. OpenCode 2.0.18 was available and its exact npm declarations
confirmed sidebar/session-panel slots, native keymaps and cancellable RPC. A scratch native
rendering probe succeeded before the full inspector was built.

The native view uses the host's theme and narrow-screen panel behavior. Context is a bounded,
volatile projection of the adapter's latest receipt; it preserves the supplied reasons, authority,
confidence, gaps and immutable handles. Source expansion stays in Core. No model call, index
refresh, stored prompt/receipt journal, second graph, or completion authority is introduced.
RPC uses OpenCode's existing client trust boundary, checks session project/location for queries,
and does not treat possession of a session ID as authentication. Freshness is the state when
observed; external edits are not continuously watched.

Pre-change query and path-impact receipts were retained under the private Git directory. Query
returned four omitted candidates and nineteen withheld test-path candidates; these remain context
limitations. Path impact found the harness contract and importing adapter tests. `affected` was
run after the source diff; its broad Go recommendation was not treated as coverage proof. The
owner's scoped-work policy selects adapter/UI regressions and documentation checks; repository-wide
`make gate` remains NOT_RUN. No retrieval/ranking or core wire semantics changed, so frozen retrieval
experiments, mutation trials, corpus providers and learning routes are not applicable.

The initial `dogfood-change` at the unchanged base produced no CEM (`git-diff-failed`) and no
outcome input. Those pre-change NOT_PRODUCED rows are retained, not reported as a passing change.
The implementation cycle caught a stale runtime adapter-version constant and a native fixture's
already-created config directory; both were corrected before final verification. The package-version
history check also requires the implementation commit, so its precommit failure is retained.

The real 2.0.18 native witness exercised sidebar → context request → panel → Core-expanded
two distinct source files through Down/Up and Enter, then a 72-column screen and gap navigation. A separate interrupted run reached the same
path and left no observed descendant alive. Its report binds the source file digests and host
version. This is UI functional evidence, not full AHI-032 qualification, a latency/recall gate,
execution attestation, or external task-success evidence. Exact qualification records for an older
package are invalidated naturally by the new package/version digests.

Plan review by the independent native reviewer found no HIGH concern. Its request-admission,
session/location, stale-result, native-witness and presentation-escaping constraints were adopted.
Independent review identified cross-session freshness invalidation and insufficient native keyboard
selection coverage. Repository edits now invalidate every retained session view and any in-flight
expansion; the focused regression passes. The native witness now selects and expands two distinct
sources at wide and narrow widths, including interruption cleanup. Final review, frozen selected
checks, CEM/OCM and local dogfood closeout are retained in the private handoff evidence. Unlinked
OCM requirements remain unassessed; JavaScript test evidence is not presented as Go claim linkage.
