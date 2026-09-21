# Learned Trace Admission V0

Owner: Russell Lewis
Date: 2026-09-05
Status: accepted
Intent status: accepted (decision 0058)
Delivery status: experimental
Requirement prefix: `LTA-V0`
Authoritative inputs: `docs/decisions/0058-learning-loop-evaluation-gate-and-retention-2026-09-05.md`;
`docs/decisions/0037-release-owner-calls-2026-09-03.md` item 4; AGENTS.md invariant 5;
`docs/specs/go-production-kernel-migration-v0.md` `GPK-V0-044`; and
`docs/specs/local-trace-producer-migration-v0.md`.

## Agent digest
- Claim: Learned-path mechanism changes require a pinned two-arm gate, while `record` reclaims unchanged trace-store caps by deterministic whole-file eviction.
- Status: accepted (decision 0058); experimental.
- Exists: the local trace writer and reader, Python-parity learned-path query behavior, and the baseline frozen-corpus evaluation arm.
- Blocked on: a first registered development run under both engines; blind-v4 remains sealed.
- Read next: `LTA-V0-001`, `LTA-V0-002`, then `LTA-V0-003`.

## User and measurable job

A maintainer must be able to measure whether enabling the learned trace path harms retrieval before
changing its shipped scoring configuration. A local operator must also be able to keep recording
within the existing bounded store without wall-clock-dependent or partial-row retention.

Success is a reproducible baseline-versus-fixture report over the same pinned cases, an executable
contamination refusal, and deterministic reclamation that never mutates stored rows. The evaluation
admits or rejects the learned-path mechanism at build time; it does not admit a user's store or any
individual row.

## Verified current state

At decision 0058's accepted base, `corvint eval` produced the baseline report but refused when a
passed local trace existed, so the learned path had never been scored. The trace writer refused at
the 1,000-row, 1,000-file, or 16 MiB store cap and reclaimed nothing. The writer and reader secret
screens shared the assignment-pattern vocabulary, which did not include `credential`,
`credentials`, `passphrase`, or `passwd`.

## Requirements

- `LTA-V0-001`: `corvint eval` MUST score a baseline arm and a second learned-trace arm over the
  same pinned corpus and golden cases when the evaluation runner supplies the registered frozen
  trace fixture. Its delta block MUST report, in order: critical evidence misses, which MUST NOT
  rise; abstention accuracy and epistemic-state accuracy, which MUST NOT fall; development
  serialized-result byte-weighted precision against the 0.80 floor under both engines; then
  recall and top-five task success as informational metrics. A change smaller than two underlying
  cases MUST be classified exactly `not distinguished`, never improved or regressed. An evaluation
  without a fixture MUST retain the existing baseline bytes, and an unfixtured live store holding a
  passed trace MUST retain its refusal. Learned-path score constants MAY change only after this gate
  reports no harmful delta and the development precision floor holds under both engines.
- `LTA-V0-002`: Before either scored arm runs, the evaluation MUST fail closed when any frozen
  fixture trace has the same canonical task as a scored case or names the scored repository's
  outcome commit as its revision. The fixture path and digest MUST be registered and verified; a
  warning, omitted case, or post-score contamination label is insufficient.
- `LTA-V0-003`: At an unchanged 1,000 rows, 1,000 files, or 16 MiB cap, `record` MUST reclaim space
  only by deleting complete trace files and MUST order candidates deterministically as follows:
  revisions unreachable from HEAD or outside the bounded ancestry window first; then reachable
  files containing no `passed` row, farthest from HEAD first; then all remaining files, farthest
  from HEAD first. Equal-distance candidates MUST use revision filename order. `record` MUST never
  evict the file being appended to. If that file alone prevents admission, `record` MUST fail
  closed with a recovery message. No read command may evict, and a below-cap store MUST be unchanged.
- `LTA-V0-004`: The current writer screen (`internal/secretscreen.Pattern`, consumed by `record`
  admission and by `query` history-candidate screening) MUST also match, in addition to the shapes
  above: a bare assignment of the credential vocabulary whose value is single- or double-quoted,
  consuming the complete lexical string under the same whole-value and unterminated-through-EOF
  rules as the quoted JSON property; the bare `pass` vocabulary stem only when followed by an
  assignment operator, except on a complete Go verbose-test marker line shaped as
  `[whitespace]--- PASS: TestName (seconds)`; `whsec_`, `hf_`, `dop_v1_` and `xapp-` tokens at or above their length
  floors, and Slack incoming-webhook URLs; an AWS access-key ID immediately followed by its
  40-character secret, redacted together; and an `authorization` assignment whose value is an HTTP
  scheme word (`basic`, `bearer`, `digest`, `negotiate`, `ntlm`, `token`) followed by a
  space-separated credential, redacted together. Decision 0164 adds: an `account[_-]?key`
  assignment (Azure `AccountKey=`); Google `ya29.` and `GOCSPX-`, Shopify `shpat_`/`shpss_`/`shpca_`
  and GitLab `glrt-`/`gldt-` tokens at or above a 20-character floor; a credentialed URL whose
  password contains a raw `@`, redacted through the last `@` before whitespace, `/`, `?`, `#` or a
  quote, so the match never ends inside a later secret (the user part stops at the same characters,
  so `https://host?x="a:b@c d"` is not a credentialed URL); URL userinfo with no password that carries a known vendor-token prefix at any length or is at least 32 letters
  and digits (plain user names such as `git@` stay unmatched); a credential-vocabulary flag
  (`--password`, `-token`, `--db-password`) followed by a separate argument, where a single- or
  double-quoted argument is consumed as one complete lexical string (through EOF when unterminated)
  and a bare argument ends at whitespace or an unescaped quote, so a following JSON property is never
  cut in half; a bare or double-quoted argument starting with `$` is a variable reference and MUST
  NOT match; a `-p` separate or
  `=`-joined argument after `login`, `-u` or `--user` on the same line; and `curl -u`/`--user`
  with a bare or whole quoted `user:password` pair, where the flag starts an argument (a `-u` inside a hyphenated host
  such as `svc-users:8080` MUST NOT match). A flag argument counts only when it contains both a letter and a
  digit, so commit prose such as `add --token flag`, `docker login -p value` and git's
  `log -p HEAD~3` MUST NOT match. `record` MUST refuse such input and `query` MUST
  drop such a history candidate. `StoredV1Pattern` and the dashboard readers MUST NOT gain these shapes. The
  retired Python oracle's narrower screen is not a compatibility target (decision 0092).
- `LTA-V0-005`: (numbered 2026-09-12 by decision 0093; text unchanged from the accepted
  trust-boundary paragraph) An ordinary Go trace read distinguishes a candidate proven to predate the
  bounded replay window from a revision that is unreachable from HEAD. The former has read-failure
  kind `replay-window`, uses the diagnostic `candidate revision predates the bounded replay window`,
  and discloses the number of ancestry rows truncated from the bounded probe. Repository query still
  maps that kind to `unsupported-query-trace-state`, preserving `GPK-V0-044` semantics. Only `record`
  may pass such a candidate into `LTA-V0-003`'s unreachable-first retention tier; below the cap it
  remains a refusal.

## Non-goals and simpler baseline

The simpler baseline is the empty-trace evaluation arm and hard refusal at the existing store cap.
V0 does not evaluate or admit an individual user's store or row, change trace schema v1, change the
three caps, tune learned-path constants, authorize blind-v4 observation, add wall-clock retention,
rewrite a row, change dashboard bytes, add `auth`, or add an entropy heuristic.

The current secret detector in `internal/secretscreen` and the shipped Python oracle includes
`credential|credentials|passphrase|passwd`. The 2026-09-06 user-authorized audit repair also detects
double-quoted JSON property names containing the existing credential vocabulary, followed by a
colon and a value. For an ordinary double-quoted value, the match MUST consume the complete
lexical string, including whitespace and backslash-escaped characters, so redaction cannot retain
a credential tail. An unterminated quoted value consumes through absolute end of input, including
newlines and a dangling backslash; the malformed remainder is conservatively redacted. Complete
quoted values preserve following benign fields. Non-quoted values retain the existing non-whitespace match. This is a lexical
screen, not a JSON parser or general secret discovery mechanism: escaped property names and
decoding of encodings remain outside this bounded repair.

The 2026-09-12 writer-screen repair extends only the Go writer (`internal/secretscreen.Pattern`),
not `StoredV1Pattern` and not the since-retired Python oracle, closing four gaps: a bare (non-JSON) assignment
whose value is single- or double-quoted now consumes the complete lexical string instead of
truncating at an internal space, matching the same whole-value and benign-following-field guarantees
as the quoted-JSON-property case above; the credential vocabulary gains a bare `pass` stem alongside
`passphrase|passwd|password`; the vendor-token alternation gains `whsec_`, `hf_`, `dop_v1_`, and
`xapp-` prefixes plus Slack incoming-webhook URLs (which carry no `@`, so the credentialed-URL rule
does not reach them); and an AWS access-key ID (`akia|asia`) immediately followed by what is shaped
like its paired 40-character secret access key redacts both together, where previously only the key
ID was covered. `LTA-V0-004` states these four as a requirement. They stay out of the frozen
stored-v1 matcher; the Python oracle that lacked them is retired (decision 0092).

The dashboard adapter and frozen
dashboard-snapshot reader remain unchanged until row-level read rejection is separately authorized,
so a writer rejects those new assignments while either reader may still accept a previously stored
row containing one. The shared parity corpus therefore has a stable common section on which all four
copies agree and an explicit writer-only section for this accepted asymmetry; it must not disguise
the asymmetry as four-way parity.

Schema v1 has no writer-screen version marker, so Go and Python stored-row validation also retain
the pre-expansion assignment vocabulary for every existing v1 row, including commit-named,
tree-named, and unreachable retention candidates. Stored-v1 matchers retain the exact pre-expansion pattern, including its quoted-property gap;
the writer rejects new quoted credential fields without retroactively invalidating an old row
whose digest and all other v1 fields remain valid. The current detector also filters Git-history
subjects and candidate paths at query time in Go and Python, and `Screen` redacts observation-ledger
fields. Those existing consumers receive the same expansion; filtering a history candidate is not
stored-trace rejection or a scoring-rule change. No read operation rewrites a historical row.

## Trust boundary, limits, and failure modes

The fixture is repository-owned, immutable input to a local evaluation. It grants no authority to
trace contents. Fixture digest drift, task contamination, outcome-commit contamination, a changed
repository revision, or a changed history fails the run before a delta is accepted. A real local
store is never substituted for the fixture.

Eviction occurs only inside the explicit `record` mutation boundary. An unreadable candidate,
ancestry failure, unsafe file, malformed row, append-target-only overflow, or inability to establish
the required order fails closed without partial-file rewriting. The existing reader, row, file, and
byte bounds remain authoritative. Clarification (2026-09-13, bug hunt): interrupted-append recovery
is part of that mutation boundary. After acquiring and confirming the trace-operation lock, `record`
MUST revalidate repository stability before it removes a staged target, restores an eviction, or
clears a published eviction; drift leaves those recovery bytes unchanged and refuses the append.

The bounded replay-window read distinction is `LTA-V0-005`.

## Acceptance evidence and testing matrix

| Requirement | Deterministic acceptance evidence |
|---|---|
| `LTA-V0-001` | baseline output without a fixture remains byte-identical; a registered fixture emits both scored arms and the ordered delta; one-case metric changes render `not distinguished`; an unfixtured passed live store retains its refusal |
| `LTA-V0-002` | exact-task and outcome-commit fixture contaminants each fail before scoring; registered fixture digest mismatch fails |
| `LTA-V0-003` | a cap fixture evicts whole files in all three ancestry/outcome tiers, preserves the append target and unchanged row bytes, and fails with recovery guidance when only that target can be reclaimed; `TestAppendRepositoryDriftPreventsInterruptedRecoveryMutation` proves repository drift cannot mutate staged recovery bytes |
| `LTA-V0-005` | a candidate older than the bounded replay window reads as kind `replay-window` with the exact diagnostic and truncated count, distinct from an unreachable revision |
| writer/stored-reader screen compatibility | one shared fixture corpus proves the four baseline patterns agree; new assignment-key and quoted-property cases reject in both writers while Go/Python stored-v1 validation and both dashboard readers retain their previous result; benign quoted properties remain admissible, Git-history screening uses the current detector, and ledger output redacts the synthetic value |

Blind-v4 is not acceptance evidence for this spec. A first development result is first-observation
evidence and must be registered before it is read; it cannot retroactively change these rules.

## Rollout, maintenance, and drift

Ship the frozen fixture and its digest with the evaluation runner, then run both engines against the
development corpus. Keep the baseline and fixture on identical request, revision, and intent inputs.
Any fixture edit changes its registered digest and requires a fresh contamination check and result.
Any learned-path scoring or floor change requires a fresh `LTA-V0-001` report.

`GPK-V0-044` continues to describe shipped Python-parity semantics. A Go/Python behavior difference
must move the oracle in the same change or enter `conformance/divergence-register.md`; it is not
repaired by silently changing the oracle after evaluation.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `LTA-V0-001` | `internal/evalrepo`, `cmd/corvint`, `benchmarks/run.py` | focused eval repository, CLI, and benchmark-runner tests |
| `LTA-V0-002` | `internal/evalrepo`, frozen trace fixture registration | focused task-contamination, outcome-commit-contamination, and digest-drift tests |
| `LTA-V0-003` | `internal/trace`, `internal/tracerecordrepo` | focused cap-order, whole-file, append-target, and bounded-ancestry tests |
| `LTA-V0-004` | `internal/secretscreen.Pattern`, consumed by `internal/trace` record admission and `internal/contextindex` `containsSecret` | `TestSecretPatternParityCorpus` (writer-only rows, stored-v1 non-match, and length-floor, bare-`pass` and hyphenated-host curl non-matches), `TestGoVerbosePassMarkerBoundary`, `TestScreenConsumesWholeQuotedAssignmentValue`, `TestScreenRedactsAWSSecretAdjacentToItsKeyID`, `TestScreenRedactsCredentialAfterAuthorizationScheme`, `TestScreenRedactsWholePasswordContainingAtSign`, `TestCredentialedURLPasswordStopsAtQueryFragmentOrQuote`, `TestLTAV0004RecordRefusesWriterOnlySecretShapes`, `TestSecretPatternMatchesHistorySecretShapes` |
| `LTA-V0-005` | `internal/tracerecordrepo` | `TestReadBoundsTraceReplayWithoutRefusingLargeRepositories` (subtest `candidate outside bounded replay`) |
| writer/stored-reader screen compatibility | `internal/secretscreen`, `internal/trace`, `src/context_corvint_trace.py`, `internal/dashboard/adapters/trace.go`, `conformance/dashboard-snapshot-v0/trace_corpus.go` | `TestSecretPatternParityCorpus`, `TestQuotedCredentialsRejectNewRecordsButRetainStoredV1`, `TestSecretPatternMatchesHistorySecretShapes`, `TestAppendRedactsQuotedCredentialPath`, `TestScreenConsumesWholeQuotedAssignmentValue`, `TestScreenRedactsAWSSecretAdjacentToItsKeyID`, Python `SecretPatternParityTest` and `CorvintLearningTest.test_trace_inputs_fail_closed`; stored-v1 compatibility and intentional-asymmetry tests |

## Rollback

Remove the fixture registration and second arm, restoring the baseline-only report and live-store
refusal; remove the eviction branch, restoring hard refusal at the cap; and revert both writer
screen additions together. The reader copies remain unchanged. Rollback deletes no trace, rewrites
no row, mutates no frozen partition, and requires no data migration.

## Unresolved decisions and promotion or kill criteria

No product decision remains open inside this V0 charter. Promotion to implemented requires every
acceptance row above to pass. Promotion to validated additionally requires the registered
development gate under both engines; blind-v4 may be opened only under its separate preregistered
condition. Kill or reverse the second arm if it cannot run locally without a network, daemon, or
mutable external database, or if the arms alter request, revision, or intent inputs.
