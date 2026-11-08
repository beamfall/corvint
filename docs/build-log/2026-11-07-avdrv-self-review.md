# avdrv self-review repair

Owner repair charter: fix the four reported P1 driver soundness gaps, preserve frozen
contracts and existing assertions, and stop after dogfood-check without sealing.

The repairs close these classes under existing requirements:

- AFP-V0-9051 / AFP-V0-3128: a containing-directory edge is not proof of a glob's
  targets. Certification admits only `literal-dir/*literal-suffix`, a plain relative
  directory, a successful listing, and regular-file metadata for every matching entry.
  Links, directory matches, unknown kinds, failed observations and other glob forms
  retain the frontier. `TestDriverGlobProvenance_AFPV09051` covers linked files and
  directories, an unmodeled glob form, a directory match and a regular-file control.
- AFP-V0-3121: nested makefile location does not establish invocation directory.
  Literal wildcard evaluation stays limited to the existing source-root make context;
  nested files retain both condition outcomes and filesystem uncertainty. This avoids
  inventing invocation provenance or introducing a new wire field.
  `TestDriverNestedWildcardDirectory_AFPV03121` covers `make -f build/Makefile`
  semantics with only `build/marker` present and an unknown filesystem function.
- AFP-V0-3128 / AFP-V0-9051: basename equality does not prove an executable is a
  utility. Utility allowlists apply to bare command names; a path requires the
  existing recorded shell-test-script proof. Absolute paths cannot satisfy that proof.
  `TestDriverExecutableProvenance_AFPV03128` covers pure, bounded and Go basenames,
  unknown utility names, and relative repository executables.
- AFP-V0-9401 / V1-5033: every target inherits shared file-set reads, just as it inherits
  shared paths. This is independent of recipe expansion and the particular special rule.
  `TestDriverSharedEnumeration_AFPV09401` covers `.DEFAULT`, `.SUFFIXES` and `.PHONY`, including new, unindexed and hidden files without an evaluated expansion.

No JSON/wire shape, field name, error/exit code, stderr text, golden or existing assertion
changed. No new capability or native ticket-state claim. Other original ticket dispositions
remain as recorded in the lane handback; V1-5087's frozen refusal remains out of scope.
Rollback reverts this repair's code, new regressions and specification clarifications together,
restoring the prior conservative-proof gaps; no stored-data migration is needed.

Verification and routing: the required `corvint affected --base
 a7d38e73236cea3a237ce972f3bd50e11e723c29 --format json` completed and selected
`go:github.com/Beamfall/corvint/internal/liveverify/affected` by direct source change.
It retained 71 LANGUAGE_FRONTIER unknowns and a stale installed-binary warning; this is
selection evidence from that binary, not current-tree qualification. Raw receipts and test
logs are outside the repository in `/home/russell/lanes/avdrv-evidence/`.
Pre-change collection is reused by the owner's repair instruction, never rerun.
The whole affected package runs through the configured heavy semaphore, without a test-name
filter or exported commit identity. Final results and artifact digests are retained below.
Independent review, host qualification and broader gates belong to the orchestrator.
Publication, native Tasks writes and sealing are prohibited by this round's charter.

The first whole-package run (116.275 s, exit 1) found a mistaken new test fixture:
`%.generated` was an ordinary pattern rule, not shared remainder, and `check: missing`
could not invoke it. Replaced that new case with `.PHONY`, exercising the separate
shared-recipe parser branch with the same selection assertion. The full logs also exposed the original enumeration fixture using a disk-only fake with
`BuildFS`, and the new nested selection fixture encountering the existing `build/`
discovery exclusion. No assertion was changed. A fixture-only SourceLanguage adapter
lets the existing enumeration assertions execute; the nested regression constructs real
driver units through `makeUnits` before selection, preserving the exact reported path
without changing unrelated directory-discovery policy. The second whole run also failed
on those fixture issues (121.598 s); its log remains `self-review-tests-final.log`. The failed log is retained as `self-review-tests.log`.

`make -s spec-requirements-check line-citations-check` passed. Its existing heavy-wrapped
checks ran `internal/specindex` (0.984 s) and selected `internal/lrfrepo` checks (0.066 s).
Staged `script/check-line-citations.sh --repin` found no moved citation; the initial unstaged
attempt explicitly requested staging the two specs and was repeated after staging.

Retained SHA-256 receipts:

- `self-review-affected.json`: `7657deded6453a8a05dd94a53a8970065a590fbd944279a794bc15f156aff777`.
- `self-review-tests.log` (failed new fixture): `a21ae31f4695b689dca72caf8c73694576e7f55c59560caf633692b0fbf9e1de`.
- `self-review-doc-checks.log`: `53ff03cc523a87c61f9ea48f49f6097513e21d3adf3ef8a411d4fb28c613f49d`.

Final whole affected-package run passed: 114.100 s, exit 0, all existing tests and new
regressions, no `-run` filter. Focused repaired fixtures passed in 0.030 s.

- `self-review-tests-final.log`: `fba905ca843dbf3df2b2120fb9b11b8534da2ea33fa975073b72aa0d410771a0`.

- `self-review-focused.log`: `1e0ab28f0e226c6615e888ce277ff5d512c363d509c8fb1e3da741f07e48ed24`.

- `self-review-tests-passed.log`: `4482e499cd4615e5022c8d428520f703bd3dd3122082675a54f95df0a94d5a78`.

The final staged citation repin again found no moved citation; the final standalone
line-citations check passed. No independent or cross-platform qualification is claimed.
