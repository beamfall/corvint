# Issue 389 seeded successor: immutable delta intent seed

Human-owned intent: GitHub issue 389, native V1-0536. The owner approved the seeded-successor
decision packet `OWNER-DECISION.proposed.md` (SHA-256
211f112fb92b232e5cba02fb1043aff25e3fcc51926c277532f7b439d95433cd) on 2026-10-04. This entry
records that decision and the seed's provenance once; it is not a delivery or qualification claim.

The original #389 integration enrollment stays a NON_SUCCESS: original BASE
79bd635d91f0f3944c781882ae6f6d4bb968d58b, session key
61668a08d33c7865786bc2f8fb8cf982997d7072c78fae7ad8a81d9f92c261cd, plan
94aa3939437eed96da6955440485e861e8fe7489a9804b937ddef7742d026fea and its ten checks. Its last
report set was produced at 469969beaed20f72228525bead0b57ab9766ee85 and the retained candidate is
4be1a767a5390f6fd2bb14f08c7d4dd2a877689b. Its live evaluation is satisfied=false. The explicit
`dogfood cancel` belongs to its own checkout and is not recorded by this entry. Those are
historical evidence only. The successor neither resets nor requalifies that enrollment, and it does
not revoke any genuine historical check.

The seed adopts the reviewed `immutable-delta-v0.md` intent from 79bd onto public main
4b10a02144faed4a77b36eba4b0d1cbc315706d3. The spec bytes at 79bd and 4be are identical (SHA-256
16f2646987bb7d0bf0dfd87bd0874d434fc5d785a19f6bc7b15e6d86698afb9b). The seed adds four paths: the
spec, its `INDEX.json` record, its README row and its ten generated `REQUIREMENTS.tsv` rows. The
record and the row are the exact 79bd bytes, inserted at their 79bd neighbours, and every other
catalog entry keeps its main bytes. Intent status stays proposed and delivery stays experimental.
No numbered requirement and no original human acceptance criterion changed.

The only difference from the 79bd spec is a `(PLANNED)` marker after each of the 30 named
traceability tests that main does not define yet. The `## Requirements` block and the line count
are byte-identical. Each marker comes off only when the test it names arrives with the merged
implementation; no stand-in tests are added. Statements in the spec that some source exists
describe the retained 79bd/4be branch, not delivery on current main.

At the seed, source, tests and qualification of the delta capability are NOT_PRODUCED. The seed is
bound under its own documentation enrollment, which uses the bootstrap profile for the new spec
and checks only the documentation. Its checked seal becomes the successor's evidence BASE. The
successor enrolls a new key and reruns all ten original checks, unchanged and fresh. The original
four-class human acceptance is still PARTIAL, and the native docs-only class stays open under
V1-0579. This decision completes no ticket.

Rollback: revert the seed and seal commits. Nothing else depends on them until the successor merge.

## Successor enrollment

The seed's documentation enrollment (key
3e09dda7295876cf702c1349358155b6f95805d3986bb42c2aa54d2c89ee6fbe) finished satisfied at its bound
target 541b6462468b5cf0d5db23d3aa070e980b24ac73, and `dogfood-check` and `dogfood-seal` passed.
The seal c58325c4b128d69828640952b421d660a5aebc7b made that key stale, so it was cancelled as the
decision requires. Its pre-seal result stands and is not a failed seed check.

The successor key 02e7d9fcf8283d9beb6669260414a80502d96a43f7bf7412c5619a374b815c81
was enrolled at c58325c4 with plan digest
3ee04d546ffbf10134bab4fceb340e098140424e99826a7450c175c86f6c1849: the three original intents and
all ten original checks, unchanged. The retained candidate 4be1a767 was merged ordinarily. The three
catalogs merged to the seed bytes, the DLT spec took the candidate bytes so its `(PLANNED)`
markers came off with the arriving tests, and main's archives 0ab7b241 and 1722ce93 keep their
BASE bytes. The candidate's historical shared CEM (SHA-256
01f1d27210f86ab1a816dbeae7fd2b5e8ea5dda10c985c5f17f4ee3d6d4abd76) is not carried, and a fresh map
is bound against c58325c4. The candidate's `2026-10-04-immutable-delta-current-main-preservation.md`
entry describes the candidate branch's own archive disposition, not this successor's.

## Native-replay root decision

Under key 02e7d9fc, at target 05075d048d04e80d462717275cb847c37116b944, nine of the ten checks
qualified. `native-replay` failed with `source is dirty`. Its frozen `--root` named the retained
Codex checkout, which is pinned at 4be1a767 with a modified shared CEM. A clean run there would
still have built 4be1a767, not the successor. That failure stands as a disclosed NON_SUCCESS of key
02e7d9fc, which was then cancelled with its observations retained. The original key 61668a08 and
its checkout are untouched.

On 2026-10-04 the owner decided: "New plan, root = successor". Plan digest
fd38b4891deb8a3d1b060c1633e4054d1ca57e8160eba0356b94a5da2bf9c2b1 under key
8f0a348bceb30a78beca153bde5c0ee42a2a0bb52d763e9a0be8c4f38c02d977 keeps the base, the intents and
every check id, timeout and reuse flag. Only these argv values change:

- `native-replay --root` names the successor checkout, so the replay builds the bound target.
- The runner copy keeps the frozen runner bytes except one constant: `native-replay.py` moves its
  Go build cache from the shared batch directory into the private copy. Its SHA-256 changes from
  3365c325d3cda59f3ab78b49f1c2df96f2cbabb60a29c639cd85d922c6b67f3a to
  933b18a896260174625646b54bec6affce48836b9e2e416b2a6c0e17b47dcee6. `go-test-check.py` is
  byte-identical. The manifest SHA-256 changes from
  4edbaf7a489cccb002c204ba6fbbda9152a01b5844f8f9e33a8af28f60e430d3 to
  91dacf237f46fba839301e7788115179c94ccdd255964e60cf3344fad317e3dd. The Python runtime, Go binary
  and distribution pins are unchanged. `--out-parent` and the `GOCACHE=` arguments follow the copy.

The focused documentation and format checks still run `make` targets whose recipe-level Go cache
lives under `/tmp`; that is repository behaviour and is not changed here.

## Review findings after bc645212

An independent review of bc645212d5af1229dfa93d14183ef3d7456af00e returned PASS-WITH-FINDINGS,
including one HIGH finding. The fixes are new commits on top of bc645212. Nothing was amended.

### HIGH: Git operation budget

The default `gitrun` budget allows 1,024 operations. `revisionFS` spent one `ls-tree` per path
lookup plus one per blob read, so every entry cost about two operations. On this repository,
`BuildFS` failed with `git-budget-exceeded` at entry 4,947, against 6,005 files and 816
directories. As a result, every delta on a repository past roughly 500 entries reported
`immutable-graph-unavailable` and selected 0 tests.

The fix has two parts:

- **Cheaper lookups.** `revisionFS` resolves each path through its parent directory's cached,
  verified listing. A listing costs one operation per directory, and each blob read costs one
  operation. `Stat` reuses the blob size cached by object ID.
- **A declared delta budget.** `delta.GitOperations` is 1,024 + 2 × `affected.MaxWalkEntries`,
  which is 801,024 operations within the same 30-minute wall budget. This is enough for the
  largest walk the graph accepts. Running out is still fail-closed: the result is
  `immutable-graph-unavailable` with the full suite required.

Both bounds are recorded under "Bounds and failures" in `immutable-delta-v0.md`.

Each part has a test that fails without the fix:

- `TestRevisionFSListingCostScalesWithDirectories_DLT_V0_003` uses 600 entries, each stat'ed
  twice, under the default budget.
- `TestDeltaGraphAboveDefaultGitBudget` uses 1,200 Go files.

Evidence on this repository, delta from c58325c4 to bc645212:

| | Before | After |
|---|---|---|
| Graph | `immutable-graph-unavailable` | builds (about 65 s) |
| Tests selected | 0 | 867 |
| Affected units | 0 | 156 |

`runFullSuite` stays true in both runs, because the remaining unknowns still require the full
suite. Those unknowns are `documentation-baseline-unavailable`, `provider-coverage-missing`,
`affected-scope-incomplete` and `unit-assertion-witness-unavailable`.

### MED: CLI error codes

The CLI mapped every compile error to `delta-unavailable`, a code outside the closed error table.
Compile errors now carry a typed `delta.Refusal`, and the CLI prints that code unchanged. An
untyped error prints `delta-record-invalid`, because no record can be emitted. The new test
`TestDeltaCLIPreservesClosedRefusalCodes` covers five codes:

- `delta-invalid-arguments`
- `delta-repository-unavailable`
- `delta-head-unavailable`
- `delta-change-set-unavailable`
- `delta-unrepresentable-path`

### MED: license marker

`conformance/delta-v0/README.md` declared AGPL-3.0-or-later. `LICENSING.md` assigns
`conformance/**` to Apache-2.0, so the marker now reads Apache-2.0.

### LOW findings

- **Schema validation.** `TestDeltaRecordsMatchPublishedSchema` validates the decision vectors and
  two compiled records against `protocol/delta/schema.json`. Its validator refuses any keyword it
  does not implement, instead of skipping it.
- **Shared reach step.** `ReachedUnitIDs` and `Select` now share one `reach` step, so declared
  read-scope readers are reached by both. `TestReachedUnitIDsIncludeDeclaredReadScopeReaders`
  fails without this change.
- **Documented limits.** `O_NOFOLLOW` guards only the final path component. Documentation impact
  costs at most 32 scopes × 4 builds = 128 context builds.

### Known test limit

`TestSelectionOnTheLiveDirtyWorktree` reads the caller's live dirty worktree. With only an
uncommitted `.md` edit under `conformance/`, it fails on unmodified bc645212 as well, with an
`ENCLOSING_PACKAGE` witness that has no owner. The failure is therefore an artefact of a dirty
worktree, not of this change. The test is run on clean commits.
