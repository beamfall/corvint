# Stable Readiness Record V1

Owner: Russell Lewis
Date: 2026-09-26
Intent status: accepted (decision 0422, 2026-09-26)
Delivery status: experimental
Authoritative inputs: decision 0420 (owner, 2026-09-26), `AGENTS.md`,
`docs/SPEC-DRIVEN-DEVELOPMENT.md`, tickets V1-0018 AC2, V1-0019, V1-0020 AC3 and V1-0021 AC3,
PRS-V1-004 and PRS-V1-005 in `corvint-1.0-product-and-release-v1.md`, PUB-V0-021 and PUB-V0-023
in `public-release-v0.md`, ARTIFACT-RDY-V0-001 and ARTIFACT-GO-V0-008 in
`release-artifact-integrity-v0.md`, GOC-V0-005 in `go-only-cutover-v0.md`,
`host-lifecycle-qualification-v1.md` and `stable-operations-v0.md`.

## Agent digest
- Claim: One canonical JSON record binds a verified Core candidate to digested gate, platform, compliance and policy evidence before any tag.
- Status: accepted (decision 0422, 2026-09-26); experimental delivery; SRR-V1-001 to SRR-V1-012 are coded: the internal package and the `corvint-readiness-record` command.
- Exists: `BuildReadinessRecord` and `VerifyReadinessRecord` in `internal/releasecandidate`, `cmd/corvint-readiness-record` over them, and tests named after each requirement.
- Blocked on: first use on the `1.0.0-rc.1` candidate (V1-0018 AC2, V1-0020 AC3); no release has used the record yet.
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
- guarding the output against a concurrent writer that controls an ancestor of the candidate or
  the source root, or a directory inside one: such a writer can already replace the root or write
  inside it;
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
  64-hex `candidateSha256`. The builder MUST refuse a partial or malformed binding, and the verifier
  MUST refuse a non-null binding that is empty, partial or malformed. The digest is recorded as
  supplied and is not checked against the task store.
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
- `SRR-V1-007`: A platform row without native lifecycle evidence MUST be FALLBACK (PRS-V1-004) and
  MUST NOT be NOT_RUN. The linux/amd64 rows MUST cite decision 0420 until the operator supplies the
  hosted ubuntu-24.04 lifecycle or host-lifecycle evidence as a file. The builder and the verifier
  MUST refuse a linux/amd64 FALLBACK that cites another decision or only a reason.
- `SRR-V1-008`: The vulnerability section MUST record three things: decision 0420, the number of
  `require` directives in the Core `go.mod` at the bound commit (read through git, with space, tab
  and carriage return separating words as in the go.mod lexer), and the
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
  operator-named output path is the only place a caller may write them. Source-root git reads MUST
  run with lazy fetching from a promisor remote disabled (`GIT_NO_LAZY_FETCH=1`), and (V1-0362) with
  an empty credential helper and no transport (`GIT_ALLOW_PROTOCOL=`). The candidate
  verifier's transient host-probe directory is created and removed inside the system temporary
  directory.
- `SRR-V1-012`: (command shape accepted by decision 0422) A new operator binary,
  `cmd/corvint-readiness-record`, MUST expose the builder and the verifier as two modes of one flag
  set. Build mode, `-candidate DIR -source-root DIR -evidence-file TSV [-store-release ID
  -store-candidate-sha256 HEX] -output FILE`, MUST write the canonical record to FILE and MUST refuse
  an existing FILE rather than replace it. Verify mode, `-verify FILE -candidate DIR -evidence-file
  TSV`, MUST write nothing. Passing `-verify` selects verify mode, so an empty `-verify` value is
  a usage error, never build mode. The evidence file names each supplied row's evidence explicitly.
  `cmd/corvint-release-candidate` keeps its single flag set unchanged. Implementation detail: each
  line of the evidence file is `ROW`, `STATUS`, `PATH`, `DECISION` and `REASON` separated by tabs,
  with an absent value left empty. A CRLF line ending is read as LF. An error that names a row from
  the evidence file quotes it, so a hidden character in the row shows. Every reason must be valid
  UTF-8 without a control, format, line or paragraph separator, private-use, noncharacter,
  variation selector or other default-ignorable code point, and only a reason with a letter or
  digit explains a row. By design this refuses text that needs a format character or a variation
  selector: a zero-width joiner or non-joiner (emoji sequences, Persian, Devanagari conjuncts), a
  soft hyphen, the LRM, RLM and ALM marks, and the emoji presentation selector U+FE0F, as in a
  red heart emoji. A reason is plain release evidence, so write the words without them. A record
  built from a CRLF file before CRLF was read as LF stored reasons ending in a carriage return and
  no longer verifies; rebuild it. A relative `PATH` is appended to the evidence file's directory
  as spelled, without lexical cleaning, so each `..` is resolved after the symlinks before it, as
  opening the path resolves it; build and verify read it the same way. A line without exactly five
  fields, or a row named twice, is refused. Before building, build mode refuses a FILE whose name
  is empty, `.` or `..`, or whose temporary name (a dot, the name, a dot and 26 random characters)
  would exceed 255 bytes. Build mode resolves FILE's directory once to an absolute path free of
  symlinks: a relative directory is appended to the
  working directory's resolved path, and each component, `..` included, is resolved in order. The
  resolver follows up to 255 symlinks and the kernel far fewer, so build refuses FILE unless its
  directory as spelled (`.` when FILE has none) opens that same directory; otherwise the record
  would land where the reported path cannot reach. Build opens the resolved directory once,
  confirms the handle is that directory, and refuses it when it is, or lies below, the candidate
  or the source root, compared by file identity, so a symlink, a symlinked working directory, a
  `..` segment or a case alias cannot hide the overlap. A mount alias of a directory below a root
  (a Linux bind mount, a Windows `subst` drive, an SMB or NFS mount) has its own parents and is
  outside this guard. The parents are found by path, not from the open handle, because `os.Root`
  cannot open a handle's parent and Go has no portable `openat`. So after opening, build resolves
  the directory again, confirms that settled path is the handle's directory, and compares the
  settled path's parents, so an ancestor swapped for a link before the open is walked through the
  link's target. A concurrent writer that controls an ancestor of a root, or a directory inside
  one, is outside this guard, a non-goal. Go reads a
  Windows junction or volume mount point as neither a directory nor a symlink, so the resolver
  cannot pass one, and an output directory that passes through or ends in one is refused; this
  over-refusal is known. A directory that does not resolve is named as spelled in the error. On
  Windows the resolver's not-a-directory error is `ERROR_PATH_NOT_FOUND`, which a missing drive
  also gives, so the error names a file, a junction or mount point, and a missing drive as the
  possible causes. A
  Windows directory rooted on a drive or a separator but not absolute is refused (SRR-V1-011).
  Build then writes a completed temporary file and hard-links it to FILE, both through the open
  handle, so a partial record never appears at FILE and a path component replaced by a symlink
  during the build cannot redirect the write. A temporary name already taken is skipped, never
  truncated. A link refused because FILE exists is reported as an existing record; any other link
  failure is reported as a failed publication. A directory moved whole into a root during the
  build is not detected. Verify mode also rebuilds the rows from the evidence file and refuses a
  record whose rows differ. Without that check, a record that relabels a FAIL log as PASS would
  still reproduce its digest. Supplying only one of the two store flags is a usage error.

## Failure modes

| Failure | Behavior |
|---|---|
| Candidate fails `VerifyContext` | Builder and verifier refuse; no record (SRR-V1-002). |
| Source root lacks the commit, or its VERSION or build number differs | Builder refuses (SRR-V1-002). |
| Gate log not supplied | Row is NOT_RUN "evidence not supplied"; never PASS (SRR-V1-005). |
| Evidence file changed after the record was built | Verifier refuses on the digest mismatch (SRR-V1-006). |
| Core `go.mod` gains a `require`, or the local toolchain drifts | Vulnerability status FAIL (SRR-V1-008). |
| Record edited to claim a tag, signing or PASS without evidence | Verifier refuses (SRR-V1-006, 009, 010). |
| Record edited to reorder, duplicate or drop rows, or to empty the store binding | Verifier refuses (SRR-V1-004, 005). |
| Source root is a partial clone missing the bound objects | Git read fails without fetching; builder refuses (SRR-V1-011). |
| Store candidate digest is stale | Not detected; the owner cross-checks it (SRR-V1-004, open). |
| Output file already exists | Build mode refuses, reporting the existing record, and leaves it unchanged (SRR-V1-012). |
| Output file name is empty, `.` or `..`, or its temporary name would exceed 255 bytes | Build mode refuses before building; nothing is written (SRR-V1-012). |
| Output path is inside the candidate or the source root | Build mode refuses before building; nothing is written there. A symlink alias is caught; a mount alias of a directory below a root (bind mount, `subst` drive, network mount) is outside the guard (SRR-V1-011, 012). |
| Output directory does not resolve: it is missing, or passes through a file, a Windows junction or volume mount point, or a missing Windows drive | Build mode refuses before building, naming the directory as spelled; nothing is written (SRR-V1-012). |
| Output directory as spelled does not open the resolved directory: a symlink chain longer than the kernel follows, or a spelled link or the resolved directory replaced between resolving and opening | Build mode refuses before building; nothing is written (SRR-V1-012). |
| An ancestor of the output directory is swapped for a link into a root before opening, whether or not it is swapped back | Build mode refuses before building: the directory is resolved again after opening and the settled path's parents are compared; nothing is written (SRR-V1-012). |
| A concurrent writer that controls an ancestor of a root, or a directory inside one, moves directories during the check | Not detected, a non-goal: such a writer can already replace the root or write inside it (SRR-V1-012). |
| A path component of the output directory is replaced by a symlink during the build | The record is written through the directory handle opened before the check, never through the new link (SRR-V1-012). |
| Output directory's filesystem has no hard links | Build mode refuses, reporting a failed publication, and removes its temporary file; no record is published (SRR-V1-012). |
| Evidence file has CRLF endings, a reason that is invalid UTF-8 or carries a hidden character, or a reason with no letter or digit | CRLF is read as LF; a reason with invalid UTF-8 or a hidden character, a variation selector or a joiner included, is refused on every row; a reason with no letter or digit explains nothing, so NOT_RUN or FALLBACK without a decision is refused (SRR-V1-006). |
| Record relabels a supplied row, such as FAIL evidence as PASS | Verify mode refuses: the rows differ from the evidence file (SRR-V1-012). |

## Acceptance and rollback

SRR-V1-001 to SRR-V1-012 are covered by the focused tests below, which run against a git fixture
and a Core-only 1.0.0-rc.1 candidate fixture. Decision 0422 accepts this spec, and decision 0420 is
accepted. No release has used the record yet; its first use is the `1.0.0-rc.1` candidate.

Rollback deletes `cmd/corvint-readiness-record`, `internal/releasecandidate/readiness.go` and
their tests, and inlines `runSourceGit` back into `sourceBuildNumber`. No record, candidate, store or
wire state depends on the package yet.

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
| SRR-V1-011 | `readiness.go` (no writer), `candidate.go` (`runSourceGit`) | TestSRRV1011BuildAndVerifyWriteNothing, TestSRRV1011SourceGitHasNoCredentialHelperOrTransport |
| SRR-V1-012 | `readiness.go` (`ReadReadinessEvidence`, `WriteReadinessRecord`, `outputName`, `physical`, `unresolvedDirectory`, `sameDirectory`, `within`, `publishNoReplace`, `linkFailure`, `createTemporary`, `temporaryName`, `VerifyReadinessFile`), `cmd/corvint-readiness-record/main.go` | TestSRRV1012EvidenceFileAndNoReplaceRecord, TestSRRV1012ModesTakeTheirOwnFlagsOnly, TestSRRV1012ReportNamesRecordAndEveryRow |
