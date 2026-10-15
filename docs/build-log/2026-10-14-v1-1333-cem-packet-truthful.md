# 2026-10-14 Truthful CEM 1.0 packet and native citation vectors (V1-1333)

Branch `claude/v1-1333-cem-packet-truthful`, from `origin/main`
`adf587fdb65f4d87dc94a5e374a4fda1f87c5e7f`. Requirements `CEM-V1-107..112` (new, proposed) in
`docs/specs/cem-stable-v1.md`, within the 1.0 packet scope of `PRS-V1-018`.

## Intent

A panel finding (2026-10-13) said the Apache-2.0 `protocol/cem-1.0` packet was not enough for an
independent consumer. Its status labels still called stable `cem/1.0` an unimplemented proposal.
The citation rules, the stop list, the result serialization and the canonical diff recipe were
given only in the AGPL spec, in Go code or by pointing at the deleted 0.3 packet. No vector pinned
the citation bytes the native verifier produces for anchored hunks, nonzero specificity and a
same-path deletion. Every item was checked again at the base before acting. V1-1340 had since
added the tamper matrix, whose clean map is anchored with specificity 2, so this lane does not
repeat any of its classes.

## Decisions

- **Status wording (`CEM-V1-107`).** The packet now says what is true: the stable `cem/1.0` wire is
  implemented and produced on main, its technical contract is proposed and not promoted, and its
  qualification is bounded. This applies to `protocol/cem-1.0/README.md`, `stable/README.md`,
  `stable/ALGORITHMS.md`, `tamper-matrix/README.md` (which also said eighteen tampered maps; it has
  nineteen), and the `status` members of `stable/OPERATION.json` and `stable/manifest.json`, both now
  `IMPLEMENTED_CONTRACT_PROPOSED_NOT_PROMOTED`. No code reads either member. The spec header status is
  unchanged. The inherited `OPERATION.json` copies under `repository-envelope-packet/inherited/` keep
  their bytes, because that packet pins them as historical inputs.
- **Residual: "accepted".** The acceptance text says the packet status "reflects the accepted
  stable profile". The technical contract is not accepted or promoted, so the packet does not say it
  is. The literal wording waits on promotion (V1-0021), not on this ticket.
- **R1-EXPECTED.** No test reads `R1-EXPECTED.json`, and `repository-envelope-packet/INPUTS.json`
  pins its bytes. It is now described as a declarative expectation set, kept as construction
  history, that no test executes and that makes no conformance claim. Executing it would need a
  harness for its environment selectors, which is out of scope. Its own status member already says
  `DECLARATIVE_NOT_EXECUTED`.
- **Normative text (`CEM-V1-108..110`).** Four new sections at the end of `stable/ALGORITHMS.md`.
  They were written from the code at the base (`internal/cem/verify/support.go`, `stable_verdict.go`,
  `internal/cem/wire`, `internal/cem/gitauth/diff.go`, `object.go`, `patchproof.go` and the portable
  `interop/cem01-go` equivalents), as new Apache-2.0 prose. Only requirement IDs and field names were
  copied. The defining-line rule is stated as span-relative: the first piece of the span counts as a
  line even when the span starts mid-line. The added-bytes join keeps each line's LF, including
  before a "No newline" marker.
- **Finding: nested member order.** The two consumers write the nested objects of `drift`, `hunks`
  and `references` in different member orders. The native verifier sorts them; the portable one
  uses struct order. All eight vector outputs differ byte for byte on stdout. They are equal as
  parsed JSON, and their top-level order, citation bytes and `verdictSha256` agree. Neither consumer
  was changed (scope). `CEM-V1-109` states that the nested order is not fixed and that results
  compare by digest, citation bytes or parsed value. Fixing one order is follow-up work.
- **Diff recipe limit (`CEM-V1-110`).** The packet now carries the full Git invocation, settings,
  environment, attribute source, exclusion and provenance proof. The patch bytes themselves are
  still defined by Git's Myers output for those options, which the packet names but does not restate.
- **Vectors (`CEM-V1-111`).** `stable/citation-vectors/` has a two-commit recipe, a clean map, and
  seven refused copies. The refused copies are stale drift, same-path deletion drift, base mismatch,
  and the wire classes unsupported spec, missing field, duplicate member and non-object. The clean
  map came from `corvint cem prepare` and `cem cite`. The drift maps were edited by hand, because
  `cem cite` refuses stale spans; their changed evidence IDs were recomputed. `EXPECTED.json` was
  generated from the native binary's output, and both consumers assert it. The clean case covers
  every tier but `held`, both anchor kinds, two annotation hunks, a span that starts mid-line, a
  `&` in a path (which exercises escaping), and a cited deletion anchored through its base path. Its
  counts are anchored 6, specificity 5 and samePathCitations 2. The 10 files are in the stable
  manifest, and the directory is in `.corvint/test-read-scopes.json`.
- **Requirements.** Owner standing auto-accept of lane requirements (2026-10-11) applies. The
  requirements stay proposed until recorded under that acceptance, following the charter and the
  V1-1340 precedent. The lane addendum's inline marker "(accepted under the owner's standing
  auto-accept (2026-10-11); V1-NNNN)" was therefore not applied here. Recording it is a single-line
  follow-up once the acceptance is recorded. IDs come from the reassigned range `CEM-V1-107..119` and
  `171..179`.

## Mutation check (`CEM-V1-112`)

The check is a scratch harness, not committed. It copied the clone, applied one exact source
replacement per mutant to the native verifier, and ran
`GOTOOLCHAIN=local go test -count=1 -run TestCEMV1111CitationVectorsNative ./internal/cem/verify/`.
For comparison it also ran `TestCEMV1226TamperMatrixNative|TestCEMV1083NeutralCriterionPublicCase`
on the same mutant. All 13 mutants built and were killed by the new vectors. Ten of them survived the
existing tests.

| Mutant | Killed by | Existing tests |
| --- | --- | --- |
| M1 LF-stripped added-bytes join | clean | survived |
| M2a citation member order swapped | clean | survived |
| M2b digest with `", "` / `": "` separators | all 8 | killed |
| M2c digest with HTML escapes | clean, evidence-stale, same-path-deletion-cited, base-revision-mismatch | survived |
| M2d top-level member order swapped | all 8 | survived |
| M3 absolute defining line | clean | survived |
| M4a stop list omits `with` | clean | survived |
| M4b case-sensitive stop terms | clean | survived |
| M5a shift null uses hunk i+1 | clean | survived |
| M5b specificity = anchored | clean | killed |
| M6a deletion hunk has no path | clean | survived |
| M6b samePath counts distinct evidence | clean | survived |
| M6c deleted cited file not drift | same-path-deletion-cited | survived |

## Independent review (one Codex round)

`codex exec -s read-only` reviewed the staged change and returned BLOCK with eight findings. Each
was checked against the code.

- **Fixed.**
  - (1) The `info/attributes` refusal is now named by its public code,
    `unsupported-repository-envelope`, not the internal one.
  - (2) `OPERATION.json` `resultKeys` lacked `citation` and `verdictSha256`; both are now in the
    order the verifiers write.
  - (3) The result-line escapes were mangled into literal characters while the file was written;
    they now read as `\u003c`, `\u003e`, `\u0026`, `\u2028` and `\u2029`.
  - (4) The diff recipe now has `--git-dir=GITDIR` and the `SystemRoot` and `USERPROFILE`
    passthrough.
  - (5, in part) The `stable.schema.json` description no longer says the implementations are not
    delivered.
  - (6) The structural-runtime text in `ALGORITHMS.md` and `OPERATION.json` now names both
    admitted tuples, go1.27.1 and go1.27.2 (decision 0522).
  - (7) The stop list has 60 terms, not 63, and the defining-line prefix is three bytes, not four.
  - (8) Both tests now check that the clean case has anchored hunks, nonzero specificity,
    same-path citations, and a path-anchored whole-file deletion hunk.
- **Recorded, not changed.** For (5), `docs/specs/cem-stable-v1.md` line 47 (a historical
  candidate traceability row) and the spec header and digest status lines still read "planned".
  The charter keeps the spec header status out of this lane. Codex found no remaining claim that the
  stable technical contract is accepted or promoted.

## Verification

The commands and results are in the PR description. They cover:

- `corvint affected` over the base;
- the focused package tests;
- the interop module tests and vet;
- `-race` on the touched packages;
- `go run ./tools/cem-stable-regen --check`;
- `gofmt -l`;
- the twelve doc gates.

## Rollback

Revert the V1-1333 commit. The vectors and their manifest entries are removed, and the packet
prose returns to its previous text. No committed map, seal or golden depends on them.
