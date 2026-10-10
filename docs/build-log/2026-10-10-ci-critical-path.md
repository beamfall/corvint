# CI critical path: go-static job, pinned ripgrep, trusted Go build cache (V1-1093, V1-1095, V1-1098)

Human-owned intent: the owner's 2026-10-10 CI audit (Batch 1, items 1, 3 and 4) and the three
tickets filed from it. The audit measured a median pull-request wall time of 39.8 min over 27
successful runs, with `go-product-shard (0)` always the critical path. Contracts:
AFP-V0-013 (static, vet, format, build and interop checks stay full), AFP-V0-014/015 (qualification
rows and the pinned narrowed invocation use an owned cold cache), AFP-V0-023 (test read scopes),
AFP-V0-024 (main-push REUSE) and AFP-V0-027 (`ci:batched`) in `docs/specs/affected-plan-v0.md`.
No requirement is added. The only spec edit is AFP-V0-027's wording: it now names `go-static`
beside `go-product-shard` as a job a `ci:batched` constituent does not start. AFP-V0-027 is
owner-directed and proposed, and the edit changes no line numbers.

## Changes

1. **V1-1093: `go-static`.** Shard 0 ran two groups of checks that no other shard ran:
   - the protected `.github/cishards` and `.github/testconfine` self-tests (race tests, vet and
     the Windows build), and
   - after its race tests, gofmt, `go vet ./...`, the Windows `go build ./...` and the
     `interop/cem01-go` tests and vet.

   A new `go-static` job runs the same commands, with the same environment and the same isolated
   helper modules. It needs no `docs-plan` output, so it starts at once, beside the shards. Like
   the shard-0 steps it replaces, it runs on the FULL, DOCS and REUSE paths, and it never starts
   for a `ci:batched` constituent. `go-product` now needs `go-static` and tests its result, so the
   required check covers exactly what it did. The shards still build both helpers and probe
   Landlock.

   V1-0359's Windows test vet is **not** included. `make cross-vet` fails at the base,
   284c43129df038decb36d5a5f736879174b41b17: `internal/tasks/dispatch/prompt_fragments_test.go`
   has no build constraint and uses `testConfig`, which is declared only in a
   `//go:build darwin || linux` file. Adding the vet would fail every run until that is fixed.
   The linux/arm64 legs and the interop legs pass.
2. **V1-1095: pinned ripgrep.** The shards no longer run `apt-get update` and install the apt
   package. They now:
   - download the official release asset `ripgrep-15.2.0-x86_64-unknown-linux-musl.tar.gz`;
   - check it with `sha256sum --check --strict` against
     `33e15bcf1624b25cdd2a55813a47a2f95dbe126268203e76aa6a585d1e7b149c`, so a mismatch, a missing
     file or a malformed line fails the step before extraction;
   - install `rg` to `/usr/local/bin`;
   - confirm that the product's fixed Linux PATH resolves that binary at that version.

   15.2.0 is the version that local development and the macOS release leg (Homebrew) already use.
   `pr-tests-qualification.yml` provisions its rows "exactly as CI does" (decision 0320), so its two
   provisioning steps use the same pin. `release-gates.yml` keeps its apt install, and its comment
   no longer claims to be identical to `ci.yml`.

   Checksum source, read 2026-10-10 from GitHub:
   - Release: `BurntSushi/ripgrep` 15.2.0, published 2026-07-15T16:26:10Z, not a prerelease. The
     tag object is 6ec72defacfb042f203ca0b4bf2513a0a5505a7e.
   - The asset's `digest` field in the releases API is `sha256:33e15bcf…149c`.
   - The release's own `ripgrep-15.2.0-x86_64-unknown-linux-musl.tar.gz.sha256` asset has the same
     value.
3. **V1-1098: trusted Go build cache.** Only the full-run shard path uses it.
   - **Restore.** `actions/cache/restore` (v6.1.0, pinned by SHA) restores Go's default GOCACHE,
     `~/.cache/go-build`. It does so only when the race invocation will run, so never on DOCS or
     REUSE, and only when the pinned narrowed driver cannot run (a push, a merge group, or a
     pull request with `CORVINT_PR_TOOL_SOURCE` empty). The driver keeps its own owned cold cache
     (AFP-V0-014/015).
   - **Key.** OS, architecture, Go version (the setup-go output), the race flag, the shard and shard
     count, and the `go.sum` hash. The suffix is the commit; the restore key is the same prefix
     without it.
   - **Save.** Only a `push` to `main` whose race invocation passed saves the cache. Before saving,
     it confirms that GOCACHE is the restored path and runs `go clean -testcache`, which expires
     every cached test result. `actions/cache/save` then writes the entry under the same key.
     Pull-request and merge-group runs never save, so no pull-request-written entry exists for
     main or another pull request to restore. The cache is a speedup only: every cache step is
     `continue-on-error`, and a failure only means a cold build.
   - `-count=1` stays on every `go test` invocation, so no test result is reused.
   - **Stays cold:** `go-static`, `docs-plan`, `go-interop`, `artifact-integrity`
     (`script/go-archive-gate` builds with private cold caches by design) and every qualification
     row (`pr-tests-qualification.yml` has no cache step). The audit also proposed restoring in
     `go-static` and `go-interop`. V1-1098 scopes this change to the full-run shard path, and
     neither of those jobs is on the critical path after V1-1093, so both stay cold here.

   Compliance with the spec's cold-cache text: the cold-cache requirements (AFP-V0-014's owned
   build cache, AFP-V0-015's empty HOME/TMP/GOCACHE per row and in run mode, and the runtime
   environment paragraph's exclusively owned cache) bind the qualification rows and the trusted
   driver. They do not bind the unpinned full invocation, and no spec or decision text forbids a
   cache there.

   Read scopes: `.github/testconfine` grants a confined test every path outside the checkout
   root (`Rules` in `scopes.go`). The cache stays at Go's default path in the home directory,
   outside `source/`, so `.corvint/test-read-scopes.json` needs no new entry.

   `docs-plan` is not given the cache, because the restore does not apply cleanly there. The cold
   `cmd/corvint` build is the advisory order planner. It runs only on pull requests, builds
   non-race with `-trimpath` from the base checkout, and no main run produces those compile
   entries. Warming it would add a build to every main push.

## Evidence

- At lane start, `corvint affected --base 284c4312…` had an empty selection (no diff yet). The
  advisory `make gate` and Verify rows stay mandatory.
- `make dogfood-change BASE=284c4312…` at lane start: the prechange-query and prechange-impact
  rows were `NOT_OBSERVED` (agent-receipt-absent). The CEM map was `NOT_PRODUCED`: git-diff-failed
  on the empty range, the intent scope was missing, and the outcome input was not provided.
- `actionlint` (with shellcheck) is clean on `ci.yml`, `pr-tests-qualification.yml` and
  `release-gates.yml`. `make ci-least-privilege-check` passes.
- A local replay of the `go-static` run blocks, extracted from `ci.yml`, ran against this worktree
  on macOS and passed: gofmt, `go vet ./...`, the Windows build, both helper modules' race tests,
  vet and Windows build, and the interop tests and vet. A gofmt violation planted in a scratch tree
  makes the gofmt line exit nonzero, which fails `go-static` and therefore `go-product`.
- The checksum line was replayed with GNU coreutils `sha256sum` 9.12. A mismatched hash, a
  malformed line and a missing file each exit 1; the matching hash exits 0.
- Size inference for the cache: a fresh GOCACHE after compiling every root package's race test
  binary (`go test -race -run '^$' -exec /usr/bin/true ./...`, darwin/arm64) is 1.0 GB. A shard's
  entry holds a subset of that plus the shared dependencies, so one generation of six entries is
  at most about 6 GB before compression. The repository limit is 10 GB, evicted least recently
  used first.

## Expected savings (audit estimates; hosted timing `NOT_OBSERVED`)

| Item | Expected saving |
| --- | --- |
| `go-static` | About 2–2.3 min off the critical path (shard 0 takes 2.9 min on REUSE against 0.6 min for the others). A gofmt or vet failure surfaces in about 3 min instead of after about 38 min. |
| Pinned ripgrep | 8–18 s per shard, about 0.2 min on the critical path and about 1.5 runner-min per run. |
| Build cache | About 1–1.5 min on the critical shard, from a median 85 s build/link gap (32–123 s). Restoring the cache costs some of that back, by an amount not yet measured. |

Hosted wall time, the ripgrep step time, the cache entry sizes, the restore time and the change in
the build/link gap are all `NOT_OBSERVED` until this branch's own CI run and the first main push
after it. The tickets' acceptance criteria that need hosted runs also remain open: the REUSE shard-0
time, the go-product failure within 5 min, the step at most 2 s, the gap falling, the cache API
listing, and a testconfine run passing.

## Limits

- The shard and `go-static` jobs copy the same helper file lists. A new helper file must be added
  in both.
- A restored entry can be up to one main push stale, and stale entries only miss. Go's cache is
  content-addressed, and its action IDs include the compiler and the flags.
- If the runner sets `XDG_CACHE_HOME`, GOCACHE moves. The restore then lands unused, and the save
  gate's GOCACHE assertion refuses to save.

## Rollback

Each item is one commit and reverts independently:
- **V1-1093:** restore the shard-0 blocks and steps, drop `go-static` from `go-product`'s needs,
  and revert the AFP-V0-027 wording.
- **V1-1095:** restore the two apt lines in the three provisioning steps and the release-gates
  comment.
- **V1-1098:** delete the restore, expire and save steps and the setup-go `id`. To drop entries that
  were already written, delete the `go-build-` caches through the Actions cache API or UI.
