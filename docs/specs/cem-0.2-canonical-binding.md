# CEM 0.2 canonical patch binding

Owner: Russell Lewis
Frozen: 2026-08-22
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `docs/PRODUCT.md`, `docs/CHANGE-EVIDENCE-MAP.md`,
`docs/specs/verified-absence-frontier-v0.md`, and `docs/specs/ocm-v0-dogfood.md`

## Agent digest
- Claim: CEM 0.2 binds evidence maps to the canonical Git-derived base-to-target patch after only the declared sidecar is excluded.
- Status: proposed/experimental
- Exists: `internal/cem`, `cmd/corvint`, and `conformance/cem-0.2`.
- Blocked on: repository-owner acceptance after independent interoperability evidence.
- Read next: `cem-pilot-kit.md`, `cem-external-interop-v0.md`, and `change-frontier-v0.md`.

## User and job

For one repository, one exact base revision, and one exact target revision, a maintainer or CI
consumer must be able to verify that a Change Evidence Map covers the canonical whole-repository
text patch after only its declared sidecar file is excluded. A producer-written patch file must not
be able to hide target changes, and a custom map path must not silently remove a directory from the
obligation universe.

This capability strengthens the CEM proof boundary used by the Verified Absence Frontier. It does
not decide whether cited evidence is relevant or whether the change is correct.

## Verified starting state

- `cem/0.1` is a closed five-field wire profile. `docs/CHANGE-EVIDENCE-MAP.md`,
  `docs/cem-0.1.schema.json`, and `interop/cem-0.1/ALGORITHMS.md` require unknown fields and every
  spec other than exact `cem/0.1` to be rejected.
- `corvint cem prepare` derives a base-to-target patch and excludes the requested map path, but that
  exclusion is not recorded in the map.
- `corvint cem status`, `verify`, and `report` normally trust patch bytes read from a local file even
  when the caller supplies a target revision.
- The default patch and report locations already resolve the per-worktree Git directory, but the
  gitfile itself is not reciprocally checked against the requested worktree root.
- OCM independently derives the CEM patch with a fixed `.corvint/change.cem.json` exclusion but does
  not dispatch that authority by CEM profile, so WP2 must update both verifiers together.
- WP1 owns the deterministic Git diff configuration, object-mode checks, bounded parsing, and
  fail-closed resource repairs. WP2 MUST reuse that patch profile rather than define a second one.

## Definitions

- **canonical patch**: the exact bounded bytes produced by the WP1-pinned CEM Git diff profile from
  the independently expected base commit to the resolved target commit, over the whole repository,
  excluding exactly `.corvint/change.cem.json`;
- **declared exclusion**: the required `cem/0.2` field
  `"excludedPath":".corvint/change.cem.json"`; it records the fixed V0 recursion exclusion rather
  than granting the producer a choice of omitted source;
- **out-of-band patch**: patch bytes supplied by the caller rather than independently derived from
  a target revision;
- **primary worktree**: a worktree whose `.git` marker is its Git directory;
- **linked worktree**: a worktree whose regular non-symlink `.git` gitfile and supported Git
  administrative metadata reciprocally identify the requested worktree and its common object store;
- **canonical verification**: verification that resolves base and target, derives the canonical
  patch, checks its digest and hunks, and reports the resolved target and declared exclusion.

## Requirements

### Wire and versioning

- `CEM-CB-001`: `cem/0.1` MUST remain the existing closed five-field profile. Its schema,
  conformance vectors, unknown-field behavior, and exact-patch verification behavior MUST NOT be
  changed by WP2.
- `CEM-CB-002`: canonical binding MUST use a new exact profile identifier, `cem/0.2`. A `cem/0.2`
  document MUST require exactly the six top-level fields `spec`, `baseRevision`, `patchSha256`,
  `excludedPath`, `evidence`, and `hunks`; no other fields are allowed.
- `CEM-CB-003`: `cem/0.2` evidence, hunk, identity, disposition, canonical JSON, size, count, path,
  integer, Git-object, and drift rules MUST be byte-for-byte compatible with `cem/0.1` except where
  this specification explicitly changes patch binding and exclusions.
- `CEM-CB-004`: a conforming consumer MUST dispatch on the exact `spec`. An upgraded Corvint
  consumer MUST accept both profiles. A historical `cem/0.1`-only consumer MUST reject `cem/0.2`
  and must never interpret it as 0.1; this specification does not prescribe that historical
  consumer's error code.
- `CEM-CB-005`: mutation operations such as `cite` and `mark` MUST preserve the input profile and
  every validated top-level binding field. No command may silently rewrite a 0.1 map as 0.2. When
  `cite` selects base evidence from a path modified by the candidate patch, the producer MUST
  obtain the exact patch from a `baseRevision`-to-`HEAD` canonical derivation or producer cache,
  validate it against `patchSha256`, apply that path's complete hunk group to the base blob with the
  inherited patch simulation, and accept the selected bytes when the simulated blob is byte-identical
  to the base (`stable`) or they occur exactly once in an existing changed result (`relocated`). A
  same-path target absent after deletion or rename (`deleted`), zero occurrences (`stale`), or
  multiple occurrences (`ambiguous`) in an existing changed result MUST fail
  `cite-span-not-stable`. The producer MUST use the verifier's
  same-path drift classifier for this decision; path identity alone MUST NOT refuse the citation.
  A `deleted` or `stale` refusal message MUST also name the refused base span as the detail token
  `removed-intent.<base blob OID>.<start>-<end>` (zero-based half-open bytes), so intent the change
  removes can be recorded out of band as an unknown hunk's pinned detail (`DOGFOOD-BIND-007`,
  decision 0165); the token is never written to the map and is not evidence.
  A low-level 0.1 `begin` publishes the map only, as the frozen oracle does; the exact input patch
  is retained by 0.2 `prepare` in the private per-worktree Git cache, so a later `cite` after
  `begin` derives the patch from `HEAD` and refuses `patch-unavailable` when it cannot. This
  producer precheck does not alter the inherited verifier drift rules.
- `CEM-CB-025`: from 2026-09-22 the `cem/0.2` profile, with `cem/0.1` as its N-1 profile, is
  frozen by the canonical conformance packet `protocol/cem-0.2/manifest.json` (`status`
  `frozen`). A consumer MUST verify the packet's pinned raw-manifest SHA-256 and every
  `artifactSha256` entry before it runs any case. A frozen vector's bytes and expected decoded
  outcome MUST NOT change; a wire change MUST take a new exact profile identifier with its own
  packet (`CEM-CB-002`, `CEM-CB-004`). The reader of a frozen profile N MUST read every frozen N-1
  vector, with its explicit patch, to the same acceptance, the same drift records (evidence ID,
  path, status, target blob and span) and the same unknown count, and MUST NOT grant it N's
  canonical assurance. Migration rule: a retained map is never rewritten in place to a newer
  profile (`CEM-CB-005`); moving to a newer profile means preparing a new map from Git under that
  profile and citing again, every `unknown` hunk stays `unknown` until it is cited or marked, and
  no frozen digest is regenerated.

`cem/0.2` is the closed `cem/0.1` shape plus required `excludedPath`; its versioned JSON Schema is
the normative shape contract.

### Exclusion contract

- `CEM-CB-006`: `excludedPath` MUST be the exact UTF-8 string `.corvint/change.cem.json`. Missing,
  non-string, case-variant, Unicode-variant, alternate, repeated, directory, or glob exclusions are
  invalid. Custom sidecar source exclusions are deferred; V0 has no producer-controlled exclusion.
- `CEM-CB-007`: every fresh, replaced, or resumed 0.2 `prepare` MUST use
  `.corvint/change.cem.json` as its map output and source exclusion. A different `--map` for that
  operation fails `invalid-arguments`. A verifier MAY read a copied 0.2 artifact from another input
  location, but that location never changes the recorded exclusion, canonical patch, or
  target-artifact binding.
- `CEM-CB-008`: at the base, `.corvint/change.cem.json` MAY be absent. If present, it is a historical
  sidecar excluded from the patch and MUST be exactly one regular Git blob with mode `100644`; its
  historical bytes are not compared with the current CEM. Any other base-side object or mode fails
  `excluded-path-not-file`.
- `CEM-CB-009`: canonical patch derivation MUST exclude exactly `.corvint/change.cem.json` using
  literal repository-root-relative semantics. No map field or CLI value is evaluated as a Git glob
  or pathspec. During `status`, `verify`, and `report`, that path MAY be absent at the target. If
  present, it MUST be exactly one regular Git blob with mode `100644` whose raw blob bytes are
  byte-identical to the bounded raw `cem/0.2` input being verified; any different bytes, mode
  (including `100755`), or object kind fails `excluded-artifact-mismatch`. Candidate generation by
  `prepare` MUST ignore the target-side object and bytes: the target can contain an inherited stale
  sidecar which this invocation is about to replace. This comparison does not include the sidecar
  in its own patch. JSON and human reports MUST expose the recorded exclusion without exposing
  artifact bytes.

### Canonical patch verification

- `CEM-CB-010`: `cem/0.2` canonical verification MUST require caller-supplied `--expected-base` and
  `--target`. It MUST independently resolve both to exact full commit OIDs, require the map's
  `baseRevision` to equal the resolved expected base, derive the canonical patch, require its raw
  SHA-256 to equal `patchSha256`, and verify all inherited patch, hunk, evidence, and drift rules
  against those bytes. A base learned only from the map is never independent authority. The map
  binds those canonical patch bytes, not a target identity: the resolved target belongs to the
  invocation and command result/report and MUST NOT be added to the CEM wire.
- `CEM-CB-011`: canonical derivation MUST use the single WP1-pinned CEM diff profile over the whole
  repository. WP2 MUST call the same bounded implementation used by CEM and OCM; it MUST NOT copy or
  approximate the Git flags, configuration overrides, environment, deadlines, or byte ceilings.
- `CEM-CB-012`: `corvint cem status`, `verify`, and `report` for `cem/0.2` MUST require independent
  `--expected-base` and `--target`, derive the canonical patch, and reject `--patch` as
  `invalid-arguments` rather than admit a second patch source. Fresh or replaced `prepare` uses its
  required `--base` as the independent producer baseline; resumed 0.2 next actions MUST carry that
  resolved value as `--expected-base`.
- `CEM-CB-013`: for `cem/0.1`, `corvint cem status`, `verify`, and `report` MUST retain the historical
  optional `--patch`. An explicit value is read out of band. When omitted, the command reads the
  historical default `corvint/change.patch` beneath the validated per-worktree Git directory. An
  optional `--target` performs only inherited evidence drift. Existing optional `--expected-base`
  behavior is preserved: when present it independently resolves and checks `baseRevision`; when
  absent the 0.1 command remains valid. Neither patch selection is canonical verification.
- `CEM-CB-014`: `status`, `verify`, and `report` JSON MUST add exactly the three top-level
  patch-binding fields `patchSource`, `excludedPath`, and `warnings`; no alias such as
  `excludedPaths` is allowed. The legacy top-level `patch` field and every new value are frozen by
  the envelope table below. Other existing command-specific fields remain unchanged. A direct
  structural `cem/0.2` result MUST include nested `"assurance":"structural-only"`; only
  `status`, `verify`, or `report` after independent repository derivation and target binding may
  emit nested `"assurance":"canonical"`. CEM 0.1 results MUST omit `assurance` unchanged.
- `CEM-CB-015`: a `cem/0.2` `status`, `verify`, or `report` invocation supplying `--patch` MUST fail
  `invalid-arguments`; missing `--expected-base` fails `expected-base-required`; missing `--target`
  fails `target-required`; and an unavailable omitted 0.1 default patch retains the existing bounded
  patch-read failure rather than inventing `patch-required`.
- `CEM-CB-016`: a fresh or explicitly replaced `prepare` MUST emit `cem/0.2`, record the fixed
  exclusion, and generate canonical next actions with `--expected-base` and `--target` but no
  `--patch`. `prepare --target CODE_REVISION` is candidate generation: it MUST derive from that
  revision while ignoring any target-side sidecar, write the candidate, and return a verification
  action whose literal target argument is `HEAD`, not the resolved code revision. The operator MUST
  commit the candidate sidecar before running that action; `HEAD` is then independently resolved by
  the consumer as the final revision. A caller MAY instead invoke the consumer with an explicit
  final full commit OID. This two-phase flow contains no invented final-OID placeholder and preserves
  the patch because the fixed sidecar is excluded. On `prepare`, `--patch PATH` selects only where
  the newly derived convenience cache is written; it never supplies verification bytes or changes
  canonical authority. It MAY resume an
  identical valid `cem/0.1` map at the historical default map path without rewriting it. A resumed
  0.1 invocation derives and writes the patch selected by that current invocation alone: an explicit
  `--patch PATH` produces a next action containing that path, while omission uses the historical
  default and produces a next action with no `--patch`. No prior patch selection is persisted or
  inferred from the map. A valid 0.1 map at any non-default `--map` path is not resumable and fails
  `invalid-arguments` without a write. Resumed 0.2 actions always use the canonical form.
  Every `prepare` next action's first element is the literal contract command name `corvint`, never
  the invoked program path or the `corvint` build name; a caller running the `corvint` binary
  substitutes its own executable for that element (decision 0122).

### Repository and Git boundary

- `CEM-CB-017`: the global `--root` MUST resolve to the exact Git worktree top level. Every Git
  operation MUST remain pinned to that verified worktree and its resolved Git administrative
  directory; ambient discovery, repository redirection variables, replace objects, `info/grafts`
  parent grafts, interactive prompts, and implicit network fetch are forbidden. Repository
  administrative metadata MUST remain stable for one invocation; hostile concurrent mutation by the
  same operating-system identity is outside the V0 trust boundary. A revision operand not already
  validated as a full object ID MUST follow `--end-of-options`, so an option-shaped revision is
  refused as a revision and is never read as an option. A Git child MUST be reaped within its
  per-operation timeout plus a bounded pipe-drain delay, even when a descendant that left the child's
  process group still holds its pipes; a forced pipe close fails the operation.
- `CEM-CB-018`: primary worktrees and valid linked worktrees MUST be accepted. The requested root's
  `.git` marker MUST have the supported non-symlink form, and Git's resolved per-worktree
  administrative metadata MUST reciprocally identify that exact root before its common object store
  becomes authority. A forged gitfile, symlinked `.git` marker, nonreciprocal association, or direct
  redirection into an unrelated or sibling repository fails `repository-object-unavailable`. V0
  does not prescribe directory names, nesting, or one on-disk linked-worktree layout. Every regular
  `.git`, linked `gitdir`, and `commondir` metadata read MUST share one bounded no-follow reader that
  checks size before open, revalidates type, inode, and size after open, reads at most the bound plus
  one byte, and rejects truncation, growth, or observed metadata change as
  `repository-object-unavailable`.
- `CEM-CB-019`: V0 MUST reject a non-empty ambient `GIT_ALTERNATE_OBJECT_DIRECTORIES` and any
  filesystem entry at repository `objects/info/alternates`, including an empty file, before accepting
  repository objects. Alternate support is deferred until a race-resistant isolation mechanism
  exists. A locally complete partial clone MAY be used only when every required object resolves
  locally without fetch. A configured alternate fails `unsupported-object-alternates`; a missing
  promised object, attempted fetch, reciprocal-worktree failure, or exceeded bound fails
  `repository-object-unavailable`.
- `CEM-CB-020`: for the same base, target, and exclusion, a primary checkout and a valid linked
  worktree MUST derive byte-identical patches and decisions. Default local patch and report
  artifacts MUST remain in the per-worktree Git directory so concurrent worktrees do not overwrite
  one another.
- `CEM-CB-023`: literal-path tree lookup MUST NOT return an entry, or report an absence, that only
  the selected Git object reader vouches for. Every tree object consumed while resolving the path
  from the revision's root tree to the named component, the root tree included, MUST be re-read
  through the pinned Git boundary and re-hashed as `tree <length>\0<body>` with the repository
  object format's hash (SHA-1 or SHA-256); the digest MUST equal the OID the tree was fetched under.
  The returned mode, type, and OID MUST be the verified parent tree's entry and MUST agree with
  Git's own listing of that path. A verified entry's raw regular-file mode (`100` followed by three
  octal permission digits, such as the legacy `100664` Git still reads) MUST be taken in the
  canonical form Git itself reads it as, `100755` when the owner-execute bit is set and `100644`
  otherwise (decision 0126); every other raw mode is compared as stored. A digest mismatch, a
  malformed tree body, a tree body past the 4 MiB tree read bound, or a disagreement between the
  verified walk and Git's listing fails `repository-object-unavailable` with no entry, no absence,
  and no memo write. The complete body of every consumed parent tree MUST be parsed before a found or
  absent component is returned, so malformed trailing entries and duplicate names receive the same
  refusal. A prior successful read of the same object, an earlier worktree scan, or a
  signed execution result never authenticates a later read of live object bytes. A corruption that
  Git itself refuses while listing the path (a commit or root tree whose bytes no longer hash to
  their OID) keeps its existing Git failure code; this requirement adds the check Git does not
  perform.
  The default local path MUST verify every Git object body it turns into evidence
  against that object's name before trusting it, because Git's own tree, path, and diff reads do not
  check a loose or packed body against its name. When a commit peel is present, root readers MUST
  stream and self-hash its body, validate its exact first `tree <oid>` header in the same
  object format, and require the verified root tree to have that identity. Canonical change-set
  roots MUST also match full pinned base/target commit identities. OID-shaped input at the
  repository's native hash width is pinned; other symbolic revision selection remains Git-owned,
  including the `HEAD` used by citation patch recovery. Git still owns ref/tag peeling;
  the tree-ish lookup and `CommitTree` profiles preserve direct tree inputs when the commit peel
  is missing and the independently verified root tree succeeds. A present invalid commit cannot
  use that fallback. Git already checks corrupt commits while peeling; this adds Corvint's own
  commit-body proof rather than repairing an accepted corrupt-commit path.
  Commit bodies MUST be hashed incrementally without retaining their messages or consuming the
  existing 4 MiB tree-batch output allowance. Batch headers are bounded to 512 bytes, body sizes
  are nonnegative signed 64-bit decimal values, and only the exact first commit tree header is
  retained. `CommitTree` streams its root body too, preserving large-root admission. The existing
  Git operation count, timeout, cancellation, stderr and descendant-containment rules remain in
  force; runtime still grows with commit size. The tree batch retains its existing framing and
  byte ceiling, and malformed, incomplete, extra or inconsistent records refuse. A streaming
  consumer refusal retains its typed code and aborts the contained process; if process completion
  wins the race, the existing immediate post-reap descendant sweep applies. Cancellation and
  timeout selections retain their own codes; on completion or consumer abort the consumer's
  recorded error precedes the process exit error. Git diff still runs before canonical proof,
  so failures before proof retain the existing canonical-diff error mapping.
  A path lookup MUST self-hash the root tree and
  every intermediate tree body it walks and check each link against its verified parent. A path
  listing MUST resolve its directory through that walk, read each directory level beneath it as
  self-hashed tree bodies requested by the object names their verified parents carry, and list
  entries in each body's own order; it refuses more than 128 directory levels or 4 MiB
  (`MaxTreeBytes`) of listed tree bodies with the same code. A
  canonical patch derivation MUST derive the change set from self-hashed tree bodies on both
  sides, requesting each child tree by the object name its verified parent carries, MUST self-hash
  every blob a patch section derives from, and MUST accept the patch Git emitted only when it is
  the canonical derivation of those verified objects: one section per verified changed entry in
  its order, header lines naming the verified modes and object names, binary sections pinned by
  object name, and every textual section replaying the verified old bytes into the verified new
  bytes. A mismatch, a missing body, or a patch that fails that proof fails
  `repository-object-unavailable`. Because the proof is against verified content rather than a
  second Git read, an object swapped between the derivation read and the verification read fails
  the proof. The tree walk descends at most 128 directory levels (`MaxTreeDepth`) and refuses a
  deeper change set with the same code, so its one-child-per-level cost stays bounded. Hunk
  shape beyond that proof (context width, hunk boundaries, function-name hints) is re-derived at
  verify rather than proved here.

- `CEM-CB-024`: after canonical derivation and before the patch bytes are returned or memoized,
  every old and new blob named by the derived patch's `--full-index` `index OLD..NEW` lines,
  excluding the null OID and gitlink (`160000`) entries, MUST be re-read through the same pinned Git
  boundary and re-hashed as `blob <length>\0<body>` with the repository object format's hash; each
  digest MUST equal the named OID. A digest mismatch or a missing or non-blob object fails
  `repository-object-unavailable`; a blob set past the 128 MiB cumulative read bound retains
  `git-output-exceeded`. The derived patch MUST also account for the paths it does not name: every
  path whose entry, under `CEM-CB-023`'s canonical regular-file mode, differs between the two
  revisions' verified root trees, gitlink (`160000`) entries and the excluded sidecar path aside,
  MUST be an old or new path of one of the derived patch's groups. The two root trees, and every
  subtree descended into because its OID differs, MUST be re-read and re-hashed as `CEM-CB-023`
  requires. This coverage check MUST run on every derivation, including one that produced zero bytes
  or named no blob: a blob body forged to equal the other side removes its path, and can remove
  every path, from Git's own output, so a check driven only by the blobs the patch names cannot see
  it. An omitted changed path fails `repository-object-unavailable`. Derived bytes that do not parse
  as a patch are left to the frozen patch validation that already precedes every consumer of them.
  Any of these failures returns no patch bytes and writes no memo. This verifies the Git-derived
  bytes and is not a second diff implementation: coverage compares path sets and never re-derives
  hunk content, the Git-derived patch remains the only canonical patch source, and the optional
  protected immutable view path is unchanged.


### Error and determinism behavior

- `CEM-CB-021`: the new stable WP2 error codes are `invalid-excluded-path`,
  `excluded-path-not-file`, `excluded-artifact-mismatch`, `expected-base-required`, and
  `target-required`. The producer-only stable code `cite-span-not-stable` reports a same-path base
  span that would be stale, ambiguous, or deleted after candidate-patch simulation.
  Existing `invalid-arguments`, `unsupported-spec`, `missing-field`, `patch-digest-mismatch`,
  `base-revision-mismatch`, `unsupported-object-alternates`, `repository-object-unavailable`,
  bounded patch-read failures, `git-diff-failed`, and `git-diff-timeout` retain their meanings.
  Historical 0.1-only consumers' rejection codes are outside this profile.
- `CEM-CB-022`: equivalent invocations in fresh processes MUST emit byte-identical compact JSON.

## Exact patch-binding envelope

The following fields freeze every WP2-controlled top-level envelope decision for `status`, `verify`,
and `report`. The three fields `patchSource`, `excludedPath`, and `warnings` are the complete WP2
top-level addition. `warnings` order is normative; no extra warning or patch-authority field is
permitted. The nested assurance field is governed by `CEM-CB-014` and is not part of the CEM wire
schema.

| Command, profile, and input | legacy `patch` | `patchSource` | `excludedPath` | `warnings` |
|---|---|---|---|---|
| `status`, 0.2 canonical | `null` | `canonical-derived` | `.corvint/change.cem.json` | `[]` |
| `verify` or `report`, 0.2 canonical | omitted | `canonical-derived` | `.corvint/change.cem.json` | `[]` |
| `status`, 0.1 explicit `--patch` | existing resolved path string | `explicit-out-of-band` | `null` | `["patch-supplied-out-of-band"]` |
| `status`, 0.1 omitted `--patch` | existing resolved historical-default path string | `default-out-of-band` | `null` | `["patch-defaulted-out-of-band"]` |
| `verify` or `report`, 0.1 explicit `--patch` | omitted | `explicit-out-of-band` | `null` | `["patch-supplied-out-of-band"]` |
| `verify` or `report`, 0.1 omitted `--patch` | omitted | `default-out-of-band` | `null` | `["patch-defaulted-out-of-band"]` |

Thus 0.2 `status` preserves its legacy field name without claiming a file authority, and 0.2
`verify` and `report` do not invent that field. `prepare` retains its existing command-specific
envelope, including the convenience-cache path selected for that invocation, and returns
profile-aware `nextActions`.

`excludedPath:null` for 0.1 means the map records no canonical exclusion. It MUST NOT be
reinterpreted as a 0.2 declaration even when OCM uses the historical profile convention internally.

## Validation precedence

When more than one defect is present, `prepare`, `status`, `verify`, and `report` apply this exact
precedence and stop at the first failing stage:

1. common CLI syntax and bounded input-path validation;
2. bounded map read, UTF-8/JSON parsing, exact `spec` dispatch, and that profile's closed schema,
   including exact validation of 0.2 `excludedPath`;
3. profile-forbidden arguments (`--patch` on 0.2 `status`, `verify`, or `report`; a non-default
   `--map` for any 0.2 `prepare`; or a non-default input `--map` for resumed 0.1 `prepare`);
4. profile-required independent inputs: `--expected-base`, then `--target`, for 0.2 verification;
5. repository-root and reciprocal-worktree validation, with ambient Git inputs scrubbed and no
   object read;
6. ambient and repository-file alternate denial;
7. for 0.2 verification, expected-base resolution and `baseRevision` equality, target resolution,
   the base-side historical-sidecar check, then the target-side raw CEM artifact comparison;
8. canonical derivation for 0.2 or bounded explicit/default patch read for 0.1, followed by that
   profile's inherited CEM verification. The optional 0.1 `--expected-base` check retains its
   historical position inside this inherited verification and is not hoisted ahead of patch read or
   digest validation;
9. drift, policy limits, deterministic rendering, and any requested local report write.

Within stage 2, the inherited parser order remains binding: encoding/JSON and duplicate-key errors,
root object and `spec`, missing required fields, unknown fields, then field values in normative wire
order. Thus a present but non-exact 0.2 `excludedPath` fails `invalid-excluded-path` only after the
closed key set passes. Later WP2 stages MUST NOT reorder inherited validation inside stage 8.

Fresh or replaced `prepare` has no input map: after stage 1 it validates its fixed map output and
bounded convenience-cache output, then resolves required `--base` and `--target`, validates the
repository reciprocally, denies alternates, validates the historical base sidecar, and derives the
candidate map. It MUST NOT inspect or compare the target-side sidecar. A resumed `prepare` follows
the same candidate-generation rule; its `--base` is the independent expected base and its current
`--patch` selection is only an output choice. Target-artifact comparison begins only in `status`,
`verify`, and `report`, after the candidate has been committed. Failures MUST NOT fall through to a
lower-precedence code or emit a partial success envelope.

## CLI decision table

| Inputs | Profile | Patch authority | Required result |
|---|---|---|---|
| `--expected-base` and `--target`, no `--patch` | 0.2 | independently derived | canonical verification |
| no `--expected-base` | 0.2 | none | `expected-base-required` |
| no `--target` | 0.2 | none | `target-required` |
| any `--patch` on `status`, `verify`, or `report` | 0.2 | invalid | `invalid-arguments` |
| explicit `--patch`, optional `--target`, optional `--expected-base` | 0.1 | explicit out of band | legacy verification; supplied warning; check expected base when present |
| omitted `--patch`, optional `--target`, optional `--expected-base` | 0.1 | per-worktree default out of band | legacy verification; defaulted warning; check expected base when present |

`report` is governed by the same table as `status` and `verify`; rendering is not a second trust
path. After profile dispatch, the validation precedence above decides combinations that occupy more
than one row.

For `prepare`, `--base` and `--target` are required producer inputs for both profiles and `--patch`
is an output-cache selector, never patch authority. Fresh or replaced preparation emits 0.2. An
identical resumed 0.1 map remains 0.1; an explicit current `--patch` is written and repeated in its
next action, while omission writes the historical default and leaves `--patch` out of that action.
An identical resumed 0.2 map may write the selected convenience cache, but its canonical next action
always uses `--expected-base BASE --target HEAD`, omits `--patch`, and is run after committing the
candidate sidecar. The resolved code revision returned by `prepare` is not reused as the verification
target because committing the sidecar creates the final revision.

## Compatibility matrix

| Artifact or consumer | WP2 behavior |
|---|---|
| Existing valid `cem/0.1` plus explicit exact patch | Accepted with unchanged 0.1 semantics and `patch-supplied-out-of-band`; optional target and expected base retain their drift and independent-base checks. |
| Existing valid `cem/0.1` with omitted `--patch` | Reads the historical per-worktree default and emits `patch-defaulted-out-of-band`; a missing file fails through the existing bounded read path; optional expected base remains supported. |
| Existing `cem/0.1` made with a custom unrecorded exclusion | It cannot make a canonical target claim; regenerate as 0.2. The verifier MUST NOT infer an exclusion from the current map filename. |
| Existing 0.1 map resumed at the default path | Preserved as 0.1 without silent migration; patch output and next action follow only the current invocation's explicit/omitted `--patch`, with no persisted prior selection. |
| Existing valid 0.1 map at a non-default `--map` path | `prepare` rejects resume as `invalid-arguments` without rewriting or producing patch output. Direct legacy verification remains governed by the 0.1 rows above. |
| New or explicitly replaced `prepare` output | Produced as a `cem/0.2` candidate only at `.corvint/change.cem.json`, with that exact path recorded; inherited stale target bytes are ignored until the candidate is committed and a consumer verifies the final revision. |
| Upgraded Corvint consumer | Accepts 0.1 and 0.2 under their distinct rules. |
| Historical strict 0.1 consumer given 0.2 | Rejects it under that consumer's existing behavior; this profile does not prescribe its code. |
| OCM consumer | MUST dispatch on CEM profile: 0.1 uses the historical fixed convention, while every 0.2 CEM-consuming command requires independent expected base and target, validates recorded `excludedPath`, and applies target artifact binding. |

The `cem/0.1` schema and conformance artifacts remain frozen. `cem/0.2` receives a separate schema
and separate conformance cases. No existing expected 0.1 map, digest, or decision may be regenerated
to make WP2 pass.

## Simpler baseline and non-goals

The simpler baseline is the current trusted CI wrapper: a reviewed script derives the patch and
passes exact bytes to the 0.1 verifier. WP2 exists because that protection is not present in the
ordinary local completion command and because the fixed recursion exclusion is not recorded.

WP2 does not add:

- custom, arbitrary, repeated, directory, glob, generated-file, binary, or policy exclusions;
- a target revision, patch body, signature, producer identity, repository UUID, or whole-map digest
  to CEM wire documents;
- automatic migration or rewriting of existing 0.1 maps;
- remote fetch, hosted verification, a daemon, database, UI, account, role, or policy service;
- relevance scoring, human assertions, test execution, frontier calculation, retrieval, adapters,
  multi-repository evidence, or semantic correctness claims;
- support for object alternates or protection from hostile same-identity mutation of repository
  administrative metadata during an invocation;
- a second diff implementation or speculative Git repository abstraction;
- any claim that a prior read, a worktree scan, or a signed execution result authenticates live Git
  object bytes; `CEM-CB-023`/`CEM-CB-024` verify each consumed tree and blob per read, and the
  optional protected immutable view remains a separate, unchanged path.

## Acceptance tests

Every advertised rejection has a genuine-pass, fabricated-fail, and no-input or unavailable-input
case.

| ID | Class | Fixture and expected result |
|---|---|---|
| `CEM-CB-AT-001` | genuine pass | Delete the cached patch after `prepare`; `status --expected-base BASE --target TARGET` derives the canonical bytes and passes. |
| `CEM-CB-AT-002` | fabricated fail | A map matching a producer-supplied one-hunk patch is checked against a target containing another hunk; fail `patch-digest-mismatch`. |
| `CEM-CB-AT-003` | fabricated fail | A 0.2 `status`, `verify`, or `report` supplying `--patch` fails `invalid-arguments`; producer bytes are never considered. |
| `CEM-CB-AT-004` | no input | A 0.2 map missing expected base fails `expected-base-required`; with expected base present but no target it fails `target-required`. |
| `CEM-CB-AT-005` | envelope | Explicit/default 0.1 and canonical 0.2 invocations emit the exact envelope table, including 0.2 `status` `patch:null`, omitted `patch` on 0.2 `verify`/`report`, nested `assurance:"canonical"`, and no aliases; direct structural 0.2 reports `structural-only`, while 0.1 omits the field. |
| `CEM-CB-AT-006` | compatibility | Omitting `--patch` for 0.1 reads the historical per-worktree default and warns; absence returns the existing bounded read failure. Optional matching `--expected-base` passes, mismatch fails `base-revision-mismatch`, and omission remains valid. |
| `CEM-CB-AT-007` | authority | A self-consistent 0.2 map with a substituted base fails `base-revision-mismatch` against independent `--expected-base` before derivation. |
| `CEM-CB-AT-008` | precedence | Multi-defect fixtures cover every adjacent validation stage, including forged worktree plus configured alternate, and assert reciprocal-worktree failure before alternate denial with no partial envelope. |
| `CEM-CB-AT-009` | prepare cache | Fresh/resumed 0.2 `prepare --patch CACHE` writes only a convenience cache and emits a canonical next action without `--patch`; resumed 0.1 explicit/omitted invocations derive and write the current selection and include/omit `--patch` accordingly. |
| `CEM-CB-AT-010` | target identity | One 0.2 map verified against caller-supplied targets that resolve to the same canonical patch bytes reports each invocation's resolved target without changing the map shape or bytes. |
| `CEM-CB-AT-011` | two-phase bootstrap | `prepare` succeeds when its code target contains an inherited stale sidecar and emits a `--target HEAD` verification action; a consumer rejects that stale target, then accepts the final commit containing the exact generated bytes. |
| `CEM-CB-EX-001` | genuine pass | Exact `.corvint/change.cem.json` is recorded and solely excluded; target absence and a target `100644` blob byte-identical to raw CEM input both pass, while every other textual target hunk remains mapped. |
| `CEM-CB-EX-002` | fabricated fail | A base where `.corvint/change.cem.json` is not a `100644` regular blob fails `excluded-path-not-file`; a target with forged bytes, mode `100755`, tree, symlink, or gitlink fails `excluded-artifact-mismatch` before canonical acceptance. |
| `CEM-CB-EX-003` | fabricated fail | A 0.2 map containing a case variant, custom path, array, empty value, or glob fails `invalid-excluded-path`; fresh, replaced, or resumed `prepare --map OTHER` fails `invalid-arguments`. |
| `CEM-CB-EX-004` | no input | A 0.2 map missing `excludedPath` fails `missing-field`; unchanged 0.1 remains valid. |
| `CEM-CB-EX-005` | raw binding | A copied 0.2 input verifies only when its raw bytes equal a present target sidecar; semantic-equivalent reserialization, one-byte mutation, and executable-bit substitution each fail `excluded-artifact-mismatch`. Historical base-side bytes are not compared. |
| `CEM-CB-CO-001` | compatibility | Every existing 0.1 conformance vector remains byte- and decision-identical under the upgraded consumer. |
| `CEM-CB-CO-002` | compatibility | The upgraded consumer accepts legacy 0.1 via explicit/default out-of-band patch bytes; a historical 0.1 consumer rejects 0.2, with code unspecified. |
| `CEM-CB-CO-003` | mutation | `cite` and `mark` preserve 0.1 or 0.2 exactly and never drop or invent `excludedPath`. |
| `CEM-CB-CO-004` | resume | Resumed 0.1 and 0.2 maps produce profile-correct next actions without migration. Two default-path 0.1 resumes with different current explicit/omitted patch choices prove no prior choice is inferred or persisted; a non-default-path 0.1 resume fails `invalid-arguments` with no writes. |
| `CEM-CB-GIT-001` | genuine pass | The same commits in a primary checkout and a real linked worktree produce byte-identical canonical patches and verification decisions. |
| `CEM-CB-GIT-002` | genuine pass | Linked-worktree `prepare`, `status`, and `report` pass and write local artifacts beneath that worktree's Git directory. |
| `CEM-CB-GIT-003` | fabricated fail | Forged gitfiles pointing into a sibling or unrelated repository fail `repository-object-unavailable`; external canaries never appear in output. |
| `CEM-CB-GIT-004` | unavailable input | Oversized, growing, symlinked, FIFO, malformed, missing, or nonreciprocal `.git`, linked `gitdir`, or `commondir` metadata, and a locally missing promised object, fail `repository-object-unavailable` without blocking, unbounded allocation, or fetch. |
| `CEM-CB-GIT-005` | boundary | After reciprocal worktree validation, a non-empty ambient alternate and every repository `objects/info/alternates` entry, including an empty file, fail `unsupported-object-alternates` before an alternate-only object is accepted; its canary never appears. A locally complete partial clone passes without fetch. |
| `CEM-CB-GIT-006` | fabricated fail | In an ordinary loose-object repository, a nested tree's decompressed body replaced under its original object filename with a valid tree pointing at another blob fails literal-path lookup below it `repository-object-unavailable`, with no entry and no absence, whether or not the same path was read earlier in the invocation; the untouched repository still resolves the path. |
| `CEM-CB-GIT-007` | fabricated fail | An old-side or new-side blob body replaced under its original OID fails canonical derivation `repository-object-unavailable` with no patch bytes, whether or not the same diff was derived earlier in the invocation; the untouched repository still derives byte-identical canonical bytes. |
| `CEM-CB-GIT-008` | genuine pass | A tree entry stored with the legacy regular-file mode `100664` resolves by literal-path lookup to the `100644` blob entry Git lists, and a derivation from the same blob at `100644` to it at `100664` yields zero patch bytes without an omitted-path refusal, matching Git's own diff. |
| `CEM-CB-DE-001` | determinism | Hostile ambient/repository diff settings covered by WP1 do not change canonical bytes. |
| `CEM-CB-DE-002` | determinism | Two fresh CLI processes emit byte-identical JSON, including exclusions and warnings. |
| `CEM-CB-OCM-001` | integration | OCM dispatches 0.1 to the historical fixed exclusion and 0.2 to validated recorded `excludedPath`; both derive the same patch as CEM, and 0.2 applies identical target-side raw-artifact binding. |
| `CEM-CB-OCM-002` | integration | OCM accepts a real linked worktree, rejects forged/symlinked/nonreciprocal metadata without canary disclosure, and requires independent expected base and exact caller target for every 0.2 CEM-consuming command. Shared canonical authority and repository preflight precede target/intent blob reads; ambient alternates fail first and retain `unsupported-object-alternates`. |

## Delivery dependencies and gate

### Frozen portable proof wire (V1-0013)

Frozen 2026-09-22 (decision 0356): `cem/0.2` with its N-1 `cem/0.1` reader (`CEM-CB-025`,
`protocol/cem-0.2/`), `ocm/0.1-experimental` (`OCM-V0-014`/`OCM-V0-015`, `conformance/ocm-v0/`)
and `frontier/0` with `frontier-error/0` (`CF-V0-034`, `conformance/frontier-v0/`). Each packet
pins every vector file by SHA-256 and pins the stable, relocated, stale, ambiguous, deleted and
unknown states to cases or to a stated gap. The freeze fixes the conformance bytes and the
successor rule; it does not promote any profile's intent or delivery status. It was made on owner
direction before predecessor V1-0010 delivered a verified daily-loop minimum, which remains open.

The minimum is the existing `cem/0.2` canonical map with its
`cem/0.1` reader retained, one `ocm/0.1-experimental` map for each owning intent, and the existing
`frontier/0` advisory report over that bound scope. OCM linkage does not prove passing checks;
frontier output does not confer closure authority. This inventory adds no fields, profile versions,
optional evidence families or accepted requirements. If the daily workflow later shows this
minimum is wrong, the correction is a new profile identifier under `CEM-CB-025`, never an edit of
the frozen packets.

`protocol/cem-0.2/manifest.json` supplies the frozen raw canonical vectors for stable, relocated,
stale, ambiguous, deleted and unknown evidence, with two-record drift ordering and exact committed
sidecars. `TestPortableCanonicalVectors` consumes the packet through the native status workflow;
`TestPortableProfileCompatibility` consumes its separately pinned 0.1 maps through the existing
independent parser and requires the historical reader to reject 0.2. Both pin the same raw manifest
but share no parser or reconstruction helper. The frozen 0.1 matrix is unchanged. Historical
exact-patch compatibility is not a downgrade/migration of canonical assurance.

`TestPortableCanonicalVectors` also reads each case's 0.1 map and explicit patch through the
current native reader (`CEM-CB-025`).

Remaining promotion evidence is explicit: verified daily-loop minimum, independently authored
0.2 consumer and producer interoperability, OCM/frontier portability and compatibility qualification,
the ticket's full and interop gates, and owner acceptance. Existing OCM/frontier conformance is
retained; this packet neither duplicates it nor treats a Corvint implementation seam as an
independent consumer. See `protocol/cem-0.2/README.md` for packet reconstruction and non-claims.

```text
WP1 CEM proof-hole repairs
  -> WP2 canonical patch binding and recorded exclusion
    -> WP5 corvint frontier
```

- WP2 MUST NOT implement or freeze independent patch bytes until WP1's canonical Git profile and
  hostile-configuration fixtures have landed.
- WP5 MUST NOT start from a CEM result that can be satisfied by an unrecorded exclusion or
  producer-supplied patch.
- WP2 may become `implemented` only when all requirements above have executable evidence, both
  profiles have distinct schema/conformance coverage, OCM consumes the effective exclusion, and
  the full existing CEM/OCM/CLI suites remain green.
- Independent interoperability remains a later delivery gate; Corvint producer-to-Corvint verifier
  tests do not establish it.

## Kill gates

WP2 cannot promote and WP5 remains blocked if any of these is true:

- an existing valid 0.1 conformance vector changes or becomes invalid;
- any target-based path accepts producer patch bytes without independent derivation;
- any 0.2 canonical verification proceeds without independent expected base and target;
- any CEM-consuming OCM 0.2 command infers expected base or target from either map;
- a resumed 0.1 next action depends on patch selection from an earlier invocation;
- any path other than exact `.corvint/change.cem.json` can leave the canonical obligation universe;
- a present target `.corvint/change.cem.json` is accepted with non-`100644` mode or raw bytes different
  from the verified CEM input;
- primary and linked worktrees derive different bytes for the same resolved inputs;
- a nonreciprocal gitfile can redirect reads into a sibling repository;
- a non-empty ambient alternate or any repository `objects/info/alternates` entry is accepted;
- canonical verification attempts a network fetch or rejects a locally complete partial clone only
  because partial-clone configuration exists;
- CEM and OCM derive different patches from the same CEM profile.

## Rollback

If WP2 fails its gate, keep `cem/0.1`, its exact-patch verifier, and the reviewed CI derivation
script unchanged. Remove the unaccepted 0.2 producer, schema, conformance additions, CLI canonical
claims, and OCM integration; do not rewrite retained maps. Mark this spec delivery `failed`, record
the failing fixture, and keep WP5 blocked until a smaller canonical-binding design is accepted.

## Traceability

The `src/context_corvint*`/`src/corvint_cli.py` citations below are historical implementation
references, not live authority: decision 0012 R0
(`docs/decisions/0012-expert-panel-ratifications-2026-09-01.md`) rules the Python implementation
non-authoritative and slated for separate removal.

| Requirement | Planned implementation | Required evidence | Current state |
|---|---|---|---|
| `CEM-CB-001..005` | `internal/cem/{wire,workflow,verify}`, versioned schemas and conformance; historical Python producer | core, dual-schema and dual-conformance tests; `TestCiteAcceptsRelocatedEvidenceSpan`, `TestCiteRefusesStaleEvidenceSpan`, `TestCiteRefusesAmbiguousEvidenceSpan`, `TestCiteRefusesDeletedEvidenceSpanWithVerifierClassification`, `TestCiteRefusesRenamedSourceEvidenceWithVerifierClassification`, `TestCiteRemovedIntentRefusalNamesBasePin`; independent interop remains external | `PASS`; external interop `NOT_RUN` |
| `CEM-CB-006..009` | `src/context_corvint_cem.py`, `src/context_corvint_cem_workflow.py` | fixed exclusion, base/target mode, raw-byte, path-denial tests | `PASS` |
| `CEM-CB-010..016` | CEM core/workflow and `src/corvint_cli.py` CEM commands | independent authority, structural/canonical assurance, canonical/explicit/default, resume, exact-envelope tests | `PASS` |
| `CEM-CB-001..004`, `CEM-CB-009..012` | `protocol/cem-0.2` frozen packet; native workflow and separate historical reader | `TestPortableCanonicalVectors`, `TestPortableProfileCompatibility`: raw committed sidecars, mixed ordered drift, explicit unknown policy and N-1 refusal | frozen reference evidence; independent 0.2 qualification pending |
| `CEM-CB-025` | `protocol/cem-0.2/manifest.json` (`status` `frozen`), `internal/cem/workflow/portable_test.go` | `TestPortableCanonicalVectors`: manifest and artifact digests, and each 0.1 `legacyMap` read by the current native reader to the same acceptance, drift and unknowns without canonical assurance | `PASS` locally; full and interop gates `NOT_RUN` |
| `CEM-CB-017..020` | shared CEM/OCM repository-boundary validation | primary/linked equivalence, bounded oversized/growth/symlink/FIFO metadata, alternate precedence, locally complete promisor tests, `TestResolveIgnoresRepositoryGrafts`, `TestRevisionOperandsAreNeverOptions`, `TestTimeoutReapIsBoundedWhenEscapedDescendantHoldsPipes`, `TestSessionStopIsBoundedWhenEscapedDescendantHoldsPipes` | `PASS` |
| `CEM-CB-023..024` | `internal/cem/gitauth/{object,diff}.go` per-read tree and blob identity plus changed-path coverage | `TestAccuracyWholeTreePublicReadersAgree`, `TestGitIntegrityCommitAndTreeLinks` nested-tree case, `TestCanonicalDiffRejectsMislabeledBlobInputs`, `TestCanonicalDiffRefusesBlobFreeOmission`, `TestRequestMemoPrimitiveParityAndCopies` child counts | `PASS` |
| `CEM-CB-021..022` | CEM verifier, workflow, and CLI envelopes | stable error and fresh-process deterministic JSON tests | `PASS` |
| `CEM-CB-023` | `internal/cem/gitauth/{object,diff,tree,treediff,patchproof,commitstream}.go`, `internal/cem/gitrun/{gitrun,stream}.go` | `TestCommitStreamSplitFramesAndBoundedMemory`, `TestCommitStreamRefusesMalformedFrames`, `TestCommitTreeLinkRejectsValidUnrelatedObjects`, `TestVerifiedCommitTreesRejectsUnpinnedCommit`, `TestCanonicalDiffPreservesSymbolicRevisions`, `TestCommitVerificationLargeMessagesAndTreeishParity`, `TestCommitVerificationRejectsLooseAndPackedCorruption`, `TestCommitVerificationPreservesGitChildBudget`, `TestCommitTreeStreamsLargeRoot`, `TestRunStreamProcessFailureParity`, `TestRunStreamFastExitKeepsConsumerRefusal`, `TestRunStreamAdmissionAndCancellationParity`, `TestRunStreamContainsDescendants`, `TestGitIntegrityCommitAndTreeLinks`, `TestLookupTreeEntryVerifiedWalkMatchesLsTree`, `TestTreePathsMatchesLsTreeOrder`, `TestTreePathsRefusesMislabeledSubtree`, `TestTreePathsBoundsDepthAndBytes`, `TestCanonicalDiffRefusesMislabeledInputBlob`, `TestCanonicalDiffRefusesMislabeledIntermediateTree`, `TestRequirePatchProvenanceRefusesDoctoredPatch`, `TestCanonicalDiffProvesEveryHonestShape`, `TestCanonicalDiffBoundsChangeSetDepth`, `TestBlobBytesRejectsMislabeledLooseAndPackedObjects` | `PASS` |

## Decisions requiring acceptance

- Adopt `cem/0.2` rather than changing the closed `cem/0.1` shape under its existing identifier.
- Record exact `.corvint/change.cem.json` as V0's sole recursion exclusion; defer configurable paths.
- During consumer verification, when that fixed path exists at the target, bind its `100644` raw
  blob bytes to the verified CEM input; allow absence, ignore target-side bytes during candidate
  preparation, and treat a base-side blob only as historical excluded state.
- Keep low-level arbitrary-patch creation and verification explicitly legacy rather than presenting
  it as canonical.
- Accept reciprocal linked worktrees, reject alternates in V0, and allow locally present
  partial-clone objects without fetch; same-identity concurrent metadata mutation remains outside
  the V0 trust boundary.

Until the repository owner accepts these decisions, this document is proposed intent and does not
govern or advertise delivered behavior.
