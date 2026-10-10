# Behavior-adapter check mode, guide additions and the legacy-key fork (GH #717)

Date: 2026-10-10

GitHub #717 reported friction with `docs corpus behavior-adapter`. This entry covers three of its
tickets: V1-1084 (gap 3), V1-1086 (gaps 4 and 6) and V1-1085 (gap 5). It adds two requirements to
`docs/specs/documentation-corpus-v1.md`, both proposed and pending owner acceptance:

- `DCP-V1-044`: the check mode.
- `DCP-V1-045`: the guide's anchor placement rule and its migration example.

No decision record is written.

## V1-1084: `--check` (DCP-V1-044)

`corvint docs corpus behavior-adapter --input REQUEST.json [--previous RESULT.json] --check` prints
one `corvint-behavior-adapter-check/1` report. It neither emits nor writes a result or artifact.
Final-stage refusals still need internal reconciliation, so the check runs it, but it skips the
coverage computation.
`doccorpus.CheckBehaviorAdapter` runs the same stages as `BuildBehaviorAdapter`, in the same order:

1. request
2. previous
3. identity
4. revisions
5. the three bounds
6. inputs
7. required inputs
8. mappings
9. observations
10. previous lineage
11. migration
12. discovery
13. flows
14. variations
15. candidates
16. tests
17. declarations
18. observation subjects
19. artifacts
20. delta

Each stage that can still be evaluated runs, and every refusal it finds is kept. Build mode keeps its
first-refusal behavior through `behaviorAdapter.keep`, which only collects while checking.

A stage whose prerequisite refused is reported as `not-evaluated` with `blocked_by`. Two items are
reported the same way inside a stage:

- a mapping that names a refused input;
- a required input that was refused.

The bound check was split into one refusal per bound, so a check can name each violated bound.
Build's bound message stays `behavior adapter bound exceeded` for all three bounds, as on base; the
check report's stage name (`inputs-bound`, `mappings-bound`, `observations-bound`) says which bound
refused.

The invariant is pinned by `TestBehaviorAdapterCheckParity` over 17 cases, from request decode to
observation subject:

- the check is accepted exactly when Build succeeds;
- its first refusal equals Build's error code and message;
- the report bytes repeat;
- the report carries no legacy key.

`TestBehaviorAdapterCheckReportsEveryRefusal` covers one request with refusals in identity, two
inputs and two mappings. Build stops at identity. The check lists all of them, lists
dependent stages as `not-evaluated` in stage order, and still evaluates the independent `discovery`
and `tests` stages.

Review round 1 found three stages that stopped at their first refusal even when later checks did not
depend on it:

- a declaration field refusal skipped the global identity-duplicate check;
- a missing discovery project skipped the duplicate-execution check;
- an observation identity refusal skipped the source-mapping check.

Round 1 made each run every independent check; round 2 below narrowed that to the item rule. A
check that depends on a refusal is listed as `not-evaluated`, for example the execution checks of
an undecodable discovery record.
`TestBehaviorAdapterCheckIndependentRefusals` covered each pair and asserts that Build's first refusal
is unchanged. `TestBehaviorAdapterCheckListsDependentChecks` covers the dependent items. Both tests
fail on commit `d2d62848` and pass after.

The CLI exits:

- 0 when the check is accepted;
- 1 when it refuses, with the report still on stdout;
- 2 for unreadable input, `--check=VALUE`, or `--check` on another corpus operation.

`--check` was added to the help boolean flags.

Review round 2 found four more places where one item still hid or over-reported checks. Chasing
every independent check inside one item kept failing review, so `DCP-V1-044` was narrowed to an
item rule:

- Items are inputs, mappings, observations, mapped records and discovery executions. Each item is
  evaluated independently of the other items.
- Within one item, evaluation stops at its first refusal. Exactly one `not-evaluated` entry then
  names the item: `remaining checks for <item> not evaluated after <refusal>`. The entry is emitted
  on every item refusal, even when the refused check was the item's last one, so it never claims
  that nothing was skipped.
- An item whose dependency was refused is listed as `not-evaluated`, as before.

Under that rule:

- An input stops at its first refusal and gets the item entry.
- A mapping is its own item. Its identity, field bound, supported fields and required fields are
  validated even when its input was refused. Only the record checks that need the decoded input are
  listed as `not-evaluated`.
- A receipt observation stops at its first refusal and gets the item entry. So does an observation
  in the observations and observation-subjects stages, and each mapped record and declaration.
- The round-1 bound split had changed Build's mapping-count message. It is restored to the base
  bytes, `behavior adapter bound exceeded`, for all three bounds.

`TestBehaviorAdapterCheckStopsAtFirstItemRefusal` covers an input, an observation and a receipt.
`TestBehaviorAdapterCheckReportsEveryRefusal` now also asserts that the mapping of a refused input
is validated as its own item. `TestBehaviorAdapterBuildBoundMessage` pins Build's message for input,
mapping and observation overflow to the literal base string. The round-1 observation case moved to
the new test because it now expects one refusal plus the item entry. These tests fail on commit
`12b87f4d` and pass after.

Review round 3 found two more gaps:

- **Unbounded report.** After `mappings-bound` refused, each mapping was still evaluated. A
  30 KB request of 10,000 empty mappings produced about 5.2 MB of entries, over the 4 MiB encoder
  bound, so the CLI exited 2 with no report. Now, when a count bound (inputs, mappings or
  observations) refuses, the items it bounds are not evaluated, and one `not-evaluated` entry on the
  bound stage says so. The mappings stage now also needs `mappings-bound`. The report also keeps at
  most 1024 entries and about 1 MiB of encoded entries, always including the first, and ends with
  `further entries omitted after <N>` when it drops any. A readable refused request therefore always
  gets a bounded report and exit 1.
- **Identity refusals without an item entry.** Discovery-execution, candidate and test identity
  uniqueness used `keep`, so an empty identity gave a refusal but no item entry. Every
  identity-uniqueness check (discovery executions, observations, flows, variations, candidates,
  tests and the declarations cross-kind check) now goes through `keepIdentity`. It charges the
  refusal to the first item, not already refused, whose identity is invalid or repeats an earlier
  one, refused or not. The item is named by identity, or by position when the identity is invalid
  (`candidates record 0`). Because already-refused items are skipped, an observation with an invalid
  identity no longer also gets a duplicate-identity refusal. Build still returns the same error at
  the same point.

`TestBehaviorAdapterCheckBoundsReport` covers the 10,000-mapping request and the entry cap (4096
invalid observations give 1024 entries plus the omission entry). The CLI check subtest asserts exit
1 and a report under 64 KiB for the same request. `TestBehaviorAdapterCheckStopsAtFirstItemRefusal`
gains empty discovery-execution, candidate and test identities, a duplicate candidate and a duplicate
observation. These fail on commit `62187812` and pass after.

Review round 4 found one more gap. The first refusal was exempt from both report bounds, and it
echoes request strings without a bound. A 4,194,304-byte request whose single input has an anchor
revision of 4,193,511 `x` bytes gave a first refusal of about 4.19 MB. The report then exceeded the
encoder bound, and the CLI exited 2 with no report. Now every report entry's message, including the
first, is capped at 4 KiB. The cut falls on a UTF-8 boundary and is followed by
` … [truncated <N> bytes]`. Build's own error bytes are unchanged. The parity rule is now: the
first entry equals Build's refusal truncated by the same cap.

Messages that echo request-supplied strings, all covered by the cap:

- `fieldError` echoes the input identity, the field pointer, and the anchor revision and digest.
  This is the reviewed path, reached from the input anchor check.
- The mapping identity refusal echoes the mapping's input and record pointer.
- Field-mapping refusals echo the mapping kind and field name. Field pointers echo the mapped field
  pointer values.
- The invalid input document refusal echoes the input identity.
- Check-only `remaining checks for <item> ...` entries echo item identities and the item's refusal.

`TestBehaviorAdapterCheckCapsMessages` builds the reviewed request at exactly `MaxBytes`. It
asserts that Build's error is unchanged and over 4 KiB, that entry 0 equals the capped Build message
and ends with the marker, and that the report encodes within 64 KiB. It also checks that a multibyte
rune at the cut is not split. The CLI check subtest asserts exit 1, empty stderr and a report under
64 KiB for the same request. On commit `a597b55f`, the CLI subtest exits 2 with `output bound
exceeded`. A behavior-only probe of the same request also fails there: the first refusal is
4,193,265 bytes and `Encode` refuses. Both pass after.

Limits:

- The report does not list every independent refusal within one item. Repairing an item's first
  refusal can reveal another one in the same item.
- A repaired refusal can reveal stages that were not evaluated before. The report's limitations
  say so.

## V1-1086: guide additions (DCP-V1-045)

`docs/DOCUMENTATION-CORPUS.md` now documents four things:

- **Check mode.** How `--check` works.
- **Input anchor placement.** Every input anchor names the provider repository (`source`, which
  equals the provider entry of `revisions`), and the guide gives the correction text of the refusal.
- **Minimal migration record.** A minimal schema-2 migration identity record.
- **Self-reference pitfall.** The record describes an earlier provider commit and is committed and
  anchored in a later one.

The example writes the legacy key as the placeholder `PROVIDER_MEMBER`. It points to the
declaration on `BehaviorRevisions.E2E` and does not spell the key. `TestBehaviorAdapterGuideMigrationExample`
checks the guide's example end to end:

- It extracts the marked block and substitutes the declared member through reflection.
- It closed-decodes the block into `BehaviorMigration`.
- It builds an accepted request from the block, with every input anchored at a later provider commit.
- It confirms that the same inputs anchored in the application or documentation repository are
  refused with the documented correction.

## V1-1085: not changed, and why (fork for the owner)

The ticket asks the adapter to emit a generic role name for the end-to-end member of
`BehaviorRevisions` and to keep the legacy key only as an input alias. That conflicts with proposed
`AFU-V1-054..056` (V1-0985), which already govern this exact type:

- The `/1` behavior provider and every `/1`-family wire sharing its `revisions` shape keep the legacy
  /1 member. The adapter request, the adapter result, the migration, discovery and runtime records
  and the corpus manifest are all such wires.
- `/1` encoding stays byte-identical.
- `/1` gains no alias, and the closed decoder refuses a neutral spelling.

There are two consequences:

- Changing the emitted name would change every `/1` contract digest, because the migration
  `revisions` participate in `contract_sha256`. Prior adapter results would then fail lineage.
- `TestAFUV1BehaviorProviderV1BytesUnchanged` pins the `/1` bytes.

DCP-V1-032 already exempts "the legacy compatibility member" from its vocabulary rule as attributed
input. The neutral path is `/2`, whose `source` and `repositories` carry the repository without the
legacy key (AFU-V1-055).

No code changed for V1-1085. The owner can choose one of two paths:

- amend the AFU-V1-054/056 proposal to admit a `/1` input alias, accepting the digest break;
- add a neutral adapter request/result version that maps onto the `/2` provider.

Vocabulary scan of `internal/doccorpus` JSON tags: the legacy key is the only adopter-specific
emitted key. Other keys, such as `app`, `docs_corpus`, `missing_e2e_review` and
`playwright_workers_per_node`, are generic role or tool names. The new check report has no
adopter-specific key, and `TestBehaviorAdapterCheckParity` asserts this.

## Evidence

Base: `684cca5f7c0e3e4b4900e07203f63949bc73984f`.

Failing on base, run from a `git archive` of the base with the new tests copied in:

- `TestBehaviorAdapterGuideMigrationExample`: "guide lacks the marked DCP-V1-045 migration example".
- `TestBehaviorAdapterCheck*` and the `TestBehaviorAdapterCLI` check subtest: build failure, because
  `CheckBehaviorAdapter` and its report types are undefined. On base, the CLI rejects `--check` as an
  "unsupported corpus option".

Passing after:

- `go test -timeout 30m ./internal/doccorpus/...` passes, including the new tests of every review
  round.
- `go test -run 'TestBehaviorAdapterCLI|Help' ./cmd/corvint` passes.
- `go vet` passes for both packages.
- These doc gates pass: `spec-requirements-check`, `requirement-definitions-check`,
  `traceability-tests-check`, `decision-numbers-check`, `line-citations-check`,
  `unbounded-readers-check` and `use-case-receipts-check`.

`corvint affected` (1.0.0-rc.3, build 407) reported scope `UNKNOWN` and advised the full gate. The owner
waived that gate for this scoped lane, so it is `NOT_RUN`.
