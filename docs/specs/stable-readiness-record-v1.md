# Stable Readiness Record V1

Owner: Russell Lewis
Date: 2026-09-26
Intent status: proposed
Delivery status: experimental
Authoritative inputs: decision 0420 (owner, 2026-09-26), `AGENTS.md`,
`docs/SPEC-DRIVEN-DEVELOPMENT.md`, tickets V1-0018 AC2, V1-0019, V1-0020 AC3 and V1-0021 AC3,
PRS-V1-004 and PRS-V1-005 in `corvint-1.0-product-and-release-v1.md`, PUB-V0-021 and PUB-V0-023
in `public-release-v0.md`, ARTIFACT-RDY-V0-001 and ARTIFACT-GO-V0-008 in
`release-artifact-integrity-v0.md`, GOC-V0-005 in `go-only-cutover-v0.md`,
`host-lifecycle-qualification-v1.md` and `stable-operations-v0.md`.

## Agent digest
- Claim: One canonical JSON record binds a verified Core candidate to digested gate, platform, compliance and policy evidence before any tag.
- Status: proposed/experimental; SRR-V1-001 to SRR-V1-011 are coded as an internal package. SRR-V1-012 is a proposal only.
- Exists: `BuildReadinessRecord` and `VerifyReadinessRecord` in `internal/releasecandidate`, with tests named after each requirement.
- Blocked on: owner acceptance of this spec and of decision 0420; no command exposes the package yet.
- Read next: Requirements; Failure modes; Traceability.

## User and boundary

The release checklist (ARTIFACT-RDY-V0-001) judges alpha rows only. For 1.0, V1-0018 AC2 needs a
record that is bound to one candidate, carries every public-release identity and digest, and is
captured before any tag or publication. V1-0020 AC3 and V1-0021 AC3 add two constraints: the record
must complete before the owner approves publication, and it must never take publication or
promotion as a prerequisite. No spec defined that record until now.

The record is evidence for the owner's decision; it does not make the decision. It runs no gate. The
operator runs each gate, keeps the log, and passes the log's path. The builder records only that
file's SHA-256, and missing evidence stays NOT_RUN or FALLBACK, never PASS (product invariant 2).
Decision 0420 fixes four dispositions, which this spec encodes:

- (a) 1.0.0-rc.1 ships with no signing: SHA256SUMS only, publisher identity NOT_VERIFIED. 1.0.0
  stable needs its own selection.
- (b) native performance stays NOT_RUN under GOC-V0-005.
- (c) The Core vulnerability check is a require-free Core `go.mod` plus the pinned toolchain.
- (d) linux/amd64 stays FALLBACK until native evidence exists from a hosted ubuntu-24.04 runner.

Non-goals:

- running any gate, scanner or network query;
- signing, tagging, publishing, promoting or uploading;
- computing an overall release verdict;
- verifying the task-store candidate digest against `.taskman/`;
- a new spec language or database.

## Requirements

- `SRR-V1-001`: The record MUST be one JSON document with profile `corvint-stable-readiness-record/1`
  and closed fields, encoded in the package canonical form (two-space indent, trailing newline). The
  verifier MUST refuse unknown fields, a trailing value, a different profile, and any bytes that do
  not re-encode identically.
- `SRR-V1-002`: The identity MUST come from a candidate admitted by `VerifyContext`: version, tag
  `v<version>`, build number, Go toolchain, Corvint commit and tree. The builder MUST re-derive the
  commit, tree, VERSION and PUB-V0-021 build number from the operator's source root, and MUST refuse
  on any disagreement. The verifier MUST re-verify the candidate and refuse a record whose identity
  differs.
- `SRR-V1-003`: The core section MUST record the candidate profile and the SHA-256 of the
  candidate's `SHA256SUMS`. It MUST record the four `core-archive` asset rows exactly as the
  manifest lists them, with reproducibility `PASS` only because the candidate verifier admits
  nothing else. The verifier MUST refuse any core section that differs from the re-verified
  candidate.
- `SRR-V1-004`: The store-release binding MUST be null, or it MUST carry both a release id and a
  64-hex `candidateSha256`. The builder MUST refuse a partial or malformed binding. The digest is
  recorded as supplied and is not checked against the task store.
- `SRR-V1-005`: Rows MUST follow one fixed ordered catalogue: the `gate/` rows full-gate,
  interop-gate, focused-docs, companion-release and release-checklist-pre-promotion; the
  `platform/` rows darwin-arm64 and linux-amd64, each with lifecycle and host-lifecycle;
  `compliance/legal-files`; `external/untouched-repository` (the V1-0019 case seal);
  `policy/rollback-exercise`, `policy/signing` and `policy/native-performance`; and
  `owner/toolchain-security-review`, `owner/tag`, `owner/publication` and `owner/promotion`. An
  unsupplied row MUST be recorded as NOT_RUN "evidence not supplied", or as FALLBACK for a platform
  row. Evidence for an unknown or builder-owned row MUST be refused.
- `SRR-V1-006`: PASS and FAIL MUST carry the SHA-256 of an operator-named regular file. NOT_RUN MUST
  carry no digest, and MUST carry an accepting four-digit decision or a reason. FALLBACK is admitted
  only on platform rows, with a decision or a reason. The verifier MUST recompute every recorded
  digest from the operator-named files, and MUST refuse a missing, extra or mismatched file.
- `SRR-V1-007`: A platform row without native lifecycle evidence MUST be FALLBACK (PRS-V1-004). The
  linux/amd64 rows MUST cite decision 0420 until the operator supplies the hosted ubuntu-24.04
  lifecycle or host-lifecycle evidence as a file.
- `SRR-V1-008`: The vulnerability section MUST record three things: decision 0420, the number of
  `require` directives in the Core `go.mod` at the bound commit (read through git), and the
  toolchain reported by `GOTOOLCHAIN=local go env GOVERSION`. Its status MUST be PASS only when
  there are no directives and both that toolchain and the candidate toolchain equal `go1.27.1`, and
  FAIL otherwise. The verifier MUST refuse a status that its recorded inputs contradict. The fixed
  row `owner/toolchain-security-review` MUST stay NOT_RUN: the owner compares the toolchain against
  Go security releases before tagging.
- `SRR-V1-009`: For version `1.0.0-rc.1`, `policy/signing` MUST be the fixed row NOT_RUN, decision
  0420, "No signing: SHA256SUMS only; publisher identity NOT_VERIFIED". Operator evidence for it
  MUST be refused. For any other version the operator MUST supply the signing selection, and it
  defaults to NOT_RUN. `policy/native-performance` MUST be fixed NOT_RUN under GOC-V0-005 and
  decision 0420.
- `SRR-V1-010`: `owner/tag`, `owner/publication` and `owner/promotion` MUST be fixed NOT_RUN
  "subsequent owner action". The record MUST NOT take a tag, publication or promotion as input, and
  the verifier MUST refuse a record that claims any of them.
- `SRR-V1-011`: Building and verifying MUST NOT write to the candidate, the source root or the
  evidence files, and MUST NOT use the network. The builder returns the canonical bytes, and the
  operator-named output path is the only place a caller may write them. The candidate verifier's
  transient host-probe directory is created and removed inside the system temporary directory.
- `SRR-V1-012`: (proposed, not implemented) An operator command SHOULD expose the builder and the
  verifier. It SHOULD take explicit evidence arguments and write the record without replacement to
  one operator-named output path. `cmd/corvint-release-candidate` has a single flag set and no
  subcommands, so this needs owner acceptance of the command shape first.

## Failure modes

| Failure | Behavior |
|---|---|
| Candidate fails `VerifyContext` | Builder and verifier refuse; no record (SRR-V1-002). |
| Source root lacks the commit, or its VERSION or build number differs | Builder refuses (SRR-V1-002). |
| Gate log not supplied | Row is NOT_RUN "evidence not supplied"; never PASS (SRR-V1-005). |
| Evidence file changed after the record was built | Verifier refuses on the digest mismatch (SRR-V1-006). |
| Core `go.mod` gains a `require`, or the local toolchain drifts | Vulnerability status FAIL (SRR-V1-008). |
| Record edited to claim a tag, signing or PASS without evidence | Verifier refuses (SRR-V1-006, 009, 010). |
| Store candidate digest is stale | Not detected; the owner cross-checks it (SRR-V1-004, open). |

## Acceptance and rollback

SRR-V1-001 to SRR-V1-011 are covered by the focused tests below, which run against a git fixture
and a Core-only 1.0.0-rc.1 candidate fixture. Owner acceptance of this spec and of decision 0420 is
still required before any release uses the record. SRR-V1-012 needs an accepted command shape and
its own tests.

Rollback deletes `internal/releasecandidate/readiness.go` and its test, and inlines
`runSourceGit` back into `sourceBuildNumber`. No record, candidate, store or wire state depends on
the package yet.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| SRR-V1-001 | `internal/releasecandidate/readiness.go` (`VerifyReadinessRecord`) | TestSRRV1001CanonicalClosedRecord |
| SRR-V1-002 | `readiness.go` (`BuildReadinessRecord`, `readinessBinding`), `candidate.go` (`sourceBuildNumber`, `runSourceGit`) | TestSRRV1002IdentityBindsSourceRoot |
| SRR-V1-003 | `readiness.go` (`readinessBinding`) | TestSRRV1003CoreBindsVerifiedCandidate |
| SRR-V1-004 | `readiness.go` (`readinessStore`) | TestSRRV1004StoreReleaseBothOrNeither |
| SRR-V1-005 | `readiness.go` (`readinessCatalogue`, `readinessRows`) | TestSRRV1005MissingEvidenceIsNotRun |
| SRR-V1-006 | `readiness.go` (`validateReadinessRow`, `verifyReadinessEvidence`) | TestSRRV1006EvidenceDigestsReverified |
| SRR-V1-007 | `readiness.go` (`platformRule`) | TestSRRV1007PlatformRowsFallBackWithoutNativeEvidence |
| SRR-V1-008 | `readiness.go` (`readinessVulnerability`, `requireDirectives`, `vulnerabilityStatus`) | TestSRRV1008VulnerabilityRuleIsRequireFreeAndPinnedToolchain |
| SRR-V1-009 | `readiness.go` (`readinessRules`, `fixedRule`) | TestSRRV1009PolicyRowsFollowDecision0420 |
| SRR-V1-010 | `readiness.go` (`fixedRule`, `validateReadinessRow`) | TestSRRV1010OwnerActionsStayNotRun |
| SRR-V1-011 | `readiness.go` (no writer) | TestSRRV1011BuildAndVerifyWriteNothing |
| SRR-V1-012 | proposed; no implementation | none until accepted |
