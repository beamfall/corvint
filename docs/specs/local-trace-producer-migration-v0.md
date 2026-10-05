# Local Trace Producer and Legacy Migration V0

Owner: Russell Lewis
Accepted: 2026-08-23
Intent status: accepted
Delivery status: experimental
Authoritative inputs: the 2026-08-23 owner delegation; `docs/DOGFOOD.md`;
`docs/SPEC-DRIVEN-DEVELOPMENT.md`; frozen
`docs/specs/local-observability-dashboard-v0.md` at SHA-256
`2b89d71e375759dddc32c66c41a9777816826b254807698a3d3fa31cbae150cb`

## Agent digest
- Claim: Commit-bound production and digest-bound migration preserve trace identity without automatic legacy mutation; LTPM-V0-011 is implemented experimentally.
- Status: accepted/experimental; LTPM-V0-011 accepted and implemented experimentally 2026-09-01; LTPM-V0-012 accepted by the owner and implemented experimentally 2026-09-27; LTPM-V0-015/016 (trace producer provenance) owner-selected 2026-10-04, decision 0429, implemented experimentally
- Exists: the trace producer, bounded legacy reader, explicit migration contract, and exact dogfood changed-path admission.
- Blocked on: independent migration qualification and promotion; automatic migration remains unauthorized.
- Read next: User and measurable job; Verified current state; Public commands.

## User and measurable job

A Corvint operator must be able to record new local traces using Git commit identities accepted by
the qualified dashboard consumer, continue reading bounded historical traces that were incorrectly
named with tree identities, and explicitly migrate those legacy files without losing bytes or
silently selecting the wrong commit.

Success means future writes use a stably bound commit/tree pair; legacy reads remain bounded and
advisory; dry-run changes no trace bytes or directories; apply requires the exact dry-run digest;
every canonical row and quarantined original is verified at mode `0600`; and ambiguity,
unreachability, collision, drift, unsafe files, or interruption never deletes the sole accepted copy.

## Verified current state

- The legacy Python `record_trace` producer used the index tree revision for both the candidate
  filename and each row's `revision`.
- The frozen dashboard contract correctly defines a trace candidate revision as a reachable Git
  commit object, never a tree object or alias.
- Existing Python learning reads are local, bounded, secret-screened, advisory, and limited to
  10,000 reachable commits, 1,000 candidate entries, 1,000 rows, 256 KiB per row, and 16 MiB for the
  store. No automatic migration is authorized.

## Public commands

```text
corvint record ...
corvint migrate-traces --dry-run
corvint migrate-traces --apply --plan-digest LOWERCASE_SHA256
```

`migrate-traces` always targets the explicit CLI repository root. Dry-run emits a canonical JSON
plan summary and performs no deliberate write. Apply accepts only the digest of an unchanged current
plan and performs no discovery outside that repository.

## Requirements

- `LTPM-V0-001`: `record_trace` MUST obtain the clean `HEAD` commit and its tree from one repository
  probe, require the index tree to match, and revalidate the same commit, tree, and clean state before
  and after append. The candidate filename and row `revision` MUST be the full commit ID. A same-tree
  commit change is a race and MUST fail closed.
- `LTPM-V0-002`: The Python reader MUST accept canonical rows only when their filename and row name a
  commit reachable from current `HEAD` within 10,000 commits. It MAY read a legacy tree-named row only
  when that exact tree occurs in the same bounded reachable set and every recorded path validates
  against that tree. Legacy compatibility is advisory and MUST NOT upgrade a tree to dashboard
  commit authority. Reachability MUST ignore repository and ambient graft files.
- `LTPM-V0-003`: Dry-run MUST be read-only and bind its plan digest to the stable current commit/tree,
  sorted legacy tree-to-commit mappings, exact source and canonical target SHA-256 digests, and row
  counts. It MUST validate every candidate row, trace digest, normalized historical path, command,
  outcome, and store bound before emitting a plan. Clarification (2026-09-13, bug hunt): the
  candidate-directory entry bound excludes at most one exact trace-operation lock and one exact
  deterministic migration temporary, which are bounded implementation metadata rather than trace
  candidates. A migration temporary is excludable only when its object-format-specific name, bounded
  bytes, `0600` mode, regular-file identity, link count, and contents prove either the exact computed
  missing canonical target or the verified temporary member of its published two-link target pair.
  Every other regex-shaped temporary counts toward the bound and planning never removes it. A plan
  whose missing canonical targets would exceed the remaining entry bound MUST refuse before mutation.
- `LTPM-V0-004`: A legacy tree MUST map to exactly one reachable commit. Zero or multiple reachable
  commits MUST reject the migration. No heuristic, timestamp, branch name, current-HEAD preference,
  graft file, or map-derived value may break a tie.
- `LTPM-V0-005`: Apply MUST require `--plan-digest`, acquire the exclusive trace-operation lock,
  recompute the complete plan, and compare the lowercase SHA-256 digest before writing. Repository,
  candidate-set, source-byte, row, path, mapping, target, or quarantine drift MUST reject before an
  unverified legacy source is removed.
- `LTPM-V0-006`: For each admitted legacy file, apply MUST first stage canonical commit rows in the
  candidate directory, set mode `0600`, flush them, publish them under the commit filename, and read
  them back to verify exact expected bytes and digest. An existing different canonical target is a
  collision and MUST be preserved unchanged.
- `LTPM-V0-007`: Apply MUST then stage the exact byte-preserved legacy original beneath
  `.context-corvint/legacy-traces`, outside the candidate directory, set mode `0600`, flush it, and read
  it back before unlinking the legacy candidate. An existing different quarantine target is a
  collision and MUST be preserved unchanged.
- `LTPM-V0-008`: Source, existing target, lock, and quarantine files MUST use descriptor-relative
  no-follow access and admit only single-link regular files. Publication MAY create one transient
  implementation-owned two-link pair between its deterministic temporary name and the new target
  solely to obtain atomic no-replace semantics. Recovery MAY accept that pair only when both names
  resolve to the same inode, link count is exactly two, expected bytes and mode match, and the pinned
  parent identity is unchanged; it MUST unlink the temporary name and requalify the target as
  single-link before proceeding. Every other hardlink, plus symlinks (including one whose target stays
  inside the store, which MUST be refused before its target is created or re-permissioned), special
  files, oversize data,
  unsafe directories, path escape, short/changed reads, and identity drift, MUST fail closed. The
  operation performs no network request and never scans another repository implicitly.
- `LTPM-V0-009`: Apply is resumable rather than globally atomic. Interruption MAY leave verified
  canonical or quarantined files, but MUST retain either the legacy candidate or its byte-identical
  quarantine copy. The operator MUST rerun dry-run after partial progress and apply its new digest;
  matching already-staged bytes are idempotent, while conflicting bytes fail closed.
- `LTPM-V0-010`: Query, impact, dashboard acquisition, and ordinary trace reads MUST never invoke the
  migration. Tests and dogfood MUST use temporary fixtures only; automated verification MUST NOT run
  apply against the repository's real `.context-corvint/traces` store.

### Accepted amendment: typed argv verification (2026-09-30, issue 408)

The owner accepted the schema-v2 proposal on 2026-09-30. Its reviewed proposal SHA-256 is
`707d696a76b240423084f10a44ddfae22b2b5da46391eedde5aa306be1ec716f`.
This additive profile remains experimental; it does not change the v1 contract above.

- `LTPM-V0-013`: `record` MUST accept repeatable `--verify-argv-json JSON_ARRAY` (including the
  `=JSON_ARRAY` form). The transport is exactly one JSON array of strings, decoded without shell
  parsing. A row MUST use schema 2 if and only if it includes an argv verification. Its fields
  otherwise match schema 1; `verification` becomes a sorted unique array of closed entries:
  `{"command":"go test ./...","kind":"command"}` or
  `{"argv":["go","test","a b",""],"kind":"argv"}`. Command normalization, punctuation and the
  512-character command bound remain unchanged. Argv MUST retain exact argument boundaries and
  values, with 1..32 arguments, a nonempty first argument, valid UTF-8, no Unicode controls,
  at most 512 Unicode code points per argument, and at most 4,096 canonical bytes per vector.
  Empty later arguments, spaces, literal JSON, quotes, backslashes and shell operators are data.
  JSON nulls, duplicate/unknown members, malformed UTF-8 and unpaired surrogates MUST refuse.
  The combined pre-deduplication command/argv input count remains at most 50. Canonical entries
  use the existing ASCII-escaped JSON encoding, sorted by their complete canonical JSON bytes;
  exact duplicates collapse. The trace ID remains SHA-256 over the canonical row without
  `trace_id`, including schema 2 and the typed entries. Stored entries MUST already have canonical
  values/order and unique entries; alternate valid JSON escape spellings have the same identity.
  Refusals MUST occur before mutation and use `unsupported-verify-argv`; legacy
  `unsupported-verify-syntax` and v1 bytes, IDs, screening and acceptance MUST remain unchanged.
- `LTPM-V0-014`: All ordinary trace consumers MUST admit validated mixed v1/v2 revision files
  under the unchanged row (256 KiB), file/row count (1,000), and store (16 MiB) bounds. Migration
  and stranded-row recovery MUST preserve entry kinds and values, changing only revision-bound
  identity when the existing migration contract requires it. Whole-file retention is unchanged.
  Query/eval use task/outcome/paths; typed verification MUST NOT change ranking. Batch and record
  output MUST retain typed entries in their existing projections and digests. Console and skill
  export MUST display argv as labelled canonical JSON arrays, never shell commands. Neither writer
  nor any consumer may execute verification. A reader without schema-2 support MUST refuse an
  unsupported schema. Rollback disables new v2 writes but MUST retain shipped v2 readers; an older
  binary that predates this profile cannot read mixed stores, an explicit release limitation.

Screening is owned by `LTA-V0-013`; the dashboard extension is owned by `LOD-V0-035`.
Arbitrary base64, hex or percent decoding is excluded. The contract is bounded pattern screening,
not general secret discovery. Dogfood verification-file syntax and command execution are unchanged.

Acceptance: `TestLTPMV0014TypedGolden`, `TestLTPMV0014TypedRefusals`,
`TestLTPMV0014MixedMigrationAndNonexecution`, `TestTypedStoreAppendAndRetention`,
`TestTypedUnicodeAndMalformedWireBounds`, `TestRecordTypedArgvRoundTripAndRefusal`,
`TestTypedTraceFixtureAdmission`, `TestLODV0035MixedTraceSnapshot`, `TestConsoleTypedArgv`,
`TestExportTypedArgv`, and `FuzzTypedVerificationRoundTrip` bind this amendment to source evidence.

## Non-goals and simpler baseline

The simpler baseline is to retain bounded legacy read compatibility and never migrate. V0 does not
change schema version 1, dashboard bytes, dashboard conformance fixtures, ranking authority, trace
secrecy claims, retention policy, timestamps, remote storage, signatures, encryption, a database,
background repair, or automatic operator action. Retention is owned separately by
`docs/specs/learned-trace-admission-v0.md` (`LTA-V0-003`); this migration does not modify it. It does
not modify the frozen dashboard spec.

## Failure and interruption model

Planning completes before mutation. Apply orders durable state as canonical target, quarantined
original, then legacy unlink. A failure at any earlier point leaves the source candidate; a failure
after quarantine verification leaves a byte-identical copy outside the candidate directory. Because
multiple legacy files are not one filesystem transaction, a later dry-run is the sole authority for
remaining work after interruption. Temporary files are never candidates and may be safely reused
only after exact-byte verification.

### Named failure reasons

The producer and `dogfood-record` coordination emit the kebab-case reasons below (decision 0100).
Each row cites the first emitting site and states only the condition checked there.
`internal/trace` attaches the first six to the returned error; `internal/tracerecordrepo` carries
each as the dogfood failure reason, which `corvint dogfood-record` writes as `code` on stderr with
exit 2.

| Code | First emitting site | At the cited site |
|---|---|---|
| `admitted-path-limit` | `internal/trace/record.go:358@bd1c5902` | more than 200 candidates were admitted as current source paths |
| `candidate-limit` | `internal/trace/record.go:358@bd1c5902` | `AdmissibleCurrentPaths` received more than 200,000 unique changed-path candidates |
| `changed-path-acquisition-failed` | `internal/tracerecordrepo/adapter.go:105@f49f60cd` | listing the base-to-target changed paths failed |
| `changed-path-admission-failed` | `internal/tracerecordrepo/adapter.go:111@69bd23b7` | `trace.AdmissibleCurrentPaths` failed with an error `trace.AdmissionFailureReason` maps to no reason |
| `dogfood-record-failed` | `cmd/corvint/dogfood_record.go:147@d0afa17b` | the stderr `code` written when the dogfood-record error carries an empty reason |
| `invalid-base-revision` | `internal/tracerecordrepo/adapter.go:94@8006b57c` | the base argument does not resolve to a commit |
| `malformed-path` | `internal/trace/record.go:487@c94787ea` | a path is empty or `pythonString` rejects it (its value cannot be decoded as Python string units); also at `internal/trace/record.go:504@c94787ea`, a normalized, unforbidden path that is tracked (or any stored-row path) breaks the `corvint-dashboard-trace-path-witness/0` lexical profile: more than 4,096 bytes, not valid UTF-8 (an encoded surrogate), a Unicode control, a backslash, or an ASCII-letter-colon prefix (decision 0235) |
| `record-failed` | `internal/tracerecordrepo/adapter.go:162@0416b61a` | the stability check or recording failed for a reason that is neither repository drift nor a verification reason |
| `record-index-failed` | `internal/tracerecordrepo/adapter.go:90@a87e1e48` | building the record index and tracked set failed |
| `secret-shaped-path` | `internal/trace/record.go:490@ba12231a` | a path matches the secret screen |
| `unnormalized-path` | `internal/trace/record.go:493@5e4376e2` | a path is absolute, contains `//`, is not `path.Clean`-equal to itself, or has a `..` part |
| `unsupported-verify-syntax` | `internal/trace/record.go:271@e427c406` | a verification command is empty or contains a byte outside ASCII letters, digits, and `_./:@=+, -` |

## Acceptance matrix

| Requirement | Deterministic evidence |
|---|---|
| `LTPM-V0-001` | commit filename/row positive; same-tree commit race before and after append |
| `LTPM-V0-002..004` | reachable commit and tree reads; unreachable and ambiguous tree fixtures; SHA-1/SHA-256 graft-file isolation; ancestry bound |
| `LTPM-V0-003,005` | byte-for-byte dry-run nonmutation; digest drift, repository drift, and missing-digest rejection |
| `LTPM-V0-006..008` | canonical/quarantine collision, mode, symlink, hardlink, special, oversize, malformed row/path, and read-race fixtures; `TestOpenRegularRejectsInRootSymlinkWithoutTargetMutation` (`internal/trace`) |
| `LTPM-V0-003,005,006,009` | `TestPlanMigrationCountsOrphanTemporaryAtCandidateLimit` proves an unrelated regex-shaped temporary consumes the bound; `TestApplyMigrationAtCandidateLimitDoesNotCountOperationLock` proves an exact staged target resumes at that bound without the confirmed operation lock or qualified temporary becoming a false overage |
| `LTPM-V0-009` | interruption after staging/quarantine/unlink boundaries, fresh dry-run, idempotent resume, byte preservation |
| `LTPM-V0-010` | fixture-root CLI dogfood and before/after fingerprint of the real trace store |
| `LTPM-V0-012` | stranded dry-run without writing, byte-preserved apply, non-commit refusal, probe failure, quarantine collision, identical-copy resume, CLI amend fixture |

## Rollout, rollback, and traceability

The corrected writer is the only future producer. The reader retains legacy compatibility while
unmigrated files exist. Migration remains operator-invoked and dry-run-first. Rollback removes the
migration command and restores the previous writer only if no dashboard consumer receives its output;
quarantined originals remain byte-preserved and are never automatically restored.

The `src/context_corvint_trace.py`/`src/corvint_cli.py` citations below are historical implementation
references, not live authority: decision 0012 R0
(`docs/decisions/0012-expert-panel-ratifications-2026-09-01.md`) rules the Python implementation
non-authoritative and slated for separate removal. The Go producer row below is the native surface.

| Requirement | Implementation | Evidence |
|---|---|---|
| `LTPM-V0-001..002` | `src/context_corvint_trace.py` | learning and trace-migration writer/reader tests |
| `LTPM-V0-003..009` | `src/context_corvint_trace.py`, `src/corvint_cli.py` | hostile migration tests and fixture-only CLI dry-run/apply tests |
| Go producer and migration support for `LTPM-V0-001..004,008,010` | `internal/trace`, `internal/tracerecordrepo`, `internal/tracemigraterepo`, `cmd/corvint` | Python-oracle success, argument, Git-failure, store-state, and write-nothing CLI tests plus Go format, bound, hostile-store, rollback, migration-transform, and `TestAccuracyTraceReadIgnoresRepositoryGrafts` / `TestAccuracyMigrationDryRunIgnoresRepositoryGrafts` SHA-1/SHA-256 tests; command inventory remains `UNSUPPORTED` pending the complete parity matrix |
| `LTPM-V0-010` | public CLI and test harness | real-store fingerprint plus fixture-only dogfood |
| `LTPM-V0-011` | `internal/trace`, `internal/tracerecordrepo`, `cmd/corvint`, `script/dogfood-change.sh`, `script/dogfood-check.sh` | shared tri-state admission tests; `TestTracePathScreenIsTheIndexScreen`; `TestRecordUnsupportedVerifySyntaxAppendsOneObservation` (verification refusal `code`) (`IDX-SNAP-V0-018` screen); mixed and source-free CLI fixtures; NUL-safe line-feed path fixture; malformed and over-bound refusal; `TestTraceTaskMustBeValidUTF8`; target/digest stability checks; shell coordination, evidence-drift, and interruption tests |
| `LTPM-V0-012` | `internal/trace`, `internal/tracemigraterepo`, `cmd/corvint` | `TestMigrationQuarantinesStrandedCommitTrace`; `TestMigrationRefusesUnknownOrCollidingStrandedTrace`; `TestMigrateTracesQuarantinesTraceStrandedByAmend` |

## Accepted amendment: dogfood changed-path admission

**Status: accepted 2026-09-01 by explicit repository-owner decision selecting design (b).** This
amendment does not change this document's accepted intent status, widen `record`, alter schema
version 1, change the Python oracle, or change a conformance fixture or manifest. Its implementation
and evidence must be added to the traceability and compatibility ledgers before the dogfood behavior
may be reported as delivered. Nothing else is newly accepted.

At revision `620c47833ef7d2455933af18988de50cbd0b25f2`, no accepted requirement owns current
`record --changed` path admission. `LTPM-V0-001` binds the producer to a stable commit/tree and
`LTPM-V0-002` validates historical paths, but neither defines the producer's current path set. The
dashboard contract owns consumer validation of schema-version-1 rows and permits any normalized,
secret-screened path resolving to a regular Git blob at the row revision; it does not make that
broader set producer authority. The Go and frozen Python producers instead both require every
supplied changed path to belong to the context index's tracked source set
(`internal/trace/record.go:238-250`; `src/context_corvint_trace.py` (historical Git `9ca27f9a62a2add263ff559fd711feea5cdfd93d`, lines 126-134)). That matching runtime
behavior is implementation evidence, not accepted authority.

The OCM-V0-013 follow-up at `5be66cb7869fe842d18e3f7e64940260490b17ad` changed `.gitignore`
and `script/dogfood-change_test.sh`. The `.sh` path is index-admissible; the documented local outcome
failed because the same `record` call also contained the non-source `.gitignore`. The original
private report is not retained as repository evidence. The observed gap is therefore broader than
a source-free change: one non-source path rejects an otherwise admissible mixed list. A genuinely
source-free diff remains the zero-path case after coordination filtering.

- `LTPM-V0-011`: public `record` semantics MUST remain unchanged. `--changed` is required
  and MUST contain at least one path; every supplied changed path MUST be a normalized member of the
  producer's exact context-index source set at the stably bound trace revision. A missing, empty,
  malformed, forbidden, untracked, excluded, or non-source changed path MUST fail closed before
  trace-store mutation. This clause claims authority for the existing narrower producer admission;
  it does not make every Git-tracked blob record-admissible.

  Dogfood coordination MAY derive a recordable subset from the exact base-to-target Git changed-path
  list. It MUST classify every candidate against the same target-revision source admission used by
  `record`, pass all and only admitted source paths as `--changed`, and preserve the complete Git
  change set in the CEM and dogfood evidence rather than representing the trace subset as a complete
  file-change ledger. The classifier MUST be shared with the producer or consume an exact bounded
  result from it; shell code MUST NOT copy or approximate the suffix, forbidden-path, generated-file,
  size, mode, or pinned-blob grammar. Repository, target, candidate-set, or classifier drift and any
  malformed candidate MUST fail closed. Only a successful classification with zero admitted source
  paths receives the exact coordination state `local-outcome NOT_PRODUCED no-source-paths`.

  `no-source-paths` is an expected terminal coordination state, not a recorded outcome and not
  evidence that verification passed. The coordinator MUST NOT invoke `record`, create a source-free
  trace row, or mutate the trace store in that state, and the state alone MUST NOT make the dogfood
  coordination or final dogfood check fail. Any other failure retains its exact reason and remains a
  failing `NOT_PRODUCED` state. An implementation commit MUST update `docs/DOGFOOD.md` step 7 and
  focused dogfood tests together; no policy, conformance, fixture, or manifest bytes change before
  owner acceptance and implementation.

  Added 2026-09-11: a `recorded` dogfood-record receipt MUST disclose `truncated_ancestry`, the
  number of ancestry rows the bounded replay probe omitted while binding the producer's revisions.
  The member is always present in that state and is `0` when the probe read the whole ancestry. It
  is derived disclosure only: it never changes admission, the recorded row, the bounded-window
  refusal text, or public `record` output, whose bytes remain unchanged.

  Added 2026-09-12 (decision 0102): the forbidden-path screen applied to supplied opened and changed
  paths, to stored-row paths on read, and to the dashboard trace path witness is exactly
  `IDX-SNAP-V0-018`'s screen, read from its one `internal/contextindex` source with no copy. A
  supplied path it excludes fails closed as `forbidden path`; a stored schema-version-1 row naming
  one (for example under `.claude/`) fails closed as a forbidden path on read. Row bytes, identity,
  and the accepted outcomes for paths outside that screen are unchanged.

  Added 2026-09-13 (decision 0235): trace path admission also applies the lexical limits of
  `corvint-dashboard-trace-path-witness/0` (`docs/specs/local-observability-dashboard-v0.md`), so the
  recorder writes no row the dashboard trace adapter refuses. A supplied opened or changed path or
  an `AdmissibleCurrentPaths` candidate that is in the tracked source set, or any stored-row path on
  read, that is more than 4,096 bytes, is not valid UTF-8 (a surrogate code unit the Python string
  decoding admits), or contains a Unicode control character, a backslash, or an ASCII letter
  followed by `:` as its first two bytes fails closed as `malformed-path`. An untracked or non-source
  candidate is still filtered, so the line-feed path fixture keeps `no-source-paths`. Git can track
  such paths on Unix; they are not recorded. Row bytes and identity for every other path are
  unchanged.

  Added 2026-09-13 (decision 0246): the trace task MUST be valid UTF-8 after trimming. A task
  holding a surrogate code unit the Python string decoding admits fails closed as `trace task must
  be valid UTF-8` in `record` before trace-store mutation, and a stored row whose task is not valid
  UTF-8 (for example the escaped `\ud800` the retired Python writer emitted) fails closed on read
  and on migration (`TestTraceTaskMustBeValidUTF8`). This matches the dashboard trace adapter, so
  the recorder writes no row it refuses. Row bytes and trace ID for every valid UTF-8 task are
  unchanged.

  Added 2026-09-13: the writer trims each verification command before it sorts and deduplicates the
  list, so one `NewRecord` pass seals a row `DecodeStore` accepts (`TestNewRecordWritesAStoreAcceptedRowInOnePass`).
  Before this, verification such as `[" z","a ","a"]` sealed the unsorted `["z","a","a"]`, which the
  reader refused as a digest mismatch. `corvint record` never wrote such a row, because its first
  validation pass already normalized the list, and every row the reader accepted keeps its bytes and
  trace ID. The only pin of the old single-pass ID was the retired-Python-writer golden in
  `internal/trace/record_test.go`, now `trim before command sort`. The validation pass stays: it
  reports a verification refusal before the outcome check and trace-store recovery.

  Added 2026-09-12: a `record` refusal of the verification command carries its bounded reason
  (`unsupported-verify-syntax`) as a leading `code` member of the stderr envelope,
  `{"code": ..., "error": ..., "ok": false}`, so `SOL-V0-007` can record it. Every other `record`
  refusal keeps the code-free `{"error": ..., "ok": false}` envelope, and exit status, stdout, and
  trace-store bytes are unchanged.

  Trace-consuming query keeps the accepted `GPK-V0-044` semantics. For a mixed diff, only the
  recorded index-source subset may contribute advisory `learned-path` candidates; filtered
  non-source paths never rank. For `no-source-paths`, no trace exists, so query MUST NOT increment
  `local_trace_count` or `matched_local_traces`, add advisory candidates or uncertainty, or otherwise
  treat the dogfood report row as learning evidence.

This is the primary design, design (b), because it closes the dogfood coordination gap while
preserving public `record` behavior, schema-version-1 bytes, the current conformance corpus, and
Go/Python oracle parity while `src/**` remains frozen until W15. It also keeps query learning tied
only to paths the current index can rank and names the honest zero-source state instead of
manufacturing a trace.

Rejected design (a), widening `record` to every Git-tracked repository path, would make revision
binding and dashboard consumption possible but would deliberately diverge from the frozen Python
oracle until W15, require a new accepted parity/conformance story, and record paths that
`GPK-V0-044` still forbids from becoming learned candidates. Rejected design (c), accepting absent
or empty `--changed` and emitting a source-free trace row, would change the public CLI contract and
both runtimes, admit a new producer state into schema-version-1 consumption, and increase trace
counts without any path that query can rank. Neither widening is needed to report the coordination
outcome honestly.

This amendment overlaps but does not answer the open dogfood CEM-policy-exception question in
`docs/agent-memory/questions.md`: filtering grants no exception and CEM retains the complete change
set. It also does not answer how the shell pipeline may append SOL observations: the existing
dogfood report row is not a new observations mutation surface. Preserving the frozen Python oracle
does not answer whether the Python verification lines should remain in `AGENTS.md`.

**Accepted owner decision (verbatim):** “proposed `LTPM-V0-011` is ACCEPTED with design (b) — public
`record` continues to require one or more index-admissible changed source paths; dogfood coordination
uses the producer's shared exact admission to record only the admissible source subset, preserves the
complete Git change set in CEM/dogfood evidence, and emits the accepted terminal state
`local-outcome NOT_PRODUCED no-source-paths` without creating a trace when that subset is empty.
Malformed input and every other admission or coordination failure remain fail-closed. Query ranks
only recorded index sources, and a source-free dogfood state is not learning evidence. No schema,
Python-oracle, conformance, fixture, or manifest change is authorized. The implementation-time
reconciliation of `docs/DOGFOOD.md` section 7 is authorized. Nothing else is newly accepted.”

## Amendment: stranded commit traces

- `LTPM-V0-012`: (accepted by the owner 2026-09-27; V1-0144) After an amend or rebase, a trace
  file named for a commit that is no longer reachable from `HEAD` makes every later dogfood pass
  refuse (`local trace store contains unreachable revision`), and no command retires it. Rule:
  dry-run treats a candidate name as stranded when it is an exact object ID, is neither a reachable
  commit nor a reachable legacy tree, and `git cat-file --batch-check` reports it as a commit
  object. A stranded file is read, bounded and split into rows under the same store bounds as other
  candidates, but not decoded. It enters the plan and its digest as kind `stranded` and is listed in
  `stranded_revisions`. Apply stages its exact bytes as `.context-corvint/legacy-traces/<rev>.jsonl`
  under `LTPM-V0-007` and `LTPM-V0-008`, verifies both copies, then unlinks the trace. A different
  quarantine copy is a collision; a failed object probe refuses; a name that is not a commit keeps
  the `LTPM-V0-004` refusal; `LTPM-V0-009` resume applies. A plan with no stranded file keeps its
  digest. The owner accepted this retirement path on 2026-09-27 instead of keeping the refusal.
  Rollback: move the quarantined file back into `.context-corvint/traces/`, and revert the change.

## Recorder source-cleanliness clarification (V1-0469, 2026-09-29)

LTPM-V0-001 and GPK-V0-007/008 govern this correction.

Recorder clarification (V1-0469, 2026-09-29): source cleanliness does not require an operator
ignore rule for the recorder's own untracked artifacts. Admission may classify only exact
recorder paths whose raw porcelain status digest matches the frozen observation: an empty
operation lock, canonical revision JSONL, and the exact descriptor-owned append temporary at
its final prepublication check. This classification requires read-only no-follow, single-link,
bounded-size checks with 0700 parent and 0600 member modes; it does not replace canonical
content, revision, snapshot, or temporary-byte validation. Tracked artifact paths, arbitrary
untracked paths, real source/commit/tree drift, and unignored pre-existing staging residue
refuse before mutation. Unignored interrupted-store recovery remains unsupported; existing
ignored-store recovery is unchanged. Neither `init`, user ignore rules, trace layout, nor
read/query status admission changes. Regression witnesses are
`TestRecordFreshRepositoryWithoutIgnore`, `TestRecordUnignoredUnsafeArtifactsRefuseWithoutMutation`,
`TestRecorderStabilityKeepsSourceAndCommitDriftVisible`, and
`TestRecordStagingAdmissionRequiresOwnedAppendPhase`.

## Amendment: trace producer provenance (V1-0745, 2026-10-04)

The owner chose the schema-3 producer field for V1-0745 on 2026-10-04 (decision 0429). This
additive profile remains experimental. For new writes it supersedes the LTPM-V0-013 rule that
the schema follows the presence of argv verification. Schema-1 and schema-2 bytes, IDs,
screening and acceptance stay unchanged.

- `LTPM-V0-015`: (owner-selected design 2026-10-04, decision 0429; requirement text proposed)
  **Writers.** Every new trace row MUST be schema 3 and MUST store a `producer` from the closed
  set below. Each writer stores its own value:

  | `producer` | Writer |
  |---|---|
  | `cli` | `record` |
  | `dogfood` | dogfood recording and the aggregate-outcome validator row |
  | `pi-tool` | the pi tool adapter |

  A writer given any other value, including `UNKNOWN`, the empty string or a case variant, MUST
  refuse before mutation.

  **Row shape.** A schema-3 row has the schema-1 fields plus `producer`. Its `verification` keeps
  one of two shapes:
  - the schema-1 sorted command-string array, when no argv entry is present;
  - the LTPM-V0-013 typed entries, when an argv entry is present.

  **Identity.** `producer` occupies its sorted position in the canonical basis, so the trace ID
  covers it. The same observation recorded by two producers has two distinct IDs.

  **Decoding.** Schema-3 rows use the strict typed decoder, which refuses duplicate members, and
  report row-shape and producer failures with the `schema-3 local trace:` prefix; a malformed typed
  verification entry keeps the existing `unsupported-verify-argv` refusal and message. The decoder
  MUST refuse:
  - a schema-3 row whose `producer` is missing, non-string, `UNKNOWN` or outside the set;
  - a schema-1 or schema-2 row that carries `producer`;
  - an unknown member;
  - a typed verification without an argv entry;
  - a mixed verification array;
  - a producer that is not covered by the stored ID.

  **Reading legacy rows.** Readers MUST report schema-1 and schema-2 rows as producer `UNKNOWN`,
  a read-side label that is never stored.

  **Consumers.**
  - Every consumer that LTPM-V0-014 names MUST admit validated mixed v1/v2/v3 revision files under
    the unchanged row, file and store bounds.
  - Query, impact and eval ranking MUST NOT use `producer`.
  - `record` and batch output MUST include `producer` for schema-3 rows.
  - The console chain detail and skill export MUST label it.
  - The dashboard MUST NOT disclose it (`LOD-V0-035`).
  - Migration and stranded-row recovery MUST preserve it.

  **Older binaries.** A binary that predates this profile, including N-1 0.8.1, MUST refuse a store
  containing a schema-3 row rather than misread it; a reader without schema-3 support refuses an
  unsupported schema. This is an explicit release limitation, as LTPM-V0-014 recorded for
  schema 2.
- `LTPM-V0-016`: (owner-selected design 2026-10-04, decision 0429; requirement text proposed)
  `calibrate` and `eval` MUST accept repeatable `--exclude-producer NAME`, where `NAME` is one of
  `cli`, `dogfood`, `pi-tool` or `UNKNOWN`. Repeats deduplicate. Any other value refuses with the
  `invalid-arguments` shape and exit code 2. This amends the flag list of `OCL-V0-001`.

  **`calibrate`.**
  - The report MUST carry `producers`, which counts every record read by producer. All four keys
    are always present, including zeros.
  - When any producer is excluded, the report MUST carry `excluded_producers` in the fixed order
    above.
  - Exclusion applies before `--since` or `--window` selection.
  - The table format prints both lines.

  **`eval`.**
  - The local arm carries the same counts as `trace_producers`, plus `excluded_producers`. It does
    so only when the store held a trace or an exclusion was requested, so an empty-store evaluation
    keeps its exact baseline bytes (`LTA-V0-001`).
  - A fixture's `learned_trace_arm` always carries them.
  - Exclusion removes those producers' traces from the traces that this read admits: the fixture
    arm and the live-store passed-trace check. A store whose passed traces all come from excluded
    producers therefore evaluates instead of refusing. The report names the exclusion, and the
    unexcluded refusal of `LTA-V0-001` is unchanged.
  - `--learn-slot-weights` MUST refuse the flag.

  Neither command writes a trace, fixture, index or ledger byte; exclusion never changes stored
  records.

Acceptance: `TestLTPMV0015ProducerGoldens` (goldens computed independently over spec-authored
JSON), `TestLTPMV0015ProducerRefusals`, `TestLTPMV0015LegacyRowsReadAsUnknown`, `TestLTPMV0015MigrationKeepsProducer`,
`TestRecordTypedArgvRoundTripAndRefusal`, `TestLODV0035MixedTraceSnapshot` (v1, legacy v2 and
v3 rows in one physical member), `TestProducerTraceConformance` (the standalone verifier and the
actual dashboard producer), `TestCalibrateCountsAndExcludesProducers_LTPM016` and
`TestEvalCountsAndExcludesProducers_LTPM016`. Live check, 2026-10-04: a schema-3 row written by
this build made the installed pre-profile binary (1.0.0-rc.1, build 163) refuse both `query` and
`calibrate` with `unsupported-query-trace-state` ("invalid local trace fields"). Moving that
revision file aside restored the older binary.

Rollback: revert the change, so that new writes return to schema 1 or schema 2. Existing schema-3
rows are not rewritten. Before an older binary reads the store, move each whole revision file
containing a schema-3 row out of `.context-corvint/traces/`; `grep -l '"schema_version":3'` lists
them. To restore them, move them back under a binary that has this profile. No migration rewrites
schema-3 rows to an older schema, because the producer is part of each row's identity.
