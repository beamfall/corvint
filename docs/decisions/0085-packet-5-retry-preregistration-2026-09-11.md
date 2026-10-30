# Decision 0085 — Packet 5 retry preregistration under `P5R-V0-004`, and the checklist row that can bind it

Date: 2026-09-11. Status: accepted. Authority: repository owner, verbatim instruction "fix the
packet-5 row and do the retry preregistration" (2026-09-11), applied as a delegate call under the
standing 2026-09-04 instruction "I want you to make the calls for me" (decision 0049 records the same
authority). The owner may reverse it by deleting the retry manifest and restoring the previous
`script/release-checklist` packet-5 row.

## What is decided

1. **The retry preregistration is `conformance/perf-v0/packet-5-retry-manifest.json`.** It is a
   scoped `corvint-perf-v0/2` manifest whose claim scope stays decision 0016 (id
   `packet-5-current-pin-requalification-retry-2026-09-11`), registered at `2026-09-11T13:16:02Z`, with
   the six 2026-08-31 harness rows verbatim (ids, argv, stdin) plus their six snapshot-present
   mirrors, `<id>-snapshot-present` with the same class, argv, and stdin, which decision 0084 item 3
   names and binds for GPK-V0-017(a)'s index-building events and (b): twelve rows in total. The
   three Beamfall mirrors are copied from the committed `/1` manifest; the three Corvint mirrors are
   new and run on a new corpus, `corvint-snapshot-present`, the Corvint pin with setup exactly
   `[["index"]]`. Pins: Corvint `d4f69c2feb29c99ba75e7367d585b9dc0f742991` (tree `2867494ce348a902834791ee67e83c151f94f874`) for both Corvint corpora, Beamfall
   `fa3b1e7fe5bc6c10e4b09b2729f364780f567a48` for both Beamfall corpora. Five discarded warmups, 100
   measured samples per runtime and row, alternating interleave, and the 120-second per-sample
   timeout decision 0042 set. The raw manifest SHA-256 is `4c34a4c9716276524d70a084e3817c5e8a6e7752e3a020f34aa74416460a72b6`, admitted by name in
   `conformance/perf-v0/manifest.go`; any edit to the file un-admits it.
2. **Each `P5R-V0-004` precondition is met as follows.** New owner ratification: this record under
   the owner's instruction above. Harness-level discriminator: decision 0044's `diagnose`
   reproduction of the DR-0009 shape member by member for the three Beamfall invocations the first
   run invalidated. Source-closed exact grant: the DR-0004/DR-0009/DR-0021 entries of
   `ratifiedAcceptedDivergences` in `conformance/perf-v0/manifest.go`, matched by task id and exact
   argv (decisions 0007, 0040, 0041, 0044). This record ratifies four further entries, dated
   2026-09-11, so that the new Corvint mirrors carry exactly the grants of the rows they mirror:
   `DR-0021` for `harness-user-prompt-snapshot-present`, and `DR-0004`, `DR-0009`, `DR-0021` for
   `harness-file-change-snapshot-present`, each pinned to the same argv as the cold row. The same
   shape (repository-local stdout that the oracle and candidate order or word differently) is
   already established for those argv on the cold corpus; a snapshot present does not change the
   shape, and a divergence outside those keys still invalidates the row. The retry rows declare no
   other keys. New prospective preregistration: item 1, registered before any sample
   process starts. New Corvint pin: `d4f69c2feb29c99ba75e7367d585b9dc0f742991`, the commit that repaired the checklist row.
3. **The runner accepts scoped `/2` manifests again.** `readManifest` admits a `/2` manifest only
   when its raw SHA-256 is in `admittedScopedManifests`; `run` refuses `--oracle`, `--samples`, and
   `--candidate-revision` for it; the receipt is `corvint-perf-report-v0/2`, carries
   `manifestSha256`, `claimScope`, and `slicePerformanceStatus` (`NOT_RUN` unless every row is valid
   and every threshold criterion is `PASS` or `FAIL`; `FAIL` if any is `FAIL`; else `PASS`), and its
   global `outcome` stays `insufficient_evidence` (`P5R-V0-003`). The 2026-08-31 manifest
   (`3705ff2ced308c350b94c4a25457378d107aec22532d27fe066ccde7a5a3dfe1`) and its `NOT_RUN` result
   stay untouched and are not re-admitted: they are immutable evidence, not a runnable
   registration.
4. **The runner implements decisions 0048 item 1 and 0084.** An index-building non-query event
   (`file-change`, or `session-start` with `startSource` `compact`) measured on a corpus with an
   index setup is held to the 250 ms portable cap only; the same event on a cold corpus, and a
   query event on a cold corpus, receive the verdict `REPORTED`: the p95 is in the receipt beside
   the binding row, and the row is neither a threshold nor a `NOT_RUN`. `REPORTED` rows are
   excluded from `slicePerformanceStatus` and from the global outcome; every other row is
   unchanged. A `/1` run of the committed `manifest.json` reports its cold-corpus harness and query
   rows the same way.
5. **The checklist packet-5 row binds by source identity.** The candidate is built from the
   manifest's committed Corvint pin, so a committed formal result can never name the commit that
   contains it. The row now reports on a result whose `builtFromRevision` is `HEAD` or an ancestor
   of `HEAD` with no change since it to `go.mod`, `go.sum`, or any main-module package directory in
   the `./cmd/corvint` dependency closure (`go list -deps`, offline). A non-ancestor stays
   `NOT_RUN` "not bound to HEAD"; a changed closure is `NOT_RUN` "candidate build inputs changed
   since the formal result"; a closure the script cannot enumerate is `NOT_RUN`.
   `ARTIFACT-RDY-V0-002` is amended in the same change.
6. **When the retry result lands.** The formal run is
   `GOTOOLCHAIN=local go run ./conformance/perf-v0 run --manifest conformance/perf-v0/packet-5-retry-manifest.json --out conformance/perf-v0/results/packet-5-current-pin-requalification-retry-2026-09-11-formal`
   from a clean worktree at or after `d4f69c2feb29c99ba75e7367d585b9dc0f742991`, once, with no override. Decision 0037 item 1's
   report-path line is then amended by a dated edit to name the retry result, because
   `script/release-checklist` reads the path from that record; the 2026-08-31 result stays where it
   is. Expected outcome, stated before the run: the 2026-09-03 `/1` run measured snapshot-present
   compact `session-start` at 955 ms p95 against the 250 ms cap and snapshot-present `user-prompt`
   at 193 ms against 500 ms, so unless the compact path has improved the retry is expected to report
   `slicePerformanceStatus: FAIL`. A `FAIL` is a valid terminal result; it does not authorize a
   wider cap or a further retry without a new record.

## What this record does not do

It does not run the protocol, tag, publish, or promote. It does not change the 2026-08-31 manifest
or result. It does not make newly executed `/1` runs fail closed (`P5R-V0-003`, last sentence): a
`/1` receipt keeps the outcome semantics `conformance/perf-v0/README.md` documents, and the release
checklist cannot consume one because the packet-5 row requires a `/2` receipt's `decision` and
`slicePerformanceStatus`; closing that gap is filed in `docs/agent-memory/fixes.md`.

Rollback: delete `conformance/perf-v0/packet-5-retry-manifest.json`, remove its entry from
`admittedScopedManifests` and the four 2026-09-11 grants from `ratifiedAcceptedDivergences`, restore
the previous packet-5 row of `script/release-checklist` and the previous `ARTIFACT-RDY-V0-002` text,
regenerate `docs/specs/REQUIREMENTS.tsv`, and revert the `REPORTED` verdict in
`conformance/perf-v0/runner.go`.
