# CEM Additive Profiles V0

Owner: Russell Lewis
Date: 2026-10-28
Intent status: experimental under the standing lane auto-accept
Decision: `https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-28-cem-additive-profiles.md`
Delivery status: experimental
Authoritative inputs: `docs/specs/cem-stable-v1.md`,
`docs/specs/test-claim-qualification-v0.md` (`TCQ-V0-051..054`),
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-28-cem-witness-paired-range.md`,
`https://github.com/beamfall/corvint-history/blob/1c4d8793cf71cbb01b8509b68a6cb1502a40a13e/docs/build-log/2026-10-28-paired-range-prove-refusal.md`, GitHub #1948, #1950, #1941 and #1981, `docs/specs/corvint-witness-v0.md` (`AGW-V0-041..045`)

## Agent digest
- Claim: what the frozen `cem/1.0` wire cannot carry ships as opt-in, separately versioned profiles a `cem/1.0` consumer can ignore.
- Status: experimental under the standing lane auto-accept; delivery experimental
- Exists: the `cem/v3` attestation predicate with per-disposition counts and applied ceilings (`CAX-V0-001`); the `cem-coverage-states/1` document with a distinct `not-instrumented` state (`CAX-V0-010..011`); the `cem-non-regular/1` profile, under which a range that adds or retypes a symbolic link (mode `120000`) or another non-regular entry maps every regular-file hunk and records each non-regular path as an explicit, never-followed unknown row, agreed by the native and portable readers on one shared vector (`CAX-V0-020..024`); the `cem-paired-evidence/1` document, which cites a second repository's pinned range for a map's hunks, its `cem cite`/`cem status` flags, and the signed `cem/v3` `pairedRange` member (`CAX-V0-030..034`); the `cem-path-policy/1` profile, under which `cem prepare` and `cem status` read the witness path policy from the base tree and report each unknown hunk on a declared inert path as `declared-inert`, counted apart from `--max-unknown` (`CAX-V0-060..065`).
- Blocked on: nothing.
- Read next: Requirements; Non-goals; Failure modes.

## Boundary

`cem/1.0` is closed: the map's root members are closed, its coverage witness has exactly the states
`covered` and `uncovered` (`TCQ-V0-052`), the `cem/v2` predicate is `{baseRevision, patchSha256,
target}`, and the canonical patch admits only modes `100644` and `100755`, refusing every other
changed path by name (`CEM-V1-655`). Every profile below is named by its own identifier, is produced
only on an explicit operator flag, and leaves the `cem/1.0` result of a consumer that does not opt in
byte-identical.

## Requirements

### Attestation predicate (#1948 item 1)

- `CAX-V0-001`: `corvint prove --cem MAP --attest-cem-v3` signs the predicate type
  `https://corvint-context.dev/attestation/cem/v3`: the `cem/v2` predicate members plus
  `counts {total, supported, unknown, mechanical}`, counted from the map exactly as `cem status`
  counts them, and `ceilings` naming exactly the applied `--max-unknown` and `--max-mechanical`
  (an empty object when none applied). Signing refuses a map whose counts exceed an applied ceiling.
  The verifier refuses a predicate with any unknown member at any level, counts that do not sum to
  `total`, a negative count or ceiling, counts above the signed ceilings, and, when a map is
  supplied, counts that differ from that map's counts. A `cem/v2` statement is unchanged and a
  `cem/v2`-only verifier refuses a `cem/v3` envelope by type.

### Coverage states (#1948 item 3)

- `CAX-V0-010`: `corvint cem cover ... --states-output PATH` also writes one
  `cem-coverage-states/1` document `{hunks:[{id,state}], mapSha256, profile, profileSha256,
  testRun}` in canonical form: one entry per hunk in map order, `state` one of `covered`,
  `uncovered` or `not-instrumented`, where `not-instrumented` is an uncovered hunk whose path no
  block of the coverprofile names. `mapSha256` is the SHA-256 of the exact map bytes written; the
  map and its two-state witness are unchanged by the option.
- `CAX-V0-011`: `wire.ValidateCoverageStates(document, mapBytes)` accepts a document only when it is
  closed and canonical, names the profile, binds `mapSha256` to the supplied bytes, lists every map
  hunk once in order, every hunk carries the witness from the same `profileSha256` and `testRun`,
  and each state agrees with that witness (`covered` only for covered; `uncovered` or
  `not-instrumented` only for uncovered). Any failure refuses the whole document.

### Non-regular entries (#1950, V1-2974, V1-2964)

- `CAX-V0-020`: the `cem-non-regular/1` profile derives its patch from the proved canonical patch by
  removing every section (split before each line that begins `diff --git `) of a changed path whose
  base or target entry is not a regular file (`100644`/`100755`). Producers (`cem prepare`, the
  cite fallback) and both readers derive it the same way; the map pins the SHA-256 of that
  regular-only patch. A link is never followed or resolved: its target bytes never appear in a
  result. On a range with no non-regular path the profile excludes nothing and the result equals
  the plain `cem/1.0` result. A range whose every changed path is non-regular is refused
  `unsupported-tree-mode`.
- `CAX-V0-021`: each non-regular path is reported once as the row `{path, baseMode, targetMode,
  disposition:"unknown"}` (`absent` for a missing side), sorted by path, beside the `cem/1.0`
  result; the result object and its `verdictSha256` are unchanged by the rows.
- `CAX-V0-022`: under the profile `cem status` counts each row as one `unknown` change and adds
  `counts.nonRegular`, so a non-regular path is never silently treated as covered.
- `CAX-V0-023`: `--non-regular unknown` opts in on `cem prepare`, `cem status`, `cem verify` and
  `cem verify-stable` (native) and on `verify-stable-canonical` (portable). The verify-stable forms
  print `{nonRegular, nonRegularProfile:"cem-non-regular/1", stableVerification}`; any other value is
  refused `invalid-arguments` before repository work. Without the flag every command keeps its
  `cem/1.0` behaviour: prepare refuses the range (`CEM-V1-655`) and a reader refuses a profile map
  `patch-digest-mismatch`, so the map is fail-closed for a reader that does not opt in.
- `CAX-V0-024`: the shared vector `interop/cem01-go/testdata/non-regular/` (a recipe with `links`,
  materialized at mode `120000`; the profile map; `EXPECTED.json`) is asserted by both readers: the
  opted-in case ACCEPTs with the regular-only patch digest, the mapped regular paths and the rows;
  the plain case REJECTs `patch-digest-mismatch`. Existing `cem/1.0` vectors are unchanged.

### Paired range (#1941, V1-2936)

A `cem/1.0` map binds one repository's base and target, and `CEM-V1-648` still refuses a paired
change (`--range`, `--range-root`, `--paired-from`) on every `cem` verb. The paired range is
therefore a separate document bound to the map's change, never a map member.

- `CAX-V0-030`: the profile `cem-paired-evidence/1` is a canonical JSON document `{change
  {baseRevision, patchSha256}, citations, profile, range {baseRevision, headRevision, name}}`. It is
  produced only by an explicit flag, never changes the map's bytes, and a `cem/1.0` consumer that
  never reads it loses nothing. `cem prepare`, `cite`, `status`, `verify` and `prove` without the
  paired flags are unchanged, and the interop readers stay `cem/1.0`-only.
- `CAX-V0-031`: a reader refuses the whole document when it exceeds the map byte bound, has an
  unknown member, is not in canonical form (sorted members, no insignificant whitespace, one
  trailing newline, citations sorted by hunk, path, start, end, relation and unique), names another
  profile, lacks a full commit ID or SHA-256 in `change`, has a range name outside 1..64 bytes of
  `[a-z0-9._-]` starting alphanumeric, a non-full or equal range base and head, no citation, or a
  citation with an invalid path, blob ID, span digest or relation, an empty hunk ID, or a span
  that is empty, negative or longer than the `cem/1.0` citation span bound. Binding to a map refuses
  a document whose `change` differs from the map's or whose citation names a hunk the map lacks.
- `CAX-V0-032`: `cem cite --paired-output DOC --paired-range NAME=BASE..HEAD --paired-root ABS`
  (all three, or `invalid-arguments` naming the missing ones; `--output` is refused beside them)
  resolves both range ends in the repository at the absolute `--paired-root` to full commit IDs,
  refuses an end that does not resolve by naming it, an equal base and head, and a base that is not
  an ancestor of head; reads `--evidence-path` as a regular blob at head (else `missing-evidence`);
  compiles the span as `cem cite` does; and adds the citation, under the map path's lock, to `DOC`,
  creating it bound to the map's change. An existing `DOC` pinning another change or range is
  refused, a repeated citation reports `added: false`, and `DOC` may not be the map or its lock or
  backup name.
- `CAX-V0-033`: `cem status --map MAP --paired DOC --paired-root ABS` (both, and no other status
  option) parses and binds `DOC` to `MAP` (`invalid-field` on failure), re-resolves the pinned
  range in the repository at `--paired-root` and, for every citation, requires the same blob at
  head, the span in bounds and the same span digest; any difference refuses `evidence-drift`. It
  writes nothing and reports the document's SHA-256, the range and `counts {citations,
  pairedHunks, total, unknown, unknownWithPairedEvidence}`.
- `CAX-V0-034`: `corvint prove --attest-cem-v3 --paired-evidence DOC` (refused without
  `--attest-cem-v3`) parses and binds `DOC` to the map and signs the optional `cem/v3` member
  `pairedRange {name, baseRevision, headRevision, evidenceSha256}`, the last being the SHA-256 of
  the document's exact bytes. The verifier refuses a `pairedRange` with an unknown member, an
  invalid name, a non-full or equal base and head, or a malformed digest, and reports it in the
  receipt. A `cem/v3` statement without the flag carries no `pairedRange`.

### Declared path policy (#1981, V1-3085)

A range touching only a PR template, a locale catalogue or CI configuration maps every hunk
`unknown`/`no-evidence`, so `cem status --max-unknown 0` cannot tell it from a behavioural change.
`cem/1.0` has no disposition for "declared harmless" and its bytes are frozen, so the declaration is
read from the witness path policy (`AGW-V0-041`) and reported beside the map and verifier result,
never inside them.

- `CAX-V0-060`: `--path-policy base` opts `cem prepare` and `cem status` into the profile
  `cem-path-policy/1`; any other value is refused `invalid-arguments`, and `cem verify`, `cem report`
  and `cem status --paired` do not admit the flag. The map `cem prepare` writes is byte-identical with
  and without the flag, and the verifier result, its `verdictSha256` and every plain `cem/1.0`
  reader (native and `interop/cem01-go`) are unchanged: the profile adds only the envelope members
  `pathPolicyProfile`, `pathPolicy` and `pathPolicyHunks`, and `counts.declaredInert`. Under the flag
  `cem prepare`'s next action carries `--path-policy base`.
- `CAX-V0-061`: the policy is the blob `.corvint/witness-paths.json` (`corvint-witness-paths/0`) in
  the range's **base** tree, decoded by the decoder `corvint witness` uses (`internal/cem/witnesspaths`, `AGW-V0-041`) and read through
  the map repository's bounded object reads, never the worktree or the
  target tree, so neither an uncommitted edit nor a range that edits the policy changes the result.
  `cem prepare` reads the resolved `--base`; `cem status` reads the map's base only after the stable
  verifier accepts, else `pathPolicy.status` is `NOT_PRODUCED` and nothing is classified.
  `pathPolicy` is `{path, status, detail, rules}` (`BOUND` or `INVALID`, an invalid policy
  classifying nothing), or `null` when the base holds no policy.
- `CAX-V0-062`: a rule classifies a map hunk only when its disposition is `unknown`, its path is not
  the policy file, and no extractor reads its path (the witness `LANGUAGE_COVERAGE_NONE` set); the
  first matching rule in file order applies. A `supported` or `mechanical` hunk, a path an extractor
  reads (a behaviour change in code, whatever glob matches it) and an undeclared path are never
  listed and keep today's disposition and count.
- `CAX-V0-063`: an `inert` rule's hunk is the row `{selector, id, path, state:"declared-inert",
  reason:"declared-inert", declaration:{file, line, glob, class}}`, `line` being the 1-based line where
  the rule object starts. `cem status` moves each such hunk from `counts.unknown` to
  `counts.declaredInert` (present, possibly 0, whenever the flag is given), so `--max-unknown` does not
  bound it; `total` is unchanged.
- `CAX-V0-064`: a `data` rule's hunk stays `unknown` and counted as unknown. Its row carries
  `state:"unknown"`, the declaration with `readers` (each `{path, origin}` with the witness origins and
  reasons, `AGW-V0-044`, found in the target revision's index by the reader search `corvint witness` uses), and `reason` `data-reader` with
  `detail` `read by <path> (<origin>), ...` naming every counting reader, `data-reader-missing` when
  none counts, or `data-reader-not-produced` with the failure when the target index cannot be
  built or no reader search is installed (a library caller that installs none).
- `CAX-V0-065`: fixtures over a template range (`.github/*.md`), a locale range (`config/locales/`,
  one catalogue quoted by a Go source and one not), a CI-config range (`.circleci/config.yml`,
  `package.json`), a code mutant under an inert glob, and a range that edits the policy itself with
  an uncommitted worktree edit, assert the rows, counts and ceilings above.

## Non-goals

- No `cem/1.0` wire, schema, vector, or predicate byte changes; no new state inside the `cem/1.0`
  coverage witness.
- The non-regular map is not self-describing: a reader must be told to opt in. `cem discriminate`,
  the legacy `verify.go` path, external mutators, the git-notes anchor, the portable `verify-stable`
  (non-canonical) and `verify-candidate`, and every other canonical-diff caller stay `cem/1.0`-only.
- No mode other than `unknown`: links are never followed, resolved, or cited.
- The path-policy profile adds no `cem/1.0` disposition or reason and never upgrades a hunk to
  `supported`; it does not implement path-scoped check modes (V1-0420), and a policy outside the base
  tree is never read.
- The paired profile does not lift `CEM-V1-648`: no `cem` verb accepts a paired change or a second
  base/target, and the map carries no paired member. Signing does not re-read the second
  repository; `cem status --paired` is the check that its citations still hold.

## Failure modes

| Failure | Behaviour |
|---|---|
| a plain reader is handed a profile map | `REJECT` `patch-digest-mismatch` (fail-closed) |
| `--non-regular` other than `unknown` | `invalid-arguments` before repository work |
| every changed path is non-regular | `unsupported-tree-mode` |
| `cem/v3` counts disagree with a supplied map or exceed signed ceilings | verification refused |
| a coverage-states document edited, reordered or bound to other bytes | whole document refused |
| a paired range end does not resolve, or base is not an ancestor of head | refused naming the end, or `invalid-arguments` |
| a paired-evidence document edited, unsorted or bound to another change | whole document refused |
| a paired citation's blob or span changed at the pinned head | `evidence-drift` |
| `--path-policy` other than `base`, or on verify/report | `invalid-arguments` |
| the policy is malformed or not a blob in the base tree | `pathPolicy.status` `INVALID`, nothing classified |
| the stable verifier does not accept the map | `pathPolicy.status` `NOT_PRODUCED`, nothing classified |
| the target index for a data path's readers cannot be built | the hunk stays unknown, `data-reader-not-produced` |

## Acceptance evidence

| Requirement | Implementation | Evidence |
|---|---|---|
| `CAX-V0-001` | `internal/attest/cem_v3.go`, `cmd/corvint/prove_attest_cem.go` | `TestCAXV0001StatementAddsCountsAndAppliedCeilings`, `TestCAXV0001RefusesExceededCeiling`, `TestCAXV0001VerifiesCountsAgainstTheMap`, `TestCAXV0001PredicateIsClosed`, `TestCAXV0001ProveAttestCEMV3CountsAndCeilings` |
| `CAX-V0-010..011` | `internal/cem/workflow/cover.go`, `internal/cem/wire/coverage_states.go` | `TestCAXV0010CoverWritesNotInstrumentedState` |
| `CAX-V0-020` | `internal/cem/gitauth/nonregular.go`, `internal/cem/workflow/commands.go`, `internal/cem/verify/stable.go`, `interop/cem01-go/non_regular.go` | `TestCAXV0020SplitPatchSectionsCutsOnlyAtHeaders`, `TestCAXV0020PrepareMapsRegularHunksAndRecordsLinks`, `TestCAXV0024ProfileLeavesRegularRangesUnchanged`, `TestCAXV0024PortableProfileLeavesRegularRangesUnchanged` |
| `CAX-V0-021..022` | `internal/cem/workflow/read.go`, `internal/cem/workflow/stable_read.go` | `TestCAXV0020PrepareMapsRegularHunksAndRecordsLinks` |
| `CAX-V0-023` | `internal/cem/cli/cli.go`, `internal/cem/cli/stable.go`, `interop/cem01-go/stable_repository.go` | `TestCAXV0023VerifyStableNonRegularEnvelope`, `TestCAXV0020PrepareMapsRegularHunksAndRecordsLinks`, `TestCAXV0024PortableReaderAgreesOnNonRegularVector` |
| `CAX-V0-024` | `interop/cem01-go/testdata/non-regular/` | `TestCAXV0024NativeReaderAgreesOnNonRegularVector`, `TestCAXV0024PortableReaderAgreesOnNonRegularVector` |
| `CAX-V0-030..031` | `internal/cem/wire/paired_evidence.go` | `TestCAXV0031PairedEvidenceIsClosedAndCanonical`, `TestCAXV0032CitePairedResolvesTheRangeHead` |
| `CAX-V0-032` | `internal/cem/workflow/paired.go`, `internal/cem/cli/cli.go` | `TestCAXV0032CitePairedResolvesTheRangeHead`, `TestCAXV0032PairedFlagsAreWhole` |
| `CAX-V0-033` | `internal/cem/workflow/paired.go`, `internal/cem/cli/cli.go` | `TestCAXV0033PairedStatusReverifiesTheRange`, `TestCAXV0032PairedFlagsAreWhole` |
| `CAX-V0-034` | `internal/attest/cem_v3.go`, `cmd/corvint/prove_attest_cem.go` | `TestCAXV0034StatementV3SignsPairedRange`, `TestCAXV0034ProvePairedEvidenceSignsTheRange` |
| `CAX-V0-060` | `internal/cem/workflow/path_policy.go`, `internal/cem/workflow/commands.go`, `internal/cem/cli/cli.go` | `TestCAXV0060PathPolicyFlagIsBaseOnly`, `TestCAXV0063TemplateRangeIsDeclaredInert`, `TestCAXV0061PolicyIsReadFromTheBaseTreeOnly` |
| `CAX-V0-061` | `internal/cem/witnesspaths/witnesspaths.go`, `internal/cem/workflow/path_policy.go` | `TestCAXV0061PolicyIsReadFromTheBaseTreeOnly` |
| `CAX-V0-062` | `internal/cem/witnesspaths/witnesspaths.go` | `TestCAXV0062CodeMutantAndUndeclaredPathsStayUnknown`, `TestCAXV0062DeclareNeverClassifiesCodeOrThePolicy` |
| `CAX-V0-063` | `internal/cem/workflow/path_policy.go`, `internal/cem/workflow/stable_read.go` | `TestCAXV0063TemplateRangeIsDeclaredInert`, `TestCAXV0063CIConfigRangeIsDeclaredInert` |
| `CAX-V0-064` | `internal/cem/workflow/path_policy.go`, `internal/witnesspaths/datareaders/datareaders.go`, `cmd/corvint/ocm.go` | `TestCAXV0064LocaleRangeStaysUnknownNamingItsReader`, `TestCAXV0064BinaryInstallsDataReaderFinder` |
| `CAX-V0-065` | `internal/cem/workflow/path_policy_test.go` | `TestCAXV0063TemplateRangeIsDeclaredInert`, `TestCAXV0063CIConfigRangeIsDeclaredInert`, `TestCAXV0064LocaleRangeStaysUnknownNamingItsReader`, `TestCAXV0062CodeMutantAndUndeclaredPathsStayUnknown`, `TestCAXV0061PolicyIsReadFromTheBaseTreeOnly` |

## Rollback

Revert the commits that introduced these requirements. Each profile is opt-in and additive, so
removal leaves every `cem/1.0` map, statement and vector valid; only artifacts produced under a
profile (cem/v3 envelopes, coverage-states documents, non-regular maps, paired-evidence documents)
lose their reader. The path-policy profile writes no artifact; removing it only drops its envelope
members.
