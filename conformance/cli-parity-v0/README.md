# Corvint CLI parity corpus V0

`manifest.json` is source-free conformance evidence, not runtime authority. The fixtures contain only
synthetic repositories. The runner is implemented entirely with the Go standard library and invokes
only the supplied commands, the Go tool for candidate build information, and Git.

## Evidence model

Decision 0088 and `GOC-V0-002` (`docs/specs/go-only-cutover-v0.md`) retired capture and the live
Python oracle. `replay` against the already-committed `manifest.json` expectations is the only
supported path. The `capture` command now refuses unconditionally, and `replay --oracle` refuses a
live override.

The manifest retains the immutable revision and producer metadata for the retired oracle run that
originally produced its exact process and repository expectations. That metadata is historical
provenance, not an executable refresh path. Replay cannot regenerate expectations, and candidate
output cannot update them.

A case may declare `locationNormalization` only for a closed, named output field and normalization
kind. Thirteen declarations are admitted, all with kind `repository-root-prefix`, and each is pinned
in `validLocationNormalization` to its case ID, mutation class, and exact field set:
`record-create-trace` field `stdout.store`; `eval-frozen-corpus` field
`stdout.evaluation.promotion.frozen_corpus.path`; `ocm-prepare-resume` fields `stdout.cem` and
`stdout.map`; `cem-prepare-create` and `cem-prepare-resume` fields `stdout.map` and `stdout.patch`;
and `stdout.map` alone for `cem-status-bound`, `cem-begin-map`, `cem-cite-evidence`,
`cem-cite-lines-span`, `cem-mark-hunk-unknown`, `cem-mark-hunk-mechanical`, `ocm-link-obligation`,
and `ocm-mark-obligation`. Each names an action that reports a resolved artifact path, which is
location-dependent under decision 0004. During replay, the runner substitutes only the exact encoded
repository-root prefix in that declared field before checking the candidate's stdout digest and byte
count against the frozen expectation. The path suffix and every other byte remain exact. Missing,
extra, changed, or undeclared fields still fail. Replay performs no second-runtime comparison.
Replay marks the case `PASS-WITH-LOCATION-NORMALIZATION`; it is never reported as an unqualified
byte-exact replay. Twelve of the thirteen declared cases are retired (below), so one such row
executes.

Decision 0308 gives the candidate one name while the frozen oracle inputs keep the previous one
(`DR-0038`). Two closed declarations record that: `identityRenameDivergence`, pinned by
`validIdentityRenameDivergence` to the ten harness cases and to the single rewrite
`"profile":"corvint-harness-event/0"` for `"profile":"atlas-harness-event/0"`, applied exactly once
outside the counted packet and reported `PASS-WITH-IDENTITY-RENAME`; and `retired`, pinned by
`validRetired` and `retiredCases` to the twenty-nine cases whose frozen argv, fixture or
expectation address the oracle's `.atlas/` or `.context-atlas/traces` state paths or its
product-name tooling term, and by `validRetiredRefusal` and `retiredRefusalCases` to the two
refusal items whose frozen argv or setup does the same (`lrf-ocm-python-claim-refusal`,
`query-present-trace-store-refusal`). A retired item is neither executed nor scored: replay prints
`RETIRED <id> decision=0308 reason=…`, never a `PASS` or `UNSUPPORTED` vocabulary, and its frozen
expectation stays in the manifest unchanged. `validateManifest` refuses a manifest whose retired or
renamed counts differ from the pinned sets, so neither declaration can widen into a blanket
exclusion. The `SUMMARY` line reports `parity=` over executed cases only, then `retired=` and
`identity-renames=`, `unsupported-refusals=` over executed refusals and `retired-refusals=`, and
every other count ranges over executed cases. A command with a retired case or refusal carries
inventory status `PARTIAL` with the retired count in its reason, printed like the `UNSUPPORTED`
and `NOT_RUN` rows.

`replay` gives every parity case and refusal its own workspace root
(`conformance/cli-parity-v0/runner.go:99-136`) and executes the candidate once per item.
`replayParityCase` compares that one execution with the immutable manifest expectation;
`replayRefusalCase` validates that one execution as a bounded refusal
(`conformance/cli-parity-v0/runner.go:176-218`). Every execution gets fresh external cache,
temporary, and home directories (`conformance/cli-parity-v0/runner.go:311-332`). The manifest's
positive `workers` field bounds cross-item concurrency; the effective count is the smaller of that
field and `GOMAXPROCS`. The committed default is `1`, preserving sequential replay. Complete item
output is buffered and flushed in manifest order (`conformance/cli-parity-v0/runner.go:136-150`). A
parity case must match exact process bytes and independent before/after repository snapshots against
the frozen manifest expectation
(`conformance/cli-parity-v0/runner.go:459-498`); JSON equivalence is insufficient.

Snapshot directory discovery combines the absolute Git and common-directory lookups only when
both values are unambiguous. A known newline in the repository path uses the original lookups;
ambiguous resolved metadata or a failed combined lookup falls back to them. Status, complete
worktree/Git/common metadata observations and all digests are unchanged. Fallback may add one
bounded Git invocation (90 seconds of configured timeouts versus 60, excluding bounded shutdown
and capped by caller cancellation). The fixed development screen, original regression and repair
are retained in [parity-snapshot-discovery](../../benchmarks/parity-snapshot-discovery/README.md).

Every successful replay records one line immediately before `SUMMARY` (or `SUMMARY-FILTERED`) as
`CANDIDATE-IDENTITY sha256=<hex> go-version-m=<json-array>`. The array is the complete line sequence
from `go version -m`; its path-bearing first line is reduced to the Go version and JSON encoding
keeps the transcript record on one stable line. Replay hashes, inspects, and executes the same
owner-private, read-only staged copy, so replacement of the supplied candidate path cannot split
the recorded identity from the subject under test.

A case may declare `acceptedDivergence` only for operator-approved historical rejection-path
behavior, where the retired oracle scaffolded exactly one thing before it rejected and Go rejects
first. Two classes qualify, both governed by the `GPK-V0-008` rejected-mutator invariant:

- `oracle-only-trace-operation-lock` — the retired Python run created the empty mode-`0600`
  `.context-corvint/traces/.trace-operation.lock`; Go rejects before acquiring the lock.
- `oracle-only-private-parent` — the retired Python run created the mode-`0700` directory
  `.git/corvint`, the patch cache's owner-private parent, before resolving the revisions that reject
  the invocation; Go resolves first and creates nothing (DR-0003).

The declaration names that exact historical oracle-only path, requires `candidateMutation: "none"`,
and carries the operator's reason. Because the Git directory is snapshotted under `git` and the
worktree under `worktree`, the snapshot prefix is derived from the declared path rather than
assumed. Replay validates that the candidate adds nothing and compares its read-only projection with
the frozen expectation; it does not rerun the oracle or re-observe the historical mutation.
Candidate-only mutation, process-byte, exit-status, Git-status, fixture, file-mode, or
cleanup-evidence divergence still fails. Replay marks these cases
`PASS-WITH-ACCEPTED-DIVERGENCE` and reports their count; they are not clean parity passes.

A case may declare `knownDivergence` only for an adjudicated `python-defect` whose observable is
bounded process output rather than a repository mutation: the candidate satisfies ratified spec
text that the retired oracle contradicted, so its frozen expectation differs (`GPK-V0-033`). Nine
case declarations are admitted across eight distinct divergence classes. This inventory is by class
(the divergence-register entry), and each class names its manifest case count and case IDs:

- `DR-0007` / `CF-V0-031` (1 case: `impact-self-authored-adr`) — three stdout rewrites for the
  withheld authority label, its confidence, and the uncertainty entry that explains them.
- `DR-0008` / `GPK-V0-039` (2 cases: `harness-user-prompt-out-of-scope` and
  `query-repository-out-of-scope`) — eight stdout rewrites per case for `abstention`, the three
  coverage counts, `critical`, `results`, `state`, and `verification`. The same hermetic accidental
  one-word match independently discriminates both adapters that compile query packets.
- `DR-0009` / `GPK-V0-040` (1 case: `impact-ranked-past-limit`) — three stdout rewrites for the
  omitted count, admitted count, and uncertainty produced when `--limit 3` ranks four of seven
  admitted results out.
- `DR-0015` / `GPK-V0-043` (1 case: `query-version-token`) — seven stdout rewrites for the inactive
  `abstention`, positive included and requested counts, syntax-only uncertainty, nonempty `results`,
  `READY` state, and verification plan that replace the oracle's withdrawn packet.
- `DR-0016` / `GPK-V0-028` (1 case: `query-repository-task-oversized`) — no stdout rewrite and one
  stderr rewrite: both sides refuse an 8,001-character task, but the candidate names the 8,000-byte
  bound and the frozen oracle expectation names its retired 2,000-byte bound.
- `DR-0017` / `GPK-V0-027` (1 case: `impact-go-root`) — five stdout rewrites for the reverse-import
  row admitted by root-package resolution, its three coverage counts, and the importer verification
  command.
- `DR-0025` / `GPK-V0-040` (1 case: `query-repository-limit-1`) — three stdout rewrites for the one
  omitted result, the two-result admitted count, and the omission uncertainty when `--limit 1`
  ranks one of two admitted results out.
- `DR-0035` / `GPK-V0-044` (1 case: `query-repository-trace-matching`) — two stdout rewrites that add
  the trace-recording commit to the changed-path and opened-path learned-evidence reasons.
- `DR-0042` / `GPK-V0-075` (2 cases: `impact-python-module` and `impact-python-nomodule`) — two
  stdout rewrites per case that name `tests/test_engine.py`, the importing test carrying the
  `feature:a-first` and `scenario:z-last` markers, where the oracle says the changed path carries them.

A case may also declare `exclusionCountDivergence`, a separate field that composes with
`knownDivergence` and follows the same rewrite and `packet_bytes` rules below. Twenty case
declarations are admitted for one class:

- `DR-0023` / `GPK-V0-063` (20 cases, closed in `exclusionCountDivergenceCases`: the three
  `harness` context cases, `query-authority-start-default`/`-learned`/`-limit-50`/`-non-ascii`,
  `query-clean-authority-start`, the eleven receipt-emitting `query-repository-*` cases, and
  `query-version-token`). Each has exactly one stdout rewrite of the `exclusions.count` value, which
  must be a larger integer than the oracle's. The candidate counts the fixture's `.gitignore`, a
  tracked path that the suffix allow-list never reads. The value was enumerated from the fixture
  tree, not from the candidate (decision 0166).

Each rewrite names what the clause requires the candidate to emit and the historical oracle bytes
stored in their place. Each must occur **exactly once** in its declared process stream, and once every
rewrite is applied the whole stream must equal the frozen expectation byte-for-byte. A candidate that
stops emitting the spec-required form fails the uniqueness check rather than passing quietly, which
is what keeps the declaration from becoming an exclusion. For stdout receipt rewrites, the receipt's
own `packet_bytes` count is deliberately NOT declared: no clause can author a byte count and
transcribing it would take an expected byte from the candidate, so the runner instead requires the
candidate and frozen historical counts to differ by exactly the length the declared rewrites account
for — plus the width of the count's own encoded member. That member is part of the packet it counts
and moves when a rewrite carries the count across a digit boundary. The declaration applies only to
the candidate projection; the frozen manifest expectation retains the retired oracle's exact
captured bytes. Replay marks these cases `PASS-WITH-KNOWN-DIVERGENCE` and reports their count. The count covers declarations, so a case carrying both fields counts twice.

Rows have closed evidence states:

- `parity` compares the Go candidate with an immutable Python-produced historical expectation.
- `unsupported` validates the owner-requested, contract-scoped refusal type, nonzero exit, empty
  stdout, and exactly one bounded JSON error object with `ok:false` and one trailing LF. Error bytes
  remain diagnostics and are never frozen expectations.
- `NOT_RUN` inventory and qualification cells remain visible. They are never skips or parity passes.

The POSIX runner bounds input, output, execution, shutdown, pipe draining, and owned-process-group
quiescence. That proves owned-process-group cleanup. A descendant that deliberately creates another
session is outside that containment; detached-descendant cleanup is therefore `NOT_RUN`, and the full
GPK-V0-005 process matrix remains `PARTIAL` rather than being inferred from the seed.

## Run

Build the candidate, then replay the committed corpus:

```sh
GOTOOLCHAIN=local go build -o /tmp/corvint-cli-parity ./cmd/corvint
GOTOOLCHAIN=local go run ./conformance/cli-parity-v0 replay \
  --manifest conformance/cli-parity-v0/manifest.json \
  --candidate /tmp/corvint-cli-parity
```

Omitting `--candidate` never falls back to a `PATH`-resolved `corvint`: a manual sweep run that way
builds the manifest's `candidateCommand` fresh from `go.mod`'s module root into the replay workspace
instead, so a stale installed binary elsewhere on `PATH` cannot be scored in its place. An explicit
`--candidate` is still resolved like any other command, `PATH` included, since it names the exact
binary the caller chose to exercise.

To exercise bounded parallel replay without changing the corpus, copy the committed manifest and
change only its top-level `workers` value. A value above `GOMAXPROCS` is capped rather than creating
unbounded fan-out.

`capture` and live `--oracle` overrides are retired compatibility surfaces that refuse; they are not
refresh instructions. `replay --only <id-prefix>` runs one slice while authoring it, which turns a
multi-minute corpus pass into seconds. It prints `SUMMARY-FILTERED ... NOT-CORPUS-EVIDENCE` instead
of `SUMMARY` and withholds the command inventory, so a filtered run can never be read as corpus
evidence or as a statement about any command's certification. Only an unfiltered replay produces
either.

## Current seed and extension rule

A case may declare `structuralComparison` for a field that measures the run rather than results from
the computation (decision 0005). The sole admitted declaration is `eval-frozen-corpus` field
`stdout.evaluation.metrics.latency_ms` with kind `measured-run-latency`. The field MUST be present,
numeric, and non-negative; only then is its value replaced by a placeholder before comparison. A
missing, non-numeric, or negative value still fails, and every other byte stays exact. That case is
retired under decision 0308, so the declaration is validated but not exercised.

The `impact` seed carries six fixtures. `impact-go` is the clean tracked-path case. `impact-go-root`
is a root-package `.go` file with a sibling test and a nested importer (decision 0023, DR-0017): the
candidate admits the importer through the module path, the oracle's `module/.` target never finds
it, and the case is known-divergent under five rewrites (below). `impact-adr` is
the same repository plus `docs/adr/0012-token.md` (`status: accepted`) and a ledger record citing it,
which is the only fixture in the corpus with a `docs/adr/` tree — before it, the ADR authority branch
of `impact` was unreachable by every case, so no change to authority resolution on either runtime was
observable. It is the discriminating case `DR-0007` required.

`impact-python` and `impact-web` are the two rules `GPK-V0-027` gained on 2026-08-29 (decision 0007,
D2), which widened the supported profile from `.go` to any suffix the specification names a
reverse-import rule for. `impact-python` holds a package with an `__init__`, importers that name the
module by all three dotted spellings the oracle admits, and a marked test importer, so
`impact-python-module` and `impact-python-package` cover the module and package forms separately,
and `impact-python-nomodule` replays the module form on the same tree without `go.mod` or any Go
source, the profile decision 0015 admitted.
`impact-web` holds a component under `internal/web/app/src/` reached by a sibling, a two-level `..`
climb, the profile-configured `@/` alias, and a directory specifier that resolves through an
`/index` child, plus a bare specifier that must NOT resolve because it names a dependency;
`impact-web-component` and `impact-web-directory` cover the file and directory forms. Every
specifier in `impact-web` is a live statement rather than a commented-out or quoted one, so these
rows carry no `DR-0011` entanglement — see that entry.

`impact-truncated` is `impact-go`'s shape plus three further same-package tests that carry no markers
and exist only to be ranked, so a change to `pkg/sample.go` admits seven results and
`impact-ranked-past-limit` can issue `--limit 3` and rank four of them out. It is the discriminating
case `DR-0009` required: before it, no case in the corpus ranked past its own limit, so the
truncation arithmetic was never exercised and the suite was green in both directions whether or not
coverage was denominated in the admitted universe. It has its own fixture rather than borrowing
`impact-go`'s, which exists to serve an unqualified byte-exact row at `--limit 10` and may
legitimately change shape.

The `cem` seed certifies all seven actions, read and mutating alike, plus the whole argparse
surface: the argument negatives pin argparse's own ORDER (a parse-time failure beats a missing
required argument, which beats an unrecognized one), and the semantic negatives each hold a code the
candidate previously got wrong. `cem status` reports the resolved map path, so it carries a
`locationNormalization`; `cem verify` reports no path and carries none.

The four mutating actions each carry a success case with mutation evidence under the `cem-artifact`
class: `begin` creates a map from a supplied patch, `prepare` both creates (`cem-prepare-create`) and
resumes (`cem-prepare-resume`) while deriving the patch cache, and `cite` and `mark` rewrite an
existing map — `cite` twice, once per span form, because `--lines` compiles to bytes through a path
`--bytes` never takes. Every artifact these touch sits outside the index (the map under the
gitignored `.corvint/`, the cache inside the Git directory), so repository content moves while Git
status must not.

The seed also pins the oracle's two argparse **conversion types**, which the candidate had been
missing entirely. `_path` (declared on `cem prepare --map`/`--patch` and `cem cite --evidence-path`,
and on no other cem argument — `begin --patch` and `cite --map` are `type=Path` and get no such
check) rejects an absolute, blank, bare-`.`, or dot-dot value at PARSE time, which is what keeps the
code `invalid-arguments` rather than a later containment code. `_span` splits into seven distinct
outcomes the candidate had collapsed into one message: four parse-time conversions (`span must be
START:END`, `value must be an integer`, `value must be non-negative`, `span end must not precede its
start`) and four semantic codes (`invalid-span`, `span-out-of-range`, `invalid-line-range`,
`line-range-out-of-range`). Eight cases pin them individually, because a single collapsed message
passes a test that only checks "it failed".

One `cem prepare` rejection path was parked as a `spec-gap` while it was authored and is now shipped:
the oracle creates the patch cache's private parent before validating its revisions, so a rejected
invocation leaves `.git/corvint` behind where Go leaves nothing. `GPK-V0-008` originally constrained
only the artifact, not an empty parent directory, so the spec did not decide it. The clause was
amended to require a rejected mutator to leave the worktree and Git directory byte-identical
(operator decision, 2026-08-28); Go already satisfied it, Python became known-divergent with `src/`
deliberately unrepaired, and `cem-prepare-unresolvable-base` now carries the divergence explicitly.
See `../divergence-register.md` (DR-0003).

A fixture may declare `parentFiles`, an entry set forming a first commit whose tree `files` then
replaces. A single-commit fixture cannot exercise any command that compares a base revision against
a target, because the two are necessarily equal; that is what kept a SUCCESSFUL OCM verification
out of the corpus. `ocm-linked` uses it, and its claim anchor is a Go table-case rather than a
Python test on purpose: `GPK-V0-037` requires a typed `unsupported-ocm-python-claims` refusal for a
`.py` claim, so no Python test can reach that path.

The `feature`, `init`, and `adopt` seeds certify commands that already dispatched but had never been
replayed. All three are read-only and their receipts carry no absolute path and no measured value,
so every case is unqualified. `adopt` shares `init`'s bootstrap parser, so the option matrix is
certified once under `init` and `adopt` covers only what its own history walk changes.

The `harness` seed carries all six lifecycle events, both compact rehydration shapes, six argument
and input negatives, two hostile bounds, and a non-ASCII stdout case, with no qualification of any
kind. The non-ASCII case pins Python-compatible `ensure_ascii` spelling for BMP characters and an
astral surrogate pair while leaving the receipt digest basis unchanged. Its two index-backed events
— `file-change`, and a compact start whose dirty set contains tracked paths — inherit the Go impact
profile, so the fixture holds a slash-qualified Go module with a non-root package. Outside
that profile Go refuses (`unsupported-impact-repository`, `unsupported-impact-path`,
`unsupported-impact-path-suffix`) where Python answers. The suffix half of that profile was widened
on 2026-08-29 to `.py` and the web set; the rest — the slash-qualified module, the non-root package
— is unchanged.

The `harness` seed carries a second fixture for one case. `harness-out-of-scope` names its canonical
feature `a11y-high-contrast` — a hyphenated id with a common English component — so that
`harness-user-prompt-out-of-scope` can issue a query sharing exactly one accidental word with the
repository and reproduce that accident hermetically, without depending on any downstream corpus. It
is the discriminating case `DR-0008` required: before it, no case in the corpus issued a below-floor
query, so the suite was green in both directions and its green said nothing about `GPK-V0-039`.

**The two mechanisms differ in scope, deliberately.** During replay, `locationNormalization` changes
only a declared repository-root prefix before the candidate is checked against the frozen manifest.
`structuralComparison` replaces a declared measured value before the same comparison because that
value cannot be stable across runs. Each is a narrow reduction confined to its declared fields.

Replay names every qualification a case carries — a case declaring both prints both lines — so no
qualification is hidden behind another and neither reads as an unqualified byte-exact pass.

A fixture entry may use `"type": "untracked-file"`. It is materialized to disk exactly like a
`file` but is never added to the Git index, so it does not affect the fixture tree or commit
revision. This exists for working-tree artifacts that are gitignored in real use and whose content
must name the fixture's own commit: the CEM map records `baseRevision`, so committing it would make
the tree hash depend on a revision derived from that same tree. Keeping it untracked breaks the
cycle structurally rather than by pinning a precomputed digest, and the fixture still fails closed
because a changed tracked file moves the commit and the recorded `baseRevision` stops resolving.

The seed contains only the historically captured profiles: the authority-start `query`
prompt at limits 1, default, and 50, with one uniquely highest root `AGENTS.md` and an absent
clean-tree trace store; default tracked nested-Go `impact`; the ported `migrate-traces` profiles; and the reproducible
positive, negative, and hostile `record` profiles. The positive case declares only its
location-dependent `stdout.store` repository-root prefix; the hostile case retains the accepted
directional lock divergence. Boundary query profiles are explicit generic typed refusals.
Go-only untracked/range impact profiles, partial CEM/harness surfaces, SHA-256 repositories,
non-POSIX execution, and unclaimed command profiles are named `UNSUPPORTED` or `NOT_RUN`.

When a command port lands:

1. Add or extend a synthetic fixture without Corvint or other product source.
2. Add sorted positive, negative, and hostile cases with bounded stdin and stable requirement IDs.
3. Change its command-inventory status only after replay evidence exists.

The replay-only runner cannot add or refresh immutable expectations. A row without an existing
independent expectation remains `UNSUPPORTED` or `NOT_RUN`. An unavailable fixture, candidate,
process-containment capability, or executable case fails the run unless the manifest explicitly
records that state.

## General repository query profile

`GPK-V0-043` widens the standalone `repository` intent and, since decision 0013, routes
`agent-tooling` through the same path. The synthetic `query-repository`
fixture freezes one canonical feature, scenario, source marker, and Go symbol without depending on
Corvint source. Fresh processes cover the default limit, explicit limits 1, 10, and 50, unbudgeted
packets, a 2,200-byte budget that changes packet selection, empty and oversized tasks, malformed and
out-of-range limits and budgets, and read nonmutation. The pinned Python oracle produced every
parity expectation; `query-repository-out-of-scope` carries the registered relevance-floor
divergence instead of laundering the oracle's one-word accidental match into a pass.
`query-version-token` is a second fixture of one Go symbol and no versioned path, queried with a
task carrying `v18`; it is the discriminating case `DR-0015` required, since every other query
fixture is unversioned too but no task in the corpus carried a version token, so the oracle's
unconditional filter and decision 0017's conditional one were green together.

Non-ASCII repository text (`query-repository-non-ascii`) and `agent-tooling`
(`query-repository-agent-tooling`) replay against frozen expectations since decision 0013 lifted
their `GPK-V0-043` refusals; both replay byte-exactly, matching the historical withdrawal. The
authority-start profile replays at the default limit, at limits 50 and 51 (including the frozen
limit-error expectation), on a non-ASCII task, and with advisory learned paths
(`query-authority-start-learned`, whose prompt shares two terms with the fixture commit subject).
The missing-authority and present-trace-store refusals remain unchanged.

`GPK-V0-044` adds four oracle-authored repository-intent rows over deterministic fixture trace
stores: matching passed evidence, nonmatching plus non-passed evidence, absent/mixed-worktree state,
and compact budget selection. They replay exact packet bytes, including trace counts, advisory
candidates, evidence authority, history digest, and compacted learning fields. The unchanged
`query-present-trace-store-refusal` still exercises `project-operations`; it does not describe the
repository-intent profile. Malformed, unreadable, ancestry-fault, mid-read drift, and mutation
checks remain focused Go regressions because this runner's static fixtures cannot inject those
process-time faults portably.

## `ocm` is certified across all seven actions

All seven actions replay byte-exactly — stdout **and** the written map. The three read actions
(`status`, `verify`, `report`) are joined by the three write actions (`prepare`, `link`, `mark`),
and `unsupported-ocm-action` no longer exists: every action is ported, so a refusal would now assert
a divergence rather than an agreement.

The write actions carry mutation evidence, not just process bytes. `ocm-link-obligation` and
`ocm-mark-obligation` declare `mutation: "ocm-map"` and are checked against independent before/after
repository and file-mode snapshots; `ocm-prepare-resume` declares `mutation: "none"` because a
resume that finds an equivalent binding must not rewrite the map. One caveat is deliberate rather
than hidden: the OCM map publisher rewrites the map owner-private (`0644`→`0600`), so the mutation
validator does not assert that file modes are unchanged across a write action.

Each write case was proved to fail on a **value**, not on a binding digest, by seeding one defect at
a time into the candidate and replaying:

| Case | Seeded defect | Caught by |
|---|---|---|
| `ocm-link-obligation` | claim span `end` off by one in the written map | `repositoryAfterSha256` |
| `ocm-mark-obligation` | `disposition` written as `unresolved` instead of `unknown` | `repositoryAfterSha256` |
| `ocm-prepare-resume` | `resumed` flag inverted in stdout | `stdout` |

The first two are invisible to stdout and were caught only by the repository snapshot, which is what
makes the map itself — not merely the report about it — covered evidence.

**A SUCCESSFUL OCM verification is now reached**, by `ocm-verify-linked`, which exits `0` — and
`emitOCMEnvelope` returns `0` only for `ok: true`, so the exit status is the verification result,
not merely a clean run. (`ocm-verify-cem-binding` still covers the full parse, canonical-form, and
binding path ending `invalid-cem` at exit `1`.) Success had been structurally unreachable: a
single-commit fixture makes an OCM's `targetRevision` necessarily equal the CEM's `baseRevision`.
The `parentFiles` mechanism described above supplies the second commit. `GPK-V0-037` still requires
a typed `unsupported-ocm-python-claims` refusal for any `.py` claim, so the fixture's test blob is
Go rather than Python — with a Python claim the success path stays
unreachable by contract.

Two `ocm` argument-error paths also diverge in prose while agreeing on code and exit status: `ocm
bogus` (Python appends the choice list) and `ocm --bogus` (argparse reports the missing subcommand
before rejecting the option). Same class as the `lrf` finding below — the message text is
unconstrained — so neither is a parity case.

## `lrf` error-path bytes are outside byte parity by spec

`docs/specs/lexical-relevance-floor-v0.md:314-320` constrains structural failures to preserve their
**error code** and exit `2`, and states that I/O and operational aborts carry "no conformance claim
about stderr bytes". Measured 2026-08-28, Go and Python agree on code and exit status but differ in
message text on at least three error paths: a missing `--cem` file (Go `map-unavailable` with the
absolute path; Python emits no `code` field at all), a structurally invalid map (both
`missing-field`, different prose), and a map outside a repository (both
`repository-object-unavailable`, different prose).

Neither runtime is wrong: the spec constrains the code, not the prose. So these paths are
deliberately **not** parity cases, and `lrf-missing-cem` covers the one error path that is
byte-stable because argument parsing precedes the unconstrained surface. Stated rather than hidden:
**`lrf` error message text has no parity coverage, by spec design.**

## `lrf --ocm` refuses a Python claim rather than approximating its grammar

`lrf-ocm-python-claim-refusal` is the fifth generic typed refusal, authorized by `GPK-V0-041`. The
`lrf` OCM leg used to verify a `.py` claim with `pythongrammar.SourceSyntaxValid`, the analyzer's
closed fact-extraction subset, while the oracle used `ast.parse` under the frozen Python 3.9
grammar. `GPK-V0-014` admits only the exact `python-ast/1` grammar or an abstention and names the
third option — "accepting the Go host's idea of Python syntax, or silently narrowing Python
support" — as a parity failure, so this was a `go-defect` (DR-0010) and Go was repaired to abstain,
exactly as `GPK-V0-037` already requires of `ocm status|verify|report`.

The cost is stated rather than hidden. The fixture `lrf-ocm-python` carries a **valid** Python
claim, linked by the oracle's own `ocm link`, and on it the pre-repair candidate was byte-identical
to the oracle. The repaired candidate refuses it. Agreement on the blobs that were tried is not
parity when the grammar underneath is a different grammar: the same code path published a full
evaluation of a claim the oracle rejected outright as soon as the blob left the closed subset. So
the `lrf` `.py`-claim profile is `UNSUPPORTED`, never parity, and it is a refusal row rather than a
`knownDivergence` because a `go-defect` is repaired, not declared.

The refusal is candidate-only evidence and has no captured expectation — `capture` never ran for it,
and could not have. Its argv is pinned to one invocation in `validLRFPythonClaimRefusalArgv`. The
row was proved live: with the single `rejectPythonClaims` argument flipped back to `false` and the
resulting binary resolved from `PATH` as `corvint`, the row FAILS with `not a bounded nonzero typed
refusal`.

`query-repository-budget-selection` now supplies the former missing `--budget-bytes` parity
coverage. Its immutable provenance records production by the retired oracle without a candidate;
the replay-only runner cannot replace that frozen expectation with candidate-produced bytes.
