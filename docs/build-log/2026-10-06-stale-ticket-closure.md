# 2026-10-06: Stale ticket closure after the rc.3 backlog review

## Intent

The owner asked for a review of the remaining Corvint tickets for rc.3 (2026-10-06) and then
"close the stale tickets". Five read-only reviewers flagged open tickets whose acceptance criteria
looked already met on `origin/main`. Each flagged ticket was then re-verified independently against
its acceptance criteria by a read-only Codex pass (`codex exec -s read-only`, five batches A to E).
Only tickets with a CONFIRM verdict are closed here; PARTIAL and REJECT verdicts stay open.

## Method

- Verifier verdicts: CONFIRM means every acceptance criterion is met by a cited commit on
  `origin/main` plus retained repository evidence (test, spec text or build-log entry).
- Mechanical check at `a5aeee41` (this entry's base): every cited commit is an ancestor of
  `origin/main`, every cited `Test*` name and every cited repository path exists, and every ticket
  was `OPEN` before closure. All 78 rows passed.
- Excluded although CONFIRMed: V1-0797 and V1-0839 (closed with the rc.2 candidate evidence).
  Every other rc.2-flow ticket (V1-0753, V1-0765, V1-0840, V1-0841, V1-0842, V1-0846) also stays
  with the rc.2 closeout.
- REJECT and PARTIAL rows (for example V1-0466, V1-0494, V1-0285, V1-0449, V1-0181, V1-0289,
  V1-0459, V1-0113, V1-0130, V1-0225, V1-0267) remain open with the verifier's reason as the
  remaining work.

## Closed tickets

| Ticket | Commit(s) | Batch | Acceptance evidence |
|---|---|---|---|
| BUG-BREAKAGE-CONSTRAINT-LOOKALIKES-0930 | f27901eb,8c7c8d24 | C | AC1–2: breakagemap/go_refs.go:58 checks actual header comments and trailing platform suffixes, with Compile lookalike fixtures and constraint controls; AC3: committed ticket/build-log records retain red/green, package/vet and independent-review outcomes, the sealed 2508765e CEM remains, and PR #415’s merge is ancestral. |
| V1-0124 | b58dbe3b | C | AC1: internal/genesis/repository.go:70 preserves git-timeout; activation_fallback_test.go:81 uses hanging Git and asserts INVALID/git-timeout. |
| V1-0129 | fe6567b0 | E | AC1: ROADMAP.md:17,41 labels both unreachable commits pre-snapshot; remaining cited commit 1894b9e is an origin/main ancestor |
| V1-0133 | e1a2dec2 | E | AC1: interop/cem01-go/ci.go:198 validates map structure before comparing its base; interop/cem01-go/ci_test.go:162 asserts structural rejection despite base mismatch |
| V1-0134 | 05404f56 | E | AC1: conformance/frontier-v0/manifest.go:46 and conformance/ocm-v0/manifest.go:41 share the vocabulary; conformance/states_test.go:24 checks both against one list |
| V1-0136 | c496a2ce | E | AC1: examples/evidence-provider/v0/conformance/main.go:263 bounds the probe and reports timeout refusal; main_test.go:38 tests a never-exiting provider and elapsed bound |
| V1-0137 | 58ccd4cb,7dcd6216 | C | AC1: .corvint/changes/82a113a8c48017685a2449ca3cf2a4bcb9847628.cem.json retains the integration CEM; script/dogfood-seal.sh:19 refuses unarchived-base-cem. |
| V1-0138 | 5c501ce3 | C | AC1: docs/specs/cem-pilot-kit.md:201,207,213,222 labels all four CEM-PILOT-020..023 requirements proposed. |
| V1-0140 | 71c9b0ba | E | AC1: cmd/corvint/main.go:1427 emits context errors through stderr; script/cem-recipes_test.sh:57,92,100 checks identical envelope shape and empty stdout for both refusals |
| V1-0141 | b4df1a96 | E | AC1: internal/cem/workflow/read.go:316 admits external absolute output paths and names protected-path refusals; internal/cem/workflow/workflow_test.go:1121 covers both |
| V1-0152 | d01f0641 | E | AC1: internal/console/chain.go:622,670,752 shares edge-state logic; internal/console/chain_test.go:271 checks unsupported dispositions and duplicate-ID ambiguity on both panels |
| V1-0153 | 20e9ba4d | E | AC1: internal/console/chain.go:434 adds the PARTIAL cap disclosure; internal/console/chain_test.go:315 renders 65 maps and asserts the 64-map gap row |
| V1-0156 | db6ce45a | E | AC1: cmd/corvint/context_summary.go:237,280 names non-regular entry modes; cmd/corvint/context_summary_test.go:284 exercises a symlink summary handle and its named refusal |
| V1-0158 | cf62fb7b | E | AC1: docs/specs/experimental-source-views-v0.md:134 explicitly identifies 1024 as the parsing floor and explains fixed-member budget refusal |
| V1-0165 | 20b7f46b | C | AC1: tools/retrieval-bench/main.go:386 rejects non-object-ID base_commit with its line number; main_test.go:605 covers plain and ContextBench traversal rows. |
| V1-0166 | cca2eb03 | E | AC1: docs/specs/task-context-packet-v0.md:1117 states non-lexical rows are never blamed and absence proves no agreement; internal/contextindex/blame_test.go:116 checks ownership coverage against row reasons |
| V1-0167 | 7c41267b,d0c821c2 | E | AC1: internal/lspprovider/provider.go:566 emits only method, fixed class and relative origin; provider_test.go:254,281 rejects file:/// and foreign-home leakage and compares reasons across two roots |
| V1-0170 | 328979c2 | E | AC1: internal/contextindex/sufficiency.go:89 states each precedence rule once and matches its implementation and docs/specs/task-context-packet-v0.md:761 |
| V1-0177 | 910630e7 | E | AC1: docs/DOGFOOD.md:131 explicitly lists NOTE unbound-commits count=N, its window and per-commit lines |
| V1-0214 | d8256de1 | E | AC1: internal/contextindex/taskcontext.go:2049 describes camel splitting accurately; AC2: internal/contextindex/termtable_test.go:278,300 proves compound body/path postings and separate Words matching |
| V1-0216 | 0be4e746,421b9b68 | C | AC1–2: contract receipts pin clause extracts instead of the whole DCW spec; TestUCV0ClausePins covers unrelated edits; AC3: main.go:472 detects clause drift, documented in use-case-conformance-v0.md:162 and BUILD-LOG.md:6501. |
| V1-0224 | a44f4dcd | E | AC1: cmd/corvint-mcp/main.go:67 pins both runners; conformance/mcp-2026-07-28/lifecycle_unix_test.go:146,192 tests planted Git; docs/MCP-SERVER.md:179 cites code/tests and recorded passes; AC2: incomplete-pinning alternative unnecessary |
| V1-0227 | bf52befc | C | AC1: internal/dogfoodflow/change.go:813 emits numbered NOT_PRODUCED/ocm-map-not-prepared rows; AC2: script/dogfood-change_test.sh:882 checks the report and refusal. |
| V1-0228 | ecf08427 | E | AC1: internal/dogfoodflow/change.go:409,699 handles failed preparation and decoded paths; docs/DOGFOOD.md:53,329 records wording/retention behavior; script/dogfood-change_test.sh:560,1262 covers regressions; AC2: script/dogfood-bind-range.sh:206 and its test :124 reject stale plans; AC3: internal/observations/observations.go:566 and observations_test.go:905 admit/test both refusals |
| V1-0230 | 838af71a | C | AC1: affected/readers.go:48 selects unbounded readers with explicit witnesses; AC2: resolves at :123 narrows CEM readers, covered by TestChangeEvidenceReadersAreNarrowed_V1_0230. |
| V1-0235 | c139a7ec | B | AC1: accepted decision 0381 §10 defines store promotion versus published prereleases; AC2: .taskman/releases/v0-8.json retains candidate 142d6790, five PASS gate attestations and owner promotion; release readiness exposes those attestations. |
| V1-0244 | 421b9b68 | C | AC1: check-error-code-ownership.sh rejects newly unowned emitted codes; AC2: .github/workflows/ci.yml:40 runs all six requested documentation checks on pull_request without a path filter. |
| V1-0256 | 3897ccbb | E | AC1: internal/gitstatus/status.go:443 classifies split-index; conformance/mcp-2026-07-28/reason_class_test.go:134 creates a real split index and checks the MCP reason-class response; AC2: enum-removal alternative unnecessary |
| V1-0258 | d6628fa6,5cd9419a | E | AC1: docs/decisions/0158-local-excludes-remain-status-policy-2026-09-12.md:32 pins internal/gitstatus/status.go:294; normalized hash 07a84940 matches the info/exclude copy-loop input; docs/build-log/2026-09-26-rc1-doc-fixes.md:24 records a clean citation check |
| V1-0261 | a9676ff8 | C | AC1: internal/dogfoodflow/change.go and check.go use corvint dogfood subverbs in remediation; AC2: cmd/corvint/dogfood_flow_portable_test.go asserts adopter-facing remediation. |
| V1-0262 | 5650d6c1 | C | AC1: docs/DOGFOOD.md:205 uses path impact and :216 explains when range impact applies; AC2: prechange-impact.json remains the agent’s pre-change receipt, consistent with DCW-V0-026. |
| V1-0264 | a92dda35,9ae2fe09 | C | AC1: DCW-V0-025 specifies visible, nonblocking typed abstentions and their scope-limit rationale; AC2: TestDogfoodDailyPathCompletesWhenImpactRefusesTheRepositoryOrModuleRoot covers both codes through change/check/seal. |
| V1-0270 | b63c40e4 | C | AC1: docs/AGENT-ROUTES.md explains indexed-input checks, SHA-256 prefixes of cited lines, and script/check-line-citations.sh --hash. |
| V1-0272 | 6ef8cede | C | AC1: internal/dogfoodflow/change.go:1022 and docs/DOGFOOD.md:42 give repack, alternates removal, commit-graph rebuilding and the adopter rerun. |
| V1-0274 | e0e52bf2 | C | AC1: internal/lrfrepo/ocm_write.go:854 names supported Go case shapes; ocm_selector_test.go:51 covers rejection of map-keyed cases. |
| V1-0275 | e0e52bf2 | C | AC1: docs/specs/ocm-v0-dogfood.md:489 documents normalization, digit removal, singularization, truncation and an exact input/output example. |
| V1-0277 | 0b64c141 | C | AC1: internal/appflows/docs.go:439 skips fenced ranges; TestAFUV1033AnchorParsing covers backtick and tilde fences. |
| V1-0278 | 0b64c141 | C | AC1: internal/appflows/input.go:184 rejects case-folded .git first components with git-path-refused; docs_test.go:159 and confine_unix_test.go:116 cover page, claims and record outputs. |
| V1-0282 | c02c535b | E | AC1: internal/console/views.go:488 prints corvint dogfood change; internal/console/console_test.go:625 asserts the rendered portable hint |
| V1-0287 | 5285b22c | C | AC1: genesis/repository.go:57 derives the budget from inventoryReads plus gitstatus.MaxProcesses and :275 maps exhaustion to git-budget-exceeded; TestInventoryBudgetCoversSparseIndexStatusProbes covers both. |
| V1-0290 | 8f78f605 | C | AC1: affected/readers.go:254 restricts lone unanchored tokens to filenames; TestDirectoryShapedLiteralNamesNoPath covers the directory-token regression. |
| V1-0291 | 5af794ae | C | AC1: affected/graph.go separates reverse test-import edges; select.go:178 traverses ordinary dependencies before adding test users; TestTestOnlyImportSelectsTheTestUserButNotItsImporters verifies propagation stops. |
| V1-0293 | 14dbf5c6 | C | AC1: host_adapter_compaction.go:121 falls back to the prompt packet revision; TestAHI027ClaudeCompactionPinsCleanAndUntrackedOnlyTrees asserts a pin, zero tracked paths and no fault. |
| V1-0294 | fe73b796 | C | AC1: observations.go admits pre/post-compact events and compaction failure codes; TestAdapterDegradationAdmitsCompactionEventsAndCodes checks each event/code combination. |
| V1-0295 | bbce7646 | C | AC1: localcompletion/lifecycle.go:592 exposes another active owner; local_completion_event.go:551 releases with local-policy-other-session-active and the owner key; lifecycle/event regressions cover it. |
| V1-0297 | 597c0407 | E | AC1: cmd/corvint/host_adapter.go:103 passes deadline context and settles abandoned packets as refused; host_adapter_experimental.go:145 prevents late delivery records; host_adapter_test.go:304 covers settlement and late records |
| V1-0298 | 98ddfe53 | E | AC1: cmd/corvint/host_adapter.go:593,628 includes unmet categories and keyed status argv; cmd/corvint/local_completion_event_test.go:345 asserts both without leaking private check IDs |
| V1-0299 | 0739d7f5 | E | AC1: internal/liveverify/affected/graph.go:34,299 hashes an explicit domain-tagged projection; docs/specs/affected-plan-v0.md:88 documents it; affected_test.go:466 verifies derivation and projected fields |
| V1-0302 | 1aeb3762 | E | AC1: internal/contextindex/snapshot.go:45,48,477 enforces ten-minute temporary cleanup, byte budget and engine-first retention; snapshot_test.go:530 covers all three |
| V1-0304 | 4e3f95c6 | E | AC1: docs/PRODUCT.md:5 positions Corvint as a change-evidence verifier; :34 explicitly limits verification to structural integrity rather than semantic support or correctness |
| V1-0306 | 14ee3699 | E | AC1: docs/PRODUCT.md:74 marks work-tracking/E2E adapters not-started integration goals; :262,263 places them in the goals table |
| V1-0307 | cde1c164 | E | AC1: docs/PRODUCT.md:140 names Agent Trace; :149 explains attribution versus evidence and the conceptual attribution-to-CEM mapping |
| V1-0308 | a26f54d1 | C | AC1: companionrelease.go:129 injects the build stamp, Tasks version/--version prints it with regression coverage, Tasks uses the same exported source tree, and generated README text explains both stamped builds. |
| V1-0314 | f02588a6 | C | AC1: affected/dirty.go:203 converts invalid path bytes to display form; TestDirtyNonUTF8PathIsDisclosedNotRefused commits a Latin-1 path and verifies dirty/range disclosure. |
| V1-0316 | e7aff0a5 | C | AC1: dogfoodflow/change.go noteAgentReceipts reports absent and non-base-tree receipts without blocking; TestChangeNotesAbsentOrStaleAgentReceipts verifies both cases. |
| V1-0317 | fc32f8f2 | B | AC1: internal/tasks/boundary_test.go enforces import direction with negative controls; AC2: main.build injection, versionResult and companionrelease.go wire the in-tree binary; AC3: docs/BUILD-LOG.md:6604 records tasks-test PASS and tasks-build printing +build.120; AC4: corvint-1.0-product-and-release-v1.md:88 explicitly says in-tree separate companion binary. |
| V1-0325 | 40bf9e74,598bded0,9851e4be | C | AC1: accepted corvint-tasks-intent-worktree-v0.md governs linked intent writes; TestCTWV0002_TicketCreateFromAFeatureBranch verifies ticket creation while the primary remains on its feature branch. |
| V1-0327 | 210fd8e5 | E | AC1: internal/liveverify/affected/golang/golang.go:483 records the outside-root frontier; workspace_test.go:99 exercises ../sibling and asserts frontier plus UNKNOWN scope |
| V1-0329 | 8e727f3c | C | AC1: tasks/cli/mutate.go:161 qualifies local target IDs, with TestMutationTargetTakesTheLocalIDAndHelpNamesThePayload; AC2: mutationHelp emits each operation’s payloadKeys. |
| V1-0347 | a3df78aa | C | AC1: exported_current_test.go derives Tasks from the single Core export; CORVINT_PROOF_TASKS_ROOT remains only in historical documentation, with no executable consumer. |
| V1-0353 | c94de17a | C | AC1: script/local-console-release-gate resolves its own repository and changes directory before building; AC2: TestLocalConsoleGateBuildsFromRepositoryRoot invokes it from an external temporary directory. |
| V1-0354 | c21e7c19 | C | AC1: dogfoodflow/change.go:1026 prints the allowed syntax in a fix line; AC2: TestDogfoodChangeNamesVerifySyntaxRemediation supplies an anchored ^…$ pattern. |
| V1-0358 | 49f9017e | C | AC1: decision 0423 assigns separate files to new entries without a shared committed index, eliminating the append conflict; AC2: historical BUILD-LOG.md remains and AGENTS.md/docs/AGENT-ROUTES.md point to docs/build-log/. |
| V1-0361 | 5cd9419a | E | AC1: internal/contextindex/snapshot.go:457 shares the cutoff with blob-shard sweeping; blob_shards_open_test.go:117 plants stale/fresh blob and .gitignore temporaries and checks deletion/survival |
| V1-0375 | 4c29b211 | owner | AC: docs/RELEASE-RUNBOOK.md:99 and :125 run the retained-log pipelines under `bash -o pipefail -c ... \| tee "$1"`, so a failing step fails the command (follow-up commits ff6ec6fc, 6e8a68fe quote the log path arguments). |
| V1-0378 | fd115007 | C | AC1: docs/README.md:40 routes current work to Tasks; console/server.go:443 labels historical redirects, names the task-store commands and explicitly denies that empty history means no open tickets. |
| V1-0384 | d167aa45 | C | AC1: workflow/commands.go:605 checks cem/0.3 before publication; AC2: TestSpec03UpgradeNeverReplacesACoreMap covers cite/plain-mark refusals, unchanged sidecars and positive alternate outputs; AC3: CEM-SM-006’s trace row names writeMap. |
| V1-0385 | c09526c0 | E | AC1: docs/specs/documentation-citation-gate-v0.md:133 and script/check-line-citations.sh:54,239 use appended/closed rationale and retain pin enforcement; AC2: script/check-line-citations_test.sh:133 tests the revised message and pin acceptance |
| V1-0395 | 5ded5d02 | E | AC1: internal/observations/observations_test.go:448 uses a five-minute hang detector; docs/build-log/2026-09-27-v1-0395-0396-load-robust-bounds.md:17 records widened helper bounds; AC2: observations_test.go:484 retains verifyConcurrentRows for all 24 writers |
| V1-0396 | 5ded5d02 | E | AC1: cmd/corvint/local_completion_event.go:147,540 recognizes repository-probe-timeout as a time bound; AC2: docs/specs/local-completion-policy-v0.md:164,321 and local_completion_event_test.go:144 cover independent probe expiry |
| V1-0458 | 1b6ac29f | A | AC1: compiled finish/query/change/check/seal regression and its passing run retained in docs/build-log/2026-09-28-trace-ignore-rule-continuity.md; AC2: recorder/read/migration share tracerepopaths.Paths, with byte-preserving migration tests and recovery guidance; AC3: TestTraceGitignoreFinishThenChangeCheckSeal uses one checkout and unchanged task. |
| V1-0512 | 73321199 | D | AC1: read.go checks cleaned/casefolded paths and same-file identity before publication; review_projection_test.go asserts alias refusal and unchanged OCM bytes. AC2: 2026-09-29-six-change-evidence-workflows.md retains focused passes and independent re-review PASS; .corvint/changes/3519356a5a19072170877fd35a35eee6152fa6a5.cem.json retains combined binding, with binding/seal commits ancestral to origin/main. |
| V1-0525 | 7e0950fe | A | AC1: all ten diagnostics remain documented in the four owning specs and match their emitters; AC2: docs/build-log/2026-09-30-mcp-empty-diff-review.md records passing documentation CI at descendant d7749792, whose doc gate includes ownership checks; subsequent retained release evidence records focused-docs passes, with native completion separate. |
| V1-0530 | 27ebec59 | A | AC1: docs/specs/README.md begins the QAT summary with the exact spec/INDEX claim and preserves proposed/experimental limits; AC2: docs/build-log/2026-09-30-mcp-report-preview-parity.md records specindex PASS, and the repair changes no requirement IDs. |
| V1-0586 | cec4b5a7 | D | AC1: documentation-corpus-v1.md:532–537 matches all six http.go refusal conditions/statuses; repair changes documentation only. AC2: origin/main:.taskman/tickets/V1-0586.json retains passing error-code/docs/spec-index checks at 4aad8266, exact PR #418 seal d99b49cf, and explicit GH399 qualification limits. |
| V1-0644 | 5802f353 | B | AC1: TestCALV0061_CheckpointTailEqualsFullAudit counts reads and rejects prefix access; AC2: parity and unusable/unsettled fallback tests compare complete-audit answers and bindings; AC3: CLI checkpoint tests verify unchanged/absent checkpoint bytes, while audit/writers retain full auditing; AC4: 2026-10-01-tasks-read-checkpoint.md retains live-store timings, and CAL-V0-061 states detection limits. |
| V1-0648 | aaebfd0a | D | AC1: CLI ReadVerbsWaitForWriterToApplyReceipt tests both verbs reaching the new head; build-log retains concurrent-writer success. AC2: snapshot/probe.go shares a bounded deadline and reports retryable expiry; wait_test.go covers movement. AC3: timeout test asserts SameTree and reader performs no writes/locks. AC4: maintained CLI/snapshot tests and REQUIREMENTS.tsv trace CTS-V0-006. |
| V1-0754 | 0a3e2db4 | D | AC1: internal/taskman/decode.go validates optional values and rejects unknown/missing keys; issue502_test.go covers admission/refusal. AC2: 2026-10-04-issue502-record-key.md retains the restored exact-count negative control; candidate-3 full-gate evidence covers the repaired tests. AC3: Core and native record.go share taskswire.TicketRecordOptionalKeys. |

## Not claimed

- No test suite was rerun for this closure; the evidence is the retained commits, tests and
  documents cited per row, as read on `origin/main` at `a5aeee41`.
- A verdict is a read-only verification, not a fresh qualification run. A ticket reopened later
  keeps this entry as the record of why it was closed.

## Rollback

Reopen a ticket with `corvint-tasks` (`ticket reopen`) and cite this entry; nothing else depends
on the closures.
