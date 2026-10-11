# V1-1136: static application-map entry candidates

AMAP-V0-050..059 are proposed, pending owner acceptance (V1-1136); implementation is experimental.
`flows appmap entries --map FILE` adds a capped, deterministic static projection: URL template,
required path and parent parameters, optional query names, inherited permission/flag guards and
classification. UI-router screens carry optional entry metadata; legacy and other-dialect screens
remain UNKNOWN without that metadata. Resolves, transition hooks, redirects and arbitrary params
remain explicit uncertainty, including inherited constraints. No application code executes.

The projection checks committed manifest and complete lineage blobs at the evaluated revision.
Template/guard/ancestor changes produce UNKNOWN with UNVERIFIED_AT_HEAD; unrelated commits retain
STATIC_ONLY. This conservative policy can invalidate on unrelated edits in a router file. Static
inspection never claims a successful load or populates last_verified_revision. Existing flow-step
and test-sequence edges are not execution proofs: shortest PROVEN paths and ledger integration
remain deferred alongside trusted-local runtime verification. Any future runtime requires fixture,
session and read-effect admission; this CLI refuses runtime flags.

Small wiring changes in router.go/types.go and flows_appmap.go accompany separate entries files.
No edge-ledger, flows/Playwright, grouping or change-impact implementation is changed. Rollback
removes the projection and optional metadata, then rebuilds maps. Older closed readers require
maps built by their matching producer. The existing map schema and error codes are retained.

Baseline focused hierarchy/public CLI tests passed. All nine requested documentation gates passed,
including five receipt-regression cases. The repaired focused implementation run passed (details below). Tests trace all ten requirements, including
stale template/guard/ancestor/manifest changes, historical and missing revisions, alias refusal,
legacy map loading, byte caps, unknown inherited constraints and CLI nonmutation. main.go is unchanged;
receipt repinning is not applicable. Full cmd/corvint execution and make gate are NOT_RUN by scope.

Corvint routes **used**: explicit-root query, path impact and affected against
55aa6bb9f1e9d8db81f8185b5d185fc8c1bbc821; receipts under /private/tmp/w5-appmap-a-*-local.json.
Query and impact returned snapshot revision 718b550dd09f681627d2d585f72dc7fe1d59af17 rather than
worktree HEAD b5cea8769ef39fc3c662dc5129ba5fc566f60cd2, so they are limited context evidence only.
Impact retains 24 omitted results/callers; affected retains language-frontier and non-Go unknowns. Exact source
and caller reads plus the owner's selected checks govern this slice. **Unavailable**: index refresh
(shared Git directory write refused) and initial make dogfood-change (worktree Git evidence directory
write refused). Post-commit CEM/OCM binding, frozen evaluations and seal are **deferred** by the no-commit
boundary. Ranking/learning/providers are **not applicable**. No self-use outcome or qualification is claimed.

Core is 1.0.0-rc.3 (build 407), Tasks is 0.0.0-tcp01-unverified+build.407. Official component update
checks failed their process smoke (descendant snapshot unavailable); available versions/freshness
remain UNKNOWN. No verification bypass or install occurred. Account usage telemetry is NOT_OBSERVED.
The owner forbids ticket-store writes, so friction filing and ticket completion remain coordinator work.
No commits, pushes or decisions were created. Current model/effort controls are not observable here;
a native Astra/medium independent reviewer was selected for evidence-semantics review, without nesting.

The sandbox denies the linked worktree's real Git index. Requirement generation and index-based gates
use a copied temporary index and scratch object directory, with the owning spec staged there before
`make -s spec-requirements`. The normal index is unchanged; the coordinator must stage delivered files.
Bare BSD mktemp ignored TMPDIR; a scratch PATH wrapper supplies -p for default templates and passes
explicit absolute templates through. Gate scripts are unchanged. Go uses GOTOOLCHAIN=local,
TMPDIR=/private/tmp/claude-501/gotmp and GOMAXPROCS=3.

Independent read-only Astra/medium review completed with no actionable findings. It inspected
classification, inherited constraints, full-blob revision binding, legacy compatibility, bounds
and explicit runtime/path uncertainty. Reviewer use of untracked-path Corvint impact retained
WORKTREE_EVIDENCE and 309 omitted candidates; neither review nor that receipt is immutable proof.
The original passed documentation-gate output is /private/tmp/w5-entries-doc-gates.log; the
receipt regression output is /private/tmp/w5-entries-receipts-test.log.

The initial selected `TestEntries|TestAMAPV0` run failed only the new legacy-map fixture: it
removed metadata and changed edges without recomputing the artifact digest. The loader correctly
refused it; cmd/corvint passed. The repair proves baseline Encode/ParseMap first and recomputes the
synthetic legacy map digest before reloading. Product code is unchanged. Independent review of
this fixture repair found no issue. The failed run remains in /private/tmp/w5-appmap-a-tests.log
(384.885s appmap, 3.005s CLI); no broad-suite pass is claimed.

Final focused command: `go test ./internal/appmap/ ./cmd/corvint -run
'TestEntries|TestAMAPV0002HierarchyResolution|TestAMAPV0FlowsAppmapCLI' -count=1 -timeout 30m`.
Both packages passed (8.235s and 1.969s); output: /private/tmp/w5-entries-tests-final.log. This proves
the new entries witnesses and representative existing build/projection compatibility. The unchanged
receipt check reports all 18 receipts current and its regression test reports five cases passed.
Final index-dependent documentation checks are retained in /private/tmp/w5-entries-doc-gates-final.log.

Remaining integration: coordinator staging, owner acceptance of the proposed requirements,
post-commit CEM/OCM/seal and native ticket completion. Runtime, PROVEN paths and adopter qualification
remain unimplemented. This builder has not modified .taskman or the normal Git index.
