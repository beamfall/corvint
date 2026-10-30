# Corvint Witness Report V0

Owner: Russell Lewis
Date: 2026-09-12
Intent status: accepted (decisions 0163 and 0290)
Delivery status: experimental (checked by `TestCLIReadVerbsLeaveTheRepositoryByteIdentical`)

## Agent digest
- Claim: corvint witness reports the unwitnessed surface of one committed change range and writes no repository, index, or trace state.
- Status: accepted (decisions 0163 and 0290)/experimental (checked by `TestCLIReadVerbsLeaveTheRepositoryByteIdentical`)
- Exists: `cmd/corvint/witness.go` and `internal/witness` (`Profile = "corvint-witness/0"`).
- Blocked on: none stated for the read-only clause; the report's closure-authority table stays owned by CF-V0-012/014/031, not restated here.
- Read next: Scope; Requirements.

## Scope

`corvint witness` is a standalone report command, not a migrated parity verb and not the
`ocm-change-witnessed-v0` evaluator. `cmd/corvint/witness.go`'s `parseWitnessInvocation` intercepts
the `witness` verb ahead of the shared argument parser specifically so it adds no verb to the
parity-compared top-level command vocabulary `GPK-V0-007` enumerates. `internal/witness`
(`Profile = "corvint-witness/0"`) is its own package: it composes `internal/contextindex` and
`internal/cem/{cemcode,gitrun,wire}` to count closed, unproven, and not-run obligations over one
committed range, citing `CF-V0-012`, `CF-V0-014`, and `CF-V0-031` for its non-closing authority
table. It is not `internal/changewitness`, the `ocm-change-witnessed-v0` evaluator
`docs/specs/change-witness-relation-v0.md` owns; that spec's own Traceability section already states
no Frontier profile, verifier, or command consumes it, and that statement does not describe this
report, which is a distinct package with its own CLI verb.

## Requirements

### Repository read-only guarantee

- `AGW-V0-001`: `corvint witness` MUST be read-only, per AGENTS.md invariant 4: on every path — a
  rendered or `--json` report, an `invalid-arguments`/`output-failed` `gokernel.Error`, or a refused
  invocation before the index builds (an unknown option is named before any value is consumed, and
  an explicit `--root` that is not a Git repository is refused as `not a Git repository: PATH`,
  decision 0205) — it writes no tracked or untracked repository content,
  `.git`, or `.corvint` state, leaving the repository byte-for-byte unchanged. `runWitness` only
  reads the committed tree's index snapshot or, on a miss, builds `internal/contextindex` from the
  Git object store (`IDX-SNAP-V0-020`, a read that writes nothing), and calls `witness.Compile`/
  `witness.Render`; no call in that composition creates, prunes, or mutates persisted state.

- `AGW-V0-002`: Committed-range ancestry MUST follow immutable commit parents. The bounded Git
  runner MUST disable replacement objects and pin `GIT_GRAFT_FILE` to the platform null device,
  ignoring both repository `info/grafts` and ambient graft paths without changing their bytes. A
  graft MUST NOT invent an admitted ancestor or hide a real available ancestor. Graft deprecation
  advice is disabled explicitly; operation/output limits and read-only behavior remain unchanged.

### Packet cost

- `AGW-V0-003`: (proposed 2026-09-23, V1-0200, not accepted) the report MUST carry
  `packetCoverage`, one entry per context packet `Compile` compiled, in compile order: the
  `admission` packet from `contextindex.RangeImpact` (`contextindex.PerPathRangeImpact` since
  AGW-V0-004), then one `closure` packet per admitted path
  from `contextindex.Impact` (naming that `path`). An entry with `status` `PRODUCED` copies the
  packet's own `coverage.packet_bytes`, `budget_bytes`, `within_budget`, `included_results` and
  `omitted_results` under those names. When the coverage block is absent or any of those values has
  the wrong type, the entry has `status` `NOT_PRODUCED`, `reason` `packet-coverage-unreadable` and
  no numbers, never zeros. A refused stage compiled no packet and contributes no entry, so the list
  is empty, never absent, when nothing was compiled. A closure packet's `omitted_results` does not
  reflect truncation at the ranking ceiling, because the engine truncates before it counts; that
  truncation is reported by `range.closure` `TRUNCATED`, its `closureDetail` and the
  `WITNESS-P6-INCOMPLETE-REVERSE-DEPENDENCY-CLOSURE` precondition, and the text rendering marks such a packet
  `at-ranking-ceiling`. The text rendering adds a `PACKETS` section with the same numbers. The field
  is additive: `corvint-witness/0` keeps its profile and every existing member.

### Per-path admission, revisions and exit status

- `AGW-V0-004`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2796, GitHub
  #1893) Admission MUST use `contextindex.PerPathRangeImpact` (ERI-V0-264). A changed path the
  engine refuses on its own (an unsafe name, a deletion, binary content, a mode change, a Go
  package directory containing `@`, and the other ERI-V0-264 rules) MUST become its own `NOT_RUN`
  obligation with reason `ADMISSION_REFUSED_<RULE>`, while the rest of the range is admitted and
  closed over. `range.admission` is then `PARTIAL`, its detail counts the refused paths and names
  up to eight as `path (RULE): detail`, and the precondition
  `WITNESS-P7-PATHS-REFUSED-PER-PATH` repeats it. A repository-level refusal keeps `REFUSED`,
  reason `ADMISSION_REFUSED` on every changed path and `WITNESS-P4`; a malformed `refusedPaths`
  row refuses the whole range. No path is dropped: a path neither admitted nor refused stays
  `NOT_RUN` with its coverage reason. Failure mode: a consumer that treated every non-`COMPUTED`
  admission as a whole-range refusal now sees `PARTIAL`.
- `AGW-V0-005`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2797, GitHub
  #1894) `--base` and `--head` MUST accept any revision Git resolves to a commit (a branch, tag,
  `HEAD~N`, or a full or abbreviated object ID), resolved read-only by
  `git rev-parse --verify --quiet --end-of-options REV^{commit}` under the bounded, graft-isolated
  runner, and the report MUST carry the full commit object ID. An empty or option-like revision, a
  revision containing NUL or a newline, a revision that names no commit, and output that is not one
  full hex object ID refuse with exit 2 (`--base: ...` / `--head: ...`). A `--head` other than the
  checked-out HEAD commit is compiled under AGW-V0-007 (amended 2026-10-27; it previously refused).
- `AGW-V0-006`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2797, GitHub
  #1894) Without `--check` a printed report exits 0 whatever it says. With the valueless
  `--check`, the report is printed unchanged and the exit MUST be 1 when admission is not
  `COMPUTED` or any opened obligation is not closed; refused input still exits 2 with no stdout.
  Because no V0 authority closes, `--check` exits 1 for every range with an obligation; that is the
  intended honest answer, not a defect. `--check=admitted` and `--check=candidate` gate on analysis instead (AGW-V0-034). Failure mode: a script that used exit 0 as success must add
  `--check` to gate on the report.

- `AGW-V0-034`: (experimental under the standing lane auto-accept, 2026-10-28, V1-2922, GitHub
  #1928) `--check=admitted` and `--check=candidate` MUST gate on analysis instead of closure, so a
  CI step can tell "analysed with candidate witnesses" from "nothing analysed" while no V0
  authority closes. `--check=admitted` exits 1 when admission is not `COMPUTED`, closure is neither `COMPUTED` nor
  `EMPTY` (no admitted path; a `TRUNCATED` closure fails), or `summary.notRun` > 0; `--check=candidate` also exits 1 when `summary.unwitnessed` > 0. With
  `--range`, every range must pass the analysis half and the combined summary the obligation half.
  The report bytes are those of the same invocation without `--check`. Neither mode closes,
  proves or ranks an obligation: a candidate stays `CALLER_REPORTED` and non-closing (CF-V0-014).
  The valueless `--check` keeps AGW-V0-006. Fails closed: any other `--check=` value, or two
  different modes, exits 2. Failure mode: a gate on `--check=candidate` admits an obligation whose
  only candidate is a passing test that never executes the changed line; closure is still
  `--check`. Rollback: drop the two values.
- `AGW-V0-035`: (experimental under the standing lane auto-accept, 2026-10-28, V1-2922, GitHub
  #1928) A range over the 100-path, hunk or target-line bound MUST degrade per path through
  ERI-V0-285 instead of refusing whole: each refused path is a `NOT_RUN` obligation with reason
  `ADMISSION_REFUSED_PATH_BOUND`, `ADMISSION_REFUSED_HUNK_BOUND` or
  `ADMISSION_REFUSED_TARGET_LINE_BOUND`, admission is `PARTIAL` under AGW-V0-004, and the rest of
  the range is analysed. The bound counts engine records, so the source of a rename past the bound
  keeps its own `NOT_RUN` coverage reason rather than `ADMISSION_REFUSED_PATH_BOUND`. A range of
  more than 1000 changed paths stays `REFUSED`. Non-goal: a
  `--range-profile` option on `witness`. Failure mode: a reader that took every over-bound range
  to be `REFUSED` now receives a `PARTIAL` report. Rollback: restore the whole-range refusal in
  ERI-V0-285.
- `AGW-V0-038`: (experimental under the standing lane auto-accept, 2026-10-28, V1-2959, GitHub
  #1945) The admission packet holds at most 50 ranked results (`impactLimit`, the contextindex
  `maxLimit`). Each changed path the engine admitted but ranked out of that packet (its receipt's
  `coverage.critical_missing`) MUST become a `NOT_RUN` obligation with reason
  `ADMISSION_PACKET_BOUND`, and `range.admission` MUST then be `PARTIAL`, its `admissionDetail`
  naming the 50-result bound and every such path (at most 100, because the path bound refuses the
  rest), joined after any per-path refusal detail; the precondition
  `WITNESS-P8-ADMISSION-PACKET-BOUND` repeats it, and `WITNESS-P7` appears only when a path was
  refused per path. A failing `--check` (any mode) whose admission is not `COMPUTED` MUST also
  write `corvint witness: --check: admission is STATE: DETAIL` on stderr; stdout keeps the bytes of
  the run without `--check`. `witness --help` documents the bound. Non-goal: raising the packet
  bound, which is the shared contextindex result ceiling. Failure mode: a 60-path range that was
  `COMPUTED` with ten silent `NOT_RUN` paths is now `PARTIAL`, so `--check=admitted` exits 1;
  split the range to analyse every path. Rollback: drop `packetBounded` and the P8 precondition.
- `AGW-V0-007`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2797, GitHub
  #1894) When `--head` resolves to a commit other than the checked-out HEAD (or HEAD does not
  resolve), the CLI MUST build the index with `contextindex.BuildRevisionContext` at that commit
  and compile with `Options.HeadPinned`, so admission uses `CommitPinnedPerPathRangeImpact`
  (ERI-V0-265) and never reads the worktree; `range.headBinding` is then `COMMIT-PINNED` and the
  text report's range line adds `head-binding=COMMIT-PINNED`. A `--head` equal to HEAD keeps the
  snapshot-or-build path and its exact bytes (no `headBinding`). Apart from that binding and the
  admission packet's size, the report MUST equal the one compiled while the head was checked out.
  Fails closed: a head `--base` is not an ancestor of, a head whose objects are missing, and an
  index whose commit differs from the resolved head refuse with exit 2 and no stdout; a pinned
  admission that detects drift is `REFUSED`, never `COMPUTED`. Non-goal: a historical head for
  `impact`, `affected` or `prove`. Failure mode: a consumer that relied on the historical-head
  refusal now receives a report. Rollback: restore the refusal and drop `HeadPinned` and
  `headBinding`.
- `AGW-V0-008`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2797, GitHub
  #1894) `--evidence FILE` (repeatable, at most `witness.MaxEvidenceFiles` = 8, resolved to an
  absolute path) MUST read each document under the provider receipt bound
  (`testvaliditydoc.MaxInputBytes`) and accept a `corvint-test-validity/0` document or any provider
  document `testvaliditydoc.Decode` admits. A Playwright receipt binds only when it records the
  same clean test-repository identity at run start and publish and that identity is the report's
  head commit and head tree; the source (`name` `evidence`, after the `cem` source) is then `BOUND`
  with `hunks` = tests, `basis` = witnesses attached and `unattached` = tests that attached
  nothing. Every other decodable document, including a Go or Ruby provider document and a
  projection without a `playwright` member, is `UNBOUND` with its reason and attaches nothing.
  A bound test attaches one witness per obligation whose path is its clean repository-relative
  anchor file or a reported imported file: `authority` `CALLER_REPORTED`, `closing` false
  (CF-V0-014), `relation` `test-run:<state>`, `evidencePath` the anchor file, and the new optional
  `test` member naming the test. No verdict changes: witnesses attach before `decide`, `NOT_RUN`
  stays sticky, and nothing closes. `summary` gains `candidateWitnessed` (open obligations with at
  least one witness) and `unwitnessed` (open with none), which sum to `opened - closed`; the text
  summary prints them as `open-with-candidate` and `open-with-nothing` on every report. Fails
  closed: an unresolvable, unreadable, oversize, non-object or undecodable document, a document of
  an unknown kind, and more than eight documents refuse with exit 2 naming the file and no stdout.
  Non-goals: closing authority for any test evidence. Path resolution and digest comparison are
  AGW-V0-009. Failure mode: a consumer that compared whole `summary` objects sees two new
  members. Rollback: drop `--evidence`, `Options.Evidence`, `Witness.Test` and the two summary
  members.
- `AGW-V0-009`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2824) A bound
  receipt's file names MUST resolve to head-tree paths before they attach or are compared. The
  qualified reporter writes absolute paths under the checkout it ran in, and Playwright names an
  anchor relative to its test directory. The checkout root is the directory under which the most
  absolute test files (the identity's `testFileDigests` keys and test anchors) name a tracked
  path; a tie is broken only by the local checkout root. The test directory is the tree directory
  under which the most untracked relative test files name a tracked path. An absolute path
  resolves only under that root, and a clean relative path resolves as written when tracked, then
  under the test directory, and otherwise stays as written. A tie, an escaping path, or a path
  outside the root resolves to nothing and attaches nothing. Every recorded digest (each test's
  `importedFiles` and the identity's `testFileDigests`) whose path resolves to a tracked file
  MUST be a lowercase hex sha256 equal to the sha256 of that head blob, read by one bounded
  `git cat-file --batch` (`maxEvidenceBlobBytes` = 64 MiB). A malformed, contradictory,
  unreadable or different digest, or blobs over the bound, makes the whole receipt `UNBOUND`
  naming the file, so it attaches nothing; a path that resolves to no tracked file is not
  compared. Failure modes: a run whose checkout bytes differ from the blobs (end-of-line
  conversion, smudge filters such as Git LFS) is `UNBOUND` rather than trusted; an ambiguous
  layout attaches less, never to a guessed file. RSpec provider documents bind under AGW-V0-010,
  repository-relative evidence paths are admitted under AGW-V0-011, and Go test runs bind under
  AGW-V0-012. Rollback: restore `repositoryPath` as the only resolver in `attachEvidence` and
  drop `compareDigests`.
- `AGW-V0-010`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2824) An RSpec
  provider document (`corvint-ruby-test-provider-run/0`) MUST bind to the head tree before any
  example attaches. It is `BOUND` only when it records no infrastructure failure; carries a
  repository identity both before and after the run, and the two are equal; records
  `dirtyState` clean; names the head repository's object format (sha256 blob IDs only for a
  sha256 head); records the report head's revision and tree; and every helper blob and every
  example's spec blob equals the head tree's blob at that repository-relative path, read by one
  `git cat-file --batch-check` (`maxRunBlobCheckBytes` = 1 MiB of requests; the shared
  `bindRun`/`compareRunBlobs` since AGW-V0-012). Each example then
  attaches to its repository-relative `file` as a non-closing CALLER_REPORTED candidate named by
  its description (or ID) with relation `test-run:<status>`; an example whose spec blob is unbound
  attaches nothing and counts as unattached. Anything else is `UNBOUND` naming the first failing
  check and attaches nothing. Failure modes: a run against a dirty, moved or different checkout,
  or one whose spec or helper bytes differ from the head blobs, is `UNBOUND` rather than trusted.
  Non-goal: the Go live session event (`corvint-go-live-session-event/0`) carries only an
  `identity` digest over its watched working-tree file set, with no revision, tree, dirty state or
  per-file blob IDs, so it stays `UNBOUND` under AGW-V0-008 and its closed profile is unchanged;
  Go test runs bind through the new profile of AGW-V0-012. Rollback: drop `bindRuby`, the shared
  `bindRun` use and `rubyTests`; RSpec documents return to `UNBOUND`.
- `AGW-V0-011`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2824)
  `--evidence-in-repo PATH` (repeatable, sharing the eight-document bound with `--evidence`) MUST
  read a caller-named evidence document only from inside the checkout. The path MUST be a
  non-empty, clean, relative, local path that does not use `..`; the checkout root and the
  joined path are then resolved through symbolic links, and a target outside the resolved root
  is refused. The leaf is read below the resolved root without following further links, so a
  link swapped after the check cannot redirect the read. Every refusal exits 2 with no stdout and
  names the option, the path as given and the reason (`the path is empty`, `the path is
  absolute`, `the path leaves the checkout through ..`, `the path is not a clean relative file
  path`, `resolves through a symbolic link to X, outside the checkout`, or the read error); no
  path is silently dropped. An admitted document binds exactly as under AGW-V0-008 to AGW-V0-010,
  and its source `path` is the path as given. Failure mode: a document a caller wants to read
  from outside the checkout must use `--evidence` on the CLI; the pi facade (PWV-V0-014) exposes
  only this option. Rollback: drop `--evidence-in-repo`, `Options.RepositoryEvidence` and
  `readRepositoryEvidence`.
- `AGW-V0-012`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2824) A Go test
  run MUST be recordable as, and bind only as, a new closed profile `corvint-go-test-run/0`
  (`internal/gotestrun`), leaving `corvint-go-live-session-event/0` unchanged.
  `corvint-go-test-provider test-run [--dir DIR] [--timeout D] [PACKAGE...]` runs `go test -json
  -count=1` once (default `./...`; the count flag since AGW-V0-040) under the bounded process group and prints one document: the
  repository-relative run and module directories, the `module` path from the nearest `go.mod`
  inside the worktree, the object format, the repository identity (revision, tree, dirty state)
  observed before and after the run, and, per test (subtests included, keeping the last terminal
  `pass`/`fail`/`skip` action, at most 5000 with the rest counted), the import path, full name,
  action, and either the repository-relative `_test.go` file in the package's tree directory that
  declares its top-level function plus that file's Git blob ID, or an `unbound` reason. An import
  path maps to a tree directory only through the module path (`module` itself, or `module/` plus a
  clean relative rest); a package outside the module, a function no file declares, or one declared
  in more than one file is `unbound`, never guessed. A start failure, timeout, cancellation,
  signal, unobserved exit, output overflow, a stdout line that is not a test2json event, or a
  package result replayed from the build cache (AGW-V0-040) is recorded as `infrastructure`; a directory with no enclosing worktree or `go.mod` refuses with
  exit 2 before running. The witness recognizes the document by its top-level `profile`, decodes
  it closed (unknown members, escaping files, malformed blob IDs and tests with neither a file nor
  an `unbound` reason refuse the report with exit 2), and binds it under exactly the AGW-V0-010
  conditions (no infrastructure, equal clean identities at the head revision and tree in the head
  object format, every recorded file blob equal to the head blob). Each anchored test then
  attaches to its declaring file as a non-closing CALLER_REPORTED candidate named
  `<import path>.<test name>` with relation `test-run:<action>`; an unanchored test attaches
  nothing and counts as unattached. Failure modes: a test helper file or a package file a test
  exercises gets no witness (only the declaring file is anchored); a nested module is reached only
  when the run's `--dir` is inside it; a run whose test file bytes differ from the head blobs is
  `UNBOUND`. Rollback: drop `test-run`, `internal/gotestrun`, `decodeGoRun`, `bindGo` and
  `goTests`; Go test-run documents then refuse as an unknown kind.
- `AGW-V0-040`: (experimental under the standing lane auto-accept, 2026-10-28, V1-3004), amending
  AGW-V0-012. A recorded Go test outcome MUST be an executed one. The default `go test` command of
  `gotestrun.Run` is `go test -json -count=1`, so every selected test executes, and the document's
  `command` shows the flag. A run whose stream nevertheless reports a package result taken from
  the build cache (a package-level `output` event starting with `ok` and containing `(cached)`, as
  a configured command without `-count=1` produces) records `infrastructure` with reason
  `cached-result` and a detail naming the first cached package and the count of the rest; the
  replayed test events are not trusted as executions of the head content, and under AGW-V0-012 the
  witness binds no test of a document with infrastructure. The `corvint-go-test-run/0` shape is
  unchanged: `cached-result` is one more value of the existing free-text reason. Failure modes: a
  run is not cached when any of its packages is not, so a mostly cached run still records no pass;
  a run with `-count=1` takes longer than a cached one. Evidence:
  `TestAGWV0040CachedPackagesAreNotExecutedPasses` and
  `TestAGWV0040DefaultRunExecutesAndCachedRunIsInfrastructure` (both fail without the change:
  the cached stream yielded a pass and the default command had no `-count=1`). Rollback: restore
  the `go test -json` default and drop `cachedFailure`, `cachedResult` and `parseRun`.

### Declared path policy (GitHub #1962)

A range that touches a Markdown, JSON or YAML path could never pass `--check=admitted`: no
extractor reads those suffixes, so each stays `NOT_RUN` with `LANGUAGE_COVERAGE_NONE` and the
analysis half of the gate fails whatever the rest of the range shows. These requirements let the
project declare, in a reviewable committed file, which such paths carry no test obligation and
which are data some code reads. Undeclared paths keep their behaviour.

- `AGW-V0-041`: (experimental under the standing lane auto-accept, 2026-10-28, V1-3054, GitHub
  #1962) The witness path policy is the blob `.corvint/witness-paths.json` in the range's **base**
  tree, read through the bounded Git runner, never from the worktree or the head tree. A range
  therefore cannot exempt its own paths: a policy the range adds or edits takes effect only for a
  later range whose base contains it (CF-V0-031). The document is one JSON object of at most 65536
  bytes, decoded strictly (a member that is unknown, spelled in another case, or repeated is
  refused), with `profile`
  `corvint-witness-paths/0` and `rules`, an array of at most 256 objects, each with `glob`, `class`
  (`inert` or `data`) and, for `data` only, optional `readers`, at most 16 repository-relative
  paths. A glob is a clean slash-separated relative path whose segments are `path.Match` patterns
  and where a `**` segment matches zero or more whole segments (`sharedpath.Match`). Without a
  blob at that path the report has no `pathPolicy` member and its bytes are unchanged. Any other
  defect (a failed base-tree listing, not a blob, oversized, malformed, a bad glob, class or reader)
  makes the policy `INVALID`:
  the report names it, applies no rule, and adds precondition `WITNESS-P9-PATH-POLICY-INVALID`.
- `AGW-V0-042`: (experimental under the standing lane auto-accept, 2026-10-28, V1-3054, GitHub
  #1962) A rule applies only to a `CHANGE` obligation that would otherwise be `NOT_RUN` with
  `LANGUAGE_COVERAGE_NONE`: a regular file that is added or modified, that no extractor reads and
  that admission neither admitted nor refused. No rule classifies the policy file itself. An `inert`
  rule also applies to the deletion of a regular file no extractor reads (`ADMISSION_REFUSED_DELETED` or `PATH_DELETED_OUT_OF_PROFILE`); a deleted
  `data` path keeps its reason, since a reader may still need it. Symbolic links, gitlinks, files an
  extractor dropped, other admission refusals and every language with an extractor keep their
  reasons.
  The first rule in file order whose glob matches the path applies. The obligation then carries
  `declaration` `{file, line, glob, class}`: the policy path, the 1-based line where the rule
  object starts, and the rule; the text report prints it under the obligation.
- `AGW-V0-043`: (experimental under the standing lane auto-accept, 2026-10-28, V1-3054, GitHub
  #1962) An `inert` path's obligation has verdict `EXEMPT` and reason `DECLARED_INERT`: it stays in
  the denominator and counts toward `summary.exempt` (omitted when zero), not toward `notRun`,
  `unproven`, `candidateWitnessed` or `unwitnessed`. It is determined but never closed, so the
  `proven` ratio excludes it and the valueless `--check` (AGW-V0-006) still fails while it is open:
  an exemption is not a closure.
- `AGW-V0-044`: (experimental under the standing lane auto-accept, 2026-10-28, V1-3054, GitHub
  #1962) A `data` path is admitted text-only (coverage `GENERIC`) only when it has a reader: each
  declared reader tracked in the head tree (not a gitlink), and each found reader, a head-tree
  source with an extractor whose text quotes the data path, either repository-relative or relative
  to the reader's own directory, in `"`, `'` or backtick quotes. Found readers are taken in path
  order, at most 16. Each reader the range does not itself change opens an `IMPACT` obligation with
  relation `data-reader`, decided as any other, and the declaration lists `readers` with their
  `origin` (`declared` or `found`). With a reader, the data obligation is decided as an analysed
  one (`UNPROVEN` with `NO_CLOSING_AUTHORITY` in V0). Without one it stays `NOT_RUN` with reason
  `DATA_READER_MISSING`. A declared reader that is not tracked, is the data path itself, or has no
  extractor is named in the declaration with origin `untracked`, `self` or `no-extractor` and does
  not count, so a data change no code reads fails closed. Nor does a reader that is no usable
  source: one the index did not admit (an excluded, generated or oversized file, reason
  `NOT_ADMITTED: <exclusion>`), one that is not a regular file (`SYMLINK_OUT_OF_PROFILE`,
  `FILE_MODE_OUT_OF_PROFILE`), or one whose extraction was refused (its unparsed reason, such as
  `GO_SOURCE_UNPARSED`). A declared one is named with origin `unusable` and that `reason`, and a
  found one is not proposed; a reader whose facts are absent is no evidence that it reads the data.
- `AGW-V0-045`: (experimental under the standing lane auto-accept, 2026-10-28, V1-3054, GitHub
  #1962) The report member `pathPolicy` `{path, status, detail, rules}` (`BOUND` or `INVALID`) and
  a `PATH POLICY` text section appear only when the base tree holds the policy. With `--range`,
  each range reads its own base's policy. `--check=admitted` needs no change: a range whose every
  `LANGUAGE_COVERAGE_NONE` path is declared inert, or data with a reader, has no `NOT_RUN`
  obligation from them.
  Non-goals: classifying a path as code, closing any obligation, a policy in the worktree, head or
  a flag, inferring a class from a suffix, and finding readers that build a path at run time (a
  glob loader, a joined path): those need a declared reader. Failure modes: an over-broad inert
  glob exempts a path a test should cover, which is visible in review as the declaring line; a found
  reader may quote the path without reading it, which only opens an obligation, never closes one; a
  policy edited in the range is not applied to it. Rollback: delete the policy read, the
  `declaration`, `pathPolicy`, `exempt` members and the `EXEMPT` verdict; a repository without the
  policy file is unaffected throughout. Evidence: `TestAGWV0041PolicyIsReadFromTheBaseTree`,
  `TestAGWV0041InvalidPolicyAppliesNoRule`, `TestAGWV0042FirstMatchingRuleNamesFileAndLine`,
  `TestAGWV0043InertPathIsExempt`, `TestAGWV0044DataPathNeedsAReader`,
  `TestAGWV0044UnusableReaderDoesNotAdmitData` (`internal/witness`), and `TestAGWV0045CheckAdmittedPassesOnlyWithAPolicy` (`cmd/corvint`), the
  issue's repro: `--check=admitted` exits 1 without the policy and 0 with it.

### Paired application and tests ranges

- `AGW-V0-013`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2851) `corvint
  witness --range app=BASE..HEAD --range tests=BASE..HEAD [--range-root NAME=PATH]...` compiles an
  application range and a tests range, usually in two checkouts, as one logical change. Each name
  is given exactly once; `--range-root` defaults to the invocation root; `--range` replaces
  `--base`/`--head`; `--cem` and `--stable-artifact-root` name one repository and are refused;
  `--evidence` and `--evidence-in-repo` documents belong to the tests range only. Each range is
  compiled as an unchanged single-range `corvint-witness/0` report at its own head (pinned or
  checked out exactly as AGW-V0-007), and the result is the additive wrapper profile
  `corvint-witness-paired/0`: `ranges` (`app` then `tests`, each with `name`, `root` and its
  complete `report`) and a `summary` over both ranges' obligations. The text form prints
  `range NAME base= head= root=` for each range, the combined summary, then each range's
  single-range report under `RANGE NAME`. `--check` exits 1 when either range's admission is not
  computed or any obligation of either range is open. Failure modes: a malformed, repeated,
  unknown-name or single `--range`, a `--range-root` without `--range`, and a base, head or root
  that does not resolve in its own checkout each refuse the whole report with exit 2 before
  anything is printed, naming the range. Which two ranges belong together is the caller's
  explicit `--range` flags; the AGW-V0-015 trailer records that pairing in history. Rollback: drop
  `--range`/`--range-root`, `CompilePaired`, `RenderPaired` and the paired profile; the
  single-range report is unchanged.
- `AGW-V0-014`: (experimental under the standing lane auto-accept, 2026-10-27, V1-2851) A
  tests-range Playwright receipt that is `BOUND` to the tests head (AGW-V0-008/009) may cite
  application obligations only when its application attestation records no failure, records the
  application repository before and after the run, those two identities are equal and clean, and
  they name the application head revision and tree; an application checkout root is inferred from
  the receipt's absolute imported files by voting on tracked application head paths (a tie broken
  only by the local application root); and every imported-file digest that resolves there equals
  the application head blob. Each test then attaches to the application obligations of its
  imported application files as a non-closing CALLER_REPORTED witness with source `cross-range`
  and relation `test-run:<state>`, its anchor still the tests-range path; the application report
  gains one `cross-range` source per receipt and is re-decided and re-summarized. Failure modes:
  any unmet condition leaves that source `UNBOUND` with its reason and cites nothing; a relative
  imported name never resolves on the application side; Go and RSpec documents never cite across
  ranges.
- `AGW-V0-015`: (experimental under the standing lane auto-accept, 2026-10-28, V1-2851; the
  trailer form is the owner-delegated orchestrator decision of 2026-10-28) Pairing convention for
  merged history: the tests-repository commit that completes a
  paired change carries one Git trailer per range it pairs with,
  `Paired-Range: <name>=<repo>@<base>..<head>`. `<name>` is the `--range` name (`app`); `<repo>`
  is a caller-chosen, machine-independent repository identity (such as `beamfall/app` or a clone
  URL) with no whitespace; `<base>` and `<head>` are full commit object IDs in that repository, so
  the value splits at its last `@` even when `<repo>` holds one. To replay a pairing the caller maps
  `<repo>` to a local checkout and passes `--range <name>=<base>..<head> --range-root <name>=PATH`
  plus `--range tests=<parent>..<commit>` for the trailer's own commit. Explicit `--range` flags
  read no trailer: `witness` never infers a pairing from history or from a workspace declaration,
  and a report given both `--range` flags is unchanged by a trailer's presence, absence or content.
  Amended by GitHub #1943: only `--paired-from` reads the trailer, and refuses every trailer it
  cannot use (AGW-V0-036). Rollback: delete this convention and AGW-V0-036.
- `AGW-V0-036`: (experimental under the standing lane auto-accept (GitHub #1943)) `corvint witness
  --paired-from REV --range-root app=PATH [--range-root tests=PATH]` MUST read the AGW-V0-015
  trailer of the tests commit REV, resolved in `--root` or its `--range-root tests` checkout, and
  compile the AGW-V0-013 paired report of `--range app=<base>..<head>` in the `--range-root app`
  checkout and `--range tests=<parent>..REV`, adding a `pairing` member (`source` `trailer`,
  `commit`, `repository`, `trailer`; the text form prints `pairing trailer commit= repository=`).
  The member is omitted from every report built from explicit `--range` flags, so those reports are
  unchanged. `<repo>` is a label: the caller MUST name the application checkout with
  `--range-root app`, and witness never maps an identity to a path. Failure modes, each exit 2 with
  no stdout and a `--paired-from:` reason: REV does not resolve or has other than one parent; no
  `Paired-Range` trailer; more than one; a name other than `app`; a value that is not
  `<repo>@<base>..<head>` with a whitespace-free repository and full lowercase-hex object IDs of one
  length; a `Paired-Range` line that Git does not parse as a trailer (outside the closing trailer
  block); a `<base>` or `<head>` that is not that commit in the mapped checkout (unresolvable, or
  the wrong checkout); a `<base>` that is not an ancestor of `<head>` (stale); and `--range`,
  `--base`, `--head`, `--cem`, `--stable-artifact-root` or a missing `--range-root app` given with
  it. Non-goals: no check of `<repo>` against a remote URL or any other checkout identity, no
  reading of more than one trailer or of a `tests` trailer, no history walk to find a pairing
  commit, and no change to `corvint cem` (CEM-V1-648). Rollback: drop `--paired-from`,
  `ReadPairingTrailer`, `CheckPairedApplication` and the `pairing` member; explicit `--range`
  reports are byte-identical with or without it.
- `AGW-V0-037`: (experimental under the standing lane auto-accept (GitHub #1943)) Every
  AGW-V0-036 refusal MUST name the trailer it concerns. A missing `--range-root app` MUST be
  refused only after the trailer is read, naming the `Paired-Range` value and its `<repo>`
  identity and asking for `--range-root app=PATH`; a malformed trailer given without
  `--range-root app` is refused as malformed first; a REV that does not resolve is refused as
  unable to read a `Paired-Range` trailer, naming REV. Each stays exit 2 with no stdout and a
  `--paired-from:` reason, and a report without `--paired-from` is unchanged. Non-goals: no
  mapping of `<repo>` to a checkout and no remote-identity check. Rollback: restore the
  parse-time `--range-root app` refusal and the bare resolution error.

## Non-goals

This spec states the CLI report's read-only guarantee, immutable-parent ancestry boundary and
per-packet cost projection. It does not define, restate, or amend
the `ocm-change-witnessed-v0` closure semantics `change-witness-relation-v0.md` owns, and it adds no
verb to `GPK-V0-007`'s parity-compared vocabulary. A historical `--head` is delivered by
AGW-V0-007 for `witness` only. Caller test or run evidence attaches under AGW-V0-008 only as a
non-closing CALLER_REPORTED candidate bound to the head tree (CF-V0-014); no test evidence closes an
obligation, and closing authority still needs a new profile under CF-V0-029. Paired ranges
(AGW-V0-013/014) do not extend `corvint cem` or `corvint review`: a Change Evidence Map cites
hunks by single-repository path, so a cross-repository citation needs a frozen CEM wire change
that is out of scope here. The workspace declaration of V1-2846 is not read; ranges are named only
by explicit `--range` flags or one `--paired-from` trailer with an explicit `--range-root app`
(AGW-V0-036). The declared path policy (AGW-V0-041..045) classifies only paths no extractor reads; it
never closes an obligation and is read from the base tree only.

## Traceability and rollback

| Requirement | Planned implementation surface | Required evidence |
|---|---|---|
| `AGW-V0-002` (decision 0290) | `internal/witness` bounded Git runner | `TestBaseTreeIgnoresGraftedAncestry` |
| `AGW-V0-003` (proposed) | `internal/witness` `packetCoverage`, `Report.PacketCoverage`, `renderPackets` | `TestPacketCoverageEqualsEveryCompiledReceipt`, `TestPacketCoverageRefusesAnUnreadableBlock`, `TestPacketCoverageOfARefusedAdmissionIsEmptyNotNull` |
| `AGW-V0-004` (experimental) | `admit`, `changeObligations`, `preconditions` in `internal/witness/witness.go` | `TestCompileRefusesOnlyTheOffendingPaths`, `TestChangeObligationsNameARefusedAdmission` |
| `AGW-V0-005` (experimental) | `witness.ResolveCommit`, `runWitness` in `cmd/corvint/witness.go` | `TestResolveCommitAcceptsAnyGitRevision`, `TestRunWitnessResolvesRevisionsAndChecks`, `TestRunWitnessCompilesAHistoricalHead`, `TestRunWitnessRefusesAnUnknownBase` |
| `AGW-V0-006` (experimental) | `witnessExit`, `--check` in `cmd/corvint/witness.go` | `TestRunWitnessResolvesRevisionsAndChecks` (exit 0 without and 1 with `--check`; `--check=1` refused) |
| `AGW-V0-041`..`AGW-V0-045` (experimental) | `readPathPolicy`, `decodePathPolicy`, `applyPathPolicy`, `findReaders`, `readerRefusal` in `internal/witness/path_policy.go`; `changeObligations`, `decide`, `summarize`, `preconditions` in `witness.go`; `renderPathPolicy` and `renderSummary` in `render.go`; `witness` help | `TestAGWV0041PolicyIsReadFromTheBaseTree`, `TestAGWV0041InvalidPolicyAppliesNoRule`, `TestAGWV0042FirstMatchingRuleNamesFileAndLine`, `TestAGWV0043InertPathIsExempt`, `TestAGWV0044DataPathNeedsAReader`, `TestAGWV0044UnusableReaderDoesNotAdmitData`, `TestAGWV0045CheckAdmittedPassesOnlyWithAPolicy` |
| `AGW-V0-034` (experimental) | `witnessExit`, `witnessAnalysed`, `witnessGateFails`, `--check=` in `parseWitnessOptions` and `runPairedWitness` in `cmd/corvint/witness.go` | `TestRunWitnessAttachesABoundReceipt` via `assertWitnessCheckModes`, `TestRunWitnessResolvesRevisionsAndChecks` (refused values) |
| `AGW-V0-035` (experimental) | `admit`, `changeObligations` over ERI-V0-285 | `TestCompileRefusesPathsPastThePathBoundEachAlone` |
| `AGW-V0-038` (experimental) | `admit`, `packetBounded`, `changeObligations` and `WITNESS-P8` in `internal/witness/witness.go`; `witnessAdmissionNote` in `cmd/corvint/witness.go`; `witness` help | `TestCompileNamesTheAdmissionPacketBound` (60 and 101 Ruby paths, `internal/witness`); `TestWitnessCheckNamesTheAdmissionPacketBound` (`cmd/corvint`) |
| `AGW-V0-007` (experimental) | `runWitness` pinned path in `cmd/corvint/witness.go`; `Options.HeadPinned`, `admit`, `Range.HeadBinding`, `renderHeader` in `internal/witness` | `TestCompileAtAHistoricalHeadMatchesTheCheckedOutReport` (parity with the checked-out report after HEAD moved and the worktree went dirty; unpinned admission refuses), `TestRunWitnessCompilesAHistoricalHead` (pinned and HEAD bindings, unknown and non-descendant heads refused); `integrations/pi/core-host.test.mjs` live host check |
| `AGW-V0-008` (experimental) | `internal/witness/evidence.go` (`readEvidence`, `decodeEvidence`, `bindReceipt`, `attachEvidence`), `Options.Evidence`, `Witness.Test`, `summarize`, `renderSummary`; `--evidence` in `cmd/corvint/witness.go` | `TestEvidenceAttachesNonClosingCallerReportedWitnesses`, `TestEvidenceNotBoundToTheHeadTreeAttachesNothing`, `TestEvidenceRefusesAnUnverifiableDocument`, `TestAttachEvidenceKeepsNotRunSticky`, `TestParseWitnessEvidence` |
| `AGW-V0-009` (experimental) | `internal/witness/evidence.go` (`evidencePaths`, `resolveEvidencePaths`, `mostVoted`, `compareDigests`) | `TestEvidenceResolvesReporterPaths`, `TestEvidenceDigestMismatchIsUnbound`, `TestEvidenceAttachesNonClosingCallerReportedWitnesses` (head-blob digest admitted), `cmd/corvint` `TestRunWitnessAttachesABoundReceipt` (slash-qualified module, end-to-end) |
| `AGW-V0-010` (experimental) | `internal/witness/evidence_ruby.go` (`bindRuby`, `bindRun`, `compareRunBlobs`, `rubyTests`), `bindEvidence` in `evidence.go` | `TestRubyEvidenceBindsToTheHeadTree` (stock RSpec 3.13 JSON through `rubytestprovider.Run`), `TestRubyEvidenceNotBoundToTheHeadTreeAttachesNothing`, `TestRubyEvidenceUnboundSpecAttachesNothing` |
| `AGW-V0-012` (experimental) | `internal/gotestrun` (`Run`, `parseEvents`, `locator`, `Decode`, `Validate`); `test-run` in `cmd/corvint-go-test-provider/testrun.go`; `internal/witness/evidence_go.go` (`decodeGoRun`, `bindGo`, `goTests`), shared `bindRun`/`compareRunBlobs` | `TestRunRecordsARealGoTestRun` (real `go test -json`), `TestParseEventsKeepsTerminalActionsOfRealShape`, `TestLocatorMapsImportPathsThroughTheModulePath`, `TestDecodeRefusesOpenOrUnanchoredDocuments`, `TestTestRunCommandRefusesBeforeRunning`, `TestGoEvidenceBindsToTheHeadTree` (real run through `gotestrun.Run`), `TestGoEvidenceNotBoundToTheHeadTreeAttachesNothing`, `TestGoEvidenceUnanchoredTestAttachesNothing` |
| `AGW-V0-011` (experimental) | `readRepositoryEvidence`, `repositoryEvidenceReason` in `internal/witness/evidence.go`; `Options.RepositoryEvidence`; `--evidence-in-repo` in `cmd/corvint/witness.go` | `TestRepositoryEvidenceAdmitsOnlyPathsInsideTheCheckout`, `TestParseWitnessRepositoryEvidence`; `integrations/pi/core-host.test.mjs` PWV-V0-014 evidence host check |
| `AGW-V0-013` (experimental) | `internal/witness/paired.go` (`CompilePaired`, `RenderPaired`, `PairedReport`), `compile` in `witness.go`; `parseWitnessRange`, `addRange`, `checkRanges`, `runPairedWitness`, `witnessIndex` in `cmd/corvint/witness.go` | `TestParseWitnessRanges`, `TestRunWitnessPairsTwoRanges`, `TestRunWitnessPairedRefusesAnUnresolvedRange`, `TestCompilePairedRefusesSingleRepositoryOptions`, `TestPairedReceiptCitesApplicationObligations` |
| `AGW-V0-014` (experimental) | `internal/witness/paired.go` (`bindCrossRange`, `bindAttestation`, `resolveApplicationPaths`), `attachEvidenceAs` and `evidencePaths.absoluteOnly` in `evidence.go` | `TestPairedReceiptCitesApplicationObligations`, `TestPairedReceiptWithoutApplicationProofCitesNothing` (no attestation, failures, no after, moved, dirty, revision, tree, digest, no root) |
| `AGW-V0-015` (experimental) | this section and `witnessHelp` in `cmd/corvint/help.go` | `TestWitnessHelpDocumentsThePairingTrailer`; `TestRunWitnessPairedIgnoresAPairingTrailer` (with explicit `--range` flags, a tests head carrying a `Paired-Range` trailer yields the same paired report as one without) |
| `AGW-V0-040` (experimental) | the default command and `cachedFailure`, `cachedResult`, `parseRun` in `internal/gotestrun/run.go` | `TestAGWV0040CachedPackagesAreNotExecutedPasses`, `TestAGWV0040DefaultRunExecutesAndCachedRunIsInfrastructure` (real `go test` run twice) |
| `AGW-V0-036` (experimental) | `internal/witness/pairing.go` (`ReadPairingTrailer`, `CheckPairedApplication`, `Pairing`), the `pairing` member and line in `internal/witness/paired.go`; `checkPairedFrom`, `pairedRanges`, `runPairedWitness` in `cmd/corvint/witness.go` | `TestParseWitnessPairedFrom`, `TestRunWitnessPairedFromReadsTheTrailer` (equal to the explicit `--range` report but for the member), `TestRunWitnessPairedFromRefusesAnUnusableTrailer` (no trailer, two, `tests` name, abbreviated ID, no repository, outside the block, unmapped checkout, stale, root commit) |
| `AGW-V0-037` (experimental) | `pairedRanges` and `checkPairedFrom` in `cmd/corvint/witness.go`; `ReadPairingTrailer` in `internal/witness/pairing.go`; `witnessHelp` in `cmd/corvint/help.go` | `TestParseWitnessPairedFrom` (no app root parses), `TestRunWitnessPairedFromRefusesAnUnusableTrailer` (unknown repository, malformed without app root, unresolvable revision) |
| `AGW-V0-001` (accepted 2026-09-12, decision 0163) | `cmd/corvint/witness.go` and `internal/witness` | `cmd/corvint`: `TestCLIReadVerbsLeaveTheRepositoryByteIdentical`, subtest "witness refuses an unknown base without reading further" (CLI-level repository-byte assertion); pre-index refusals: `TestParseWitnessOptionsNamesAnUnknownOptionBeforeItsValue`, `TestParseWitnessInvocationRefusesARootThatIsNotARepository` |

Rollback of AGW-V0-002 removes the graft-isolation runner change and its clause; the read-only
AGW-V0-001 guarantee remains. Rollback of AGW-V0-003 removes the `packetCoverage` member and the
`PACKETS` section; readers never required them. Rollback of AGW-V0-004 calls
`contextindex.RangeImpact` again; of AGW-V0-005 restores the exact-SHA comparison; of AGW-V0-006
removes `--check`; of AGW-V0-007 restores the historical-head refusal; of AGW-V0-008 removes
`--evidence`, the `test` witness member and the two summary counts. Rollback of AGW-V0-009 restores
repository-relative-only resolution and removes the digest comparison. Rollback of AGW-V0-010
returns RSpec documents to `UNBOUND`; of AGW-V0-011 removes `--evidence-in-repo`; of AGW-V0-012 removes `test-run` and the Go test-run binder; of AGW-V0-040 restores the plain `go test -json` default and drops the cached-result check; of AGW-V0-013 removes `--range`, `--range-root`, `CompilePaired` and `RenderPaired`; of AGW-V0-014 removes `bindCrossRange` so tests-range receipts cite only their own range; of AGW-V0-034 drops the two `--check` values; of AGW-V0-035 restores whole-range bound refusal. Rollback of AGW-V0-015 deletes the trailer convention, of AGW-V0-036 removes `--paired-from` and the `pairing` member, and of AGW-V0-037 restores the parse-time `--range-root app` refusal. Retiring the report removes both requirements and this document.
