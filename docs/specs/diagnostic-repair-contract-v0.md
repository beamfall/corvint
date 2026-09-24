# Diagnostic Repair Contract V0

Owner: Russell Lewis
Date: 2026-09-12
Requirement prefix: `DRC-V0`
Intent status: accepted (decision 0201, 2026-09-13, with clause amendments; `DRC-V0-001` subject shape, `DRC-V0-004` registry location and `DRC-V0-011` ratchet start by owner instructions 2026-09-12)
Delivery status: implemented: envelope, fix registry, screen/bound, the seven-site working-tree impact conversion with its additive CLI envelope, thirty further refusal sites across twenty codes, repair-stop and the coverage ratchet at thirty-seven sites
Authoritative inputs: `../../AGENTS.md` invariants 2, 4 and 5, `../SPEC-DRIVEN-DEVELOPMENT.md`,
`self-observation-ledger-v0.md` `SOL-V0-007` (the one write a refusal is already allowed to make),
and `../agent-memory/fixes.md` (the refuse-without-naming-the-repair entries that motivated this
spec; both were closed before acceptance, decision 0201).
Prior art: the diagnostics contract in `github.com/tt-a1i/archify` (MIT) pairs a stable rule code
with a subject, measured evidence, and a closed `supportedFixes` list. The pattern is cited as an
independent convergence on Corvint invariant 2; no code, schema, or text was taken from it.

## Agent digest
- Claim: A repairable Corvint refusal names its subject, its measured evidence, and a closed list of permitted repairs instead of free prose.
- Status: accepted (decision 0201, 2026-09-13, with clause amendments; `DRC-V0-001` subject shape, `DRC-V0-004` registry location and `DRC-V0-011` ratchet start by owner instructions 2026-09-12) / implemented: envelope, fix registry, screen/bound, the seven-site working-tree impact conversion with its additive CLI envelope, thirty further refusal sites across twenty codes, repair-stop and the coverage ratchet at thirty-seven sites
- Exists: the `internal/diagnostic` envelope, screen/bound, `RepairStop` and coverage ratchet (`make diagnostic-coverage-check`), `docs/specs/FIX-REGISTRY.tsv`, the seven converted `unsupported-working-tree-impact-*` sites (`internal/worktreeimpact/diagnostics.go`) and their additive CLI envelope (`cmd/corvint/diagnostic_envelope.go`), and thirty further converted sites (`cmd/corvint/diagnostic_refusals.go`, `internal/touchsurprise/diagnostics.go`) covering the root, platform, impact budget option, impact path suffix, batch snapshot, affected, prove, checkpoint object-format, attestation key and envelope, CEM map, prove-observe document and touch-set surprise refusals; every other refusal still carries a stable code plus prose (`internal/lrf/types.go:186`, `cmd/corvint/batch.go:190`).
- Blocked on: nothing; accepted by decision 0201.
- Read next: Requirements; Trust boundary, limits, and failure modes.

## Human intent and scope

A Corvint refusal is currently a stable machine-readable code and a sentence of English. The code
says which rule fired; the sentence is the only place the refused input and the available repair
appear, and it is prose an agent must guess at. `internal/worktreeimpact/compiler.go:127` is the
good case: it names the unsupported input and two exact repairs, in prose. `cmd/corvint/local_completion.go:187`
is the bad case: it emits the code as both code and message, so the caller learns that something was
refused and nothing else. Between those two shapes there is no contract, so a caller cannot tell a
refusal it can repair from one it cannot, and cannot tell whether it has exhausted the repairs or
merely failed to imagine one.

The cost is recorded in the backlog rather than hypothesized. `../agent-memory/fixes.md` carried the
2026-09-07 entry for two dogfood coordinator inputs that refuse without naming what to fix, and the
2026-09-05 entry for a replay-window truncation count disclosed only inside error text. Both are the
same defect: the refusal holds the facts the caller needs and discloses them in a form only a human
reader can use. An agent facing that refusal has two bad options, guessing a repair or abandoning the
task, and guessing is how a bounded refusal turns into invented certainty — the failure invariant 2
exists to prevent.

Affected user: an agent or engineer whose Corvint command refused, and any harness adapter that must
decide whether to retry, escalate, or stop. Measurable job: from one refusal, without reading prose
and without inspecting Corvint source, identify the exact refused input, the measured facts that
caused the refusal, the complete set of repairs Corvint will accept, and whether any repair exists at
all.

In scope is how a refusal is reported. Out of scope is which inputs Corvint refuses: this contract
changes no refusal decision, no threshold, and no code spelling.

## Verified current state

Refusals are a `{code, message}` pair. `internal/lrf/types.go:186` defines the two-field error type
directly; `internal/doccompiler/errors.go:12` and `internal/worktreeimpact/compiler.go:536` are
per-package `failure` constructors of the same shape. The CLI serializes that pair as
`{"ok": false, "error": {"code": ..., "message": ...}}` (`cmd/corvint/batch.go:190`,
`cmd/corvint/source_handoff.go:222`), and at least one site emits the code in both fields
(`cmd/corvint/local_completion.go:187`). No site carries a subject field, a measured-evidence
field, or an enumerated repair set, and no registry of admissible repairs exists. The
`unsupported-*` code family is the one part already load-bearing beyond display: `SOL-V0-007` writes
those codes to the self-observation ledger.

## Requirements

- `DRC-V0-001`: Every refusal a covered site emits MUST carry a `subject` object with exactly two
  required string fields, `kind` and `value`, and a consumer MUST be able to identify the refused
  input from `subject` alone, without parsing `message`. `kind` MUST be one of the closed vocabulary
  `argument`, `value`, `artifact`, `repository-state`, `host-capability`, and `request`; a kind
  outside it MUST be a gate failure. `value` MUST be the exact spelling of the refused thing and
  MUST NOT carry prose, an explanation, or a second fact. A qualifier the refusal was measured
  against, such as the captured revision a path was checked in, belongs in `evidence` and MUST NOT
  be concatenated into `value`. Mechanically, `value` MUST be non-empty valid UTF-8 with no control
  character; the prose and second-fact bans are enforced by a frozen per-site expectation. Where the
  refused thing is a repository property no input spells, `value` is a stable property token and the
  value actually read is `evidence`. A covered site is one whose refusal is built from a
  `diagnostic.Refusal` literal; an uncovered site keeps `{code, message}` until converted
  (decision 0201 (a), (b)).
- `DRC-V0-002`: Every covered refusal MUST carry an `evidence` array of name/value pairs holding only facts
  measured while producing that refusal: counts, byte sizes, resolved revisions, and the value
  actually read. A refusal that measured nothing MUST emit an empty array rather than omit the
  field, and MUST NOT place an inferred, predicted, or default value in it.
- `DRC-V0-003`: Every covered refusal MUST carry a `supported_fixes` array whose entries are identifiers
  drawn from the project-owned fix registry. Each entry MUST be a bare identifier whose registry row
  names, in its `target` and `admissible` columns, the field, argument, or file the caller changes
  and the admissible target value or shape (decision 0201 (d)). No entry may be free prose instructing
  the caller, and the array MUST be the complete set of repairs Corvint will accept for that subject.
- `DRC-V0-004`: The fix registry MUST be exactly `docs/specs/FIX-REGISTRY.tsv`, one tracked
  project-owned file beside the specs, carrying two closed vocabularies with a leading column naming
  which vocabulary each row belongs to: every admissible fix identifier with its parameters, and the
  `subject.kind` tokens of `DRC-V0-001`. The registry's kind rows MUST equal that clause's list
  exactly, and the gate MUST fail on any disagreement rather than trust either copy.
  It carries the same owner-owned authority as the specs it sits beside, so a row is added in an
  ordinary spec change and no `docs/decisions/` signature is required per identifier; a row MUST NOT
  be added by a generator or by model inference. An identifier emitted but absent from the registry
  MUST be a gate failure, and no fix identifier may be produced by model inference at emission
  time.
- `DRC-V0-005`: A refusal for which no caller-side repair exists MUST emit an empty
  `supported_fixes` together with a `terminal` field naming why, one of the closed tokens
  `missing-authority`, `absent-evidence`, `unsupported-platform`, and `owner-decision-required`.
  `terminal` MUST be emitted exactly when `supported_fixes` is empty (decision 0201 (e)). An omitted `supported_fixes` MUST NOT be used to
  mean either "no repair exists" or "the repair set is unknown".
- `DRC-V0-006`: The four fields above MUST be additive to the existing envelope. Every `code` value
  present in the tree at acceptance keeps its exact spelling and meaning, and a consumer keyed only
  on `code` — including the `SOL-V0-007` ledger writer — MUST keep working unchanged. The survival
  replay covers the codes whose envelope a conversion touches and a byte-identical envelope for an
  unconverted refusal; the tree-wide code inventory stays owned by `ECO-V0` (decision 0201 (f)).
- `DRC-V0-007`: Emitting a diagnostic MUST NOT mutate repository or trace state. The one permitted
  write remains the `unsupported-*` ledger append `SOL-V0-007` already grants, whose row keeps its
  code, intent, and task-hash shape; none of the new fields may reach that row or become an input to
  ranking, learning, evidence, or authority.
- `DRC-V0-008`: `subject` and every `evidence` value MUST pass the existing secret screen before
  emission, and MUST be bounded at 512 bytes per value and 16 evidence pairs per diagnostic.
  Truncation MUST be disclosed in the emitted value rather than applied silently. The screen runs
  before the bound; a value is cut at a rune boundary with its disclosure counted inside the 512
  bytes; evidence names are bounded the same way; more than 16 pairs keeps the first 15 plus one
  `evidence-truncated` pair holding the dropped count. A redacted or truncated `subject.value` is
  not the exact spelling `DRC-V0-001` asks for, and its disclosure says so (decision 0201 (c)). A
  refused operand spelled as the empty string is emitted as the disclosure `[empty]`, since an
  emitted `subject.value` is never empty.
- `DRC-V0-009`: The bounded repair loop MUST be stated in the contract a caller reads: apply only
  fixes listed for the named subject, measure progress by the count of outstanding diagnostics, and
  stop after two consecutive rounds that fail to reach a new minimum of that count. On stopping, the
  remaining diagnostics MUST be reported unresolved rather than described as a partial success.
  A round leaving nothing outstanding ends the loop resolved, and a round reaches a new minimum
  only when its count is below every earlier count, including the count before the first round.
  Corvint drives no loop; the rule is published as the pure function `diagnostic.RepairStop` an
  adapter may call (decision 0201 (h)).
- `DRC-V0-010`: A non-zero exit or an `ok: false` envelope MUST NOT be reported as success at any
  layer, and a command that refused MUST NOT leave the caller inspecting a previously produced
  artifact as though it were the refused candidate. The assertion runs at every layer that carries a
  covered family; for `unsupported-working-tree-impact-*` that is the CLI alone, since `batch` does
  not route `--working-tree-untracked` and no host adapter calls `impact` (decision 0201 (i)). The
  later families are likewise the CLI alone: each reaches the caller only through `emitError`, the
  batch snapshot refusal is `batch`'s top-level envelope rather than an operation row, and no host
  adapter calls the verbs that emit them. The one exception is `unsupported-impact-path-suffix`,
  which three layers share: the `impact` verb, a `batch` impact operation row (`ok: false`, carrying
  the same members beside its `SBQ-V0-004` `message` and `code`) and a `harness event` `file-change`
  envelope (exit 2); the host adapters that invoke `harness event` key only on `code` and degrade.
- `DRC-V0-011`: A gate MUST enumerate every diagnostic-emitting site under `internal/` and `cmd/`
  and fail when a site emits a refusal with no `subject`, with no `supported_fixes`, or with a fix
  identifier absent from the registry. Adoption is a ratchet: the gate records the covered-site
  count and fails when that count decreases. The starting count MUST equal the number of emitting
  sites in the first converted family, measured at conversion, and MUST NOT be zero. That family is
  `unsupported-working-tree-impact-*`, whose seven emitting sites are all in
  `internal/worktreeimpact/compiler.go` (lines 84, 96, 113, 116, 127, 141, 144), so a gate wired at
  fewer than seven covered sites is itself a failure. Each of those sites calls one constructor whose
  `diagnostic.Refusal` literal is in `internal/worktreeimpact/diagnostics.go`; the gate counts such
  literals in non-test Go, and the recorded count in `script/diagnostic-coverage.count` MUST equal
  the measured count (decision 0201 (j)). A later family keeps that shape, one constructor per site
  holding one literal, and raises the recorded count in the converting change. A form the literal
  scan cannot inspect MUST also fail the gate: a `diagnostic.Error` literal with no `Refusal` member,
  a `diagnostic.Refusal` collection literal whose elements elide their type, a zero-valued
  `diagnostic.Refusal` declaration or `new(diagnostic.Refusal)`, and a dot import of
  `internal/diagnostic`. The CLI envelope emits the `Bounded` form without calling
  `Refusal.Validate`, so a literal that form would still fail MUST also fail the gate (2026-09-13 bug
  hunt): a repeated fix identifier, and an `Evidence` member that is not a list literal or whose pair
  names are not non-empty single-line string literals.
- `DRC-V0-012`: `message` MUST remain human-readable prose and MUST NOT be parsed by any Corvint
  consumer, gate, hook, or adapter. A behavior that depends on `message` text MUST be a failure
  under `DRC-V0-011`. The mechanical check is bounded to non-test Go that imports
  `internal/diagnostic`: such a file MUST NOT pass `.Message` or `.Error()` to a `strings`, `bytes`,
  or `regexp` function or compare either with `==` or `!=`; other consumers stay review-only
  (decision 0201 (k)).

## Non-goals and simpler baseline

Corvint never applies a repair. `supported_fixes` is an enumeration the caller chooses from; there is
no auto-fix verb, no retry driver, and no loop orchestration inside Corvint. No model generates
remediation text at emission time: a fix identifier either exists in the registry or is a gate
failure. No refusal decision, threshold, or code spelling changes. No new command or verb is added.
The fields are not a diagnostics event stream and are not persisted anywhere beyond the ledger write
`SOL-V0-007` already grants.

The simpler baseline this contract must beat is the present one: keep `{code, message}` and let each
caller read the prose. That baseline is adequate for a human at a terminal and is why the shape has
survived. It fails only for a non-human caller, so if agent callers turn out to repair refusals
reliably from prose alone, the baseline wins and this spec should be killed.

## Trust boundary, limits, and failure modes

Picking a `kind` is mechanical, not a judgment call. `argument` names the argument when the fault is
with the argument rather than with any one value it carries, so a count violation names `-path` and
not the hundred paths. `value` is one caller-supplied value Corvint only validated, such as a rejected
path, selector, or requirement id. `artifact` is a file whose contents Corvint actually read, named by
its repository-relative path, such as a trust attestation or a patch plan. `repository-state` is a
property of the repository or the captured index that no input names, such as a missing revision
index or an unqualified module. `host-capability` is a platform facility, such as process
containment. `request` is reserved for a request malformed as a whole.

`subject` and `evidence` values derive from caller input and from repository paths, both untrusted.
They are emitted, never executed, and never fed back as authority; `DRC-V0-008` bounds their size
and screens them. The fix registry is project-owned authority under invariant 3, so it outranks any
identifier a model might propose, and `DRC-V0-004` makes an unregistered identifier a gate failure
rather than a silently accepted extension. The contract adds no write beyond `SOL-V0-007`.

| Failure | Behavior |
|---|---|
| Site emits a refusal with no `subject` or no `supported_fixes` | gate failure, naming the site |
| Site emits a fix identifier absent from the registry | gate failure, naming the identifier and site |
| No repair exists for the refused subject | empty `supported_fixes` plus a `terminal` reason |
| Repair set is genuinely unknown for a site not yet covered | site stays outside the ratchet's covered count; it MUST NOT emit a guessed fix list |
| Evidence value exceeds 512 bytes | truncated with the truncation disclosed in the value |
| Caller parses `message` to decide behavior | gate failure under `DRC-V0-012` |
| Two repair rounds reach no new minimum | caller stops and reports the remainder unresolved |

## Acceptance criteria and testing matrix

Delivery is `implemented`; every row is `PASS` with its test. The criteria are deterministic so
that an implementation cannot be graded by inspection.

| Requirement | Evidence | Status |
|---|---|---|
| DRC-V0-001, DRC-V0-002, DRC-V0-003, DRC-V0-005 | one refusal fixture per shape (repairable, terminal, measured, unmeasured) whose emitted JSON is compared field-by-field against a frozen expectation, plus a planted `subject.kind` outside the vocabulary and a planted qualifier concatenated into `value`, both of which fail | PASS — `TestDiagnosticEnvelopeFields` (`internal/diagnostic/diagnostic_test.go`), `go test ./internal/diagnostic` 2026-09-13 |
| DRC-V0-004 | `docs/specs/FIX-REGISTRY.tsv` exists, every identifier any fixture emits resolves in it, its kind rows equal `DRC-V0-001`'s list, and a planted unregistered identifier and a planted kind-row disagreement both fail | PASS — `TestFixIdentifiersResolveInRegistry` (`internal/diagnostic/gate_test.go`), `go test ./internal/diagnostic` 2026-09-13 |
| DRC-V0-006 | a replay over the converted family asserts each converted envelope's `code` and `error` bytes survive ahead of the additive members, an unconverted envelope of the same verb is byte-identical, and the `SOL-V0-007` ledger row carries the code and none of the new fields (decision 0201 (f)); each later converted site's forced refusal asserts the same `code` and `error` prefix ahead of its members | PASS — `TestRefusalCodeSpellingsSurvive` (`cmd/corvint/diagnostic_envelope_test.go`) and `TestConvertedRefusalDiagnostics` (`cmd/corvint/diagnostic_refusals_test.go`), `go test -run` on the named tests `./cmd/corvint` 2026-09-13 |
| DRC-V0-007 | a refusal run over a clean worktree leaves the tree and trace state byte-identical, with only the permitted ledger append | PASS — `TestDiagnosticEmissionDoesNotMutate` (`cmd/corvint/diagnostic_envelope_test.go`), `go test -run` on the named tests `./cmd/corvint` 2026-09-13 |
| DRC-V0-008 | a seeded secret and a 4 KiB value in both `subject` and `evidence` are screened and truncated with disclosure | PASS — `TestDiagnosticValuesScreenedAndBounded` (`internal/diagnostic/bound_test.go`), `go test ./internal/diagnostic` 2026-09-13; the CLI emits only the bounded form once `DRC-V0-006` serialization lands |
| DRC-V0-009 | a three-round repair transcript over a seeded multi-diagnostic input stops at the stated bound and reports the remainder unresolved | PASS — `TestRepairLoopTerminates` (`internal/diagnostic/repair_test.go`), `go test ./internal/diagnostic` 2026-09-13 |
| DRC-V0-010 | a forced refusal is asserted non-zero with an `ok: false` envelope and no stdout at every layer carrying a converted family (the CLI alone for `unsupported-working-tree-impact-*`, decision 0201 (i), and for the later families except the impact path suffix, also asserted at its `batch` row and `harness event` `file-change` layers), and the pre-existing artifact is asserted untouched | PASS — `TestRefusalIsNeverSuccess` (`cmd/corvint/diagnostic_envelope_test.go`); exit 2, `ok: false` and empty stdout for each of the thirty later sites in `TestConvertedRefusalDiagnostics` (`cmd/corvint/diagnostic_refusals_test.go`); the batch row and harness envelope in `TestImpactPathSuffixDiagnosticReachesEveryLayer` (same file), `go test -run` on the named tests `./cmd/corvint` 2026-09-13 |
| DRC-V0-011, DRC-V0-012 | the gate runs over the tree, reports a covered-site count of at least seven, fails a planted uncovered site, fails a start below seven, fails a planted `message` parse, and fails each planted uninspectable refusal form | PASS — `TestDiagnosticCoverageRatchet` (`internal/diagnostic/gate_test.go`) via `script/check-diagnostic-coverage.sh` reports 37 covered sites equal to `script/diagnostic-coverage.count`; the first seven sites are frozen by `TestWorkingTreeImpactRefusalDiagnostics` (`internal/worktreeimpact/diagnostics_test.go`) and the thirty later sites, each emitting only registered fixes, by `TestConvertedRefusalDiagnostics` (`cmd/corvint/diagnostic_refusals_test.go`), 2026-09-13 |

## Traceability

Every row names the test that evidences it; `make diagnostic-coverage-check` runs the ratchet.

| Requirement | Implementation | Evidence |
|---|---|---|
| DRC-V0-001, DRC-V0-002, DRC-V0-003, DRC-V0-005 | `internal/diagnostic` `Refusal`, `Subject`, `Evidence`, `Error`, `Validate`; the later sites' constructors in `cmd/corvint/diagnostic_refusals.go` and `internal/touchsurprise/diagnostics.go` | `TestDiagnosticEnvelopeFields`; `TestConvertedRefusalDiagnostics` |
| DRC-V0-004 | `docs/specs/FIX-REGISTRY.tsv` (`kind` and `fix` vocabularies) | `TestFixIdentifiersResolveInRegistry` |
| DRC-V0-006 | `diagnosticMembers` (`cmd/corvint/diagnostic_envelope.go`) appended by `emitError`; unconverted envelopes unchanged | `TestRefusalCodeSpellingsSurvive`; `TestConvertedRefusalDiagnostics` |
| DRC-V0-007 | the CLI emission path writes stderr only; the ledger append stays `observeUnsupported` (`SOL-V0-007`) | `TestDiagnosticEmissionDoesNotMutate` |
| DRC-V0-008 | `Refusal.Bounded` (`internal/diagnostic/bound.go`): `secretscreen.Screen`, control escape, 512-byte rune-boundary cut that never splits a `\uXXXX` escape, 16-pair evidence bound | `TestDiagnosticValuesScreenedAndBounded`, `TestDiagnosticValueCutKeepsControlEscapesWhole` |
| DRC-V0-009 | `diagnostic.RepairStop` (`internal/diagnostic/repair.go`), a pure stop rule an adapter may call | `TestRepairLoopTerminates` |
| DRC-V0-010 | CLI refusal exit 2 with `ok: false` and empty stdout; the prior index snapshot is untouched; `addDiagnosticRowMembers` (`cmd/corvint/diagnostic_envelope.go`) on a `batch` operation row | `TestRefusalIsNeverSuccess`; `TestConvertedRefusalDiagnostics`; `TestImpactPathSuffixDiagnosticReachesEveryLayer` |
| DRC-V0-011, DRC-V0-012 | `script/check-diagnostic-coverage.sh` (`make diagnostic-coverage-check`, a `gate` prerequisite), `script/diagnostic-coverage.count`, `internal/worktreeimpact/diagnostics.go` (seven converted sites), `cmd/corvint/diagnostic_refusals.go` (twenty-eight), `internal/touchsurprise/diagnostics.go` (two) | `TestDiagnosticCoverageRatchet`; `TestWorkingTreeImpactRefusalDiagnostics`; `TestConvertedRefusalDiagnostics` |

## Rollout, rollback, and drift

The fields are additive and the ratchet starts wherever the first covered sites land, so adoption is
incremental, but the gate cannot be wired before one family is converted, because its start is that
family's count rather than zero. Order: `docs/specs/FIX-REGISTRY.tsv`, then the envelope type and the
`unsupported-working-tree-impact-*` family, chosen because it already names its repairs in prose,
then the gate at seven covered sites, then further families. The second step converted sixteen
sites across thirteen codes and raised the ratchet to twenty-three: the not-a-repository root
refusals, the three native-platform refusals, `unsupported-impact-option`,
`unsupported-batch-snapshot`, the `affected` Git-executable, base and HEAD refusals, the `prove`
Git-executable, checkpoint HEAD, range byte-bound and `object-format-mismatch` refusals, and the
`touch-set surprise` revision and dirty-worktree refusals. The third step converted fourteen
sites across seven new codes and raised the ratchet to thirty-seven: `unsupported-impact-platform`,
`unsupported-impact-path-suffix` at its `impact`, `batch` row and `harness event` layers, the
`prove` range diff byte-bound refusal, the `attest-key-unavailable`,
`attest-public-key-unavailable` and `attest-envelope-unavailable` refusals, the two `map-unavailable`
map path and map read refusals that `prove --cem` and `prove --verify-cem-attestation` share, and the
six `invalid-proof-document` refusals of `prove-observe`. Rollback at any point is removing the
gate target and leaving the additive fields in place, which returns callers to the present
prose-reading baseline without breaking a `code` consumer.

Drift rule: the registry and the emitting sites must be read together whenever either changes, since
a fix identifier retired from the registry while a site still emits it is exactly the failure
`DRC-V0-004` exists to catch. A converted site that later loses its `supported_fixes` must fail the
ratchet rather than silently leave the covered set.

## Unresolved

No decision is open. Whether `DRC-V0-012`'s ban on parsing `message` is mechanically checkable was
settled by decision 0201 (k): only in the bounded Go form that clause now states.

Three decisions are settled by owner instruction of 2026-09-12. The registry sits beside the specs,
at `docs/specs/FIX-REGISTRY.tsv`, which gives it the specs' own owner-owned authority and makes adding
an identifier an ordinary spec change rather than a signed decision. The file is deliberately not
Markdown: `docs/specs/*.md` is swept as the spec set by `internal/specindex` and
`script/gen-spec-requirements.sh`, so a registry written as a sibling `.md` would be read as a spec
missing its headers, and a tab-separated table matches `REQUIREMENTS.tsv` next to it.

The ratchet's starting count is likewise settled: the gate starts at the
converted family's own count, not at zero, which fixes it at seven for
`unsupported-working-tree-impact-*` and makes the first conversion a precondition for wiring the
gate rather than a later step. The count is measured against the unconverted tree, so a conversion
that merges or splits those seven sites must restate the start from the converted shape and may not
use the change as cover for a lower number.

`subject` is a typed `kind`/`value` pair rather than one string. The bare string was the cheaper
option and was set aside on evidence. One file refuses four different sorts of thing: an argument
whose count is wrong at `internal/worktreeimpact/compiler.go:90`, one supplied path at
`internal/worktreeimpact/compiler.go:116`, a missing captured revision index at
`internal/worktreeimpact/compiler.go:84`, and a module that is not slash-qualified at
`internal/worktreeimpact/compiler.go:96`. Two more sorts appear in the doc compiler: a document read
as authority at `internal/doccompiler/build.go:158` and a host facility at
`internal/doccompiler/build.go:32`. One string can hold any of those only by encoding its own sort
inside the text, which is the parsing `DRC-V0-012` forbids. The revision-alongside-a-path worry
that kept this open is answered without a third field: the revision a path was checked against is a
measured fact, so it is `evidence` under `DRC-V0-002` and the pair stays two fields. The owner
instructed this clause on the evidence above, so it carries the same authority as the other two
settlements.

Promotion: this contract is `validated` only when a converted family's refusals are repaired by an
agent caller from `supported_fixes` alone, across ten recorded refusals, with no prose read. Kill
criterion: if agent callers already repair refusals from prose at that rate, or if fewer than three
diagnostic families can name a closed repair set, the baseline wins and this spec is rejected.
