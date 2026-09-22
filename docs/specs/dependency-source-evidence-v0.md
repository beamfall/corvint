# Dependency Source Evidence V0

Owner: Russell Lewis
Frozen: 2026-09-11
Requirement prefix: `DSE-V0`
Intent status: proposed
Delivery status: experimental
Authoritative inputs: `AGENTS.md` invariants 1, 2, 4 and 7,
`docs/specs/go-production-kernel-migration-v0.md` (verb and error conventions),
`internal/contextindex/git.go` (committed-object reads and sanitized Git execution),
`golang.org/x/mod/sumdb/dirhash` as vendored in the Go toolchain source (the `h1:` algorithm).

## Agent digest
- Claim: `corvint depsource` cites third-party Go module source with the same pinning discipline as repository code, or abstains.
- Status: proposed/experimental
- Exists: `internal/depsource` and `cmd/corvint/depsource.go` are the whole implementation; nothing preceded this slice.
- Blocked on: acceptance of the pin-extension in Non-goals and authority; non-Go ecosystems; dogfood receipts.
- Read next: Pin authority and the proposed extension; Requirements; Failure modes.

## User and measurable job

An agent reviewing a change often needs to read the dependency code the change calls into. Today it
either guesses from memory or reads an unpinned copy out of the local module cache, and neither
answer carries evidence that the bytes are the bytes this repository depends on. This slice adds one
read-only verb that resolves the module version from the committed `go.mod`, locates the extracted
module in the local module cache, recomputes the `h1:` directory hash, compares it to the committed
`go.sum`, and only then emits a bounded listing or one file's content.

Measurable job: a dependency citation carries a checksum that a reader can recompute, and an
unprovable citation is an explicit abstention rather than unlabelled content.

## Pin authority and the proposed extension

Invariant 1 says evidence is pinned to immutable Git content. Third-party module source is not in
this repository's Git object store, so this slice **proposes** extending the pin to content
identified by a `go.sum` `h1:` hash recorded in the committed tree. The extension is narrow: the
hash itself is read from an immutable Git blob, and the module bytes are admitted only when their
recomputed hash equals that blob's recorded value. **This extension is proposed, not accepted.** No
other capability may rely on it, and a `depsource` receipt is not an authority input.

A local `replace` directive needs no extension: the replaced tree is inside this repository, so it
is pinned by Git blob hash exactly as repository code is. A `replace` pointing outside the
repository has no available pin and is refused.

## Requirements

- **DSE-V0-001:** `corvint [--root PATH] depsource <module-path>[@version] [--file REL] [--limit N]`
  MUST be the whole invocation surface. `--limit` is an integer in `[1, 2000]` defaulting to 200. A
  missing module path, an unrecognized flag, an out-of-range `--limit`, an empty `--file`, or a
  `--root` that is not a Git repository MUST exit with status 2 and a `gokernel`-shaped argument
  error on stderr, writing nothing to stdout. `runDepsource(ctx context.Context, arguments []string,
  stdout, stderr io.Writer) int` is the entry point; `ctx` is the process signal context. Clarifying
  amendment (2026-09-13, bug hunt): SIGINT or SIGTERM before the hash verification finishes is that
  same exit-2 Git failure with empty stdout, never an ignored signal followed by an exit-0 answer.
- **DSE-V0-002:** Version resolution MUST read `go.mod` and `go.sum` as blobs of the committed
  `HEAD` tree through a sanitized Git execution environment (no system or global config, no
  credential helper, no lazy fetch), never the worktree copy. A dirty worktree `go.mod` MUST NOT
  change any emitted member. Parsing is a small local tokenizer, not `golang.org/x/mod/modfile`
  (see Non-goals): its grammar accepts bare fields, Go string literals both quoted and backquoted
  decoded through `strconv.Unquote`, parentheses as separate tokens, and an unquoted `//` line
  comment. It models single-line and block forms of `require`, `replace`, and `exclude`, plus
  single-line `module`, `go`, and `toolchain` declarations. A literal the line never closes, one
  `strconv.Unquote` refuses, one that decodes to a structural token, another field abutting it, a
  malformed directive shape, two replacements for the same source pin that disagree on their
  target, an unsupported directive or block comment, and an unexpected, nested, or unterminated
  block MUST each fail the parse rather than admit partial fields or interpret an unsupported
  block's body as evidence. The committed `go.mod` blob MUST be valid UTF-8; invalid bytes fail the
  parse rather than being ignored inside an otherwise resolvable file. Each Git stdout capture MUST
  retain at most 8 MiB while Git runs; an overflow MUST return the existing bounded-read repository
  error without exposing partial bytes.
- **DSE-V0-003:** The module cache location MUST be `<cache>/<escaped-path>@<escaped-version>`,
  where escaping replaces each uppercase letter with `!` plus its lowercase form and refuses a path
  already containing `!`. The cache root comes from an injected option, then `GOMODCACHE`, then
  `go env GOMODCACHE`; the verb MUST NOT mutate process environment to discover it.
- **DSE-V0-004:** The verb MUST recompute the `h1:` hash over every regular file of the extracted
  directory — base64 of the SHA-256 over one `<file sha256 hex><two spaces><module>@<version>/<path>\n`
  line per file, names sorted — and MUST emit `verified: true` only when that value equals the
  committed `go.sum` `h1:` line for the resolved module and version. The `/go.mod` `go.sum` line is
  never the source hash. When `<cache>/cache/download/<escaped-path>/@v/<version>.ziphash` exists it
  MUST also equal that value; its read MUST retain at most 1024 bytes plus one overflow-detection
  byte, and overflow MUST produce `ziphash-mismatch`. For `--file REL`, the verb MUST capture `REL`
  through the same open handle that supplies its bytes to the directory hash and MUST derive the
  excerpt's `sha256`, `size`, and `lines` from that capture; it MUST NOT reopen `REL` after
  verification. Every hashed file MUST be opened inside a root handle on the extracted directory,
  without following a final symlink and without blocking on a FIFO, and its opened descriptor MUST
  be a regular file with the same identity the inventory recorded; any other open fails the hash and
  emits `cache-entry-unreadable`. The ziphash record MUST be opened inside the cache root the same
  way, refusing a record that is a symlink even when its target stays inside the cache root; only an
  absent record is skipped, and an unopenable, unreadable, or non-regular one —
  including one whose presence cannot be determined because the cache root itself cannot be
  opened — produces `ziphash-mismatch`. The hash MUST observe context cancellation before each file and each read,
  and a cancellation MUST fail the verb rather than abstain.
- **DSE-V0-005:** A recomputed hash unequal to the recorded one MUST emit
  `verified: false, reason: "hash-mismatch"` with no `files` and no `file` member, and exit status 0.
- **DSE-V0-006:** A missing extracted directory MUST emit `reason: "missing-cache-entry"`. The verb
  MUST NOT run `go mod download`, open a network connection, or create, extract, or modify anything
  under the module cache.
- **DSE-V0-007:** A module absent from the committed `go.mod`, a `@version` disagreeing with the
  committed one, two differing committed versions for one path, a missing `go.sum` `h1:` line,
  a required version the committed `go.mod` excludes, and a committed `go.mod` line the restricted
  grammar of `DSE-V0-002` refuses MUST abstain with `module-not-required`, `version-mismatch`,
  `ambiguous-version`, `missing-go-sum-entry`, `excluded-version` and `unparsable-go-mod`
  respectively. The `excluded-version` check is exact: it matches only the precise
  `(module, version)` pair resolved via `require` against an `exclude` directive, with no
  minimal-version-selection re-resolution to a different, allowed version. A revision with no
  committed `go.sum`, a committed `go.sum` over the 4 MiB read limit, a non-local `replace` with no
  replacement version, a cache root that none of the `DSE-V0-003` sources supplies, and a module
  path or version that `DSE-V0-003` escaping refuses MUST abstain with `missing-go-sum`,
  `go-sum-exceeds-read-limit`, `replace-without-version`, `module-cache-unavailable`,
  `unescapable-module-path` and `unescapable-module-version` respectively (clarifying amendment,
  2026-09-13, bug hunt: these reasons were emitted but named by no requirement). No abstention
  carries file content.
- **DSE-V0-008:** Without `--file` the answer MUST carry `file_count`, `truncated`, and at most
  `--limit` `files` rows of `path` and `size`, ordered by path. Ordering MUST be deterministic and
  independent of file-system enumeration order.
- **DSE-V0-009:** With `--file REL` the answer MUST carry that file's whole-content `sha256`, its
  `size`, and its bytes as one list entry per line capped at 65536 bytes with `truncated` set when
  the cap applies. Whole-content digest and size computation MUST stream the same open handle while
  retaining at most the 65536-byte excerpt. A `REL` not in the module MUST abstain with
  `file-not-in-module`.
- **DSE-V0-010:** A `replace` whose target is a directory inside the repository tree MUST emit
  `resolution: "local-replace"` with every listing row and any `--file` answer carrying the Git blob
  hash of that committed object, and no `go_sum_hash`. A target outside the tree MUST abstain with
  `replace-outside-repository`; an uncommitted target with `replace-path-not-committed`. A target
  committed as a file or symlink rather than a directory is not a committed module directory and
  MUST abstain with `replace-path-not-committed` (clarifying amendment, 2026-09-13, bug hunt). A
  `--file` blob listed in the committed tree whose read then fails MUST abstain with
  `file-unreadable`.

## Non-goals and authority

No non-Go ecosystem is in scope: npm, PyPI, Cargo and vendored C are out. No network access, no
proxy, no checksum database, and no module download of any kind. A `depsource` answer is never a
ranking, learning, authority, or evidence-admission input to any other verb; it is a read-only
citation aid. The verb adds no module dependency: `golang.org/x/mod` is not in the dependency graph,
so path escaping, the `h1:` algorithm, and `go.mod` parsing are implemented locally (invariant 7);
`go.mod` parsing's restricted grammar is stated under `DSE-V0-002`.

## Failure modes

- The recorded and recomputed hashes disagree because the cache entry was edited in place. Detected
  by `DSE-V0-004`; answered by `DSE-V0-005` with no content.
- A concurrent writer replaces an inventoried cache file, or an ancestor directory, with a symlink
  leading outside the cache, a FIFO, or a different file between the inventory walk and the open.
  Detected by the contained no-follow open and identity match of `DSE-V0-004`; the verb abstains
  with `cache-entry-unreadable` and neither hashes nor excerpts the substituted bytes, and a FIFO
  cannot block it. On platforms other than Darwin and Linux only root containment and the identity
  match apply; a FIFO there can still block the open.
- A caller cancels during hashing of a large or slow cache entry. `DSE-V0-004` stops at the next
  file or read and fails with the context error; no partial answer is emitted.
- The dependency was never fetched on this machine. Answered by `DSE-V0-006`, not by fetching.
- `go.mod` uses a directive form the small local reader does not model. The module then looks
  unparseable and the verb abstains with `unparsable-go-mod`; it never guesses a version or treats
  a line inside an unsupported block as a top-level directive. The same reason covers malformed
  directive arity, unbalanced blocks, and a string literal the reader cannot close or decode. This
  is the explicit precision limit of not depending on `golang.org/x/mod/modfile`.
- A `replace` points at a sibling checkout. No pin exists, so `DSE-V0-010` abstains rather than
  citing mutable bytes.
- `HEAD` has no commit, or Git is unavailable. These are repository failures, not abstentions: exit
  status 2 with a diagnostic.
- A caller sends SIGINT or SIGTERM before the answer compiles. `runDepsource`'s `ctx` is main's
  process signal context, so the in-flight Git blob read fails the same way as any other repository
  failure: exit status 2, empty stdout (clarifying amendment, 2026-09-13, bug hunt).

### Further abstention reasons

The verb also abstains with the kebab-case reasons below (decision 0100), each written as `reason`
with `verified: false`. Each row cites the first emitting site and states only the condition checked
there.

| Code | First emitting site | Condition at the cited site |
|---|---|---|
| `file-unreadable` | `internal/depsource/depsource.go:376@b2a4471b` | reading the committed blob of the `--file` entry in a local replace failed |
| `go-sum-exceeds-read-limit` | `internal/depsource/depsource.go:176@e52367be` | the committed `go.sum` is larger than 4 MiB |
| `missing-go-sum` | `internal/depsource/depsource.go:173@72804a9b` | the committed `go.sum` cannot be read and Git confirms it is absent at the revision |
| `module-cache-unavailable` | `internal/depsource/depsource.go:186@7297a9ab` | the module cache directory cannot be resolved from the option, `GOMODCACHE`, or `go env GOMODCACHE` |
| `replace-without-version` | `internal/depsource/depsource.go:146@b65c968f` | a non-local `replace` for the module names no replacement version |
| `unescapable-module-path` | `internal/depsource/depsource.go:190@693ee6d7` | the target module path is rejected by module-path escaping |
| `unescapable-module-version` | `internal/depsource/depsource.go:194@2b881d09` | the target version is rejected by module-path escaping |

## Traceability

| Requirement | Evidence |
|---|---|
| `DSE-V0-001` | `TestParseDepsourceInvocation`, `TestRunDepsourceArgumentFailure`, `TestRunDepsourceHonorsCancellation`, `TestCLIReadVerbsLeaveTheRepositoryByteIdentical` (CLI-level repository-byte assertion over the same refusal path) (`cmd/corvint/depsource_test.go`, `cmd/corvint/no_mutation_test.go`) |
| `DSE-V0-002` | `TestResolveReadsCommittedGoMod`, `TestGitOutputReadIsBounded` (`internal/depsource/depsource_test.go`); `TestParseGoModQuotedAndAlignedDirectives`, `TestParseGoModExclude`, `TestParseGoModRefusesMalformedLiterals`, `TestParseGoModDecodesLiterals`, `TestParseGoModRefusesUnsupportedSyntax` (`internal/depsource/gomod_test.go`) |
| `DSE-V0-003` | `TestEscapeModulePath`, `TestCacheEscapingModulePathIsRefused` |
| `DSE-V0-004` | `TestVerifiedModuleListing`, `TestFileExcerptCapturedDuringHash`, `TestLargeFileCaptureIsBounded`, `TestZipHashMismatchIsRefused`, `TestOversizedZipHashIsRefused`, `TestHashObservesCancellation`; `TestCacheEntrySwappedAfterInventoryIsRefused`, `TestZipHashFIFOIsRefused`, `TestZipHashBehindUnreadableCacheRootIsRefused`, `TestZipHashInRootSymlinkIsRefused` (`internal/depsource/dirhash_open_unix_test.go`) |
| `DSE-V0-005` | `TestHashMismatchAbstains` |
| `DSE-V0-006` | `TestMissingCacheEntryAbstains` |
| `DSE-V0-007` | `TestResolutionAbstentions`, `TestExcludedVersionAbstains`, `TestUnparsableGoModAbstains`, `TestCacheStageAbstentions` |
| `DSE-V0-008` | `TestListingBoundedAndOrdered` |
| `DSE-V0-009` | `TestFileExcerpt`, `TestLargeFileCaptureIsBounded` |
| `DSE-V0-010` | `TestLocalReplace`, `TestLocalReplaceTargetNotCommitted`, `TestLocalReplaceTargetNotADirectory` |

Unqualified tests live in `internal/depsource/depsource_test.go`.

## Rollback

The slice is additive and isolated: `internal/depsource/`, `cmd/corvint/depsource.go` and their
tests, plus one dispatch line, one help entry, and one spec-index row. Rollback removes those and
leaves every other verb byte-identical, because nothing reads a `depsource` answer. If the proposed
pin extension is rejected, the verb is deleted rather than weakened: an unverifiable dependency
citation is worse than none.
