# Falsifiable Packet V0

Owner: Russell Lewis
Date: 2026-09-01
Requirement prefix: `FPK-V0`
Intent status: accepted (decision 0052)
Delivery status: experimental
Revision status: revision 34 (FPK-V0-920 reports a `prove` status read that outlasts its Git deadline as `repository-probe-timeout`, V1-3042, experimental under the standing lane auto-accept); revision 33 (FPK-V0-142 retries one refused nesting preflight once and carries the launcher's quoted refusal line in the `UNSUPPORTED` detail, since a loaded Linux host's transient `bwrap` `EAGAIN` reads as a nesting refusal, V1-2940, standing lane auto-accept of 2026-10-11; revision 32 FPK-V0-143 reports a Go baseline run that go test's own `-timeout` ended as `BUDGET_EXCEEDED` naming the limit, V1-2199, standing lane auto-accept of 2026-10-11; revision 31 FPK-V0-142 keeps the Go mutation runner's cold baseline build out of the per-run kill, reports a warm per-run kill as `BUDGET_EXCEEDED` naming the limit, finds an enclosing sandbox's refusal once by a preflight, and makes the test budget overridable, V1-1483/1484/1485/1577, standing lane auto-accept of 2026-10-11; revision 30 FPK-V0-078 refuses a container workspace that resolves to, under, or above the repository root and a container CLI over the runner's 256 MiB executable bound, hashing the CLI with a bounded streaming read; FPK-V0-080 gives a containerised run whose report the runner's parser rejects or observes as incomplete no verdict, with a typed `report_error`, Codex review of V1-2116, standing lane auto-accept of 2026-10-11; revision 29 FPK-V0-078 to FPK-V0-081 let Ruby mutation rows run through a declared containerised RSpec runner (`--mutate-runner`, the corvint-test-runner rspec request with an exec prefix and path map, TRE-V0-081 to TRE-V0-086), one disposable copy and one admitted runner receipt per run, the host path unchanged without a declaration, GitHub #1354 and V1-2116, standing lane auto-accept of 2026-10-11; revision 28 FPK-V0-074 to FPK-V0-077 let `--mutate` judge a named, possibly unchanged, `.rb` source against cited RSpec examples (`--mutate-source`, `--mutate-example`), running only those examples and naming the line basis, GitHub #1354 and V1-2115, standing lane auto-accept of 2026-10-11; revision 27 FPK-V0-068 to FPK-V0-073 add a Ruby/RSpec mutation engine under `--mutate`, with per-mutant verdicts, retained receipts and typed witnesses, GitHub #1321 and V1-2094, standing lane auto-accept of 2026-10-11, and narrow FPK-V0-057 to minitest and TypeScript claims; revision 26 FPK-V0-057 gives a `--mutate` claim in a language without a mutation engine, such as an RSpec example claiming a changed `.rb` source, an explicit `UNSUPPORTED` witness, GitHub #1321, standing lane auto-accept of 2026-10-11; revision 25 decision 0393 accepts FPK-V0-052 and sets its discovery ceiling only for a root that holds its own `.git` entry; revision 24 FPK-V0-052 makes every `cmd/corvint` Git read ignore repository-configured programs and replace refs; revision 23 FPK-V0-050 adds `prove --cem ... --attest-cem-v1`, the only command that emits the `cem/v1` predicate, and FPK-V0-051 replaces the UNCONFIRMED field-name alignment with a comparison against the fetched OpenFab `openfab/generation` v0.1 draft and agentattest predicate v1, adopting none of their fields; revision 22 FPK-V0-041 to FPK-V0-049 add an explicit, bounded, secret-screened `prove --export-bundle` failure-reproduction bundle for query and checkpoint results, a `prove --replay-bundle` that recomputes it on a second checkout or refuses with a named reason, and a `historical` marker that `prove-observe` refuses; revision 21 FPK-V0-033 to FPK-V0-036 add a versioned `cem/v1` in-toto CEM predicate that also binds the base revision and patch, its verifier, a digest-pinned interop consumer with the known deviations from the OpenSSF generation draft, and an optional external Sigstore step; revision 20 FPK-V0-037 to FPK-V0-040 add an explicit `cem anchor` notes-ref mutation for a committed CEM and a read-only `cem provenance` that reads the anchor, a foreign Git AI `refs/notes/ai` note, and `Assisted-by`/`Agent-Logs-Url` trailers as untrusted `repository-history`; revision 19 FPK-V0-032 stamps one trust class on every proof row and refuses a tainted row as a basis; revision 18 FPK-V0-015 and FPK-V0-030 refuse repeated and case-variant JSON member names in a signed envelope and CEM statement; revision 17 FPK-V0-030 refuses a signed CEM claim `CEMStatement` could not have produced; revision 16 restated FPK-V0-021 and FPK-V0-024 rationale against the implemented checkpoint branch)
Authoritative inputs: `docs/plans/BREAKTHROUGH-BET-2026-09-01.md` (the bet this is slice 1 of),
`docs/reviews/FABLE-5.1-AUDIT-2026-09-01.md` (F2, F3), `docs/specs/go-production-kernel-migration-v0.md`
(GPK-V0-002 exact parity of the `query` wire), `conformance/cli-parity-v0` (byte-exact replay of committed
expectations; its Python oracle was retired by decision 0088), `docs/decisions/0063-vocabulary-rows-carry-a-citation-check-2026-09-05.md`
(accepted prototype), `docs/reviews/b2-evidence-breakthroughs.md` (B1), and `AGENTS.md` invariants
1, 2, 4, and 8.

## Agent digest
- Claim: `corvint prove` embeds the unchanged query packet and attaches a mechanical falsifier and PASS/FAIL/NOT_RUN verdict beside every evidence row.
- Status: accepted (decision 0052)/experimental
- Exists: `cmd/corvint/prove.go` and its test files, `internal/liveverify/pyresolve`, `internal/liveverify/jsresolve`, `internal/liveverify/mutate`, `internal/liveverify/pymutate`, `internal/liveverify/rbmutate` (FPK-V0-068 to FPK-V0-073, experimental), `cmd/corvint/prove_named.go` (FPK-V0-074 to FPK-V0-077 named Ruby source, experimental), `cmd/corvint/prove_container.go` and `internal/liveverify/rbmutate/container.go` (FPK-V0-078 to FPK-V0-081 containerised Ruby runner, experimental), `internal/attest`, help topic `prove`; `cmd/corvint/prove_trust.go` (FPK-V0-032 trust class, proposed); `cmd/corvint/prove_bundle.go` (FPK-V0-041 to FPK-V0-049 failure bundles, proposed). V1-0408: `internal/revieworder` is a pure experimental library projection, with no CLI; FPK-V0-053..056 are accepted, decision 0515.
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
  (`cmd/corvint/index_snapshot.go:22-25@8984cf3b`), and the observed sibling `loadSnapshotObserved = contextindex.LoadSnapshotObserved`
  (`cmd/corvint/index_snapshot.go:274-276@097dbc58`, `LCP-V0-016`). The deferred seam's one call is `deferredSnapshotIndex` (`cmd/corvint/index_snapshot.go:97-99@5f8b0e86`). The
  snapshot seams' seven current production call sites are `snapshotIndex` (`cmd/corvint/index_snapshot.go:62-64@01cf3343`) and batch (`cmd/corvint/batch.go:149@dbef447a`),
  answerability (`cmd/corvint/answerability.go:94-95@6278a445`) and surprise (`cmd/corvint/surprise.go:118-119@6278a445`),
  context lookup (`cmd/corvint/context_lookup.go:75-79@9f1de421`) and local completion events (`cmd/corvint/local_completion_event.go`, `localEventContext`),
  and the experimental host adapter (`cmd/corvint/host_adapter_experimental.go:48-53@b6d4dd7c`). The task-context path instead uses its separate
  `loadContextSnapshot` seam (`cmd/corvint/taskcontext.go:321-328@73708428`), backed by `LoadContextSnapshotDeferred` and the private loader (`internal/contextindex/observed_build.go:42-49@d916a414`).
  The `cmd/corvint` seam does not cover `LoadEventSnapshot` or `ProbeSnapshot`, called directly by the harness and index paths (`cmd/corvint/harness_context.go:35-37@44bd361a`, `cmd/corvint/index_snapshot.go:144-145@9a7d60f2`); the harness calls `LoadEventSnapshotDeferred` there too.
  The load-bearing guard scans every non-test Go file in `cmd/corvint`, rejects direct `LoadSnapshot`, `LoadSnapshotDeferred` or `LoadSnapshotObserved` references outside their seam bindings, and additionally rejects `LoadEventSnapshot`, `LoadEventSnapshotDeferred` and `ProbeSnapshot` in `prove*` files
  (`cmd/corvint/prove_checkpoint_test.go:739-806@c3f154c6`); the counting test wraps all three dynamic seams (`loadSnapshot`, `loadSnapshotDeferred`,
  `loadSnapshotObserved`; `cmd/corvint/prove_checkpoint_test.go:649-671@1ad54cf1`) and asserts the
  checkpoint run traverses none of them (`cmd/corvint/prove_checkpoint_test.go:713-717@7c769b06`)
  (FPK-V0-024).
  End of accepted amendment.
- **FPK-V0-008:** Failures MUST be typed and exit 2 with nothing on stdout: query compile errors
  pass through with their own codes; `unsupported-prove-revision` (no Git, no HEAD tree, no
  packet revision); `unsupported-prove-history` (status or `cat-file` failure, bound exceeded,
  unreadable batch stream); `unsupported-prove-drift`; `invalid-arguments` for anything the query
  parser rejects.
  Accepted amendment (AT-06, decision 0052): the
  "nothing on stdout" guarantee binds refusals only. An output-transport failure is not a refusal:
  encoding and the stdout write both happen after every verdict is decided, either can fail, and
  both exit 2 with `output-failed` (`cmd/corvint/prove.go:575-579@4c0799f8`, `cmd/corvint/prove.go:587-589@ea3220b5`). A failed write
  MAY leave partial bytes on stdout, so a consumer MUST read the exit status, never stdout
  emptiness, as the signal that no verdict was produced — the same exit-2 signal the harness
  gives for its own write failure (`cmd/corvint/main.go:1405-1407@f3b5fd7c`), which the adapter contract
  converts into a visible host-valid no-op (`docs/specs/agent-harness-integration-v0.md:68-69`).
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
  `unsupported-prove-cem` (`cmd/corvint/prove.go:1021@975538a3`).
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
  mode, checkpoint included (`cmd/corvint/prove.go:570-573@6e7d631a`), but this clause's own byte-identity
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
  emits (`internal/contextindex/impact.go:646-648@fd0a67cc`) minus `confidence`, plus the result's
  `kind`/`id`, as FPK-V0-002's rows carry; `critical`, at most 256, selectors of the same shape
  naming handles that MUST survive; `unknowns` and `failed_approaches`, free text; `verification`,
  `[{command, observed_status, provenance}]`; and `provenance` `{receiptId?, packet_sha256?}`.
  Keys are snake_case with one compatibility exception: `receiptId` keeps the camelCase spelling of
  the accepted harness response key it mirrors (`internal/gokernel/harness.go:509@cc998e6b`); every other
  key, `packet_sha256` included, is snake_case. A `receiptId` is a request-identity hash, not proof
  of possession (`internal/gokernel/harness.go:495-499@a9b1d3f6`); a checkpoint MUST NOT claim it establishes
  that a handle was read. `repository.dirty_paths_sha256` is the lowercase hex SHA-256 of the
  canonical-JSON encoding of the sorted dirty path list `affected.DirtyPaths` returns
  (`internal/liveverify/affected/dirty.go:46@abebde5b` is the entry point; the deduplication and sort happen
  in `DecodeStatus` at `internal/liveverify/affected/dirty.go:245@a543bfa1` via `NormalizePaths`,
  `internal/liveverify/affected/select.go:1435-1448@eadff8fe`), by the same construction
