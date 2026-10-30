# Documentation CI V0

Owner: Russell Lewis
Date: 2026-09-29
Requirement prefix: `DCI-V0`
Intent status: accepted direction (owner request, 2026-09-29); bounded admission requires reviewed qualification
Delivery status: experimental
Authoritative inputs: the owner's request to fix full CI for the README change; `AGENTS.md`
invariants 1, 2 and 8; `docs/specs/affected-plan-v0.md` AFP-V0-013/014/016; V1-0470.

## Agent digest
- Claim: A separately qualified README presentation policy can omit the root Go race invocation while retaining documentation and other CI checks.
- Status: accepted direction (owner request, 2026-09-29); bounded admission requires reviewed qualification/experimental
- Exists: `tools/docs-ci-plan` and the trusted-base `docs-plan` CI job, with adversarial Git-fixture tests.
- Blocked on: independent consumer review, approved source-bound qualification, exact-head control-plane acceptance and hosted DOCS-path observation.
- Read next: Requirements; Consumer inventory; Qualification and rollout.

## Intent and boundary

A prose/branding change in README PR #348 ran the full root race suite because generic affected
selection was unqualified and unresolved readers selected 125 packages. Documentation checks did
find a real shifted line citation. The user requested a safe documentation-specific path and then
explicitly authorized implementing it. This is that bounded policy, not a claim that arbitrary
Markdown changes cannot affect tests. V1-0246/V1-0081 still own broad reader-bound improvements.

The measurable outcome is an admitted README presentation change with a valid numeric citation
relocation: document checks run, the root race invocation does not, and all other checks retain their
existing behavior. Wall-time improvement is measured only on actual hosted runs; it is not inferred
from selected package counts. Missing hosted observations remain NOT_OBSERVED.

## Requirements

- `DCI-V0-001`: The classifier MUST read a clean checkout at the full captured target SHA, require
  exactly two ordered parents equal to the event base and head, classify the actual base-to-target
  delta, disable replacement reads, refuse replacement refs and grafts, and repeat HEAD/status
  validation after classification and verification. Invalid topology, missing objects or drift MUST
  choose FULL. Read subprocesses have a 10-second deadline and an 8-MiB stdout bound; documents are
  limited to 1 MiB. Git reads MUST NOT fetch. The workflow supplies a fresh checkout, trusted Git
  executable PATH and no persisted credentials, secrets or PR-controlled Git configuration.
- `DCI-V0-002`: The closed path scope MUST be modified existing mode-100644 `README.md` and,
  optionally, `docs/specs/FRONTIER-DECISION-BRIEF-2026-08-29.md`. README MUST change. Every other
  path, add/delete/rename, executable bit, symlink, submodule or unsupported tree entry MUST choose
  FULL. This includes all policy, qualification, workflow, source and semantic spec edits.
- `DCI-V0-003`: README fenced and indented code bytes MUST remain identical. The admitted Markdown
  subset MUST reject unclosed or unsupported nested fences and reference-link definitions/forms.
  HTML MUST use closed presentation tags/attributes, quote-aware tokenization, quoted values and
  no event/style/duplicate attributes. Unhandled syntax chooses FULL. The optional citation file
  may change only decimal line numbers in existing `README.md:N-M@hash` references: corresponding
  unmatched bytes, hashes and referenced span bytes MUST be identical. Position must be preserved.
- `DCI-V0-004`: Admitted documents MUST verify immutable local link targets and supported Markdown
  heading anchors, and every README citation in the known consumer, including unchanged consumers.
  A broken link/anchor/citation MUST fail the documentation job, not become a successful skip.
  Code-example headings MUST NOT create anchors. Remote URLs are syntactically checked, never
  fetched; remote availability, semantic accuracy and exhaustive Markdown rendering are not claimed.
- `DCI-V0-005`: Admission MUST require an approved `corvint-docs-ci-qualification/0` artifact read
  from the trusted event base. It MUST name this consumer inventory, nonempty independent review
  evidence, the exact fixed check list, classifier-source digest and a SHA256 of the complete sorted
  tracked tree rows (path, mode, type, blob identity), excluding ONLY the two admitted document paths
  and `.github/qualifications/docs-only.json` itself. Any other drift or missing/malformed artifact
  chooses FULL. The proposal command MUST emit `approved:false` and cannot grant its own approval.
  A PR cannot admit itself by supplying a new artifact. The artifact contains data only.
- `DCI-V0-006`: CI MUST build the classifier from a separate checkout at the captured event base,
  never from PR source. A missing classifier/build or unsupported plan MUST leave mode FULL. Only
  an exact base/head/target-bound `corvint-docs-ci/0` receipt with mode DOCS and `verified:true`, plus
  successful focused classifier/spec-index tests, may publish DOCS. Missing or failed job outputs
  must not skip tests. Main/push/release retains full execution. The decision JSON MUST be retained.
- `DCI-V0-007`: Only the root Go race invocation may be omitted. Documentation gates, shard-0
  formatting/vet/build/interop, separate interop, archive integrity and security checks MUST remain.
  The required `go-product` aggregation MUST require both docs-plan and every shard to succeed;
  documentation verification failures and cancellation cannot pass it. Existing `doc-gates` and
  `ci-control-plane` remain required. The AFP-V0-016 exact-head admin acceptance for `.github/`
  changes is unchanged.
- `DCI-V0-008`: Qualification MUST retain source-bound consumer review and actual adversarial
  fixture evidence: accepted README/citation moves; failed links and stale citations; rejected code,
  semantic-spec, workflow, artifact, topology, dirty-tree, replacement/graft and parser attacks.
  Local fixture success MUST NOT claim hosted admission. The first real hosted DOCS decision and
  omitted root race step MUST be inspected before calling the integrated route delivered.

## Consumer inventory

This is a reviewed repository verification policy with residual dynamic-reader uncertainty, not a
proof of equivalent Go coverage or an automatic waiver of generic `affected` UNKNOWN.

| Consumer | Evidence and retained check |
|---|---|
| README links, presentation markup and executable examples | `tools/docs-ci-plan` checks immutable blobs and preserves code blocks. Its own fixture tests remain selected. |
| Frontier decision brief's README line citation | Exact existing file is the only citation-relocation exception. The classifier verifies spans; the full line-citation gate still scans all tracked docs. |
| Specification index/header/digest consistency | `internal/specindex` runs under DOCS. Its `docs/specs/README.md` input is outside the admitted scope. |
| Release/archive readers | Full archive-integrity, static/build and interop jobs remain. `internal/companionrelease` and `internal/releasecandidate` README literals largely describe generated bundle/candidate READMEs, not the root file. |
| Go test-fixture README literals | `cmd/corvint/{main,init_adopt,diagnostic_refusals,kernel}_test.go`, `internal/genesis`, `internal/witness`, and conformance fixtures create their own README bytes. A matching filename alone does not make the root README their input. |
| Generic index/reader heuristics | Runtime-built repository reads remain uncertain. No AFP profile, unbounded-reader edge or full-suite qualification artifact is rewritten by DCI. Source-bound independent review must accept this limited verification policy before its artifact is approved. |

The whole-tree binding deliberately invalidates approval after changes outside the two documents,
including new test consumers or policy changes. It may require frequent requalification. A literal
search alone cannot establish completeness; review must inspect the original consumers and retain
its limitations. Do not silently expand the allowlist or ignore new inputs to improve hit rate.

## Qualification and rollout

1. Review this bounded policy and the consumer inventory independently. Run the tests below on
   the exact source, retain their results and every finding/repair.
2. Commit and seal source/evidence. Run the classifier's `--qualification-proposal --base FULL_SHA`
   outside source to obtain an unapproved tree-bound artifact. Review the immutable source and
   evidence, then set `approved:true` and identify that independent review in `review`. Add only the
   artifact in its own commit. Any additional source change invalidates that proposal.
3. The implementation/artifact PR runs full CI: its proposed artifact is not trusted by itself.
   Retain hosted results and obtain the existing exact-head control-plane acceptance before merge.
4. On that trusted base, exercise a real README presentation PR. Inspect the DOCS receipt, all
   retained checks and actual root-race omission. Until observed, integration remains unqualified.

This is an explicit bounded exception to the root-race obligation in AFP-V0-013/014, not activation
of the generic PR driver. Its 200-row historical qualification remains required for arbitrary Go
selection. The general driver's trusted pins remain empty. Rollback deletes or sets `approved:false`
in the qualification artifact; missing/stale approval already selects FULL. No branch rule is removed.

## Acceptance evidence

| Requirements | Executable evidence |
|---|---|
| DCI-V0-001 | `TestMergeBindings` |
| DCI-V0-002, DCI-V0-003 | `TestFullFallback`, `TestCodeFenceAndCitationBounds`, `TestAdversarialPresentation`, `TestCitationSentinelCollision` |
| DCI-V0-004 | `TestREADMEAndCitationRegression`, `TestBrokenDocumentation`, `TestAdversarialPresentation` |
| DCI-V0-005 | `TestQualificationInvalidation` and independently reviewed artifact/source binding |
| DCI-V0-006, DCI-V0-007 | workflow lint, focused native CLI fixtures and hosted DOCS/FULL/failure job observations |
| DCI-V0-008 | retained local test/review evidence and first hosted README qualification; absent observations stay NOT_OBSERVED |

Non-goals: general Markdown parsing, accepting changed executable examples, skipping checks merely
because a path ends in `.md`, resolving all unbounded readers, changing release validation, or
asserting test adequacy from a green run. Unknown admission inputs choose FULL; observed document
failures fail CI. A source/policy change cannot reuse an earlier consumer review through this profile.
