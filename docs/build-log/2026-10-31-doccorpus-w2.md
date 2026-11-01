# Lane doccorpus-w2 — documentation corpus, line citations and affected selection

Base `6c967b21caa2ffb0324a681f873a891996d82e34`, branch `claude/lane-doccorpus-w2-1031`, linux/amd64
WSL2 (32 threads), go1.27.3 local toolchain. The host ran four peer lanes throughout: load average
1.2 to 6.2 during every measurement below, so no number here is a quiet-host qualification.

## Delivered

- V1-2611 (`TJAA-V0-810`): `playwrightMemberAccessAt` treats a TypeScript non-null assertion `!`
  as ending an operand when an operand ends before it, so `x![k]`, `x()![k]`, `x[0]![k]` and
  `x ! ! [k]` stop the Playwright tag closure at their line; `![a]` stays a logical not of an
  array literal. Failing first on the base (every form verified).
- V1-3467 (`DCG-V0-810`): `--repin` falls back, when the base's diff proves no move, to the diff
  from the newest of the last 50 commits of the cited file at which the cited lines hash to the
  pin. Decision: history, not a widened content search, is the "context": that commit is where the
  cited text last stood at its numbers, so the diff tracks those exact lines, a short or repeated
  line included, while a span whose own text changed is still never rewritten. The reported case
  (a `Makefile` insertion committed before `--repin` ran) is case 35 of the gate's test.
- V1-2560 (`DCX-V1-810..811`): legacy discovery lists its root through `RevisionPaths` (one
  verified batch per directory level) instead of a per-tree `RevisionFS` walk; 1,100 directories
  below the root now enumerate under the default budget and `boundCorpusGitOps(phaseGitOps)`.
  Decision: keep the per-tree walk's order (segment-wise name order) so every inventory is
  byte-identical; a test compares the new listing with `fs.WalkDir` on four roots.
- V1-2561 (`DCX-V1-812`): reproduced on linux/WSL2: 4,200 files matching `app/**/*.rb` refuse at
  the 4,096 provider/1 matched-file bound and extract under `--schema 2`. Decision: the bound is
  already operator-declared by the schema, so the change is the refusal naming how to scope the
  profile (`narrow extractor "<id>" files globs to at most <bound> files`) beside `--schema 2`.
- V1-3326 (`AFP-V0-817`): `test-read-scopes-check` names each key that binds no root-module
  package unit and why: a dependent nested module (`go:<module>`, AFP-V0-582) that no confined
  shard runs, or a directory with no package. Decision: refuse with the reason rather than enforce:
  enforcement would need the confined shard to run a nested module's own `go test ./...`, a
  `.github` change under `ci-control-plane` review. The planner's `go:test-read-scopes-invalid`
  frontier is unchanged, since `internal/gateselect` allowlists frontier keys.
- V1-2565 (`DCX-V1-813`): a verify-only pin (rebuild reuse, and a full build's post-compile
  re-pin) uses `contextindex.BuildRevisionPin`: `BuildRevisionContext` without `compile()`, so the
  same listing, admission, source pins and refusals. Decision: a first attempt listed the tree only
  and read every input from Git; it cut CPU by 58% but was slower on wall time (serial `cat-file`
  over 9,500 blobs against the index's parallel worktree-verified pins), so it was replaced.
  Measured with base and head test binaries alternated five times on one prepared 9,500-file
  corpus: reuse median 2.14 s / 443 MiB before, 1.87 s / 411 MiB after (bar 2.52 s / 421 MiB);
  full rebuild 4.23 to 4.46 s before, 3.87 to 4.02 s after. `TestDCXV1532*`/`TestDCXV1533*` pass.
- V1-2748 (`DCS-V1-814`): measured, not reduced. A derived declaration adds 2,032 bytes (subject
  962, claim 1,070), 1,085 of them its two pinned anchors (401 declarations of one records-owned
  Ruby controller). Decision: halving it needs an artifact-level file table, a new artifact
  schema every reader, the streamed decode, `delta` and the MCP bridge decode, with a
  `DerivationVersion` bump; the spec records why and a test pins the 2,048-byte ceiling.
- V1-2906 (`AFP-V0-818..819`): a plugin left out only by the AFP-V0-560 terminal rule still names
  its cross-language runtime entries (TypeScript `e2e-runtime-dependency`), and
  `.corvint/runtime-edges.json` declares runtime edges, applied as `Execs` so the spec is selected
  with a `BINARY_EXEC` witness through the command it runs. Decision: AFP-V0-818 is scoped to the
  AFP-V0-560 exclusion; naming the entries of a plugin no change reaches at all would make every
  Go plan of this repository UNKNOWN (17 entries from Playwright fixtures under
  `cmd/corvint/testdata/`), so that stays the AFP-V0-004 non-goal and the declaration is the
  remedy. Replay of `corvint affected --base <first parent> --no-cache` over the 20 newest merges,
  base binary against changed binary: 0 selections lost; one merge names 17 more unknowns.

## ALREADY-DELIVERED evidence

- V1-2477: at the base, warm CLI `flow` and `gaps` on the `DCR-V1-418` fixture (66,752,223-byte
  artifact built by the measuring binary), n=20 each: p50 180.2 / 174.7 ms at load 3.6, and 183.2 /
  184.7 ms at load 2.6 (target < 200 ms); peak RSS 219.8 MB, within the `DCR-V1-430` 3.5x bound
  (233.6 MB). `DCR-V1-432` records for each listed path whether it reuses the retained artifact or
  why it rederives. `TestDCRV1416CorpusMCPFirstAndWarmCalls` with `CORVINT_READ_FIXTURE_LARGE=1`:
  first call 324 ms total, warm p50 44 ms (p95 95 ms), against 2.50 s rederivation.
- V1-2342: `go test ./internal/doccorpus` (whole package, heavy.sh) passes in 38.2 s at load 4.1,
  far under the 600 s ceiling.
- V1-2820: `DCG-V0-045` (`--repin` relocates spans moved by a constant offset using the `-U0`
  diff from the base) and case 32 of `script/check-line-citations_test.sh` are on the base
  (`9477995d7`); the test passes. `DCG-V0-810` above extends it to already-committed moves.

## Not delivered

- V1-0643: `internal/companionrelease` is on the floor through `..` literals and has no read
  scope, but at least six of its tests build or export the whole real module
  (`TestCorvintBundledBinariesBuildTwiceIdentically`, the three `exported_current_test.go` tests,
  the two `core_smoke_test.go` builds, the ruby bundle test), several through `.git`, which the
  AFP-V0-023 wrapper denies. Their assertion is about the real checkout, so a disposable fixture
  cannot stand in; moving them to a deliberate root-locator subpackage needs about 25 unexported
  identifiers exported from a release-path package. `conformance/release-artifact-v0` locates the
  root in non-test code (`publication_status.go` calls `os.Getwd`). Both need an owner decision on
  that API exposure or on keeping them on the floor.
- V1-1097: every acceptance criterion needs an external event: owner acceptance of a new spec, a
  `ci.yml` change under `ci-control-plane` review, and CI run history for the 50-PR replay, the
  runner-minute reduction and the fix-push wall time. BLOCKED on those.
- V1-2571, V1-2588: the warm-read target is < 170 ms; measured 175 to 185 ms p50 (above). The
  boundary-scan requirements of V1-2571 (`DCR-V1-551..553`, `DCR-V1-559`) are already on the base.
  Profile of six warm `gaps` reads: decode workers about 1.25 s CPU per read, of which JSON
  unmarshal about 0.5 s, the canonical re-encode about 0.33 s and decode-time claim terms about
  0.37 s. Skipping claim terms for `flow`/`gaps` is ruled out (index refusals count term postings,
  `DCP-V1-194`), and replacing the canonical re-encode is a redesign of the `DCR-V1-414` trust
  argument; neither fit the lane. A quiet host was not available for the qualification either.

## Friction

- `/tmp` (16 GB tmpfs) filled to 100% from shared lanes: `go vet` and a test fixture failed with
  `no space left on device`. Repro: `df -h /tmp` during the wave. Workaround: `TMPDIR` on disk.
- With `TMPDIR=$HOME/.cache/lane-doccorpus-w2-tmp` (mode 0700),
  `TestIndependentNestedModuleIsARunnableModuleFrontier_AFPV0650` in
  `internal/liveverify/affected/golang` fails with `cache directory is not private to this user`,
  on the base as well. Repro: `TMPDIR=$HOME/.cache/x ~/lanes/heavy.sh env GOTOOLCHAIN=local go test
  -run AFPV0650 ./internal/liveverify/affected/golang`.
- The charter's AFP block 810-819 was not free: `AFP-V0-810..816` are on origin/main. Repro:
  `git grep -c "AFP-V0-81[0-6]" origin/main -- docs/specs/REQUIREMENTS.tsv`. AFP-V0-817..819 used.
- `corvint test --dry-run` reports `DEGRADED self-binary-stale` (installed binary built from
  `71c4b1966636`) and selects 156 units for any spec edit, so heavy.sh needs `HEAVY_WHOLE_REASON`
  for every focused whole-package run.
- `corvint affected --base` refuses revision syntax such as `<sha>^1`; scripts must `rev-parse`
  first.
