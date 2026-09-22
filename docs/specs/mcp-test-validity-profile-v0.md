# MCP test-validity profile V0

Owner: Russell Lewis
Date: 2026-09-12
Requirement prefix: `MTV-V0`
Intent status: accepted (decision 0147, 2026-09-12)
Delivery status: experimental
Authoritative inputs: AGENTS.md invariants 2, 4, 7 and 8; `public-release-v0.md` `PUB-V0-006`;
`live-proof-carrying-verification-v0.md` `LPCV-V0-047..054`; `mcp-server-2026-07-28-v0.md`
`MCPV0-008`; `../decisions/0103-mcp-v0-stays-three-tools-2026-09-12.md`;
`../decisions/0147-mcp-test-validity-descendant-profile-2026-09-12.md`;
`../decisions/0202-retained-test-evidence-discovery-2026-09-13.md`.

## Agent digest
- Claim: A separate local stdio MCP tool projects repository-confined JavaScript receipts and completed Go preview-session events into the shared five-axis shape.
- Status: accepted (decision 0147, 2026-09-12) / experimental
- Exists: `cmd/corvint-test-validity-mcp`, `internal/mcp/testvaliditybridge`, `internal/testvaliditydoc` (shared with `corvint test-validity`), `conformance/mcp-test-validity-v0`.
- Blocked on: no producer yet retains evidence into `.corvint/test-evidence` for `discover` (decision 0202); Go sessions remain preview-only and non-promotable; no qualified host run.
- Read next: Requirements; Non-goals and failure modes.

## Owner intent and scope

`PUB-V0-006` requires matching test evidence to reach the agent, and the MCP surface an agent uses
carried none: MCP 2026-07-28 V0 is frozen at exactly `corvint.query`, `corvint.impact` and
`corvint.status` (`MCPV0-008`), and decision 0103 scopes `LPCV-V0-047` away from it. Decision 0147
selects the smallest descendant shape, the same one `corvint-docs-mcp` already uses: a separate
executable with its own profile identity, reusing the MCP 2026-07-28 transport and tool-error
conventions, whose only tool presents the `corvint-test-validity/0` document that
`corvint test-validity` (`LPCV-V0-051`) computes. Both surfaces call one builder,
`internal/testvaliditydoc`, so they cannot disagree. The frozen profile and its bridge enum are
unchanged; the black-box suite adds the completed Go-session input vector this requirement now
admits. Decision 0202 adds the `discover` argument (`MTV-V0-009`) so the agent need not name the
input: the tool selects the newest retained document under `.corvint/test-evidence` and binds its
freshness to the worktree's current digests, exactly as `corvint test-validity --discover` does.

## Requirements

- `MTV-V0-001`: The profile MUST be served by the separate executable `corvint-test-validity-mcp
  --root ABSOLUTE_CLEAN_ROOT` (or `--version`) over default MCP 2026-07-28 stdio through
  `internal/mcp/server`, with server name `corvint-test-validity-mcp` and tool-error profile
  `corvint-test-validity-mcp-tool-error/0`. `tools/list` MUST advertise exactly one tool,
  `corvint.test_validity`, annotated read-only, idempotent, non-destructive and not open-world. It
  MUST NOT add a tool, argv option or enum value to `corvint-mcp`, `corvint-docs-mcp` or
  `conformance/mcp-2026-07-28` as part of this projection. The separately owner-approved shared
  compatibility amendment MCPV0-021..023 permits the explicit `--protocol-version 2025-11-25`
  selector on this existing command without changing its tool, receipt or authority contract.
- `MTV-V0-002`: The tool's input schema MUST be a closed JSON Schema 2020-12 object with two
  optional members, `discover` (a boolean, `MTV-V0-009`) and `receipt`: a string of 1 to 1024 characters that is not absolute and has no
  empty, `.`, `..` or `.git` segment and no backslash. Arguments over 4 KiB or not valid UTF-8, a
  `receipt` value that violates the path rule, and an unknown tool name MUST be JSON-RPC `-32602`.
  Well-formed arguments of the wrong type or with an unknown member MUST be a tool error with code
  `invalid-arguments`. Member names match exactly: a `receipt` of `null` is the wrong type, not an
  absent member, and a name that differs from `receipt` only in letter case is an unknown member.
- `MTV-V0-003`: Receipt location contract. The only receipt source MUST be the caller's `receipt`
  argument, resolved against the canonical root the server started with; no environment variable,
  argv option or default path supplies one, and `discover` (`MTV-V0-009`) reads only the retained
  evidence location, never a caller path. At startup the root MUST hold a non-symlink `.git`
  that is either a directory or, for a linked worktree (decision 0218), a regular file of at most
  4 KiB, read through the no-follow receipt reader, whose `gitdir: PATH` line names an existing
  directory (a relative `PATH` resolves against the root). A missing or symlinked `.git`, and a
  `gitdir` that is absent or does not exist, MUST be refused as `invalid-root`. Before each call the
  server MUST confirm the root and that `.git` entry are the same files as at startup and still
  satisfy this rule, else the call fails with `repository-unavailable`. The path is resolved through symlinks; a resolution outside the
  root or into `.git` (any resolved segment equal to `.git` ignoring case, as a case-insensitive
  volume opens it) MUST be a tool error `receipt-outside-repository`. A missing, non-regular or
  unreadable target, including a resolution to the root itself, or one replaced between stat and open, MUST be a tool error
  `invalid-test-validity-receipt`. After resolution, the reader MUST pin the root identity and
  each directory descriptor, refusing symlinks in every resolved component, including the leaf.
  The leaf open MUST be nonblocking and no-follow; its descriptor MUST be regular and have the
  same device/inode identity as the pre-open leaf stat. On platforms without these safe opens
  (currently outside Darwin/Linux), receipt reads MUST fail closed with that same tool error.
- `MTV-V0-004`: With neither `receipt` nor `discover: true`, the call MUST succeed with the `corvint-test-validity/0` document
  whose `source` is `none`, `tests` is empty, and every run axis is `UNSUPPORTED` with reason
  `no-input-supplied` (`LPCV-V0-049`). An absent receipt MUST NOT yield a pass on any axis.
- `MTV-V0-005`: With a `receipt`, the document MUST be exactly what `corvint test-validity
  --receipt` emits for the same bytes: decoded as exactly one closed `corvint-js-test-provider`
  document or completed `corvint-go-live-session-event/0` document, with every per-test and run
  projection recomputed by `internal/testvaliditydoc` from the retained receipt/observation fields
  through the same shared projectors the producers use. Carried projections MUST never be trusted.
  Go output MUST carry `tier:"preview"` and `promotable:false` and MUST NOT be usable as qualified or
  policy evidence. An input with an unknown field, trailing data, absent required discriminator,
  invalid JavaScript kind, non-completed Go state, unrecognized kind, or both kind discriminators
  MUST be a tool error `invalid-test-validity-receipt` carrying no projection. Its message is fixed
  text that quotes no receipt content, because the tool-error text block is not framed (`MCPV0-016`).
- `MTV-V0-006`: A successful result's `structuredContent` MUST be `{schema:
  "corvint-mcp-test-validity-result/0", tool: "corvint.test_validity", mutates: false, receipt:
  PATH|null, document}`; its single text block is that object's canonical JSON framed by the
  `AHI-004` repository envelope. The server MUST enforce a 4 MiB receipt bound on the opened
  descriptor, reading at most 4 MiB plus one overflow-detection byte (more than 4 MiB is
  `invalid-test-validity-receipt`) and MUST refuse a canonical result over 256 KiB with the tool
  error `test-validity-limit-exceeded`, never a truncated or partial projection. Every tool error
  MUST set `isError: true`, `mutates: false` and an active `OPERATION_FAILED` abstention.
- `MTV-V0-007`: The tool MUST open no network connection or listener, start no process, run no
  test, and write nothing: no repository file, Git index, trace store or
  `.corvint/self-observations.jsonl` (AGENTS.md invariant 4). Its own production sources
  (`internal/mcp/testvaliditybridge`, `internal/testvaliditydoc`) MUST NOT import `net`, `net/*`,
  `os/exec`. Only `internal/testvaliditydoc/read_unix.go` MAY import `syscall`, solely for
  `O_DIRECTORY`, `O_NOFOLLOW`, `O_NONBLOCK` and `O_CLOEXEC` file-open constants; it MUST NOT invoke
  syscall functions.
- `MTV-V0-008`: `conformance/mcp-test-validity-v0/cases.json` MUST hold the profile's black-box
  vectors, each naming its requirement, and `blackbox_test.go` MUST run every vector against the
  built executable over stdio only, importing no Corvint package, and MUST fail when the repository
  tree changes across the run. A vector whose expected result the server no longer produces is a
  regression, not a vector to delete.
- `MTV-V0-009`: `discover: true` (decision 0202) MUST return, as the result's `document`, exactly
  the document `corvint --root ROOT test-validity --discover` emits for the same worktree
  (`LPCV-V0-053`, `LPCV-V0-054`), through the shared `testvaliditydoc.Discover`, with `receipt:
  null`; its refusals are tool errors with the same codes, `invalid-test-evidence-location` and
  `test-evidence-limit-exceeded`. `discover: false` is the same as omitting it. A non-boolean
  `discover`, or `discover: true` together with a `receipt`, MUST be the tool error
  `invalid-arguments`. The root re-identification of `MTV-V0-003` and every `MTV-V0-006` and
  `MTV-V0-007` bound and prohibition apply unchanged.

## Non-goals and failure modes

Non-goals: any change to MCP 2026-07-28 V0 or its conformance suite; running tests, watching a
workspace or pushing notifications; promoting a Go preview session into qualification or policy
evidence; discovery outside `.corvint/test-evidence` or writing retained evidence; inclusion in the
independent default core archives (the separate optional companion bundle may carry the executable
under `PUB-V0-002..015`); and any claim that `PUB-V0-006` is met while no producer retains evidence for
`discover` to find (decision 0202). This profile also defines
no cancellation behavior of its own: request cancellation for `corvint.test_validity` follows
`MCPV0-011`'s response-suppression rule (`mcp-server-2026-07-28-v0.md`), enforced by the same shared
`internal/mcp/server` dispatcher `MTV-V0-001` reuses.

Failure modes. A suite whose projection exceeds 256 KiB is refused rather than truncated; the CLI
remains the route for it. Resolution and open are separate steps: subsequent reads descend pinned
directories without following symlinks, so a symlink replacement fails closed even before the leaf stat. This does not
freeze bytes of an inode that a local writer already holds, or prohibit hard links; provider
receipts remain local, mutable input rather than immutable execution evidence. The `.git` refusal is
a naming rule over the path as resolved, not confinement against a concurrent worktree writer: a
directory renamed onto a resolved component between resolution and the pinned descent (for example
`.git` renamed to the resolved parent) is read, as a hard link to a `.git` file is read without any
race. Both require write authority over the worktree that already holds those bytes, so neither
widens what that writer can disclose, and a post-open `.git` identity check would close only the
rename; this is outside the profile's threat model (settled 2026-09-12). The receipt's facts are
provider authority: recomputing projections stops a document from asserting strength or freshness it
lacks, but not a provider from misreporting what ran. A Go event retains action observations but not
the source-locator input behind a carried file:line anchor, so recomputation omits that anchor rather
than trusting it. The binary links packages that contain network
code through `internal/tcq`; the import rule covers the tool path's own sources only, so the
no-network property for linked code is inferred from the call graph, not proven by linkage.

## Acceptance evidence and rollback

Security regression evidence (2026-09-12): `TestReceiptRejectsResolvedPathSymlinkSwap` failed
against the original reader after extraction of its post-resolution step: both a leaf and a parent
symlink returned outside bytes. The corrected reader also covers post-stat FIFO substitution,
replaced inode identity, stable confined symlinks and descriptor-bound input growth. Existing stdio
vectors keep the same refusal codes; deterministic swap tests run at the filesystem boundary.

Acceptance evidence, run 2026-09-12 on branch `wave14-mcptvprofile-20260912`:
`GOTOOLCHAIN=local go test -count=1 -timeout 30m ./conformance/mcp-test-validity-v0
./internal/mcp/testvaliditybridge` and `go test -run TestTestValidity ./cmd/corvint`, all exit 0;
the full conformance vector list is in `cases.json`. `./internal/specindex` reports no error for
this spec; it fails only on the pre-existing `documentation-citation-gate-v0.md` claim. Official MCP
schema validation is `NOT_RUN`, as for the parent profile.

Rollback: delete `cmd/corvint-test-validity-mcp`, `internal/mcp/testvaliditybridge` and
`conformance/mcp-test-validity-v0`, and mark this spec superseded. `internal/testvaliditydoc` can
stay, because `corvint test-validity` output is byte-identical with or without this profile. No
persisted state, schema or client configuration needs migration.

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `MTV-V0-001` | `cmd/corvint-test-validity-mcp/main.go`; `testvaliditybridge.Registry.Tools` | `TestToolCatalogueIsExactlyOneReadOnlyTool` |
| `MTV-V0-002` | `testvaliditybridge.Registry.Call`, `validReceiptPath` | `TestToolCatalogueIsExactlyOneReadOnlyTool`, `TestUnknownToolIsInvalidParams`; vectors `receipt-parent-escape`, `receipt-absolute`, `receipt-git-directory`, `arguments-wrong-type`, `arguments-unknown-member`, `arguments-null-receipt`, `arguments-case-variant-member` in `TestVectorsAndReadOnly` |
| `MTV-V0-003` | `testvaliditybridge.Registry.readConfined`, `sameRoot`, `worktreeMarker` | `TestNewAdmitsLinkedWorktreeOnly` (linked worktree admitted; missing `.git`, symlinked `.git`, missing gitdir refused), `TestReceiptRejectsResolvedPathSymlinkSwap`, `TestReceiptReadsConfinedSymlink`, `TestReceiptSymlinkToRootIsInvalidReceipt`, `TestReceiptRejectsReplacedFile`, `TestReceiptRejectsFIFOAfterStat`, `TestReceiptRefusesCaseFoldedGitDirectory`, `TestReceiptRejectsInRootDirectorySymlink` (`internal/testvaliditydoc`); vectors `receipt-symlink-escape`, `receipt-missing`, `receipt-git-directory` in `TestVectorsAndReadOnly` |
| `MTV-V0-004` | `testvaliditydoc.Unsupported` | vector `no-receipt-unsupported` in `TestVectorsAndReadOnly`; `TestTestValidityWithoutReceiptAbstainsOnEveryAxis` |
| `MTV-V0-005` | `testvaliditydoc.Decode`, `testvaliditydoc.Project`; `testvalidity.ProjectGoSession`, `testvalidity.ProjectGoTest` | vectors `receipt-recomputed-not-trusted`, `go-session-recomputed-preview`, `receipt-not-provider-document` in `TestVectorsAndReadOnly`; `TestTestValidityProjectsReceiptWithoutTrustingCarriedProjections`, `TestTestValidityRefusesNonReceiptInput`, `TestInvalidReceiptMessageEchoesNoReceiptText`; `internal/testvaliditydoc`'s `TestGoSessionIsProjectedFromObservations`, `TestGoSessionCarriedProjectionsAreIgnored`, `TestUnknownAndAmbiguousProviderKindsAreRefused`, and `TestGoSessionDisclosesPreviewTierAndCannotPromote` |
| `MTV-V0-006` | `testvaliditydoc.ReadFile`, `testvaliditybridge.result`; `toolFailureResult` in `cmd/corvint-test-validity-mcp` | `TestReceiptBoundsOpenedFile`; vectors `receipt-oversized`, `result-over-limit` and the result-shape checks in `TestVectorsAndReadOnly` |
| `MTV-V0-007` | `internal/mcp/testvaliditybridge`, `internal/testvaliditydoc` | `TestToolPathImportsNoNetworkOrProcessPackage`; tree digest in `TestVectorsAndReadOnly`; `cmd/corvint`: `TestCLIReadVerbsLeaveTheRepositoryByteIdentical` (CLI-level `test-validity` verb sharing the same write-nothing `internal/testvaliditydoc` path) |
| `MTV-V0-008` | `conformance/mcp-test-validity-v0/cases.json` | `TestVectorsAndReadOnly` |
| `MTV-V0-009` | `testvaliditybridge.Registry.discover`, `decodeArguments`; `testvaliditydoc.Discover` | `TestTestValidityDiscoveryMatchesMCPDocument` (`cmd/corvint`, CLI and MCP documents canonically byte-equal); `TestToolCatalogueIsExactlyOneReadOnlyTool` (two schema members); vectors `discover-retained-evidence` and `discover-with-receipt-refused` in `TestVectorsAndReadOnly` |
