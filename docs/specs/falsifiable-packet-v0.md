# Falsifiable Packet V0

Owner: Russell Lewis
Date: 2026-09-01
Requirement prefix: `FPK-V0`
Intent status: accepted (decision 0052)
Delivery status: experimental
Revision status: revision 18 (FPK-V0-015 and FPK-V0-030 refuse repeated and case-variant JSON member names in a signed envelope and CEM statement; revision 17 FPK-V0-030 refuses a signed CEM claim `CEMStatement` could not have produced; revision 16 restated FPK-V0-021 and FPK-V0-024 rationale against the implemented checkpoint branch)
Authoritative inputs: `docs/plans/BREAKTHROUGH-BET-2026-09-01.md` (the bet this is slice 1 of),
`docs/reviews/FABLE-5.1-AUDIT-2026-09-01.md` (F2, F3), `docs/specs/go-production-kernel-migration-v0.md`
(GPK-V0-002 exact parity of the `query` wire), `conformance/cli-parity-v0` (byte-exact replay of committed
expectations; its Python oracle was retired by decision 0088), `docs/decisions/0063-vocabulary-rows-carry-a-citation-check-2026-09-05.md`
(accepted prototype), `docs/reviews/b2-evidence-breakthroughs.md` (B1), and `AGENTS.md` invariants
1, 2, 4, and 8.

## Agent digest
- Claim: `corvint prove` embeds the unchanged query packet and attaches a mechanical falsifier and PASS/FAIL/NOT_RUN verdict beside every evidence row.
- Status: accepted (decision 0052)/experimental
- Exists: `cmd/corvint/prove.go` and its test files, `internal/liveverify/pyresolve`, `internal/liveverify/jsresolve`, `internal/liveverify/mutate`, `internal/liveverify/pymutate`, `internal/attest`, help topic `prove`.
- Blocked on: the 19-of-20 second-checkout witness replay gate, a falsification rate over time, and the three-arm trial in the bet.
- Read next: Requirements; Non-goals and authority; Failure modes.

## Intent and scope

The audit found that a query packet counts every non-advisory row as authoritative, including rows
that matched the task only by vocabulary, and never says "nothing here is proven" (F3). The bet
answers with one rule: every statement carries the check that would catch it being wrong, Corvint runs
the check or says `NOT_RUN`, and only checked statements count. The `query` wire cannot change
today because `conformance/cli-parity-v0` compares it byte-for-byte against the Python oracle, so
this slice ships the rule as a separate, opt-in, read-only command whose document embeds the query
packet unchanged and places the verdicts beside it.

Affected user: an agent or engineer about to act on a packet. Measurable job: for one task on one
tree, say in bounded time which results rest on content Git can confirm at the packet's revision,
and refuse to call a packet ready when none do. Before Decision 0063, the audit's out-of-domain
task (`add OAuth2 login with Google to the web dashboard`) produced five vocabulary rows that
`prove` left `NOT_RUN`. Those rows now receive `history-consistent`; a clean, otherwise `READY`
vocabulary-only packet reports `CITED`, not `PROVEN`, when every citation resolves.

2026-09-02: the premise above that the `query` wire cannot change is superseded for this one
condition. `GPK-V0-046` withdraws `authoritative_results` from a syntax-only query packet and names
the condition in `coverage.uncertainty`, taking the identical repair to the Python oracle rather
than declaring a divergence, so that packet now reports `authoritative_results: 0`. This narrows
what `prove` adds on this task — the packet already refuses to call the rows corroborated — and
removes none of it: `prove` still says which rows Git can confirm and which were never checked,
which no coverage count states. The slice's requirements are unchanged.

2026-09-12: decision 0088 retired the Python oracle, and `GOC-V0-002` forbids reconstructing it. The
`query` wire is still held byte-exact, now by replaying the expectations committed in
`conformance/cli-parity-v0/manifest.json` rather than by a live oracle comparison, so the premise
above stands with that substitution.

## Requirements

- **FPK-V0-001:** `prove` MUST be read-only. It MUST NOT write repository, trace, or
  self-observation ledger state on any path, including every error path, and its wire MUST carry
  `mutates:false`.
- **FPK-V0-002:** The document MUST embed the packet that `query` would emit for the same root,
  task, limit, and budget, byte-identical under canonical encoding. Verdicts MUST live outside the
  packet, under `proof.rows`, one entry per evidence row under `packet.results[].evidence[]`, in
  packet order, each naming the result kind and id, path, line, blob hash, authority, `falsifier`,
  and `falsified`. A row is bound to its result by kind and id together, because one packet can
  carry two results with one id (an external-package `_test.go` is both a `test` and a
  `reverse-import` result). Evidence rows elsewhere in the packet (for example `abstention.nearest_claims`) are
  outside this requirement in v0.
- **FPK-V0-003:** `falsifier` MUST be one of `history-consistent`, `reference-resolves`,
  `verifier-accepts`, `test-kills-mutant`, or `none`, assigned by a closed table keyed on the
  row's `authority`; an authority absent from the table MUST get `none`. `falsified` MUST be one of
  `PASS`, `FAIL`, or `NOT_RUN`. The table is consulted first on the pair (result `kind`,
  `authority`) and then on `authority` alone. All five run in v0; `test-kills-mutant` is judged only
  under `--mutate` (FPK-V0-014) and is otherwise `NOT_RUN`.
  Accepted amendment (decision 0063, 2026-09-05): `syntax` is assigned
  `history-consistent`. When a pair-specific falsifier cannot judge the row's language or test
  path, assignment MUST fall through to the authority table instead of returning `none`; an
  authority absent from that table still gets `none`. The falsifier vocabulary remains the same
  five values.
  End of accepted amendment.
- **FPK-V0-004:** `history-consistent` MUST be judged against Git, never against the index that
  produced the row. Its verdict is `PASS` when `git cat-file` at `<packet.revision>:<path>` yields a
  blob whose id equals the row's `blob_hash` and the row's `line` lies in `1..lines(blob)` (an
  empty blob counts as one line); `FAIL` when the object is missing, is not a blob, has another
  id, or has no such line; `NOT_RUN` when the path is dirty in the worktree or contains a newline
  or carriage return. Rows with `falsifier: none` are always `NOT_RUN`.
  Accepted amendment (decision 0063, 2026-09-05): `history-consistent` is a citation check for
  every authority that carries it and is never evidence of relevance. Its claim is exactly: `the
  cited blob and line still exist at the packet revision; this is a check of the citation, not
  evidence that the row is relevant`.
  End of accepted amendment.
- **FPK-V0-005:** A result answers for the rows that carry its kind and id only. It is `proven`
  only when it has at least one falsifier-bearing row and every
  such row is `PASS`; it is `failed` when any row is `FAIL`; otherwise it is `unproven`. Rows with
  `falsifier: none` never contribute. `proof` MUST report `proven_results`, `unproven_results`,
  `failed_results`, and `counts` per falsifier and verdict. A packet whose own state is `READY`
  with zero proven results MUST be reported with `state: UNPROVEN`; every other packet state passes
  through unchanged.
  Accepted amendment (decision 0063, 2026-09-05): before the zero-proven-results rule, a packet
  whose own state is `READY`, with at least one falsifier-bearing row and every such row carrying
  authority `syntax` and falsifier `history-consistent`, MUST report `state: CITED`. Its result
  counts and per-falsifier counts are unchanged.
  End of accepted amendment.
- **FPK-V0-006:** The whole operation MUST be bracketed: the dirty set is read before the packet is
  compiled and again after the cited blobs are read, and the HEAD tree MUST still equal the
  packet's revision at the end; any difference MUST fail closed with `unsupported-prove-drift`.
- **FPK-V0-007:** For a fixed tree, HEAD, dirty set, task, limit, and budget the document MUST be
  byte-identical across runs. No wall-clock cost appears in the wire.
  Accepted amendment (AT-06, decision 0052): under `--checkpoint` the fixed inputs are the tree, HEAD,
  dirty set, and the bytes of the checkpoint document, and EVERY member of the emitted document
  MUST be byte-identical across runs, `proof.ledger` excluded because checkpoint mode omits it
  entirely (see the FPK-V0-016 amendment). The checkpoint path reads no index snapshot and emits no
  cache-metadata member, so it claims no exemption from this clause (see FPK-V0-024). That
  read-nothing claim is not checkable from the wire, since `IDX-SNAP-V0-006` requires a snapshot
  reader to emit identical bytes on a hit and a miss, so the implementation exposes the package-level
  `var loadSnapshot = contextindex.LoadSnapshot` and its deferred sibling `loadSnapshotDeferred = contextindex.LoadSnapshotDeferred`
  (`cmd/corvint/index_snapshot.go:17-20@8984cf3b`). The deferred seam's one call is `deferredSnapshotIndex` (`cmd/corvint/index_snapshot.go:71-72@5959c784`). The
  snapshot seams' seven current production call sites are `snapshotIndex` (`cmd/corvint/index_snapshot.go:57-58@123f0830`) and batch (`cmd/corvint/batch.go:149@dbef447a`),
  answerability (`cmd/corvint/answerability.go:94-95@6278a445`) and surprise (`cmd/corvint/surprise.go:118-119@6278a445`),
  context lookup (`cmd/corvint/context_lookup.go:75-79@9f1de421`) and local completion events (`cmd/corvint/local_completion_event.go:356-360@ea059984`),
  and the experimental host adapter (`cmd/corvint/host_adapter_experimental.go:43-48@b6d4dd7c`). The task-context path instead uses its separate
  `loadContextSnapshot` seam (`cmd/corvint/taskcontext.go:156-163@73708428`), backed by `LoadContextSnapshotDeferred` and the private loader (`internal/contextindex/observed_build.go:42-49@d916a414`).
  The `cmd/corvint` seam does not cover `LoadEventSnapshot` or `ProbeSnapshot`, called directly by the harness and index paths (`cmd/corvint/harness_context.go:33-35@44bd361a`, `cmd/corvint/index_snapshot.go:117-118@9a7d60f2`); the harness calls `LoadEventSnapshotDeferred` there too.
  The load-bearing guard scans every non-test Go file in `cmd/corvint`, rejects direct `LoadSnapshot` or `LoadSnapshotDeferred` references outside their seam bindings, and additionally rejects `LoadEventSnapshot`, `LoadEventSnapshotDeferred` and `ProbeSnapshot` in `prove*` files
  (`cmd/corvint/prove_checkpoint_test.go:704-771@0c2b29a4`); the counting test asserts the checkpoint run traverses neither dynamic seam
  (`cmd/corvint/prove_checkpoint_test.go:666-681@62a5f8d6`) (FPK-V0-024).
  End of accepted amendment.
- **FPK-V0-008:** Failures MUST be typed and exit 2 with nothing on stdout: query compile errors
  pass through with their own codes; `unsupported-prove-revision` (no Git, no HEAD tree, no
  packet revision); `unsupported-prove-history` (status or `cat-file` failure, bound exceeded,
  unreadable batch stream); `unsupported-prove-drift`; `invalid-arguments` for anything the query
  parser rejects.
  Accepted amendment (AT-06, decision 0052): the
  "nothing on stdout" guarantee binds refusals only. An output-transport failure is not a refusal:
  encoding and the stdout write both happen after every verdict is decided, either can fail, and
  both exit 2 with `output-failed` (`cmd/corvint/prove.go:479-483@4c0799f8`, `cmd/corvint/prove.go:491-493@ea3220b5`). A failed write
  MAY leave partial bytes on stdout, so a consumer MUST read the exit status, never stdout
  emptiness, as the signal that no verdict was produced — the same exit-2 signal the harness
  gives for its own write failure (`cmd/corvint/main.go:1173-1175@f3b5fd7c`), which the adapter contract
  converts into a visible host-valid no-op (`docs/specs/agent-harness-integration-v0.md:64-65`).
  This clause's code list is also extended, under `--checkpoint` only, by the six codes FPK-V0-024
  enumerates: `unreadable-checkpoint-document`, `invalid-checkpoint-document`,
  `checkpoint-bound-exceeded`, `object-format-mismatch`, `unsupported-prove-index`, and
  `unsupported-prove-tree`. No other
  `prove` mode may emit any of them, so the accepted vocabulary of every mode that exists today is
  unchanged.
  End of accepted amendment.
- **FPK-V0-009:** The `query`, `impact`, `feature`, and harness wires and every parity case MUST be
  byte-unchanged by this slice.
- **FPK-V0-010:** `prove [--limit N] PATH...` MUST embed the packet that `impact` would emit for the
  same root, paths, and limit, under the same rules as FPK-V0-002. `--working-tree-untracked` MUST
  be refused with `invalid-arguments`: the falsifiers judge tracked blobs at the packet's revision
  only. `--base` selects change mode (FPK-V0-017) and MUST be refused with paths, as `impact`
  refuses it.
- **FPK-V0-011:** `reference-resolves` is assigned to `syntax` rows of results of kind
  `reverse-import` and `reference` whose cited path ends in `.go`, `.py` (judged under
  FPK-V0-013), or a web suffix (judged under FPK-V0-018); the same rows for any other language
  fall through to the authority table and get `history-consistent` under the decision 0063
  amendment to FPK-V0-003. The citing blob MUST hold the row's `blob_hash` and MUST parse as Go, else
  `FAIL`. An import row (`reason` `imports X`) is `PASS` when an import spec whose path is `X` sits
  on the cited line. A reference row (`reason` `references N declared by C`) is `PASS` when an
  identifier named `N` sits on the cited line of the citing blob and the blob at `C` declares `N`
  at top level as a function, method, type, var, or const; `NOT_RUN` when `C` is dirty; `FAIL`
  otherwise. The check is identifier occurrence, not binding: a same-named identifier bound
  elsewhere passes. Rows under `abstention.nearest_claims` remain outside v0.
- **FPK-V0-012:** `prove --cem MAP [--expected-base REV] [--target REV] [--patch FILE]` MUST embed
  the document that `cem verify` would emit for the same inputs, under the same rules as
  FPK-V0-002, and MUST emit one row per hunk of the map: `result` is the hunk id, `path` and
  `line` are the hunk's path and new-range start, `blob_hash` is the first basis evidence's blob,
  and `authority` is `cem-` followed by the hunk's disposition. `supported` and `mechanical` hunks
  get `verifier-accepts`; `unknown` hunks get `none`, because an explicit unknown is not a claim.
  A `verifier-accepts` row is `PASS` when the verifier accepted the map and every basis evidence of
  the hunk is `stable` or `relocated` at the target when a drift check ran; `FAIL` otherwise,
  including for every claim row of a rejected map, because the verifier's contract is whole-map
  acceptance. Inputs the verifier requires but did not receive (`expected-base-required`,
  `target-required`, `map-unavailable`) are typed failures with exit 2, not verdicts. `state` is
  `REJECTED` when the map was refused, `UNPROVEN` when it was accepted with no proven hunk, and
  `READY` otherwise. The bracket checks the dirty set and that the map bytes read for the rows are
  byte-identical to a second read after verification (else exit 2 `unsupported-prove-drift`); the
  verifier reads committed objects, and a worktree edit of the map is otherwise caught by its own
  target-side sidecar comparison. An absolute or climbing map path is `map-unavailable` before any
  read, as is a map path with an existing path component under `--root` that is itself a symlink
  (even one that resolves back inside `--root`) or that case-folds equal to `.git`
  (`strings.EqualFold`, the same rule `internal/companionrelease.refuseUnsafePath` uses) — a lexical
  "repository-relative" check alone would let a symlinked parent or a case-insensitive filesystem's
  `.GIT` read outside the intended tree. A map path that is not itself a regular file, including a
  symlink, or whose identity changes while it is opened is also `map-unavailable`, before any
  content read. The CEM branch MUST also
  read the HEAD tree before and after verification and refuse with
  `unsupported-prove-drift` if it changes; with an explicit non-HEAD `--target`, this tree half MAY
  refuse conservatively. A CEM failure that carries no frozen CEM code is reported as
  `unsupported-prove-cem` (`cmd/corvint/prove.go:961@975538a3`).
- **FPK-V0-013:** A `reverse-import` row whose cited path ends in `.py` (`reason` `imports X`) MUST
  be judged by the Python import grammar in `internal/liveverify/pyresolve` over the committed blob,
  never by the index. It is `PASS` when an import statement that begins on the cited line binds
  `X`, either as written (`import a.b` binds `a.b`; `from a.b import c` binds `a.b` and `a.b.c`) or
  as a relative import resolved from the citing file's package, which is its directory with a
  leading `src` or `lib` dropped, the rule the index used to make the claim. It is `FAIL` when the
  blob does not tokenize or no statement on that line binds `X`. Comments and strings are never
  imports. A `reference` row for a `.py` path is `NOT_RUN`.
- **FPK-V0-018:** A `reverse-import` or `reference` row whose cited path ends in `.js`, `.mjs`,
  `.cjs`, `.jsx`, `.ts`, or `.tsx` (a `.d.ts` path is a `.ts` path) MUST be judged by the lexer in
  `internal/liveverify/jsresolve` over the committed blob, never by the index. That lexer is a
  line-tracking copy of the index's own web import lexer (`internal/contextindex/webimports.go`,
  unexported and line-blind): comments produce no tokens, a `//` comment ends at `\n`, `\r`,
  U+2028, or U+2029, each of those four also separates statements (bare or inside a block
  comment) while lines count `\n` only, an unclosed plain-quoted literal ends at `\n` or `\r`
  (U+2028 and U+2029 are legal inside one and do not end it, and a `\` before `\r\n` continues it
  through both bytes as one escaped line break),
  a template literal is one token, and a `/` reads as division unless it follows nothing or one of
  the punctuators `( , = : [ & | ? { ; ~ ^ % *` and closes on its line, when it is one
  regular-expression token whose quotes and backticks open nothing; in particular, a `/`
  immediately after a closing `}` remains division. An import row (`reason`
  `imports X`) is `PASS` when a statement whose specifier literal is exactly `X` as written spans
  the cited line, its keyword on or before the line and the literal on or after it: ESM
  `import ... from "X"`, `import "X"`, `export ... from "X"`, dynamic `import("X")`, or
  `require("X")`; `FAIL` otherwise. `X` is the specifier the index recorded, not a resolved path,
  so the falsifier compares the literal and does not re-resolve it to the changed file;
  `require` is accepted although the index never records it (decision 0019). A reference row
  (`reason` `references N declared by C`) is `PASS` when a word token spelled `N` sits on the cited
  line of the citing blob and the blob at `C` declares `N` at top level, outside every brace,
  bracket, and parenthesis and at a statement start, as `function N` (`async` and generator
  forms included), `class N`, `const|let|var N`, any of those after `export` or `export default`,
  or a name in a top-level `export { ... }` list; `NOT_RUN` when `C` is dirty; `FAIL` otherwise.
  The check is identifier occurrence, not binding, as FPK-V0-011 states. Not admitted:
  destructuring declarations, TypeScript `type`, `interface`, and `enum`, and `module.exports`
  assignments. Accepted false positives: a quote inside a regular-expression literal opens a
  string on that line, and a name inside a regular-expression literal counts as an occurrence.
  The index today emits no `reference` rows for web paths (its same-package reference rule is
  Go-only), so that arm is exercised over committed blobs by test, not by an `impact` packet.
- **FPK-V0-014:** `test-kills-mutant` is assigned to `test-convention` rows of results of kind
  `test` whose cited path ends in `_test.go` (`reason` `same-package test for C`); other languages
  get `none`. Without `--mutate` the row is `NOT_RUN` and carries no `detail`. `--mutate` is
  accepted in impact mode only and MUST be refused elsewhere. Under `--mutate` the runner in
  `internal/liveverify/mutate` exports the packet revision from raw objects, listing its tree
  with `git ls-tree -r -z --full-tree` and streaming the blobs with `git cat-file --batch`, the
  revision passed after `--end-of-options` so an option-shaped revision is never read as an
  option. Neither command applies attributes or conversion settings, so no committed, worktree,
  or local `.git/info/attributes` `export-ignore`, `export-subst`, or eol attribute, and no
  `core.autocrlf`, can drop or rewrite a blob in the copy (`git archive` offers no way to ignore
  `.git/info/attributes`). It exports once
  per invocation into a temporary directory and judges every row on that copy, restoring the
  changed file after each mutant so no row sees another's mutation; a claim naming another
  revision is refused. The copy MUST NOT silently differ from the revision's tree: a gitlink is
  an empty directory, as in a checkout without that submodule, and a tracked symlink, any other
  entry the copy does not reproduce, or two tracked paths the volume folds into one file fails
  the export (`unsupported-prove-mutation`). For each row it runs the cited test file's tests once as a baseline,
  then applies at most eight
  single-operator AST mutants to `C` one at a time (`negate-condition`, `swap-binary`,
  `replace-literal`, `delete-statement`), stopping at the first kill. Rows claiming the same
  changed path are judged as a group, one `go test` per package per mutant whose `-json` output
  attributes each row's own test functions, and each row's verdict and `detail` are those
  judging it alone would give. Baseline, mutant, and isolated fallback invocations MUST all use
  the same `go test -json` mode: JSON implies verbose testing, so changing that mode can itself
  change test behavior. The 2026-09-06 authorized audit repair enforces this in the single-claim
  helper shared with grouped fallbacks; it does not establish exploitation of earlier default CLI
  grouped runs. A mode-only failure MUST fail baseline admission before any mutant receives credit.
  The row is `PASS` when a
  mutant is killed, `FAIL` when every mutant survives, and `NOT_RUN` when `C` holds nothing
  mutable, the ten-minute budget for the row runs out, the baseline fails, or module dependencies
  are unavailable offline; `detail` carries the runner's one deterministic line. A mutant that
  does not build (`go test` reports `[build failed]` or `[setup failed]`) is neither killed nor a
  survivor: it is counted separately in `detail`, and a file whose every mutant fails to build is
  `NOT_RUN`. A `:=` definition is never deleted, since that deletion almost never builds. The cited
  test blob MUST sit at the cited path in the packet revision, else the row is `FAIL` before any
  run. A dirty cited path is `NOT_RUN`. Every mutation row of one invocation shares a
  thirty-minute invocation budget beside the row budget, whose clock starts before the packet
  revision is exported, and the export's git commands together run under a sixty-second
  deadline whose expiry is an export failure: rows are judged in document order,
  and a row reached after that budget is spent is `NOT_RUN` with `detail`
  `invocation mutation budget exhausted before this row`, so a wide change is disclosed as
  partly judged rather than proved late. Runs happen inside the FPK-V0-006 bracket; a runner
  failure is exit 2 `unsupported-prove-mutation`, not a verdict. The runner itself never writes
  the repository, and the cited tests are never trusted: every `go test` MUST run inside a host
  sandbox that denies every network operation, every signal to a process outside the sandbox,
  and every write outside a scratch directory and the build cache, with `/dev/null` writable;
  the exported revision is read-only to the tests and only the runner writes mutants into it.
  The scratch directory is emptied before every run so no run inherits another's state.
  `TMPDIR`, `GOTMPDIR`, and `GOCACHE` point inside the writable set, `GOENV` is off, and
  `GOMODCACHE` is pinned to the host's module cache, resolved in-process by the same precedence
  `go env GOMODCACHE` uses (the `GOMODCACHE` variable, then the user go/env file, then
  `GOPATH`/`pkg/mod`) without executing `go`, so the resolution itself stays read-only, and only
  read thereafter. Each run is
  its own process group, killed after the run whatever its outcome. On macOS the sandbox is
  `/usr/bin/sandbox-exec`; on Linux it is `bwrap` with the filesystem bound read-only, the
  writable set bound read-write, a fresh `/dev` and `/proc`, `--unshare-net`, and
  `--unshare-pid` so the pid namespace dies with the run. A host with no sandbox runs no test
  and the row is `NOT_RUN` with `detail` `cited tests cannot be sandboxed: R`. A run that ends
  without go test's own failure report (`--- FAIL` or a `FAIL` package line) is a
  runner failure (`unsupported-prove-mutation`), never a kill, so a `panic:` printed by anything
  but a test go test reported failing (a toolchain crash) earns no credit; go test's `-timeout` is the
  per-run share and the runner waits five seconds longer, so a hung mutant is go test's
  failure. The rows of one invocation share the copy's temporary build cache, removed with
  the copy; the host's own cache is never used. Tests run with `GOPROXY=off`,
  `GOFLAGS=-mod=readonly`, and `CGO_ENABLED=0`. Uncited tests of the same package (`TestMain`,
  `init`) still execute under the same confinement. Accepted residuals: a test may fill the
  scratch directory or build cache until the per-run timeout; a baseline test that predicts
  the deterministic mutants could poison the shared build cache for them; on macOS a
  descendant that changes its session outlives the process-group kill but stays confined for
  its lifetime.
- **FPK-V0-017:** `prove --base FULL_COMMIT_ID [--limit N] [--mutate]` is change mode, the
  packet anchored to a committed change (decision 0016). It MUST embed the document that
  `impact --base FULL_COMMIT_ID --limit N` would emit, under the same rules as FPK-V0-002, and its
  packet rows are assigned exactly as in impact mode. It MUST then add one row per test the
  affected-plan selector (`docs/specs/affected-plan-v0.md`) names when run over the paths the
  range changed (`git diff-tree -r --name-only --no-renames --diff-filter=ACMT BASE HEAD`,
  bounded at 1 MiB and 100 paths, else exit 2 `unsupported-prove-history`): kind
  `affected-test`, `result` and `path` the test path, `line` 1, `blob_hash` the blob at
  `<revision>:<test>` (empty when Git holds no blob there), `authority` `affected-selection`,
  and the claim `affected test for C` where `C` is the selection witness's dirty path, the
  changed path that reached the test's unit. Rows follow plan order and are deduplicated by
  test path. The falsifier is `test-kills-mutant` when the test path ends in `_test.go` and
  `C` is neither a `_test.go` file nor the test itself; otherwise `none`, because mutating a
  test proves nothing about the code and the runner mutates Go only. Under `--mutate` such a
  row is judged as FPK-V0-014 judges a same-package row, with `C` as the changed file and the
  test's own package as the package under test, and the mutants of both the affected-test rows
  and the packet's same-package rows are confined to the lines the range changed in `C` (the
  new-side ranges of `git diff -U0 --no-renames --no-ext-diff --no-textconv BASE HEAD -- '*.go'`,
  bounded at 16 MiB else exit 2 `unsupported-prove-history`, a pure deletion naming the line
  before it), so a survivor is a survivor inside the change and a file whose changed lines hold
  nothing mutable is `NOT_RUN`. A `C` the diff gives no hunk (a mode-only change, a header the
  parser does not read) admits no mutant and its rows are `NOT_RUN`, never judged on the whole
  file; a test whose enclosing Go module is not `C`'s is `NOT_RUN`, since the runner runs one
  module. Without `--mutate` every such row is `NOT_RUN`. Each row keeps its own verdict and
  is tallied per row in `proof.counts`, but the falsifier answers "does this test pin the
  change" per test while the packet asks "is the change covered" per path, so affected-test
  rows are summarised per changed path, not per row (decision 0018). `proof.affected.paths`
  MUST carry one entry per changed path `C` that at least one affected-test row claims, sorted
  by path: `path` `C`; `covered_by`, the sorted test paths whose row is `PASS`; `reached_not_covering`,
  the number of `FAIL` rows for `C`; `not_run`, the number of `NOT_RUN` rows for `C`. In
  `proven_results`, `failed_results`, and `unproven_results` each such entry counts once, beside
  the packet's results: proven when `covered_by` is non-empty, failed when it is empty and
  `reached_not_covering` is positive, unproven otherwise. Packet rows keep the per-row counting
  of FPK-V0-005, and `state` follows the summary as elsewhere. `proof.affected` MUST also carry
  the plan's `graph_digest`, `scope`, the number of affected-test rows as `selected`, and the
  number of plan unknowns as `unknown`; a `scope` of `UNKNOWN`
  means the selection may be incomplete and the reader MUST NOT treat absent rows as evidence
  of no coverage. A graph the selector cannot build is exit 2 `unsupported-prove-affected`. The
  bracket, determinism, read-only, and failure rules of FPK-V0-001, 006, 007, and 008 apply
  unchanged; the selector reads the worktree's source text, so a dirty path's row is
  `NOT_RUN` as elsewhere.
- **FPK-V0-015:** `prove --cem ... --attest` MUST emit, instead of the document, an in-toto
  Statement v1 in canonical JSON (keys sorted at every level, no whitespace, no trailing newline
  inside the statement) whose `predicateType` is
  `https://corvint-context.dev/attestation/falsifiable-packet/0`, whose `predicate` is the document
  unchanged, and whose `subject` is the map path with the `sha256` of the bytes the verifier read
  followed by `git:` plus the document's `revision` with a `gitCommit` digest. `--attest-key PEM`
  implies `--attest` and wraps the statement in a DSSE envelope with `payloadType`
  `application/vnd.in-toto+json` and one Ed25519 signature over the pre-authentication encoding,
  `keyid` being the lowercase hex SHA-256 of the raw public key. The key is read only from a
  PKCS#8 `PRIVATE KEY` PEM of at most 16 KiB with nothing but whitespace before or after its one
  block (a second, even malformed, BEGIN line included), and is never generated
  or written; a missing or malformed key is exit 2 `attest-key-unavailable`, and an empty
  `--attest-key=` is `invalid-arguments`, each with nothing on stdout. `internal/attest.Verify`
  returns the statement for a matching public key and rejects any other envelope member, and a
  public key that is not 32 bytes is an error rather than a panic. It also rejects, in the envelope
  and in its signature object, a repeated member name and a member name that differs from
  `payload`, `payloadType`, `signatures`, `sig`, or `keyid` only in case, because a decoder that
  keeps the last duplicate or folds case would read a different envelope than a strict DSSE reader;
  any other signature-object member is ignored. `internal/attest.Statement`
  refuses any non-whitespace byte after the document, a stray `}` or `]` included, and a document
  that repeats a member name at any depth or is not valid UTF-8, because a last-wins or replacing
  decoder would sign a predicate that is not the document unchanged. The canonical encoder, shared
  with FPK-V0-030, refuses any string or member name that is not valid UTF-8, a subject name
  included, rather than writing U+FFFD, so two distinct byte names cannot sign as one. An
  attestation is not a verdict: it binds who produced the document to what it said.
- **FPK-V0-016:** `prove` MUST report `proof.ledger`, the repository's falsification rate over the
  `proof` rows the self-observation ledger retains: `proofs`, `judged` (rows whose verdict is
  `PASS` or `FAIL`; `NOT_RUN` and `none` rows are not judgments), `failed` (`FAIL`), and `rate`
  as `failed/judged`. The block is read from the ledger, never written by `prove`, carries no
  authority, and MUST be absent when the ledger has no `proof` row or cannot be read. The only
  writer of `proof` rows is `corvint prove-observe` (SOL-V0-009), which reads one
  `falsifiable-packet/0` document on stdin, bounded at 8 MiB, recounts `proof.rows`, and appends
  one row of kind `proof` carrying only those counts; any other document, an oversized one, a row
  with an unknown falsifier or verdict, or a `proof.counts` that disagrees with the rows is exit 2
  `invalid-proof-document` with nothing on stdout and no ledger change. Triage and `prove` skip a
  ledger row whose counts are malformed, and `none` never contributes to `judged`.
  Accepted amendment (AT-06, decision 0052): checkpoint mode omits `proof.ledger` entirely, never
  emitting it even absent. `runProve` today sets `receipt.Proof.Ledger` unconditionally for every
  mode, checkpoint included (`cmd/corvint/prove.go:474-477@77b9f0ff`), but this clause's own byte-identity
  requirement (`repository.dirty_paths_sha256`) fixes checkpoint output to the tree, HEAD, dirty
  set, and the checkpoint document alone; `prove-observe` can append a ledger row, changing
  `proof.ledger`, without changing any of those four (`cmd/corvint/prove_observe.go:45-66@2adc0365`,
  `cmd/corvint/prove_observe.go:118-126@8a1965ef`), so an emitted ledger member would make the checkpoint document vary on an input this
  proposal does not list as fixed. `runProve` MUST skip the ledger assignment when
  `options.proveMode == "checkpoint"`. End of accepted amendment.
- **FPK-V0-019:** `test-kills-mutant` is also assigned to a test row whose path names a pytest
  file (`test_*.py` or `*_test.py`) when the claimed changed path `C` ends in `.py`, is not
  itself such a test, and is not the test (decision 0020). This covers impact's same-package
  rows (`same-package test for C`) and change mode's affected-test rows, and narrows the
  "other languages get `none`" of FPK-V0-014 and FPK-V0-017 to languages other than Go and
  Python; a test row of one language claiming a changed path of another stays `none`. Under
  `--mutate` the runner in `internal/liveverify/pymutate` judges the row on the same exported
  copy, sandbox, per-row and invocation budgets, drift bracket, and line confinement as
  FPK-V0-014 and FPK-V0-017, the hunk spans of change mode being read with the pathspec
  `'*.go' '*.py'`. It resolves `python3` on `PATH` and proves inside the sandbox that it
  imports pytest, else the row is `NOT_RUN` with `detail` `python3 is not available` or
  `pytest is not available to python3`; reads the `def test*` functions the cited file
  defines (`test file declares no tests: T` when none); and runs
  `python3 -m pytest -q -x --no-header -p no:cacheprovider T -k "<names joined by or>"` from
  the export root with `PYTHONDONTWRITEBYTECODE=1`, `PYTHONPATH` the export root,
  `PYTHONHASHSEED=0`, `PYTEST_ADDOPTS` empty, and temporary files in the scratch directory,
  once as a baseline. It then applies at most eight single-operator mutants of `C`, each one
  token rewrite confined to one line, produced in Go over the repository's Python lexer with
  no Python run, in source order, stopping at the first kill: `negate-condition` (an `if`,
  `elif`, or `while` condition wrapped in `not (...)`), `swap-binary` (`+`/`-`, `*`/`/`,
  `==`/`!=`, `<`/`>=`, `>`/`<=`, `and`/`or`, only after an operand, so a sign, a star
  argument, and a positional-only marker are untouched), `replace-literal` (a number or plain
  string literal in a `return`), and `delete-statement` (one physical line holding one
  expression statement, augmented assignment, or attribute or subscript assignment; never a
  keyword statement, definition, block opener, bare-name binding, or tuple target). Before
  each run Python is asked to `ast.parse` the mutant inside the sandbox: a mutant that does
  not parse, or that pytest cannot collect (exit 2 or 5), is neither killed nor a survivor
  and is counted in `detail` as `did not parse or collect`; a run the per-run timeout ends is
  counted as `timed out`, because pytest has no timeout of its own to report; only pytest
  exit 1 is a kill and only exit 0 a survivor, and any other status is a runner error (exit 2
  `unsupported-prove-mutation`). The row is `PASS`, `FAIL`, or `NOT_RUN` exactly as
  FPK-V0-014 maps `KILLED`, `SURVIVED`, and the rest, `NOT_RUN` carrying `module
  dependencies unavailable offline` when the baseline reports `ModuleNotFoundError` or
  `ImportError`, `baseline tests do not collect: T`, `baseline tests fail before mutation:
  T`, `baseline tests exceed the per-run timeout: T`, `pytest refuses the project
  configuration: T` when the baseline is a usage error (exit 4: a configuration or
  `minversion` this host's pytest cannot read), `changed file does not tokenize: C`,
  `no mutable statements [inside the changed lines of] C`, or `none of N mutants of C could
  be asked`. Impact mode emits same-package test rows for Go paths only and change mode's
  range impact still requires a slash-qualified Go module, so today a Python claim reaches
  this runner through `prove --base` in a repository that also holds a Go module; both are
  recorded as open in decision 0020, not widened here.
- **FPK-V0-020:** (accepted 2026-09-04 for AT-06 by decision 0052) A checkpoint document is caller-owned, version `corvint-checkpoint/0`, canonical JSON, at
  most 256 KiB: `task` (visible intent text); `obligations` (spec/requirement ids the caller names);
  `repository` `{object_format, base_commit, base_tree, dirty_paths_sha256}`; `handles`, at most
  256, each `{path, blob_hash, line?, kind?, id?, authority?, reason?}`, the shape `evidence`
  emits (`internal/contextindex/impact.go:358-360@fd0a67cc`) minus `confidence`, plus the result's
  `kind`/`id`, as FPK-V0-002's rows carry; `critical`, at most 256, selectors of the same shape
  naming handles that MUST survive; `unknowns` and `failed_approaches`, free text; `verification`,
  `[{command, observed_status, provenance}]`; and `provenance` `{receiptId?, packet_sha256?}`.
  Keys are snake_case with one compatibility exception: `receiptId` keeps the camelCase spelling of
  the accepted harness response key it mirrors (`internal/gokernel/harness.go:460@cc998e6b`); every other
  key, `packet_sha256` included, is snake_case. A `receiptId` is a request-identity hash, not proof
  of possession (`internal/gokernel/harness.go:446-450@a9b1d3f6`); a checkpoint MUST NOT claim it establishes
  that a handle was read. `repository.dirty_paths_sha256` is the lowercase hex SHA-256 of the
  canonical-JSON encoding of the sorted dirty path list `affected.DirtyPaths` returns
  (`internal/liveverify/affected/dirty.go:46@abebde5b` is the entry point; the deduplication and sort happen
  in `DecodeStatus` at `internal/liveverify/affected/dirty.go:195@a543bfa1` via `NormalizePaths`,
  `internal/liveverify/affected/select.go:319-332@eadff8fe`), by the same construction
  `internal/gokernel/repository.go:397-398@425ed3ff` and `internal/gokernel/repository.go:408@54fe2926` already take — cited as a construction
  precedent only, since that digest's input is gokernel's own status list, whereas this digest's
  input is the `affected.DirtyPaths` list the run already reads (`cmd/corvint/prove.go:507-509@6b81d3a5`). It hashes
  path names, never
  content, so a caller can compute a matching value and an equal dirty set is decidably equal.
  `repository.object_format` names the Git object format the document's
  `blob_hash` values were computed under, and is consumed rather than decorative: FPK-V0-021
  compares it against the object format of the index built at the current revision
  (`Index.ObjectFormat`, `internal/contextindex/index.go:209@1e49fe84`, set at `internal/contextindex/index.go:477-479@02852e4a`) and FPK-V0-024
  refuses a mismatch, because a sha256 `blob_hash` compared against a sha1 repository would
  otherwise read as a confident `blob-changed` for every handle. Schema validation MUST also check
  every `blob_hash` against that declared format's object-id shape, stated here rather than
  borrowed as a mechanism: exactly 40 lowercase hex digits under `sha1` and exactly 64 under
  `sha256`. That is the shape the index applies to Git's own output (`validObjectID`,
  `internal/contextindex/git.go:457-470@f2d5aceb`, called at `internal/contextindex/git.go:396@842ac930`), cited as precedent only: the
  predicate is unexported, and this clause requires no change to it or to any other code outside
  the checkpoint branch. A value of the wrong shape is `invalid-checkpoint-document`, refused
  before any handle is judged. `handles` MUST be total over its paths: two entries sharing a `path`
  but carrying different `blob_hash` values are `invalid-checkpoint-document`, since a path pinned
  twice at two contents has no single verdict to report; two byte-identical entries are collapsed
  to one entry yielding one verdict, and are not a refusal. The 256-entry bound applies
  independently to `handles` and to `critical`, each counted on the entries as written, before that
  collapse: a 257-entry `handles` whose 257th entry duplicates another byte-identically is
  `checkpoint-bound-exceeded`, never accepted as 256 post-collapse, so the bound is decidable from
  the document's bytes without first computing the collapse.
  A `critical` selector's `path` need not
  occur as the `path` of any `handles` entry: such a selector has no handle verdict to be gated on,
  so FPK-V0-022 answers it with `handle-undeclared` and the document is never refused for it.
  Corvint defines no writer for this document in V0; the caller composes and
  keeps the only copy.
- **FPK-V0-021:** (accepted 2026-09-04 for AT-06 by decision 0052) `--checkpoint FILE` accepts an absolute or relative path outside the repository root,
  unlike `--cem` (`cmd/corvint/prove_attest_cem.go:184-191@2adcddfb`), and is read through `readBoundedFile`
  (`cmd/corvint/prove.go:942@9388fe37`) at 256 KiB. FILE itself MUST be an unchanged regular file;
  symlinks, directories, FIFOs, devices, and an identity change while opening are unreadable.
  `proveWrappedCommand` (`cmd/corvint/prove.go:348-367@8ae224e9`) dispatches on
  `--checkpoint` as a third branch beside `--task` and `--cem`. The flag MUST be mutually exclusive
  with `--task`, `--cem`, `--base`, and `--mutate`; combining it with any of them is
  `invalid-arguments`. Two paths reach that one code, and which one runs is argv-order dependent.
  `proveWrappedCommand` is a single loop over the arguments in argv order that returns on the first
  matching argument (`cmd/corvint/prove.go:348-367@8ae224e9`); the `--checkpoint` test MUST sit in that same loop body
  beside the `--task` and `--cem` tests, so the FIRST of the three flags to appear in argv selects
  the branch. Before this branch the loop scanned every element of `rest` with no `--` handling,
  while `parseImpactArguments` treats a literal `--` as ending flag recognition and
  reading every later token positionally (`cmd/corvint/main.go:299-306@bcf2181e`), which is how positional
  impact paths are preserved (FPK-V0-010). Wrapper flag recognition, including the
  `--checkpoint` test, MUST therefore stop scanning at the first `--` (the loop breaks there,
  `cmd/corvint/prove.go:353-355@0bd3675a`), so a tracked path literally
  named `--checkpoint=foo.go` given after `--` is read as a positional impact path, never as the
  checkpoint flag. The `=` form `--checkpoint=FILE` is accepted: the `--checkpoint` test MUST match it
  by prefix exactly as the `--task=` and `--cem=` tests do (`cmd/corvint/prove.go:356-364@474c00a2`), so a spelling
  accepted everywhere else in `prove` is not silently rejected here.
  Its `"checkpoint"` result MUST be consumed at its own early-return dispatch site
  (`cmd/corvint/prove.go:292-295@f89c7c4b`), like the one `cem` uses (`cmd/corvint/prove.go:296-299@5ee2e164`), before `withoutMutateFlag` lifts `--mutate` out
  (`cmd/corvint/prove.go:300@2b332060`): the branch then sees the arguments as written and owns its own refusal of
  `--checkpoint --mutate`, rather than reaching the existing impact-mode `--mutate` check
  (`cmd/corvint/prove.go:320-321@13b30987`), which gives the same `invalid-arguments` code under another message.
  `prove --checkpoint FILE --task T` therefore reaches the checkpoint branch, and
  `prove --task T --checkpoint FILE` rewrites to `query` (`parseProveInvocation`,
  `cmd/corvint/prove.go:300-304@2f9897c1`) and is refused by the query argument parser, whose allowlist admits only
  `--task`, `--limit`, and `--budget-bytes` and refuses everything else as `invalid-arguments`
  (`parseQueryArgumentsForPlatform`, `cmd/corvint/main.go:226-229@228dfda4`; `argumentError`, `cmd/corvint/main.go:74-75@9e55e110`).
  The requirement is therefore stated on what is observable: `--checkpoint` with `--task` MUST
  exit 2 with `invalid-arguments` in EITHER argv order. Which parser refuses is a property of the
  ordering — `--task` first is refused by the query allowlist (`cmd/corvint/main.go:227-228@66cf57ce`) without the
  checkpoint branch running at all, and `--checkpoint` first by the checkpoint branch's own
  parser, `parseProveCheckpointArguments`, which admits only `--checkpoint` and refuses every
  other flag as unrecognized (`cmd/corvint/prove_checkpoint.go:67-86@1d21b973`) — but the refusal is
  total, so no ordering admits the combination. That parser classifies the next token before
  consuming it as a value (`argparseOptionLike`, GPK-V0-064, decision 0173): a flag-looking
  checkpoint filename such as `--task` is refused with `argument --checkpoint: expected one
  argument` in the bare form, and must be written inline as `--checkpoint=--task` to name that
  literal file.
  `--checkpoint` combined with `--base` or `--mutate`, in either order,
  reaches the checkpoint parser, because neither is a wrapper flag, and is refused there; with
  `--cem` written first, the CEM parser's unrecognized-flag branch refuses `--checkpoint`
  (`cmd/corvint/prove.go:396-397@f316ecd0`) with the same code. Before this branch, `prove --checkpoint FILE` rewrote to
  `impact` and was refused by `parseImpactArguments` (`cmd/corvint/main.go:415-416@6955320f`); it now
  reaches the checkpoint branch, which is the accepted invocation. The parsers are distinct —
  `--budget-bytes` is accepted by the query parser and rejected by the checkpoint parser — so the
  requirement is stated on the code, not on one shared parser. `prove --checkpoint FILE` MUST be read-only under FPK-V0-001: no
  self-observation append, no ledger write. The read bracket of FPK-V0-006/007 MUST apply
  unchanged, and the checkpoint compile function (`compileCheckpointProof`, `cmd/corvint/prove.go:592@01d34b22`) MUST
  implement it itself, since the early-return
  dispatch (`cmd/corvint/prove.go:511-513@acc310b7`) leaves the surrounding function's own closing check unreached: the
  dirty set is read before compilation with the semantics of `cmd/corvint/prove.go:507-510@10e13ff1`, and after every
  verdict is decided the function re-reads the dirty set and the HEAD commit plus its immutable tree, refusing
  `unsupported-prove-drift` if any has moved and all were read successfully; if a closing
  read instead FAILS, the refusal is that read's own code — `unsupported-prove-history` for the
  status re-read, `unsupported-prove-revision` for the tree re-read (FPK-V0-024) — never
  `unsupported-prove-drift`, which asserts a difference, not a read failure. The closing re-read
  is `checkpointClosingRead` (`cmd/corvint/prove_checkpoint.go:515-528@9b0dd02e`): it re-reads the dirty
  set, then re-resolves the HEAD commit and its tree through `checkpointRevision` (`cmd/corvint/prove_checkpoint.go:496-513@84f318b1`).
  `compileCEMProof` brackets the tree and dirty set the same way (`cmd/corvint/prove.go:818-846@9d171913`) but not the
  commit. After the closing read, `compileCheckpointProof` also refuses `unsupported-prove-drift`
  when `contextindex.Build` pinned a commit or tree other than the opening read's
  (`cmd/corvint/prove.go:634-638@7e436d2a`), because `Build` can observe a later stable revision inside the bracket. Every handle MUST be
  judged against the CURRENT snapshot, never the checkpoint's own `base_commit`/`base_tree`.
  After the index is built and before any handle is judged or the checkpoint branch's own blob
  reads, `repository.object_format` MUST equal
  the object format of the index built at the current revision (`Index.ObjectFormat`,
  `internal/contextindex/index.go:209@1e49fe84`, `internal/contextindex/index.go:477-479@02852e4a`); a mismatch is refused under FPK-V0-024 as
  `object-format-mismatch`. This is decided after `Build`, not before every `cat-file` call in the
  run: `Build` itself reads blobs through `cat-file --batch` while pinning sources
  (`internal/contextindex/git.go:512-522@875d1117`, `internal/contextindex/index.go:457-459@16cf1c1f`, `internal/contextindex/index.go:1411-1413@b9840036`), so `Index.ObjectFormat`
  is not known until that call has already made its own `cat-file` calls. It is the unframable
  class one level up: as an LF-bearing path cannot
  be framed for Git at all, a document whose `blob_hash` values were computed under another object
  format is not comparable to this repository at all, and no per-handle verdict can express that,
  so the document is refused rather than judged. Every
  handle MUST receive exactly one verdict, decided by this total order over all inputs, first match
  wins: (1) `unframable` — the path is not normalized
  (`internal/contextindex/impact.go:345-355@00d37054`) or the LF-delimited `cat-file --batch` protocol cannot
  carry it (`cmd/corvint/prove.go:1602-1606@7d324499`), so it is never sent to Git at all; (2) `unsupported` — the current
  tree lists the path at a non-blob mode, or lists it as a blob for which the path has no entry in
  `Index.Sources` (`internal/contextindex/index.go:208-214@aa5d6289`, `internal/contextindex/index.go:476-479@478ddf23`), whether because its kind
  is unadmitted or because an index exclusion removed it — the named field, not
  `Index.Exclusions`, decides, so a path that is both dirty and excluded is settled here — ordered
  ahead of every absence test so an omission from
  the index is never mistaken for a deletion; (3) `dirty` — the path is in the current dirty set,
  which includes untracked and worktree-deleted names, content unresolved; (4) `path-deleted` —
  the current tree lists no entry for the path AND `readCitedBlobs` returned no entry for it. That
  absence is how a `missing` object MUST be detected: the batch is addressed `<revision>:<path>`
  (`cmd/corvint/prove.go:1783-1793@16a04afd`), so a path Git does not hold is simply left out of the returned map. The
  verdict branch MUST decide from that map absence rather than re-parsing Git's batch framing;
  `parseCatFileBatch` matches the exact echoed `<revision>:<path> missing` header, including a path
  bearing a space (`cmd/corvint/prove.go:1834-1840@aa98f5c8`);
  (5) `blob-changed` — the current blob differs from `blob_hash`;
  (6) `unchanged` — the current blob equals `blob_hash`. Directories and gitlinks are decided at
  step (2) by mode: `git ls-tree --full-tree <tree> -- <path>` reports `040000` for a directory and
  `160000` for a gitlink, neither of which the index admits (`internal/contextindex/git.go:389-397@2854280e`), so
  both MUST be `unsupported` whatever `cat-file` then returns — a tree object for the directory, a
  commit object or `missing` for the gitlink — and only `100644`, `100755`, and `120000` continue
  past step (2). `path-deleted` means the path is not present at the current tree, never that it
  once existed. Independent flags accompany, never replace, these verdicts: `tree-moved` (current
  tree differs from `base_tree`); `commit-moved` (current commit differs from `base_commit`; a same
  tree is reported as that fact alone, never attributed to a merge, revert, amend, or rebase);
  `dirty-set-moved` (FPK-V0-020's digest, recomputed by that one construction over the current
  dirty path list, differs from the checkpoint's `dirty_paths_sha256`; both hash names only,
  never content, the digest being taken over the path list alone
  (`internal/gokernel/repository.go:397-398@425ed3ff`), which collects status path names (`internal/gokernel/repository.go:229@e2af9ea7`), so
  equal dirty sets yield equal digests and the flag is not set);
  `authority-changed` (the current blob of a `critical` handle differs from `blob_hash` AND
  the handle's authority class — re-derived at the CURRENT snapshot from the live classifier
  `documentResult` uses: `Record.Kind == "instructions"` → `project-instructions`; `"decision"`
  → `accepted-decision`/`non-binding-decision`; else `repository-spec`/`accepted-spec`
  (`internal/contextindex/impact.go:367-380@6d7677dd`) — is instruction- or spec-authority; never keyed on
  the checkpoint's own `authority?` field, per AGENTS.md invariant 3. When present, that field is
  echoed as `claimed_authority`; its absence does not change the flag, which is computed only from
  the live class).
  The response document this branch emits is its own shape, not `proveReceipt` — it carries no
  `packet`, `profile`, `rows`, `mutates`, or `proof` member, since none of those is produced by a
  Git-tree-and-`cat-file` judgment over caller-declared handles. The attributed caller fields
  `task`, `unknowns`, `failed_approaches`, `verification`, and `obligations` are also emitted exactly as
  FPK-V0-023 requires; they carry no evidence authority. `obligations_authority` is always the
  literal `"caller-reported-unverified"`. No member outside those and the fields
  named here is emitted: `tool` (`"prove"`), `ok` (`true` for any run that reaches output, since
  every non-refusal case is a judgment, never a failure, on FPK-V0-022's authority), `revision`
  (the current HEAD tree, the same value FPK-V0-024's drift check compares), `handles` (one entry
  per input handle after FPK-V0-020's byte-identical collapse, ordered by ascending `path`, each
  `{path, verdict, tree_moved?, commit_moved?, dirty_set_moved?, authority_changed?,
  claimed_authority?}` — the four flags and `claimed_authority` are omitted, never emitted `false`
  or empty, when the condition that sets them does not hold, matching the `?` optionality
  convention FPK-V0-020 already uses for the input document), and `critical_missing` (FPK-V0-022,
  one entry per selector so reported, `{path, kind?, id?, reason, recovery}`, ordered the same way
  as `handles`, then by the selector's own `path`/`kind`/`id` for two selectors on one handle). A
  rehydrated row selectors match (FPK-V0-022) is not itself a top-level member: it is a value under
  its handle's `handles` entry, keyed `rows` on that entry, in the shape `query`/`impact` rows
  carry, present only on a `critical` handle that is eligible and matched at least one row. This
  wire is new to V0 and this requirement is its only source; the checkpoint tests named in the
  traceability table pin it. It is stated here because FPK-V0-023, FPK-V0-024, and FPK-V0-026 each assert
  facts about "the document" that presuppose a member list to be byte-identical or absent over.
- **FPK-V0-022:** (accepted 2026-09-04 for AT-06 by decision 0052) Selector matching MUST be gated on the handle's FPK-V0-021 verdict, before any row
  lookup is attempted. Only a handle whose verdict is `unchanged` or `blob-changed` is eligible:
  the path is admitted and its current content is the committed blob the evidence rows describe. A
  handle whose verdict is `dirty`, `unsupported`, `unframable`, or `path-deleted` MUST place every
  selector on it into `critical_missing` with that handle verdict as the typed reason, and MUST NOT
  look up a row for it. A `critical` selector whose `path` occurs in no `handles` entry
  (FPK-V0-020) MUST likewise be reported `critical_missing`, with the typed reason
  `handle-undeclared` and no row lookup: there is no handle verdict to gate it, and matching it
  against current rows would rehydrate bytes for a path the checkpoint never pinned. It is not a
  refusal — the document is caller-owned and the refusal list of FPK-V0-024 stays closed. The
  typed reasons are therefore exactly six: the four ineligible handle verdicts, `handle-undeclared`,
  and `selector-unresolved` below. The gate is load-bearing: current evidence rows are compiled from the
  committed tree (`internal/contextindex/impact.go:75-81@d2231c48`), so a dirty or unadmitted path still has
  matching committed rows, and an ungated match would rehydrate committed bytes as if they were
  what the agent will read — the same reason `judgeHistory` declines to judge a dirty path at all
  (`cmd/corvint/prove.go:1133-1139@72ba1935`). For an eligible handle, selectors MUST be matched by identity,
  not by position. Checkpoint lookup compiles unranked results from the current index using the
  existing direct-path, document, feature/scenario, symbol, test, reverse-import and reference
  result constructors. It uses only eligible critical paths, never the stored task prose, and
  does not apply a query/impact receipt's result-count cap; constructor evidence bounds remain
  unchanged. Matching walks RESULTS, not rows: `kind` and `id` are members of the
  enclosing result (`internal/contextindex/impact.go:172-175@ca66ec11`, `internal/contextindex/impact.go:195-200@7f665885`, `internal/contextindex/impact.go:401-403@92896c26`), never of an evidence
  row, which carries exactly `path`, `line`, `blob_hash`, `reason`, `confidence`, and `authority`
  (`evidence`, `internal/contextindex/impact.go:358-360@fd0a67cc`). A selector carrying `kind` and `id` therefore selects the
  results whose `kind` and `id` equal its own, and within them the evidence rows at the selector's
  `path`; a selector carrying neither selects the evidence rows at that `path` in every result.
  Selecting by the result's identity and the row's `path` matters because a result's evidence rows
  need not sit at the result's own id — `documentResult` emits rows whose `path` is a referenced
  file (`internal/contextindex/impact.go:397-399@25e804e8`). Identity does not single out one
  row — `impact` emits a reference row per changed path, so one result identity can supply a
  row at the same `path` more than once (`internal/contextindex/impact.go:204-212@4e3f46b9`) — so a match
  is the whole set of matching rows, never "the row". Byte-identical matched rows collapse to one,
  as FPK-V0-020's byte-identical `handles` entries do: `documentResult` emits one row per
  `references` entry (`internal/contextindex/impact.go:392-399@2a45c822`), so a `references` list naming one path twice yields
  two rows equal in all six members, and they rehydrate as one row.
  All of them MUST be rehydrated, in the shape `query`/`impact` rows carry,
  ordered by ascending `line`, then lexicographic `reason`, then lexicographic `blob_hash`, then
  lexicographic `confidence`, then lexicographic `authority`: those six members are all `evidence`
  emits, the path is fixed by the match, and two matching
  rows that agree on `line`, `reason`, and `blob_hash` can still differ in the last two, so the
  order is total over the emitted members only with all five keys. A
  selector's `line` MUST be advisory: it is refreshed from the first row of that order, and its
  movement is never by itself a miss. A selector on an eligible handle that matches no row MUST be
  reported `critical_missing` with reason `selector-unresolved`: the path is still admitted but no
  current row carries the selector's `kind` and `id`, the case of a symbol renamed or removed
  inside a file that is still admitted. `selector-unresolved` MUST NOT change the handle's own
  FPK-V0-021 verdict. Every `critical_missing` entry MUST carry a recovery operation (for example,
  re-run `query`/`impact` at the current revision for the handle's path). A document whose
  `handles` or `critical` exceeds 256 entries MUST be refused before any verdict is computed, never
  truncated to the bound.
- **FPK-V0-023:** (accepted 2026-09-04 for AT-06 by decision 0052) Stored `verification` rows are echoed as attributed observations of a prior run —
  original `command`, `observed_status`, `provenance` — and MUST NOT be re-executed or treated as
  current. Stored `task`, `unknowns`, and `failed_approaches` prose is echoed verbatim and MUST NOT
  be interpreted, summarized, or promoted to evidence: it carries no authority (AGENTS.md invariant
  3), consistent with never claiming a supplied handle was read, remembered, or causally used
  (`PCCO-V0-009`, `docs/specs/proof-carrying-context-optimization-v0.md:95-97`).
  The 2026-09-06 user-authorized experimental audit follow-up also echoes the original
  `obligations` array exactly, preserving order, duplicates, empty strings, and an empty array,
  with `obligations_authority: "caller-reported-unverified"`. The strings are inert caller input,
  never executable commands, accepted requirement IDs, or verified completion. Neither an empty
  array, caller-reported verification, nor current, changed, or deleted handles may remove,
  complete, or reclassify obligations. `ok: true` continues to mean the judgment reached output,
  not task completion. This transport fixture does not qualify fresh-agent recovery: AT-06 remains
  experimental and open pending its separate interruption/outcome evaluation.
- **FPK-V0-024:** (accepted 2026-09-04 for AT-06 by decision 0052) `prove --checkpoint` MUST refuse — exit 2, a typed error, nothing on stdout, mirroring
  the refusal rule of `FPK-V0-008` — in exactly these cases and no others, each bearing the exact
  `code` named here, so a refusal carrying any other code is a defect: (1) the file is missing,
  unreadable, non-regular, or changes identity while opening, or exceeds the 256 KiB bound of
  `readBoundedFile` (`cmd/corvint/prove_checkpoint.go:96-100@afdb91b4`) —
  `unreadable-checkpoint-document`, the branch supplying its own message for the over-bound case
  rather than surfacing that helper's, which names a CEM `map` (`cmd/corvint/prove.go:942-955@d4769336`);
  (2) the document fails schema or canonical-JSON validation —
  `invalid-checkpoint-document`, which includes a `blob_hash` whose shape is not the one
  `repository.object_format` declares and two `handles` entries sharing a `path` with different
  `blob_hash` values (FPK-V0-020); (3) `handles` or `critical` exceeds 256 entries (FPK-V0-022) —
  `checkpoint-bound-exceeded`; (4) the flag is combined with `--task`, `--cem`, `--base`, or
  `--mutate` (FPK-V0-021) — `invalid-arguments`; (5) any other argument the parser rejects, an
  unrecognized flag, a missing flag value, or a malformed integer among them — also
  `invalid-arguments`, the single code `argumentError` gives every such refusal
  (`cmd/corvint/main.go:74-75@9e55e110`), whichever parser produces it: the query allowlist when `--task` is
  the first wrapper flag (`cmd/corvint/main.go:227-228@66cf57ce`), the CEM parser when `--cem` is
  (`cmd/corvint/prove.go:396-397@f316ecd0`), and `parseProveCheckpointArguments` otherwise
  (`cmd/corvint/prove_checkpoint.go:69-70@191ad3c1`);
  `--checkpoint` introduces no parser vocabulary of its
  own, and this case is enumerated so the closed list is not read as excluding ordinary parse
  failures; (6) the `git` executable is unavailable — `unsupported-prove-revision`
  (`cmd/corvint/prove.go:503-506@429f1ce3`), decided in `compileProof` before it dispatches to any mode
  compile function (`cmd/corvint/prove.go:511-516@75d9bf23`), so it is settled before the checkpoint compile function is ever
  entered; (7) the worktree status cannot be read — `unsupported-prove-history`
  (`cmd/corvint/prove.go:507-510@10e13ff1`), likewise decided in `compileProof` before dispatch; (8) the repository has
  no resolvable HEAD tree — `unsupported-prove-revision` (`checkpointRevision`,
  `cmd/corvint/prove_checkpoint.go:496-513@84f318b1`, refusing at `cmd/corvint/prove_checkpoint.go:506-507@c3ca96ba`), a separate case from (6): `compileProof` itself never calls `proveTreeRevision`
  before dispatch (the shared closing check that does, `cmd/corvint/prove.go:559@4667fe9f`, sits inside the
  non-checkpoint path only), so the checkpoint compile function MUST first resolve the HEAD commit
  and then its immutable tree, before `readBoundedFile`, to settle this case ahead of the file read; (9) the tree
  cannot be listed at HEAD, or the `git ls-tree` output passes the same 64 MiB bound
  `readTreeEntries`
  already applies to a full-tree listing (`maxTreeBytes`, `internal/contextindex/git.go:28@0e8ce572`,
  `internal/contextindex/git.go:370@33f5b343`)
  — `unsupported-prove-tree`, a code this requirement added because no earlier `prove` code named
  a tree-listing failure; the checkpoint compile function performs this bounded, whole-tree `git
  ls-tree -r -t -z --full-tree <tree>` read once (`readCheckpointTree`,
  `cmd/corvint/prove_checkpoint.go:308-313@8edb003a`, bounded by `checkpointTreeByteBound`, `cmd/corvint/prove_checkpoint.go:24@1dba213a`), immediately after resolving the HEAD tree in case (8)
  and
  before the checkpoint file is read, so every handle's directory/gitlink/blob classification in
  FPK-V0-021 step (2) is answered from this one map rather than a call per handle; (10) the index
  cannot be built — the
  `contextindex.Build` error raised by the checkpoint compile function's own call to
  `contextindex.Build` (`internal/contextindex/index.go:277@1cafb447`) and surfaced with its own code
  unchanged, which `--checkpoint` MUST NOT re-code, so the exact expected code is
  whatever `Build` returns for that repository. A `Build` error can carry no code at all
  (`internal/contextindex/git.go:383-384@3e48e4c5`), and `emitError` deliberately prints such an error without
  a `code` member (`cmd/corvint/main.go:1341-1343@432b3fe2`); because this clause requires every checkpoint
  refusal to bear a code, a code-less `Build` error MUST be reported as `unsupported-prove-index`,
  a checkpoint-only mapping that preserves the `Build` message verbatim as the refusal's `error`
  member — for an error carrying no `DRC-V0` diagnostic, which this refusal never does, the only
  members `emitError` writes are `code`, `error`, and `ok`
  (`cmd/corvint/main.go:1345-1349@b96186e4`), so there is no `reason` member on this wire — and MUST NOT
  change what plain `prove` emits for the same error. The mapping MUST construct a
  fresh `&gokernel.Error{Code: "unsupported-prove-index", Message: buildErr.Error()}` that does
  NOT wrap the `*contextindex.Error`: `emitError` prints without a `code` member for an error
  that unwraps to a code-less context error (`cmd/corvint/main.go:1327-1337@a7d2600f`), so wrapping to preserve the message
  would still emit an uncoded refusal, which this clause forbids. That mapping MUST live in the
  checkpoint branch's own compile function — `compileCheckpointProof` (`cmd/corvint/prove.go:592@01d34b22`), the sibling of
  `compileCEMProof` (`cmd/corvint/prove.go:813@e469a50d`) that `compileProof` dispatches to on the
  checkpoint mode (`cmd/corvint/prove.go:502@6d433de3`, `cmd/corvint/prove.go:511-513@acc310b7`) — between its
  index build and its return to `runProve` (`cmd/corvint/prove.go:605-615@5563dd2e`, `cmd/corvint/prove.go:463-471@33fabac8`). It MUST NOT be placed in
  `emitError` (`cmd/corvint/main.go:1335-1337@d2f5e1ec`), which plain `prove` shares, so plain `prove`'s
  stderr for the same code-less `Build` error stays byte-unchanged, which FPK-V0-026 requires as a
  named test; (11) `repository.object_format` differs from the object format of the index
  built at the current revision (FPK-V0-021) — `object-format-mismatch`, decided after the index
  build (10) and before the checkpoint's own blob reads (12), because every `blob_hash` in the
  document is then incomparable and an unrefused run would report a confident `blob-changed` for
  every handle. It is NOT decided before every `cat-file` call in the run: `Build` itself reads
  blobs
  through `git cat-file --batch` while pinning sources (`fetchBlobs`,
  `internal/contextindex/git.go:512-522@875d1117`,
  called from `pinCandidates`/`readResidualBlobs`, `internal/contextindex/index.go:457-459@16cf1c1f`, `internal/contextindex/index.go:1411-1413@b9840036`), so `Build` (case
  10) necessarily runs, and necessarily calls `cat-file`, before `Index.ObjectFormat` is even known
  to compare; (12) the `cat-file --batch` stream fails or passes its 64 MiB
  bound — `unsupported-prove-history` (`cmd/corvint/prove.go:1783-1791@0f63fb8a`); (13) the read bracket drifts,
  the HEAD commit, its tree, or dirty set having been read again successfully but found to differ from the
  opening read — `unsupported-prove-drift`, decided by the closing re-read FPK-V0-021 requires the
  checkpoint compile function to perform itself (`checkpointClosingRead`,
  `cmd/corvint/prove_checkpoint.go:515-528@9b0dd02e`), or the index `Build` having pinned a revision other
  than the opening read's (`cmd/corvint/prove.go:634-638@7e436d2a`). A closing re-read that FAILS outright, rather than succeeding and
  differing, is not case (13): it refuses with the code its own kind of read always carries —
  `unsupported-prove-history` for a failing closing status re-read, `unsupported-prove-revision`
  for a failing closing HEAD-tree re-read — and both are exempt from the fixed evaluation order
  below, since a closing read can only be attempted after every case it could otherwise be confused
  with has already passed; it is last by construction, not by numbered precedence.
  When more than one of cases (1)-(12) holds at once the run MUST refuse for the first in this fixed
  evaluation order, so a co-occurrence has one deterministic code: arguments (4, 5) before
  `git` availability (6), before the worktree status read (7), before HEAD-tree resolution (8),
  before the tree listing (9), before the file read (1), before schema and canonical-JSON validation
  (2), before the entry bounds (3), before the index build (10), before the object-format comparison
  (11), before the `cat-file` stream the handle verdicts need (12); the closing drift check (13) and
  the two closing-read-failure codes above are evaluated last in every case because they can only be
  decided after the reads they bracket. This order is the one `compileProof` already runs for its
  shared prefix (`cmd/corvint/prove.go:502@6d433de3`): `exec.LookPath("git")` (`cmd/corvint/prove.go:503-506@429f1ce3`) and
  `affected.DirtyPaths` (`cmd/corvint/prove.go:507-510@10e13ff1`) both decide before it dispatches to any mode compile function
  (`cmd/corvint/prove.go:511-516@75d9bf23`), so cases (6) and (7) are settled ahead of every case the checkpoint compile function
  itself decides; within that function, `checkpointRevision` as the first statement (`cmd/corvint/prove.go:593@a1870ebe`)
  settles case (8) ahead of the tree listing (9) and the file read (1). A
  document that is both over the entry bound and
  schema-invalid is therefore `invalid-checkpoint-document`, not
  `checkpoint-bound-exceeded`, and a schema-invalid document replayed in a repository with no
  resolvable HEAD refuses `unsupported-prove-revision` from case (8), not
  `invalid-checkpoint-document`.
  Output-transport failure is NOT a refusal and is not on that list. Encoding
  the receipt as canonical JSON and writing it to stdout both happen after every verdict is already
  decided, and either can fail: both exit 2 with `output-failed` (`cmd/corvint/prove.go:479-483@4c0799f8`,
  `cmd/corvint/prove.go:491-493@ea3220b5`). The "nothing on stdout" guarantee therefore binds refusals only, which return before
  the write is reached (`cmd/corvint/prove.go:467-470@21e8ca71`). `stdout.Write` can fail having already written part of
  the document (`cmd/corvint/prove.go:491-493@ea3220b5`), so an `output-failed` exit MAY leave a partial document on
  stdout; a consumer MUST read the exit status, never stdout emptiness, as the signal that no
  verdict was produced. The checkpoint branch MUST call `contextindex.Build`
  (`internal/contextindex/index.go:277@1cafb447`) directly, as prove's impact and change modes did until `IDX-SNAP-V0-020`, which
  left them `Build` only on a snapshot miss (`cmd/corvint/prove.go:1073@a1c6494d`, `cmd/corvint/index_snapshot.go:83@90129c09`), and MUST NOT read an on-disk index snapshot. `prove --task` is not the model
  for this: its project-operations query profile acquires through `standaloneQueryContext`
  (`cmd/corvint/prove.go:1059-1061@d473eb95`, `cmd/corvint/main.go:1201-1212@4e7cdb10`), which reaches `deferredSnapshotIndex` and `snapshotIndex`
  (`cmd/corvint/index_snapshot.go:71-72@5959c784`, `cmd/corvint/index_snapshot.go:57-58@123f0830`) at `cmd/corvint/main.go:1230-1231@c39315fe` and
  `cmd/corvint/harness_context.go:69-70@70282d2c` and only builds (`BuildQuery`, `internal/contextindex/index.go:396-398@9faff3e7`, called at
  `cmd/corvint/main.go:1226@e0e5c824`; `BuildEval`, `internal/contextindex/index.go:303-304@b1c33c59`, called at `cmd/corvint/harness_context.go:71@54018a6a`) on a miss — so plain
  `prove --task` does read the snapshot today, which a run of the binary confirms: with a
  populated `.corvint/index/`, the snapshot file's access time advances under `prove --task` and
  did not under `prove PATH...` before `IDX-SNAP-V0-020` (decision 0180) gave impact and change modes the same read. `IDX-SNAP-V0-008` (`docs/specs/index-snapshot-v0.md:79-81`)
  grants the snapshot read to two verbs, `corvint query` and the harness `user-prompt` event, but
  the grant is realized as two call sites inside functions `prove --task` also reaches, so the
  code's reach is wider than the clause's verb list. That overlap is an open IDX-SNAP question
  recorded in `docs/agent-memory/fixes.md`, not evidence that this proposal conforms: the
  checkpoint path's obligation here rests on the direct `Build` call, not on the verb grant.
  Extending that read to
  `prove --checkpoint` would widen an accepted spec without amending it, and the hit/miss
  byte-identity that spec requires (`IDX-SNAP-V0-006`, `docs/specs/index-snapshot-v0.md:73-74`)
  contradicts a `snapshot` cache-metadata member whose whole purpose is to differ on a hit. The
  member is therefore dropped from this proposal rather than defended: no cache metadata is
  emitted, a present or absent `.corvint/index/` changes nothing observable about the run, and
  FPK-V0-007's byte-identity holds over every member of the document with no exemption. Admitting
  this verb to the snapshot read stays open and would need its own labelled amendment to
  `docs/specs/index-snapshot-v0.md`; until one is accepted the checkpoint path builds the index
  fresh on every invocation, and a missing `.corvint/index/` is neither a refusal nor a reportable
  event. That "no snapshot read" obligation is not observable from the wire — `IDX-SNAP-V0-006`
  (`docs/specs/index-snapshot-v0.md:73-74`) requires a correct snapshot reader to emit the same
  bytes on a hit as on a miss, so byte-identity across a present and an absent `.corvint/index/`
  passes for a snapshot-reading implementation too. It MUST therefore be tested through a seam.
  The implementation exposes the package-level `var loadSnapshot = contextindex.LoadSnapshot` and
  `loadSnapshotDeferred = contextindex.LoadSnapshotDeferred`
  (`cmd/corvint/index_snapshot.go:17-20@8984cf3b`). The one call through the deferred seam is
  `deferredSnapshotIndex` (`cmd/corvint/index_snapshot.go:71-72@5959c784`). The seven current production calls through the snapshot seams are
  `snapshotIndex` (`cmd/corvint/index_snapshot.go:57-58@123f0830`), batch
  (`cmd/corvint/batch.go:149@dbef447a`), answerability
  (`cmd/corvint/answerability.go:94-95@6278a445`), surprise
  (`cmd/corvint/surprise.go:118-119@6278a445`), context lookup
  (`cmd/corvint/context_lookup.go:75-79@9f1de421`), local completion events
  (`cmd/corvint/local_completion_event.go:356-360@ea059984`), and the experimental host adapter
  (`cmd/corvint/host_adapter_experimental.go:43-48@b6d4dd7c`). The task-context path does not use
  that seam: it supplies `loadContextSnapshot` to `compileTaskContext`
  (`cmd/corvint/taskcontext.go:101-103@ccb78c62`), with that variable bound to
  `contextindex.LoadContextSnapshotDeferred` (`cmd/corvint/taskcontext.go:156-163@73708428`), which
  reaches the private `internal/contextindex.loadSnapshot`
  (`internal/contextindex/observed_build.go:42-49@d916a414`).
  `LoadSnapshot` is not the tree's only exported snapshot reader. The harness calls
  `contextindex.LoadEventSnapshot` and `contextindex.LoadEventSnapshotDeferred` directly (`cmd/corvint/harness_context.go:33-35@44bd361a`), and the
  index path calls `contextindex.ProbeSnapshot` directly
  (`cmd/corvint/index_snapshot.go:117-118@9a7d60f2`); none passes through the `cmd/corvint`
  `loadSnapshot` variables. Internally, `LoadEventSnapshot` reaches the private `loadSnapshot`
  (`internal/contextindex/snapshot.go:497-520@cd5ffb8c`), while `ProbeSnapshot` opens and validates
  the snapshot itself (`internal/contextindex/snapshot.go:456-495@47a72200`). A checkpoint compile
  function written to call either would therefore register zero calls on the dynamic seam. The
  load-bearing source guard scans every non-test Go file in `cmd/corvint`, rejects direct
  `LoadSnapshot` or `LoadSnapshotDeferred` references outside their seam bindings, and additionally rejects `LoadEventSnapshot`,
  `LoadEventSnapshotDeferred` or `ProbeSnapshot` references in `prove*` files
  (`cmd/corvint/prove_checkpoint_test.go:704-771@0c2b29a4`). A separate test substitutes a counting
  function for both dynamic seams and asserts zero calls during a `prove --checkpoint` run
  (`cmd/corvint/prove_checkpoint_test.go:666-681@62a5f8d6`), a secondary `LoadSnapshot`-specific
  check consistent with the guard but not a substitute for it. The
  byte-identity comparison stays as a secondary assertion.
  Index omission MUST NOT be evidence of deletion:
  `path-deleted` rests on the tree-entry and `git cat-file` presence checks of FPK-V0-021.
  `judgeHistory` is the precedent for judging against Git rather than the index, but only a
  partial one: it consults the `cat-file` map alone (`cmd/corvint/prove.go:1133-1144@c20da0d4`) and makes no
  `ls-tree` check, so the tree-entry half of the FPK-V0-021 test is new work here. Neither rests
  on index omission; `dirty_paths_sha256` itself hashes path names only, not content
  (`internal/gokernel/repository.go:398@8febb932`).
- **FPK-V0-025:** (accepted 2026-09-04 for AT-06 by decision 0052) This clause activates no SESSION-V0 lifecycle: SESSION-V0-001 through SESSION-V0-016
  stay deferred and unimplemented (`docs/specs/session-context-dividend-v0.md:6`, `:29`), and
  `prove --checkpoint` MUST NOT persist, cache, or index a checkpoint document server-side. The
  caller's file is the only copy; Corvint reads it once per invocation and retains nothing after exit.
- **FPK-V0-026:** (accepted 2026-09-04 for AT-06 by decision 0052) Before promotion this slice MUST demonstrate each of the following as a test.
  Determinism: a fresh process resuming a pinned fixture reproduces the verdict bytes of an
  earlier run over the same fixture, and those bytes are additionally asserted against an
  expectation written
  out in the test — the named verdict for every handle, the exact flag set, and the reason of every
  `critical_missing` entry — because two identically wrong runs are also equal, so run-to-run
  equality alone proves nothing. Refusal: an unresolvable Git repository or HEAD yields a typed
  refusal, and each other case FPK-V0-024 enumerates yields the exact code that clause names for
  it, an invented code being a failure; a `Build` error carrying its own code yields that code
  unchanged, and a code-less `Build` error yields `unsupported-prove-index` carrying the `Build`
  message verbatim as its `error` member (`cmd/corvint/main.go:1345-1349@b96186e4`), an uncoded refusal
  being a failure; and a checkpoint whose
  `repository.object_format` is `sha256` replayed in a sha1 repository refuses
  `object-format-mismatch` before any handle verdict, a confident all-`blob-changed` document being
  a failure. Index acquisition: a `prove --checkpoint` run makes zero calls to the snapshot loader,
  asserted through the `loadSnapshot` seam FPK-V0-024 names over the whole run, with a source-level
  guard asserting that no reference to `contextindex.LoadSnapshot` occurs under any import name
  outside the `loadSnapshot` initializer, scanning the non-test `.go` files of `cmd/corvint`, and
  the run additionally asserted to call no function outside `cmd/corvint` that reaches
  `internal/contextindex`'s `LoadSnapshot`, the tree's only loader, since byte-identity between
  a hit and a miss is what `IDX-SNAP-V0-006` demands of a correct snapshot reader and so cannot
  distinguish one; the same fixture with and without a valid `.corvint/index/` snapshot additionally
  yields byte-identical documents, and neither carries a `snapshot` or any other cache-metadata
  member; and the stderr of a plain `prove PATH...` (or `prove --base`, the two modes that reach
  `contextindex.Build` on a snapshot miss at `cmd/corvint/prove.go:1073@a1c6494d` and `cmd/corvint/index_snapshot.go:83@90129c09`) for a code-less `Build` error is
  byte-equal to a pinned expectation, so an `unsupported-prove-index` mapping placed in `emitError`
  rather than in the checkpoint branch fails. History
  flags: a merge and a revert each yield `commit-moved`, with the same tree reported as a fact and
  never attributed to either; and a `dirty_paths_sha256` recomputed by the FPK-V0-020
  construction — the lowercase hex SHA-256 of the canonical JSON of `affected.DirtyPaths`'s
  sorted list — over a dirty set equal to the checkpoint's leaves `dirty-set-moved` unset, while
  one added dirty path sets it, so the negative case is decidable and not merely unasserted.
  Deletion: a path the current tree does not list and `git cat-file`
  reports `missing` yields `path-deleted`; the same verdict is asserted for a space-bearing path,
  whose exact echoed missing header the shared parser accepts (FPK-V0-021).
  Precedence: a path present in the tree but absent from
  the index — an unadmitted kind or an exclusion — yields `unsupported`, never `path-deleted`; a
  directory path and a gitlink path each yield `unsupported`, never `path-deleted`; a symlink path
  (mode `120000`) is judged as a blob and MUST NOT be refused; a path bearing LF and a path bearing
  CR each yield `unframable`; a worktree-deleted path yields `dirty`, never `path-deleted`; and a
  path that is both dirty and unadmitted yields `unsupported`. Authority: a changed instruction blob
  named by a `critical` selector yields `authority-changed`, a changed `critical` instruction blob
  is the only population the flag covers, so a changed instruction handle that no `critical`
  selector names yields `blob-changed` without the flag; a changed non-authority `critical` handle
  yields `blob-changed`
  without the flag, and an UNCHANGED `critical` instruction blob yields `unchanged` without the
  flag, so the
  flag requires the blob to differ and is not set by the authority class alone. Selectors: a `critical` selector whose symbol was renamed inside a
  still-admitted changed file yields `critical_missing` with reason `selector-unresolved` while its
  handle stays `blob-changed`; a selector on a `dirty` handle, one on a `path-deleted` handle, one on an
  `unsupported` handle, and one on an `unframable` handle each yield `critical_missing` with that
  handle verdict as the reason and no rehydrated row; a selector whose `path` occurs in no
  `handles` entry yields `critical_missing` with reason `handle-undeclared` and no rehydrated row,
  and is not a refusal; a
  selector whose row moved lines survives with its `line` refreshed; a selector matching more than
  one row rehydrates all of them in the clause's order; and selectors that still resolve are
  rehydrated rather than reported. Bounds: exactly 256 handles is accepted and judged and 257 is
  refused, and the same bound is proved for `critical` independently of `handles`; neither is
  truncated. Preservation: every stored `verification` row and every prose member is echoed
  byte-for-byte as given. Non-execution: a stored `command` that would create a sentinel file is
  never run, proved by that file's absence. No retention: a tree digest over the repository and
  `.corvint/` is unchanged across the run. Forgery: a forged `blob_hash` yields `blob-changed`, never
  an error. The comparison against competent structured human notes that AT-06's acceptance criteria
  name is an AT-08 evaluation item, not a test in this repository.

- **FPK-V0-027:** `proof.claims` MUST be present in every output that carries `proof`, keyed by all
  five falsifier values, with exactly these values:
  - `history-consistent`: `the cited blob and line still exist at the packet revision; this is a check of the citation, not evidence that the row is relevant`
  - `reference-resolves`: `the cited line holds the claimed import or identifier and the declaring blob declares it; this is a check of the reference, not evidence that the row is relevant`
  - `verifier-accepts`: `the frozen CEM verifier accepted the map and this hunk's evidence is stable or relocated at the target`
  - `test-kills-mutant`: `the cited test failed on a mutant of the changed lines and passed on the baseline`
  - `none`: `nothing about this row was checked`
  A `READY` packet whose only falsifier-bearing rows are `syntax` with `history-consistent` MUST
  report `state: CITED`; at least one falsifier-bearing row is required. The legend is emitted once,
  never copied into individual rows, and does not alter the embedded packet.

- **FPK-V0-028:** (experimental prototype, decision 0063 item 5) When `--mutate` attempts a
  `test-kills-mutant` row, that row MUST carry `witness` beside its verdict and outside the embedded
  packet. An attributable Go kill MUST carry `status: PRODUCED`, the checkout commit in
  `checkout_revision`, the named `killing_test`, `observed_kill: KILLED`, and `mutant` containing
  the changed source `path`, its original Git `blob` at that commit, a zero-based/end-exclusive
  `byte_span` in that blob, and the closed-table `mutation_operator` that was applied there. These
  fields are sufficient to check out the commit, confirm the blob, reapply the deterministic
  operator to the span, establish that the named test passes on the baseline, and observe that it
  fails on the mutant. A runner that is unavailable MUST leave the row `NOT_RUN` and carry
  `witness: {"status":"NOT_PRODUCED","reason":"mutation-runner-unavailable"}`; other attempts
  without an attributable Go kill MUST also say `NOT_PRODUCED`, never invent witness fields.
  Without `--mutate`, `witness` is absent. This prototype neither adds a packet member nor changes
  the oracle-pinned packet bytes. Acceptance remains `NOT_RUN` until at least 19 of 20 produced
  witnesses replay as kills on a second checkout; the focused fixture replay is development
  evidence, not that acceptance cohort.

- **FPK-V0-029:** (accepted 2026-09-07 by decision 0080) A document's `status`, which decides every
  authority label in FPK-V0-011 and the `authority-changed` flag above, MUST be read as a labelled
  field wherever this repository writes one: at column zero, optionally after a list marker,
  emphasis markers, one qualifier word (`Intent status:`), or a run of preceding `Key: value.`
  segments on the same line. A preceding segment MUST itself have that field shape, so prose ending
  in a full stop MUST NOT introduce a status, and an indented `status:` MUST NOT be read, being a
  nested key rather than the document's own field. The captured value MUST be the status token
  alone — letters, digits and hyphens, optionally followed by `:argument` with optional spaces after
  that colon, so both `partially-superseded-by:0031` and a quoted `"partially-superseded-by: 0042"`
  survive with the colon the prefix test needs — and never the remainder of the sentence, because
  `binding` compares it against a fixed vocabulary by equality. The Go pattern and the Python
  oracle's pattern MUST remain the same pattern (`GPK-V0-002`).

- **FPK-V0-030:** (experimental prototype, not advertised) A CEM MUST be attestable in the same DSSE
  envelope FPK-V0-015 emits, so an external verifier reads one envelope shape for both artifacts.
  Intent: the CEM is the other artifact a consumer is asked to trust, and a second bespoke reader
  would be a second trust surface. `internal/attest.CEMStatement(name, cem)` MUST refuse an empty
  name and any bytes `wire.ParseMap` rejects, and otherwise return a canonical in-toto Statement v1
  whose `predicateType` is `https://corvint-context.dev/attestation/cem/0`, whose single `subject` is
  `name` with the `sha256` of the map bytes, and whose `predicate` is a content-addressed reference
  `{"digest":{"sha256":HEX},"name":NAME,"size":BYTES,"spec":SPEC}`. The map bytes MUST NOT be
  embedded: a map may be 4 MiB and the digest already binds it. The statement is signed by the
  unchanged `Envelope`. `internal/attest.VerifyCEM(envelope, publicKey, cem)` MUST first pass
  `Verify`, then refuse a statement that repeats a member name at any depth or carries a member
  name differing only in case from `_type`, `predicateType`, `subject`, `predicate`, `digest`,
  `sha256`, `name`, `size`, or `spec` at that position, so one signed payload cannot yield one claim
  to Corvint and another to a strict in-toto reader; members the statement does not define are
  ignored, as in-toto consumers must ignore unrecognized fields. It then refuses a statement whose
  `_type` or `predicateType` differs, whose subject count is
  not one, or whose subject and predicate disagree, and then refuse a claim `CEMStatement` could not
  have produced: a sha256 that is not 64 lowercase hex digits, a negative size, or an empty name or
  spec. With `cem` supplied it MUST refuse bytes whose
  sha256 or size differ from the signed claim, an empty map included, and otherwise report status
  `VERIFIED`. With no bytes supplied it MUST return the signed claim with status `NOT_RUN` and
  reason `cem-bytes-not-supplied`, never `VERIFIED`. Local only: no network, no signing service,
  and no key is generated, written, or read outside the FPK-V0-015 key path, the FPK-V0-031
  public-key path, and test fixtures. FPK-V0-031 is the only command surface; publishing the
  predicate type under `protocol/**` waits on promotion. Rollback: delete `internal/attest/cem.go`,
  its test, FPK-V0-031, and this clause; FPK-V0-015 is untouched.
- **FPK-V0-031:** (experimental prototype, not advertised; help labels it experimental) The CLI
  MUST expose FPK-V0-030 without changing any existing output. Intent: an external verifier needs
  a way to obtain and check the CEM envelope that does not break a caller already parsing the
  single FPK-V0-015 line. Emission: in CEM mode `--attest-cem` takes no value, may not repeat, and
  implies `--attest`. stdout is then two lines, each ending in a newline: first the FPK-V0-015
  statement or envelope, byte-identical to the output of the same invocation without
  `--attest-cem`; second `CEMStatement(MAP, bytes)`, where MAP is the `--cem` path with `/`
  separators and bytes are the map bytes the verifier read (the FPK-V0-006 bracket already binds
  them), unsigned under `--attest` and signed by the same `--attest-key`, read once, otherwise.
  Without `--attest-cem` every existing mode's output is unchanged. Verification:
  `prove --verify-cem-attestation ENVELOPE --attest-public-key PEM [--cem MAP]` selects its own mode
  wherever the flag appears before `--`, takes each flag once with one non-empty value, and accepts
  no other flag. It reads ENVELOPE (one envelope, at most 8 MiB, from any path) and an Ed25519 PKIX
  `PUBLIC KEY` PEM under the FPK-V0-015 key bounds, reads MAP unparsed as a repository-relative
  path under `--root` bounded at the CEM map bound, runs no Git, writes nothing, and calls
  `VerifyCEM` with MAP's bytes or, without `--cem`, none. Success is exit 0 with one canonical
  JSON line
  `{"cem":{"digest":{"sha256":HEX},"name":NAME,"size":BYTES,"spec":SPEC},"mutates":false,"ok":true,"predicateType":"https://corvint-context.dev/attestation/cem/0","status":STATUS,"tool":"prove"}`,
  STATUS being `VERIFIED`, or `NOT_RUN` with `"reason":"cem-bytes-not-supplied"` inserted before
  `status` when no map was given. The signed NAME is reported, not compared with `--cem`: the
  digest and size bind the bytes. Refusals exit 2 with nothing on stdout and the code on stderr,
  as FPK-V0-015 does: `attest-public-key-unavailable` (key missing, over 16 KiB, not one PKIX PEM
  block, or not Ed25519); `attest-envelope-unavailable` (envelope unreadable, non-regular, changed
  identity while opening, or over its bound);
  `attest-verification-failed` (any `VerifyCEM` refusal other than a byte mismatch: signature,
  envelope members, repeated or case-variant member names, `_type`, `predicateType`, subject count, subject/predicate agreement, or claim
  shape);
  `attest-cem-mismatch` (MAP bytes differ from the signed sha256 or size); `map-unavailable` (MAP
  absolute, climbing, unreadable, non-regular, changed identity while opening, over its bound, or
  having an existing path component under `--root` that is itself a symlink, even one that resolves
  back inside `--root`, or that case-folds equal to `.git`);
  both ENVELOPE and MAP must be paths that themselves name regular files, so symlinks are refused.
  `invalid-arguments` for a missing,
  empty, repeated, or unknown flag, including `--attest-cem=VALUE` and `--attest-cem` outside CEM
  mode. An emission whose CEM statement cannot be built is `attest-failed`. Rollback: delete
  `cmd/corvint/prove_attest_cem.go`, its test, `attest.ReadPublicKey`, the `--attest-cem` branch of
  `attestProof`, the help paragraph, and this clause; FPK-V0-015 output is unchanged either way.

## Simpler baseline and why it is insufficient

Re-reading the cited file from the index and comparing hashes would be cheaper, but it would check
the claimant against itself. The falsifier's value is that it consults an authority the packet
did not produce; Git is that authority for a citation of committed content.

## Trust boundary and resource limits

Git runs with a scrubbed environment, `--no-optional-locks`, a 30-second deadline per call, and one
`cat-file --batch` for all cited paths, whose output is bounded at 64 MiB; exceeding the bound is a
typed failure. Paths that cannot be framed by the LF-delimited batch protocol are never sent.
Python blobs are tokenized, never executed. Under `--mutate` the only writes are to a temporary
directory the runner removes and a temporary build cache the command removes; the sandbox of
FPK-V0-014 enforces this against the cited tests themselves, keeps the export read-only to them,
denies them the network and signals to outside processes, and their process group is killed
after every run. Each `go test` runs offline with output bounded at
64 KiB, and the whole row is bounded by a ten-minute budget. The attestation key is read once, bounded at 16 KiB,
and the command never generates one. Otherwise nothing is written. No network, daemon, or embedding
is involved.

## Non-goals and authority

The verdict is not an authority and does not alter the embedded packet's `coverage` block, which
remains the wrapped contract's number. `prove` runs no test unless `--mutate` is given, and then
only in an exported temporary copy; it does not type-check or bind
identifiers, does not check that an import path names the changed file's package, does not verify
CEM bindings, and does not decide relevance: a
`PASS` says the cited content exists where the row says it does, not that it answers the task. The
falsification rate of FPK-V0-016 is a count over what `prove-observe` was given, not a measurement
of the repository's claims; it says nothing about proofs nobody recorded. A
`verifier-accepts` `PASS` restates the CEM contract: structural integrity and mapping
completeness, never that the evidence semantically supports the hunk. No adapter or hook calls
`prove`. (Proposed, FPK-V0-020..026) A checkpoint verdict is not a completeness or correctness
claim about the caller's task: `unchanged` says a cited handle still matches Git, not that the
task is on track, and rehydrating a `critical` selector is not a claim that the model read or
retained it. It is also not a claim about undeclared handles: a binding instruction file added
since the checkpoint was written, and never cited in it, is not flagged by this verb — the caller
reruns `context`/`query` at the current revision to discover it.
The FPK-V0-030 CEM attestation binds who signed a map to its bytes; it is not a verdict on the map,
does not run the CEM verifier, and is not a build-provenance claim. Corvint evidence is not mapped
onto SLSA, SPDX, or CycloneDX predicates: those schemas describe how an artifact was produced, not
why context was judged relevant, and a consumer reading a Corvint judgement through such a field
would read a claim Corvint never made.

## Failure modes

| Mode | Observable behaviour |
|---|---|
| Git missing or HEAD unresolvable | exit 2, `unsupported-prove-revision`, no stdout |
| Worktree or HEAD changes mid-run | exit 2, `unsupported-prove-drift` |
| `cat-file` fails, output over bound, or stream unreadable | exit 2, `unsupported-prove-history` |
| Cited path dirty or unframable | row `NOT_RUN`; result `unproven` |
| Cited blob absent, replaced, or line out of range | row `FAIL`; result `failed` |
| Packet with only vocabulary rows | every row `history-consistent`/`PASS` when its citation resolves; `state: CITED` when the packet was `READY` |
| Citing Go blob does not parse, or the claimed import or identifier is not on the cited line | row `FAIL`; result `failed` |
| Declaring blob lacks the named top-level declaration | row `FAIL`; result `failed` |
| Import or reference row for a path that is neither Go, Python, nor a web language | row falls through to the authority table; `syntax` gets `history-consistent` |
| Python blob does not tokenize, or no import on the cited line binds the claimed module | row `FAIL`; result `failed` |
| Web blob has no statement with the claimed specifier spanning the cited line, the identifier is not on it, or the declaring blob lacks the top-level declaration | row `FAIL`; result `failed` |
| Test-convention row without `--mutate` | row `test-kills-mutant`/`NOT_RUN`, no `detail` |
| Every mutant survives the cited test | row `FAIL` with the runner's `detail`; result `failed` |
| Every mutant fails to build | row `NOT_RUN` with `detail`; result `unproven` |
| Cited test blob absent or replaced at the packet revision | row `FAIL` without running anything |
| Map bytes change between the row read and the verifier's read | exit 2, `unsupported-prove-drift` |
| Nothing mutable, budget exhausted, baseline red, or dependencies offline | row `NOT_RUN` with `detail` |
| Mutation row reached after the invocation's thirty-minute budget | row `NOT_RUN` with `detail` `invocation mutation budget exhausted before this row` |
| Mutation runner cannot export or write its temporary copy | exit 2, `unsupported-prove-mutation` |
| Mutation runner is unavailable after an attempted row | row `NOT_RUN`; `witness.status: NOT_PRODUCED`, reason `mutation-runner-unavailable` |
| Mutation attempt yields no attributable Go kill | `witness.status: NOT_PRODUCED`; no mutant, test, revision, or observed-kill field is invented |
| `--mutate` outside impact or change mode, `--attest=VALUE`, `--base` with paths, or a repeated flag | exit 2, `invalid-arguments` |
| Range changes over 100 paths or 1 MiB of names | exit 2, `unsupported-prove-history` |
| Affected-plan graph cannot be built in change mode | exit 2, `unsupported-prove-affected` |
| Affected-test row for a test in a language neither runner mutates, a changed test, or a test that is the changed path | row `none`/`NOT_RUN`; result `unproven` |
| Affected-test row whose test is not a blob at the packet revision | row `FAIL` without running anything |
| Changed path reached only by tests that let its mutants survive | `paths` entry with empty `covered_by`; one failed result for the path |
| Changed path pinned by one reached test among many that do not | `paths` entry naming the test in `covered_by`; one proven result, no failed result; the `FAIL` rows stay in `proof.counts` |
| Range diff, changed-path list, or `cat-file` stream past its byte bound | exit 2, `unsupported-prove-history`, read stops one byte past the bound |
| Attestation key missing, oversized, not PKCS#8, or not Ed25519 | exit 2, `attest-key-unavailable` |
| (FPK-V0-030) Bytes given to `CEMStatement` are not a CEM, or the name is empty | error; no statement |
| (FPK-V0-030) CEM envelope signature, `_type`, `predicateType`, subject count, or subject/predicate agreement fails | `VerifyCEM` error; no claim returned |
| (FPK-V0-015, FPK-V0-030) Validly signed envelope, signature object, or statement repeats a member name or carries a case variant of a defined member name | `Verify` or `VerifyCEM` error; no claim returned; undefined statement and signature members are ignored |
| (FPK-V0-030) Validly signed claim with a sha256 that is not 64 lowercase hex digits, a negative size, or an empty name or spec | `VerifyCEM` error, with or without bytes; no claim returned, never `NOT_RUN` |
| (FPK-V0-030) Supplied CEM bytes differ from the signed sha256 or size | `VerifyCEM` error; no claim returned |
| (FPK-V0-030) No CEM bytes supplied | signed claim returned with status `NOT_RUN`, reason `cem-bytes-not-supplied` |
| (FPK-V0-031) `--attest-cem=VALUE`, repeated, or outside CEM mode; verify mode missing, empty, repeated, or unknown flag | exit 2, `invalid-arguments` |
| (FPK-V0-031) Verify public key missing, over 16 KiB, not one PKIX PEM block, or not Ed25519 | exit 2, `attest-public-key-unavailable` |
| (FPK-V0-031) Verify envelope unreadable, non-regular, changed while opening, or over 8 MiB | exit 2, `attest-envelope-unavailable` |
| (FPK-V0-031) Envelope fails any `VerifyCEM` check other than the byte comparison | exit 2, `attest-verification-failed` |
| (FPK-V0-031) `--cem` bytes differ from the signed sha256 or size | exit 2, `attest-cem-mismatch` |
| (FPK-V0-031) `--cem` path absolute, climbing, unreadable, non-regular, changed while opening, or over the map bound | exit 2, `map-unavailable` |
| (FPK-V0-031) Verify without `--cem` | exit 0, `status: NOT_RUN`, `reason: cem-bytes-not-supplied` |
| CEM map rejected by the verifier | every `supported`/`mechanical` row `FAIL`; `state: REJECTED` |
| Basis evidence stale, deleted, or ambiguous at the target | that hunk's row `FAIL` |
| CEM verifier inputs missing or map unreadable, non-regular, or changed while opening | exit 2 with the verifier's own code, no stdout |
| `prove-observe` stdin is not a `prove` document, or its counts are malformed | exit 2, `invalid-proof-document`, no ledger change |
| Ledger unreadable or without `proof` rows | `proof.ledger` absent; verdicts unaffected |
| Python test row without `--mutate` | row `test-kills-mutant`/`NOT_RUN`, no `detail` |
| `python3` absent, or pytest not importable by it inside the sandbox | row `NOT_RUN` with `detail` `python3 is not available` or `pytest is not available to python3` |
| Python mutant does not parse, pytest cannot collect it, or the per-run timeout ends its run | mutant neither killed nor survivor, counted in `detail`; a row where every mutant is such is `NOT_RUN` |
| Python baseline red, uncollectable, over the timeout, refused by pytest as a usage error, or missing a dependency | row `NOT_RUN` with `detail` |
| Test row of one language claiming a changed path of another | row `none`/`NOT_RUN` |
| Checkpoint document missing, unreadable, non-regular, changed while opening, or over 256 KiB | exit 2, `unreadable-checkpoint-document`, no stdout |
| Checkpoint document fails schema or canonical-JSON validation | exit 2, `invalid-checkpoint-document`, no stdout |
| Checkpoint `handles` or `critical` count over 256 | exit 2, `checkpoint-bound-exceeded`, never truncated |
| (Proposed) `--checkpoint` combined with `--task`/`--cem`/`--base`/`--mutate`, or any other rejected argument | exit 2, `invalid-arguments`, no stdout, in either argv order (`--task` first the query argument allowlist refuses it, `--checkpoint` first the branch's own check does; on the pre-change binary `parseImpactArguments`) |
| Checkpoint `repository.object_format` differs from the current repository's | exit 2, `object-format-mismatch`, after the index build and before any handle verdict or per-handle `cat-file` call |
| Checkpoint verdicts cannot be encoded, or the stdout write fails | exit 2, `output-failed`; not a refusal; stdout MAY hold a partial document |
| `critical` selector on a handle whose verdict is not `unchanged`/`blob-changed` | `critical_missing` with that handle verdict as reason and a recovery operation; no row lookup |
| Checkpoint names a repository or HEAD Git cannot resolve | exit 2, `unsupported-prove-revision` |
| Checkpoint's whole-tree `git ls-tree` read fails or passes the 64 MiB bound | exit 2, `unsupported-prove-tree` |
| Checkpoint handle names a directory or a gitlink | `unsupported`, never `path-deleted` |
| Checkpoint handle names a symlink (mode `120000`) | judged as a blob: `unchanged`/`blob-changed`; never a refusal |
| Checkpoint handle path is not normalized or bears LF/CR | `unframable`; never sent to Git |
| `critical` selector's `kind`/`id` no longer resolve at an admitted path | `critical_missing`, reason `selector-unresolved`; handle verdict unchanged |
| `critical` selector whose `path` occurs in no `handles` entry | `critical_missing`, reason `handle-undeclared`; no row lookup; not a refusal |
| `critical` selector's `path`/`kind`/`id` match more than one current row | every match rehydrated in the clause's order; `line` refreshed from the first |
| Checkpoint index build fails with a code-less `contextindex` error | exit 2, `unsupported-prove-index`, `Build`'s message kept as `error` in a fresh error that does not unwrap to `*contextindex.Error` |
| Checkpoint `verification` row carries a command string | echoed as an attributed prior observation; never executed |
| `.corvint/index/` snapshot present or absent at `prove --checkpoint` | no observable difference; the index is always built; no cache-metadata member is emitted |
| `prove --checkpoint` against a repository with a populated self-observation ledger | no `proof.ledger` member is emitted; checkpoint output is unaffected by ledger content |

### Further named codes and witness reasons

`prove` and `prove-observe` emits the kebab-case codes below (decision 0100). Each row cites the
first emitting site and quotes the message returned there or states the condition checked there,
which is the whole of what the row asserts.

| Code | First emitting site | At the cited site |
|---|---|---|
| `attest-failed` | `cmd/corvint/prove.go:881` | "cannot build the in-toto statement", or at the signing site "cannot sign the DSSE envelope" |
| `invalid-mutation-claim` | `cmd/corvint/prove.go:1357@bd8d0b17` | witness `NOT_PRODUCED` reason when the row reason names no recognizable test-claim target; the row is `FAIL` with detail "unrecognized test claim" |
| `mutation-budget-exhausted` | `cmd/corvint/prove.go:1216` | witness `NOT_PRODUCED` reason for an unjudged mutant row reached after the invocation mutation budget context has ended while the parent context has not; the row is `NOT_RUN` |
| `mutation-input-unavailable` | `cmd/corvint/prove.go:1360@d744f811` | witness `NOT_PRODUCED` reason when a cited path is dirty (row `NOT_RUN`), the cited test blob is not at the cited path (row `FAIL`), or, after a kill, the changed blob or checkout revision is missing or the reported witness has no operator, no killing test, or an empty span |
| `no-mutant-in-range` | `cmd/corvint/prove.go:1351` | witness `NOT_PRODUCED` reason when the range changed no lines of the claimed path; the row is `NOT_RUN` |
| `no-replayable-kill-observed` | `cmd/corvint/prove.go:1373` | witness `NOT_PRODUCED` reason when the Go mutation report verdict is not killed or carries no witness, and the runner-unavailable case did not apply |
| `observation-failed` | `cmd/corvint/prove_observe.go:64` | `prove-observe` could not append the `proof` row to the self-observation ledger; the message is the append error text and the exit is 2 |
| `python-mutation-witness-not-produced` | `cmd/corvint/prove.go:1355` | witness `NOT_PRODUCED` reason set on every row whose claimed changed path ends in `.py`, whatever the Python mutation verdict |

## Acceptance evidence and traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| FPK-V0-001 | `runProve`, no `observeUnsupported` call on the prove path | `TestProveEmbedsTheQueryPacketUnchangedAndWritesNothing`, `TestProveRejectsBadArgumentsWithoutOutputOrLedgerWrite` (tree digest including the ignored ledger) |
| FPK-V0-002 | `compileProof`, `assignFalsifiers` | byte-equality of `packet` with `query` output in `TestProveEmbedsTheQueryPacketUnchangedAndWritesNothing` |
| FPK-V0-003 | `falsifierByAuthority`, `falsifierByKindAndAuthority`, `falsifierFor` | `TestProveVocabularyOnlyPacketIsCitedNotProven`, `TestFalsifyRowVerdicts`, `TestMutationVerdictVocabulary` |
| FPK-V0-004 | `falsifyRow`, `readCitedBlobs`, `parseCatFileBatch` | `TestFalsifyRowVerdicts`, `TestParseCatFileBatchHandlesMissingAndMalformedObjects`, `TestProveDirtyPrimaryRowLeavesTheResultUnproven` |
| FPK-V0-005 | `resultVerdict`, `summarizeProof`, `proveState` | `TestFalsifyRowVerdicts` (shared-id case), `TestProveVocabularyOnlyPacketIsCitedNotProven`, `TestProveDirtyPrimaryRowLeavesTheResultUnproven`, `TestProveResultsSharingAnIDAnswerForTheirOwnRows` |
| FPK-V0-006 | `compileProof`, `compileCEMProof` brackets | `TestProveRefusesDriftInEveryMode` (thirteen cases: dirty-set and HEAD-tree drift in all four non-checkpoint modes, CEM map-byte drift, and one control per mode) |
| FPK-V0-017 | `affectedTestRows`, `affectedRow`, `citeAffectedBlobs`, `citedPaths`, `rangeChangedPaths`, `rangeHunkSpans`, `boundedOutput`, `testClaimTarget`, `affectedPaths`, `proveAffectedPath.verdict`, `provePacket` change branch | `TestProveChangeModeAddsAffectedTestRowsAndJudgesThemUnderMutate` (packet byte-identical to `impact --base`, NOT_RUN then PASS under `--mutate`, determinism, tree digest unchanged), `TestProveChangeModeSummarizesCoveragePerChangedPath` (one covering and one reached-not-covering test: `paths`, one proven result, no failed result, per-row `counts` kept), `TestProveChangeModeReportsAnUncoveredPathAsOneFailedResult`, `TestProveRefusesAGitStreamPastItsByteBoundBeforeBufferingIt` (the three byte bounds), `TestProveChangeModeRefusesPathsAndUntrackedWithBase`, `TestAffectedRowMakesNoMutationClaimForAChangedTest`, `TestHunkSpansFollowTheNewSideOfTheDiff`, `TestProveMutateInvocationBudgetLeavesLaterRowsNotRun`, `TestCitedPathsBindsAnUncheckedAffectedTestRow`; runner `TestRunJudgesACrossPackageTest` and `TestGenerateMutantsConfinesToTheGivenLines` in `internal/liveverify/mutate`; dogfood runs in `docs/BUILD-LOG.md` 2026-09-01 |
| FPK-V0-007 | canonical JSON, sorted cited paths, no timing in wire | rerun byte-identity in `TestProveEmbedsTheQueryPacketUnchangedAndWritesNothing` |
| FPK-V0-008 | `compileProof` error mapping | `TestProveRejectsBadArgumentsWithoutOutputOrLedgerWrite` |
| FPK-V0-009 | no change to `query` or `impact` code paths | `conformance/cli-parity-v0` green on the same tree; `go-query-start-v0` replay `NOT_RUN` (pinned fixtures unavailable) |
| FPK-V0-010 | `provePacket`, `parseProveInvocation` | `TestProveImpactJudgesGoImportAndReferenceRows`, `TestProveImpactRefusesWorktreeModeAndUnknownBase`, `TestProveChangeModeRefusesPathsAndUntrackedWithBase`; MISSING: path-mode packet byte-equality against `impact` |
| FPK-V0-011 | `falsifierByKindAndAuthority`, `judgeReference`, `goImportsAtLine`, `goIdentifierAtLine`, `goDeclaresAtTopLevel` | `TestJudgeReferenceVerdicts` (fourteen verdicts), `TestFalsifierAssignmentByKindAndLanguage`, import line 5 and reference PASS in `TestProveImpactJudgesGoImportAndReferenceRows` |
| FPK-V0-013 | `importsAtLine`, `pythonImportsAtLine`, `pythonImportingPackage`, `internal/liveverify/pyresolve` | `TestProveImpactJudgesPythonImportRows` (absolute and relative rows PASS at line 2), `TestPythonImportsAtLineVerdicts` (ten verdicts), `TestPythonReferenceRowIsNotRun`, `TestFalsifierAssignmentByKindAndLanguage`, package tests in `pyresolve` |
| FPK-V0-018 | `jsPath`, `jsImportsAtLine`, `referenceResolves` (`cmd/corvint/prove_javascript.go`), `internal/liveverify/jsresolve` (`Imports`, `ImportsAtLine`, `IdentifierAtLine`, `DeclaresAtTopLevel`) | `TestProveImpactJudgesJavaScriptImportAndReferenceRows` (three import rows PASS at their lines in a module-free repository, a commented specifier makes no row, six reference verdicts over the committed blobs), `TestFalsifierAssignmentByKindAndLanguage`, package tests in `jsresolve` (`TestImportsAtLineVerdicts` with twenty-one verdicts, `TestImportsListsStatementsInOrderWithLines`, `TestSubstitutionBodiesLexAsCode`, `TestEscapedLineBreakInLiteralCountsALine` (a backslash line continuation in a quoted or template literal, before `\n` or `\r\n`, keeps the later import and identifier on their lines), `TestQuotedCRLFContinuationStaysInsideTheLiteral` (a `\` before `\r\n` continues a quoted literal through both bytes, so an import spelled in its remainder is not an import), `TestLineCommentEndsAtEveryLineTerminator` (an import after a `//` comment ended by `\r`, U+2028, or U+2029 is found on the same `\n`-counted line), `TestIdentifierAtLine`, `TestDeclaresAtTopLevel`, `TestMultiLineBlockCommentIsAStatementBoundary` (a block comment whose body holds `\n` or `\r\n` separates two declarations; a single-line one does not), `TestEveryLineTerminatorIsAStatementBoundary` (a bare `\r`, U+2028, or U+2029, or one inside a block comment, starts a statement for `DeclaresAtTopLevel` and does not shift the `\n`-counted line), `TestUnclosedQuoteEndsAtCarriageReturnOnly` (a declaration after an unclosed quote and a bare `\r` is found; a U+2028 or U+2029 inside a closed quote does not end it), `TestBacktickInRegexLiteralOpensNoTemplate` (a backtick in a regular expression after `=` or `(`, or inside a class, hides no later declaration or line; `x / y / z` stays division), `TestDivisionAfterClosingBraceStaysCode` (a `/` immediately after `}` remains division and leaves its operand visible)) |
| FPK-V0-014 | `withoutMutateFlag`, `judgeMutations`, `judgeMutation`, `mutationFalsified`, `internal/liveverify/mutate` (`Open`, `Judge`, `Close`, `classifyRun`, `deletableStatement`, `hostSandbox`, `seatbeltProfile`, `bubblewrap`, `newWorkspace`, `resetScratch`, `reportsTestFailure`, `configureProcess`) | `TestProveImpactMutationIsOptInAndKillsWithAStrongTest` (NOT_RUN without the flag, PASS with `detail`, tree digest), `TestProveImpactMutationFailsAnAssertionFreeTest`, `TestProveMutateFlagIsImpactOnly` (repeat, non-impact, `--`), `TestJudgeMutationFailsAMisplacedTestBlobWithoutRunning`, `TestMutationVerdictVocabulary`, package tests in `mutate` (root digest, temp-dir removal, budget, baseline, `TestRunNeverCreditsAMutantThatDoesNotCompile`, `TestRunRejectsModeOnlyFailureBeforeMutation`, `TestGroupFallbackRejectsModeOnlyBaseline`, `TestEventSinkPreservesFailureClassification`, `TestPlanNeverDeletesADefinition`, `TestClassifyRunSeparatesABuildFailureFromARedTest`, `TestRunConfinesTheCitedTests`, `TestRunKillsWhatTheCitedTestsLeftRunning`, `TestRunReportsUnsupportedWithoutASandbox`, `TestSeatbeltProfileConfinesWritesAndNetwork`, `TestBubblewrapArgvConfinesWritesAndNetwork`, `TestRunGivesEveryRunAFreshScratch`, `TestRunRefusesToJudgeARunWithoutAVerdict`, `TestRunRejectsARelativeCacheDir`, `TestOpenServesManyClaimsFromOneCopy`, `TestJudgeRejectsAClaimAgainstAnotherRevision`, `TestOpenRejectsCallerMistakes`, `TestOpenNeverReadsTheRevisionAsAnOption`, `TestOpenExportsCommittedBytesWhateverTheAttributes`, `TestOpenRefusesATreeEntryItCannotReproduce`) |
| FPK-V0-015 | `attestProof`, `cemSubjects`, `internal/attest` | `TestProveCEMAttestWrapsTheDocumentAndSignsIt` (predicate equals the document, subjects, `Verify` round trip, tree digest), `TestProveCEMAttestVerifiesOfflineWithCosignSemantics` (independent PKIX public-key, DSSE PAE, in-toto shape, and tampered-payload rejection), `TestProveCEMAttestRejectsBadFlagsAndKeys`, `TestProveCEMAttestRefusesAMapPathThatIsNotUTF8` (a map path whose bytes are not valid UTF-8 is exit 2 `attest-failed` with empty stdout; where the filesystem refuses the name, as APFS does with EILSEQ, `attestProof` refuses it with the real prove document after accepting the UTF-8 path; fails with the canonical string check disabled), package tests in `attest` (golden statement, envelope rejections, key bounds), `TestVerifyRejectsDuplicateMemberNames` (repeated envelope `payload` and signature `keyid`), `TestVerifyRejectsCaseVariantSignatureMember` (`KeyID` alone and `KEYID` before `keyid`); each accepted before the strict decoder; `TestStatementRejectsRepeatedMemberName` (top-level and nested), `TestStatementRejectsInvalidUTF8` (prove document, subject name, digest algorithm, CEM name); each accepted before the refusal |
| FPK-V0-016 | `runProveObserve`, `proofCounts`, `proveLedger`, `observations.Falsification`, `falsificationLines` | `TestProveObserveRecordsOnlyTheVerdictCounts` (a real proof piped in; the row carries kind and counts only; `ledger` absent before and present after; tree digest unchanged by the second `prove`; `observations` lines), `TestProveObserveRejectsWhatIsNotAProof` (seven rejections, unignored ledger, stray flag), `TestFalsificationRateCountsJudgedRowsOnly` |
| FPK-V0-012 | `compileCEMProof`, `cemRows`, `judgeVerifier`, `cemState`, `parseProveCEMArguments` | `TestProveCEMAcceptedMapProvesSupportedHunksAndWritesNothing` (embedded document equals `cem verify`, rerun byte-identical, tree digest), `TestProveCEMRejectedMapFailsEveryClaimRow`, `TestProveCEMUnknownHunkIsNotAClaim`, `TestProveCEMRejectsBadArgumentsWithoutOutputOrWrite`, `TestProveCEMRefusesAMapPathThroughASymlinkedParentOrCaseFoldedGit` (a symlinked parent, whether it resolves inside or outside `--root`, and a path component that case-folds equal to `.git`, exit 2 `map-unavailable`), `TestJudgeVerifierVerdicts`, `TestProveRefusesDriftInEveryMode`, `TestReadBoundedFileRefusesFIFO`, `TestReadBoundedFileRefusesDirectory` |
| FPK-V0-019 | `mutationTestPath`, `mutationClaim`, `judgePythonMutation` (`cmd/corvint/prove_py.go`); `falsifierFor`, `affectedRow`, `judgeMutation` Python branch, `rangeHunkSpans` `*.py`; `internal/liveverify/pymutate` (`Judge`, `Run`, test-file recognition, `planMutations`, `mergeCompoundOperators`, `generateMutants`, `findInterpreter`, `classifyRun`, `baselineDetail`); `Export.Dir`, `Export.Scratch`, `Export.Unsandboxed`, `Export.Run` (`internal/liveverify/mutate/shared.go`) | `TestProveChangeModeJudgesPythonTestRowsUnderMutate` (NOT_RUN without the flag, PASS with `detail` naming `swap-binary at calc.py:5`, tree digest), `TestMutationClaimSeparatesLanguagesAndTests`, `TestMutationVerdictVocabulary` (a `test_*.py` path), `TestAffectedRowMakesNoMutationClaimForAChangedTest`; package tests in `pymutate` (`TestRunKillsAMutantAndLeavesTheRootUntouched`, `TestRunReportsSurvivedWhenTheTestAssertsNothing`, `TestRunNeverCreditsAMutantThatDoesNotParse`, `TestRunReportsNoMutantsOutsideTheChangedLines`, `TestRunReportsUnsupportedWhenTheBaselineFails` (red and offline), `TestRunReportsUnsupportedWithoutPytest`, `TestFindInterpreterReportsUnavailablePythonAndPytest`, `TestAllUnaskableMutantsRemainUnjudged`, `TestRunRejectsCallerMistakes`, `TestClassifyRunSeparatesACollectionErrorFromARedTest`, `TestTestFunctionNamesSelectsOnlyTheClaimedTests`, `TestBaselineDetailSeparatesOfflineFromRed`, `TestPlanMutationsListsOperatorsInSourceOrder`, `TestRenderedMutantsAreSingleEdits`, `TestPlanNeverDeletesADefinition`, `TestGenerateMutantsConfinesToTheGivenLines`); bench snapshot run in `docs/BUILD-LOG.md` 2026-09-01 |
| FPK-V0-020 | implemented experimentally in `prove_checkpoint.go` and `contextindex/checkpoint.go`; focused tests pass, full gate and Claude review pending | `TestCheckpointDocumentSchemaIsValidated`, `TestCheckpointEntryBoundsAndExactDuplicateCollapse`, `TestCheckpointHandlePrecedenceAndCriticalGates`, `TestCheckpointCallerProseAndVerificationAreInert`, `TestCheckpointHistoryFlagsAreFactsAndDirtyDigestHashesNames`, `TestCheckpointByteBoundAndCriticalBoundAreIndependent`; MISSING: absent `repository.object_format`, sha256 document with a 40-hex `blob_hash`, and an independently constructed dirty-path digest literal |
| FPK-V0-021 | implemented experimentally in `prove_checkpoint.go` and `contextindex/checkpoint.go`; focused tests pass, full gate and Claude review pending | `TestCheckpointArgumentDispatchAndExclusivity`, `TestCheckpointHandlePrecedenceAndCriticalGates`, `TestCheckpointCurrentIdentityAuthorityAndLineMovement`, `TestCheckpointRefusesAnObjectFormatMismatch`, `TestCheckpointHistoryFlagsAreFactsAndDirtyDigestHashesNames`, `TestCheckpointReadsNoIndexSnapshotAndRetainsNothing`, `TestCheckpointRejectsSameTreeHEADDrift`, `TestCheckpointAuthorityFlagsAreIndependentOfDirtyVerdict`, `TestReadBoundedFileRefusesFIFO`, `TestReadBoundedFileRefusesDirectory`; MISSING: a positive `tree_moved` assertion |
| FPK-V0-022 | implemented experimentally in `prove_checkpoint.go` and `contextindex/checkpoint.go`; focused tests pass, full gate and Claude review pending | `TestCheckpointEntryBoundsAndExactDuplicateCollapse`, `TestCheckpointHandlePrecedenceAndCriticalGates`, `TestCheckpointCurrentIdentityAuthorityAndLineMovement`, `TestCheckpointSelectorsMatchEnclosingResultAndAllRows`, `TestCheckpointRehydratesImpactTestMarkerEvidence`, `TestCheckpointByteBoundAndCriticalBoundAreIndependent`; MISSING: the exact recovery-operation content (the current test checks only that recovery is nonempty) |
| FPK-V0-023 | implemented experimentally in `prove_checkpoint.go` and `contextindex/checkpoint.go`; focused tests pass, full gate and Claude review pending | `TestCheckpointCallerProseAndVerificationAreInert`, `TestCheckpointObligationsSurvivePartialWorkAndAuthorityDrift`, `TestCheckpointForgedAndEmptyObligationsAreInert`; MISSING: an independent non-execution assertion for command-like `task` prose |
| FPK-V0-024 | implemented experimentally in `prove_checkpoint.go` and `contextindex/checkpoint.go`; focused tests pass, full gate and Claude review pending | `TestCheckpointDocumentSchemaIsValidated`, `TestCheckpointEntryBoundsAndExactDuplicateCollapse`, `TestCheckpointArgumentDispatchAndExclusivity`, `TestCheckpointRefusesAnObjectFormatMismatch`, `TestCheckpointUnreadableFilesAndRefusalPrecedence`, `TestCheckpointOutputFailureMayLeavePartialBytes`, `TestCheckpointReadsNoIndexSnapshotAndRetainsNothing`, `TestCheckpointSourceNeverReferencesSnapshotReaders`, `TestCheckpointRejectsSameTreeHEADDrift`, `TestCheckpointRefusalsAreEnumerated`, `TestPlainProveCodelessBuildErrorStderrIsUnchanged`, `TestCheckpointByteBoundAndCriticalBoundAreIndependent`, `TestReadBoundedFileRefusesFIFO`, `TestReadBoundedFileRefusesDirectory` |
| FPK-V0-025 | implemented experimentally in `prove_checkpoint.go` and `contextindex/checkpoint.go`; focused tests pass, full gate and Claude review pending | `TestCheckpointReadsNoIndexSnapshotAndRetainsNothing` (retention digest and caller-file reread) |
| FPK-V0-026 | implemented experimentally in `prove_checkpoint.go` and `contextindex/checkpoint.go`; focused tests pass, full gate and Claude review pending | `TestCheckpointVerdictBytesAreReproducible`, `TestCheckpointCallerProseAndVerificationAreInert`, and the FPK-V0-021/022/024 tests named above |
| FPK-V0-027 | `proofClaims`, `proveState`, `citedSyntaxOnly` | `TestProveEmbedsTheQueryPacketUnchangedAndWritesNothing` and `TestProveCEMAcceptedMapProvesSupportedHunksAndWritesNothing` (exact always-present legend and rerun byte identity), `TestProveVocabularyOnlyPacketIsCitedNotProven` (`CITED` with zero authoritative results) |
| FPK-V0-028 | `proveRow.Witness`, `mutationWitnessFromReport`, `citedPaths`; `mutate.Report.Witness`, mutation site byte spans, grouped test attribution | `TestProveMutationWitnessReplaysKillOnSecondCheckout` (second checkout, original blob, baseline pass, recorded operator/span, exact named-test kill), `TestMutationWitnessDisclosesNotProducedWhenRunnerUnavailable`, `TestMutationBudgetExhaustionDisclosesTypedWitness`, `TestMutationInputUnavailableDisclosesTypedWitness`, `TestNoMutantInRangeDisclosesTypedWitness`; 19-of-20 second-checkout acceptance cohort `NOT_RUN` because the repository has one focused replay fixture, not the required 20-witness cohort |
| FPK-V0-029 | `documentStatus` (`internal/contextindex/parse.go`) and its oracle twin in `src/context_corvint_index.py`; consumed by `documentAuthority` | `TestDocumentStatusReadsEveryFieldShapeAndCapturesTheTokenAlone`, `TestDocumentStatusIsAnchoredAndHeadingsTruncateByRune`, `test_document_status_reads_every_field_shape_and_captures_the_token_alone`, `conformance/cli-parity-v0` with unchanged stdout digests |
| FPK-V0-030 | experimental prototype: `CEMStatement`, `VerifyCEM`, `ErrCEMBytesMismatch` (`internal/attest/cem.go`); wired by FPK-V0-031 | `TestCEMAttestationRoundTripsThroughTheExistingEnvelope` (predicate type, single subject with independently computed digest, no embedded bytes, `VERIFIED`), `TestCEMAttestationRefusesTamperedMapBytes` (flipped, empty, and appended bytes refused; non-CEM input not attested), `TestCEMAttestationDisclosesMissingMapBytes` (`NOT_RUN` with reason), `TestCEMAttestationRefusesASignedStatementThatIsNotACEMClaim` (validly signed statement with another `_type` or `predicateType`, zero or two subjects, or a subject name or digest that disagrees with the predicate refused with and without bytes, never as a byte mismatch), `TestCEMAttestationRefusesAMalformedSignedClaim` (validly signed empty, 63-digit, and uppercase sha256, negative size, empty spec, and empty name refused with and without bytes), `TestCEMAttestationRefusesADuplicateStatementMember` (repeated `predicateType` and predicate `size`), `TestCEMAttestationRefusesACaseVariantStatementMember` (`PredicateType` beside `predicateType`, predicate `Spec` beside `spec`), `TestCEMAttestationAcceptsUnknownStatementMembers` (undefined top-level and predicate members still `VERIFIED`); each refusal test fails with its refusal branch disabled |
| FPK-V0-031 | experimental prototype: `attestProof`, `cemAttestationInput`, `parseProveCEMArguments` (`--attest-cem`), `parseProveVerifyCEMArguments`, `runVerifyCEMAttestation`, `verifyCEMAttestation` (`cmd/corvint/prove_attest_cem.go`), `attest.ReadPublicKey` | `TestProveCEMAttestOutputIsUnchangedWithoutAttestCEM` (`--attest` and `--attest-key` bytes equal the FPK-V0-015 statement and envelope rebuilt in-test, and are the first of two lines under `--attest-cem`), `TestProveCEMAttestationRoundTripsThroughTheCLI` (emit, verify `VERIFIED` with independently computed digest and size, tree digest unchanged), `TestProveCEMAttestationVerifyRefusesAChangedMap` (exit 2 `attest-cem-mismatch`, empty stdout), `TestProveCEMAttestationVerifyDisclosesMissingMapBytes` (`NOT_RUN` with reason), `TestProveCEMAttestationVerifyRefusesAnUnusablePublicKey` (missing, private-key PEM, and over-16-KiB key exit 2 `attest-public-key-unavailable`, empty stdout), `TestProveCEMAttestationVerifyRefusesAnUnreadableEnvelope` (missing envelope and the valid envelope whitespace-padded past 8 MiB exit 2 `attest-envelope-unavailable`), `TestProveCEMAttestationVerifyRefusesAnEnvelopeThatDoesNotVerify` (other signer's key, changed `payloadType`, changed payload, and the same-key FPK-V0-015 envelope exit 2 `attest-verification-failed`), `TestProveCEMAttestationVerifyRefusesInvalidArguments` (missing key flag, empty `--cem=`, repeated and unknown flags exit 2 `invalid-arguments`), `TestProveCEMAttestCEMRefusesInvalidArguments` (`--attest-cem=yes`, repeated `--attest-cem`, and `--attest-cem` in impact mode exit 2 `invalid-arguments`, empty stdout), `TestProveCEMAttestationVerifyRefusesAnUnavailableMap` (absolute, climbing with the signed map present at its target, missing, and over-4-MiB `--cem` exit 2 `map-unavailable`), `TestProveCEMAttestationVerifyRefusesASymlinkedOrCaseFoldedGitMap` (a symlinked parent, whether it resolves inside or outside `--root`, and a path component that case-folds equal to `.git`, exit 2 `map-unavailable`), `TestProveCEMAttestFailsWhenTheCEMStatementCannotBeBuilt` (`attestProof` with an attestable proof document and an empty CEM name or non-CEM bytes returns `attest-failed` and no output; the CLI reaches this branch only after the CEM verifier accepted the map, so the test calls `attestProof` directly), `TestReadBoundedFileRefusesFIFO`, `TestReadBoundedFileRefusesDirectory`; each refusal test fails with its refusal branch disabled |

### 2026-09-12 literal marker audit

This audit classifies the 55 pre-audit literal occurrences in file order as A (an existing local
exercise), B (a focused test added by this audit), or C (not runnable in this worker). Protocol
vocabulary and expected wire values remain unchanged; PASS and FAIL below are observations of the
named exercise, not replacements for those wire literals. Locations are the pre-audit line numbers.

The C3-C6 exit-1 observations below and the nine markers first classed C for a denied nested
`sandbox-exec` were recorded inside a Codex workspace-write sandbox; rerun on 2026-09-12 outside
that sandbox, every one of them passes with a real domain verdict, so those nine markers move to A
and C3-C6 are corrected to their outside-sandbox exit.

Commands and observed exits on 2026-09-12:

- C1 (exit 0): `GOTOOLCHAIN=local go test -count=1 -timeout 30m ./cmd/corvint -run '^(TestProveEmbedsTheQueryPacketUnchangedAndWritesNothing|TestProveVocabularyOnlyPacketIsCitedNotProven|TestFalsifyRowVerdicts|TestJudgeReferenceVerdicts|TestFalsifierAssignmentByKindAndLanguage|TestPythonReferenceRowIsNotRun|TestProveImpactJudgesJavaScriptImportAndReferenceRows|TestAffectedRowMakesNoMutationClaimForAChangedTest|TestMutationClaimSeparatesLanguagesAndTests|TestConfinedSpansAdmitNothingForAPathWithoutHunks|TestJudgeMutationFailsAMisplacedTestBlobWithoutRunning|TestMutationVerdictVocabulary|TestMutationWitnessDisclosesNotProducedWhenRunnerUnavailable|TestMutationBudgetExhaustionDisclosesTypedWitness|TestMutationInputUnavailableDisclosesTypedWitness|TestNoMutantInRangeDisclosesTypedWitness|TestProveObserveRecordsOnlyTheVerdictCounts|TestProveCEMAttestationVerifyDisclosesMissingMapBytes)$'`
- C2 (exit 0): `GOTOOLCHAIN=local go test -v -count=1 -timeout 30m ./internal/liveverify/mutate -run '^(TestRunUnderAnExhaustedBudgetNeverErrorsAndCleansUp|TestRunReportsUnsupportedForATestInAnotherModule|TestRunReportsUnsupportedWithoutASandbox|TestClassifyRunSeparatesABuildFailureFromARedTest|TestPlanNeverDeletesADefinition)$'`
- C3 (exit 0 outside a nested sandbox; exit 1 inside one): `GOTOOLCHAIN=local go test -v -count=1 -timeout 30m ./internal/liveverify/mutate -run '^TestRunReportsNoMutantsForADeclarationOnlyFile$'`
- C4 (exit 0 outside a nested sandbox; exit 1 inside one): `GOTOOLCHAIN=local go test -v -count=1 -timeout 30m ./internal/liveverify/mutate -run '^TestRunReportsUnsupportedWhenTheBaselineFails$'`
- C5 (exit 0 outside a nested sandbox; exit 1 inside one): `GOTOOLCHAIN=local go test -v -count=1 -timeout 30m ./internal/liveverify/mutate -run '^TestRunNeverCreditsAMutantThatDoesNotCompile$'`
- C6 (exit 0 outside a nested sandbox; exit 1 inside one): `GOTOOLCHAIN=local go test -v -count=1 -timeout 30m ./internal/liveverify/mutate -run '^TestRunReportsSurvivedWhenTheTestAssertsNothing$'`
- C7 (exit 0): `GOTOOLCHAIN=local CORVINT_TEST_EXTERNAL_PYTEST=1 go test -v -count=1 -timeout 30m ./internal/liveverify/pymutate -run '^(TestFindInterpreterReportsUnavailablePythonAndPytest|TestAllUnaskableMutantsRemainUnjudged|TestClassifyRunSeparatesACollectionErrorFromARedTest|TestBaselineDetailSeparatesOfflineFromRed)$'`
- C8 (exit 0): `GOTOOLCHAIN=local go test -v -count=1 -timeout 30m ./internal/attest -run '^(TestCEMAttestationRoundTripsThroughTheExistingEnvelope|TestCEMAttestationRefusesTamperedMapBytes|TestCEMAttestationDisclosesMissingMapBytes|TestCEMAttestationRefusesASignedStatementThatIsNotACEMClaim|TestCEMAttestationRefusesAMalformedSignedClaim)$'`
- C9 (exit 0): `GOTOOLCHAIN=local go test -v -count=1 -timeout 30m ./cmd/corvint -run '^TestProveImpactMutationIsOptInAndKillsWithAStrongTest$'`
- C10 (exit 0): `GOTOOLCHAIN=local go test -v -count=1 -timeout 30m ./cmd/corvint -run '^TestProveChangeModeAddsAffectedTestRowsAndJudgesThemUnderMutate$'`
- C11 (exit 0): `GOTOOLCHAIN=local go test -v -count=1 -timeout 30m ./cmd/corvint -run '^(TestProveChangeModeSummarizesCoveragePerChangedPath|TestProveChangeModeReportsAnUncoveredPathAsOneFailedResult)$'`
- C12 (exit 0): `GOTOOLCHAIN=local go test -v -count=1 -timeout 30m ./internal/liveverify/mutate -run '^TestRunJudgesACrossPackageTest$'`
- C13 (exit 0): `GOTOOLCHAIN=local CORVINT_TEST_EXTERNAL_PYTEST=1 go test -v -count=1 -timeout 30m ./cmd/corvint -run '^TestProveChangeModeJudgesPythonTestRowsUnderMutate$'`

| Marker/location | Class | Observed result or precise reason |
|---|---|---|
| N01/L17 | A | PASS, C1: verdict vocabulary and packet rows |
| N02/L28 | A | PASS, C1: checked versus unchecked rows |
| N03/L37 | C | Historical pre-amendment OAuth observation; the current branch cannot reproduce its retired assignment table |
| N04/L69 | A | PASS, C1: closed verdict vocabulary |
| N05/L71 | A | PASS, C9: `TestProveImpactMutationIsOptInAndKillsWithAStrongTest`'s `--mutate` (opt-in) branch — PASS with `detail` containing `killed` |
| N06/L82 | A | PASS, C1: dirty and newline-bearing citation paths |
| N07/L83 | A | PASS, C1: `none` row remains unjudged |
| N08/L160 | A | PASS, C1: dirty Go declaring blob |
| N09/L190 | B | PASS, C1: `TestPythonReferenceRowIsNotRun` |
| N10/L207 | A | PASS, C1: dirty JavaScript declaring blob |
| N11/L216 | A | PASS, C9: `TestProveImpactMutationIsOptInAndKillsWithAStrongTest`'s no-flag branch — `test-kills-mutant`/`NOT_RUN`, no `detail` |
| N12/L233 | A | PASS, C2/C3/C4: budget, no-mutant, and red-baseline arms each report a typed verdict |
| N13/L238 | A | PASS, C5: runner classifies the uncompilable mutant without crediting it |
| N14/L240 | A | PASS, C1: dirty mutation input |
| N15/L244 | A | PASS, C1/C2: invocation and export budget branches |
| N16/L259 | A | PASS, C2: unavailable sandbox returns an unjudged row without running a test |
| N17/L291 | A | PASS, C3: runner returns the no-mutant result for a file with nothing mutable |
| N18/L292 | A | PASS, C1: a path absent from the hunk map admits no mutation |
| N19/L293 | A | PASS, C2: test and changed file in different Go modules |
| N20/L294 | A | PASS, C10: `TestProveChangeModeAddsAffectedTestRowsAndJudgesThemUnderMutate`'s no-flag branch — affected-test row is `NOT_RUN` before `--mutate` |
| N21/L300 | A | PASS, C11: `TestProveChangeModeSummarizesCoveragePerChangedPath` and `TestProveChangeModeReportsAnUncoveredPathAsOneFailedResult` — per-path `covered_by`/`reached_not_covering`/`not_run` counts |
| N22/L311 | A | PASS, C1: dirty changed path is left unjudged |
| N23/L328 | A | PASS, C1: ledger and proof counts exclude unjudged/`none` rows |
| N24/L356 | B | PASS, C7: absent Python and absent pytest branches |
| N25/L376 | A | PASS, C1: mutation verdict mapping |
| N26/L377 | A | PASS, C7: offline and red baseline detail mapping |
| N27/L903 | A | PASS, C1: unavailable runner witness |
| N28/L907 | C | Requires at least 19 successful replays from a 20-witness cohort; only one focused replay fixture exists |
| N29/L939 | A | PASS, C8: signed CEM claim without supplied bytes |
| N30/L963 | A | PASS, C1: CLI verification without map bytes |
| N31/L1029 | A | PASS, C1: dirty/unframable failure-mode row |
| N32/L1037 | A | PASS, C9: `TestProveImpactMutationIsOptInAndKillsWithAStrongTest`'s no-flag branch — `test-kills-mutant`/`NOT_RUN`, no `detail` |
| N33/L1039 | A | PASS, C5: runner completes the all-uncompilable classification, row `NOT_RUN` with `detail` |
| N34/L1042 | A | PASS, C2/C3/C4: budget-exhausted, no-mutant, and red-baseline arms each report a typed `NOT_RUN`/`UNSUPPORTED` verdict |
| N35/L1043 | A | PASS, C1/C2: typed invocation-budget result and detail |
| N36/L1045 | A | PASS, C1/C2: unavailable-runner row and witness reason |
| N37/L1050 | A | PASS, C1: changed/cross-language test rows receive `none` |
| N38/L1058 | A | PASS, C8: malformed signed CEM claims are refused, not reported as unjudged |
| N39/L1060 | A | PASS, C8: missing CEM bytes status and reason |
| N40/L1067 | A | PASS, C1: CLI missing-map status and reason |
| N41/L1073 | A | PASS, C13: `TestProveChangeModeJudgesPythonTestRowsUnderMutate`'s no-flag branch (needs `CORVINT_TEST_EXTERNAL_PYTEST=1`) — `test-kills-mutant`/`NOT_RUN`, no `detail` |
| N42/L1074 | B | PASS, C7: absent Python and absent pytest details |
| N43/L1075 | B | PASS, C7: all unparseable/uncollectable/timed-out mutants remain unjudged |
| N44/L1076 | A | PASS, C7: red, offline, collection, timeout, and usage-error baseline details |
| N45/L1077 | A | PASS, C1: cross-language mutation claim is rejected |
| N46/L1108 | B | PASS, C1: `mutation-budget-exhausted` witness reason |
| N47/L1109 | B | PASS, C1: `mutation-input-unavailable` witness reason |
| N48/L1110 | B | PASS, C1: `no-mutant-in-range` witness reason |
| N49/L1125 | A | PASS, C10/C12: `TestProveChangeModeAddsAffectedTestRowsAndJudgesThemUnderMutate` (cmd/corvint) and `TestRunJudgesACrossPackageTest` (internal/liveverify/mutate) |
| N50/L1128 | C | Replay requires `CORVINT_QUERY_PARITY_CORVINT_ROOT` and `CORVINT_QUERY_PARITY_BEAMFALL_ROOT`; their pinned repositories are not fixtures on this branch |
| N51/L1133 | A | PASS, C9: `TestProveImpactMutationIsOptInAndKillsWithAStrongTest` (NOT_RUN without `--mutate`, PASS with `--mutate`, tree digest unchanged) |
| N52/L1137 | A | PASS, C13: `TestProveChangeModeJudgesPythonTestRowsUnderMutate` (needs `CORVINT_TEST_EXTERNAL_PYTEST=1`; NOT_RUN without `--mutate`, PASS with `--mutate`) |
| N53/L1146 | C | Same unproduced 20-witness acceptance cohort as N28; one fixture cannot establish 19-of-20 |
| N54/L1148 | A | PASS, C8: missing-map-bytes result and malformed-claim refusal |
| N55/L1149 | A | PASS, C1: CLI missing-map-bytes result |

Compatibility and drift: the packet grammar is consumed, not redefined; if `query` gains or loses a
member, `prove` embeds the new packet as-is. The authority table must be extended in the same
commit as any new authority string in `internal/contextindex`, otherwise the new rows silently
become `none`. Unresolved decisions: whether `history-consistent` should also demand that no later
commit than the packet's touched the span (the packet revision is a tree, so "later" has no
meaning today; deferred with the owner); whether to fold the verdicts into the packet once the
Python oracle is retired; whether `abstention.nearest_claims` rows should be judged; whether an
import row should also prove that the import path names the package containing the changed file
(needs the module path at the packet revision); whether `reference-resolves` should bind
identifiers with `go/types` rather than match occurrences; whether CI should relax Ubuntu 24.04's
AppArmor restriction on unprivileged user namespaces so `bwrap` can run the mutation tests there.

Rollout: ships in the experimental Go candidate; no adapter calls it. Rollback: delete
`cmd/corvint/prove.go`, its tests, `internal/liveverify/pyresolve`, `internal/liveverify/mutate`,
`internal/liveverify/pymutate`,
`internal/attest`, the help entries, this spec, and its README/INDEX rows.
Promotion or kill: promote only when the three-arm trial in the bet reports a materially lower
confidently-wrong rate at non-inferior success; kill if the falsifiable packet costs more than 25%
extra tokens for no success gain, per the bet's stated criteria.
