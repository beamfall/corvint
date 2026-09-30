# Closed Vocabulary Intake V0

Owner: Russell Lewis
Date: 2026-09-30
Intent status: proposed
Delivery status: experimental
Authoritative inputs: owner-authored GitHub issue #391, native ticket V1-0538;
repository AGENTS.md and product invariants. Issue text describes intent, never execution authority.

## Agent digest
- Claim: Intake admits typed records before untrusted change context reaches an author.
- Status: Proposed intent; experimental local implementation, qualification pending.
- Exists: Strict schema, immutable-path validator, reader preflight, author-input builder and battery.
- Blocked on: Final bound evidence closeout, hosted CI, host/reader qualification and owner technical acceptance.
- Read next: Requirements; Reader boundary; Acceptance; Limits and rollback.

## Job and current state

An automation author needs change intent without directly reading a third party's instructions.
The current repository has strict `internal/cem/wire` JSON parsing and verified immutable
`internal/cem/gitauth` object reads, but no intake-specific wire record or reader boundary.
The simpler baseline is a host-reviewed typed JSON file. This version adds deterministic local
validation and an executable containment battery, preserving that host authority.

## Requirements

- `CVI-V0-001`: Publish a versioned schema whose data fields are enums, literal repository paths,
  bounded identifiers, counts, arrays/objects of those values, or the documented absent parent.
  No free-text field, rejected source value, command, connector credential or source body may
  appear in author-facing output.
- `CVI-V0-002`: Validate exact field names and presence, types, enum vocabulary, bounded identifiers,
  nonnegative bounded counts, nesting and array bounds. Reject duplicate JSON members, malformed
  UTF-8, invalid surrogates, trailing values, unknown fields, duplicate set items and unsupported
  profiles. Errors exposed to the author carry fixed codes, never untrusted strings.
- `CVI-V0-003`: Bind validation to explicit full base/head commit identities. Every behaviour and
  claimed-test path must exist as a supported entry at either commit through immutable verified
  Git objects. Reject traversal, absolute paths, pathspecs, control characters and Git metadata.
  A dirty live worktree must neither admit a missing path nor remove an immutable admitted path.
- `CVI-V0-004`: Normalize each unordered set by deterministic byte ordering and serialize the
  admitted record deterministically. Equivalent ordering gives identical output; normalization
  does not silently discard duplicates, alter source claims or assert semantic correctness.
- `CVI-V0-005`: Publish a reader-step contract: raw connector text is mounted read-only outside
  the authoring checkout; the reader has no authoring/network/write-credential capabilities;
  exactly one intake file is writable outside the authoring checkout. Preflight resolves ancestry
  aliases and refuses raw/output files inside the authoring checkout or aliasing each other.
  Preflight is an observation; host confinement is required and remains separately unobserved.
- `CVI-V0-006`: The production author-input builder must revalidate the reader file before output
  and admit only schema fields. Hostile descriptions, titles, commit messages, work items and
  parent text must yield identical author input to neutered twins except closed concerns.
  Malformed or prose-leaking reader results must fail closed without forwarding raw errors.
- `CVI-V0-007`: Publish conformance vectors and a CI containment battery against the actual builder.
  Every declared prose-leaking builder mutant must be caught; a surviving mutant fails the
  battery. Keep reader/model qualification, connector fetching and semantic extraction limits
  separate from deterministic schema/containment evidence.

## Wire record

Profile is `corvint-intake/0`. `base` and `head` are full Git object identifiers. `intent` is one
of `FIX`, `FEATURE`, `REFACTOR`, `DOCS`, `TEST`, `MAINTENANCE`, `UNKNOWN`.
`claimed_behaviours` is a set of `{kind,path}`, with kind `ADD`, `CHANGE`, `REMOVE`, `UNKNOWN`.
`flags` is a set of flag identifiers. `migrations_claimed` is a nonnegative count.
`tests_claimed` is a set of literal paths. `description_vs_diff` is `ALIGNED`, `PARTIAL`,
`CONFLICTED` or `UNKNOWN`. `concerns` is a set drawn from `INSTRUCTION_ATTEMPT`, `OUTWARD_ACTION`,
`SECRET_REFERENCE`, `MISSING_EVIDENCE`, `CONFLICTED_CLAIM`.
`work_items` is a set of `{id,alignment,acceptance_criteria,concerns,parent_review}`.
Alignment has the same vocabulary as description comparison. Criteria is `{met,unmet,unknown}`
counts. Parent review is null or `{id,alignment,acceptance_criteria,concerns}`. There is no parent
body, criteria text, raw title, description, commit message, URL or arbitrary metadata field.
IDs and flags are inert lexical identifiers; they confer no command, policy or write authority.

A validated record preserves reader claims. The validator does not prove those claims true.
Unknown interpretation stays `UNKNOWN`; absent criteria evidence increments `unknown`.
Base/head path existence is structural evidence, not test success or description/diff agreement.

## Reader boundary

The connector owns fetching and storing raw text. The reader sees that file and read-only Git
objects, and writes one candidate intake file. A trusted host then runs the validator and builder
outside the reader's write access. The author receives only normalized admitted bytes. The host
must bind its mounts and filesystem allowlist; reader declarations are not sandbox enforcement.
Raw input and candidate files must stay outside the authoring worktree even if ignored by Git.
Preflight rejects raw files whose native link count is not exactly one, closing hardlink aliases.
Reader output must not be used as CLI argv, executable code, a policy grant, or a mutable spec.

## Acceptance and traceability

| Requirement | Intended implementation | Acceptance witness |
|---|---|---|
| CVI-V0-001 | protocol/intake/schema.json; internal/intake | `TestCVI_V0_001_002_PublishedVectors` |
| CVI-V0-002 | internal/intake decode/validation | `TestCVI_V0_002_StrictGrammar` |
| CVI-V0-003 | internal/intake immutable lookup | `TestCVI_V0_003_ImmutablePathAndTrustedPins` |
| CVI-V0-004 | internal/intake normalization | `TestCVI_V0_004_Normalization` |
| CVI-V0-005 | internal/intake reader preflight | `TestCVI_V0_005_ReaderIsolation` |
| CVI-V0-006 | internal/intake author builder | `TestCVI_V0_006_007_HostileTwinsAndMutants` |
| CVI-V0-007 | owned test battery and CI workflow | `TestCVI_V0_006_007_HostileTwinsAndMutants` |

These named witnesses passed focused local execution. Final candidate binding and hosted CI remain pending. Delivery requires focused tests,
actual CLI fixture qualification, focused-docs gate, independent review, bound change evidence and
retained outcome. A green fixture battery does not prove a model reader immune to prompt injection.

## Limits, failure modes and rollback

Input bound 1 MiB; at most 256 work items, 1024 set entries per field and 64-byte identifiers;
counts at most 1,000,000. Immutable Git boundary has its existing operation/time/object bounds.
Malformed, unsupported, missing-path, unsafe-boundary and resource failures close output. No
network, raw-text fetching, natural-language correctness oracle, automatic writes or host sandbox
is included. The contract is experimental pending owner technical acceptance and candidate
qualification; no formal support claim. Roll back by removing this optional companion and its
caller invocation, preserving raw inputs and prior receipts. Version changes need new conformance
vectors; changed builder invalidates its containment qualification.


## Containment qualification boundary

Raw hostile/neutered fixture pairs use explicit fixture-owned candidate record oracles. The real
`BuildAuthorInput` takes candidate bytes plus trusted repository/base/head, never raw prose.
Mutants on that exact path bypass validation, forward unknown fields, reflect rejected errors,
or add/rename fields; every mutant must be caught. Equality outside concerns proves author-side
containment conditional on the candidate oracle. Reader extraction/model immunity remains
NOT_OBSERVED. All parser and Git errors become fixed author-facing codes. Behaviour paths admit
regular blobs, symlinks and trees; test paths admit regular blobs only; gitlinks are unsupported.
REMOVE requires base existence; ADD/CHANGE/UNKNOWN use either pin without proving semantic accuracy.
Reader preflight observes symlink-resolved ancestry and fresh output absence; the host must enforce
read-only mounts, one output file, no network and no credentials. It is not a sandbox.
