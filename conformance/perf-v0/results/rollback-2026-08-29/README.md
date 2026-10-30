# GPK-V0-024 rollback proof — W12

Date: 2026-08-29
Clause under proof: `docs/specs/go-production-kernel-migration-v0.md:422-426` (`GPK-V0-024`)
Raw transcript: [`transcript.txt`](transcript.txt)

## 0. Environment and subjects

| Item | Value |
|---|---|
| Corvint repo | `/private/tmp/corvint-integration` @ `c4b3b255a4c0ef9b6b4736ea8436faf136a51a7c`, branch `integration/go-migration` |
| Go candidate | `/tmp/corvint-rb`, built `go build -o /tmp/corvint-rb ./cmd/corvint` (rc=0) |
| Go toolchain | `go1.27.0 darwin/arm64` |
| Python oracle (source) | `PYTHONPATH=/private/tmp/corvint-integration/src python3 -m corvint_cli`, `Python 3.9.6` |
| Python oracle (wheel) | `context_corvint-0.4.0a0-py3-none-any.whl`, built from a **copy** of `pyproject.toml`/`setup.py`/`src/` at `/tmp/rb-wheelsrc`, installed into `/tmp/rb-venv2`, console script `/tmp/rb-venv2/bin/corvint` (`Corvint 0.4.0a0`) |
| Subject repo 1 | `/tmp/rb-subject` — scratch git repo, 5 source files, later + `AGENTS.md` + `.gitignore`; final HEAD `e9dcc2d82ecbc67c44ba1bc9fe69d2273c8af3eb` |
| Subject repo 2 | `/tmp/rb-subject2` @ `e3bb679a3519a973e370b11a044f62086ce1d5c7` — reverse-order round trip |
| Subject repo 3 | `/tmp/rb-subject3` — cache-interop and `--full-receipt` parity |

No file inside `/private/tmp/corvint-integration` was modified except this results directory. `src/**`
was read only. The wheel was built from a copied tree so no build artifact landed in the frozen repo.

## 1. On-disk state inventory per runtime

Observed by full-tree inventory (`shasum -a 256` + size) before and after every command, plus
`git status --porcelain`. Snapshot labels `S0`–`S10` in `transcript.txt`.

### 1.1 What the Go candidate writes

| Command | Wrote anything? | Evidence |
|---|---|---|
| `init` | **No** | S0→S1 inventory identical; receipt carries `"mutates":false` |
| `init --full-receipt` | **No** | rb-subject3 tree unchanged |
| `adopt` | **No** | S1→S2 identical |
| `query` | **No** | S4, S5 identical to prior snapshot |
| `impact` | **No** | tree unchanged (refused in this repo, §5) |
| `harness event` × `session-start` (`startup`, `compact`), `post-tool`, `stop` | **No** | S7→S8 identical |
| `migrate-traces --dry-run` | **No** | `"mutates":false` in receipt; tree unchanged |
| `cem status` | **No** | `.corvint/change.cem.json` sha256 unchanged before/after |
| `record` | **Yes** | `.context-corvint/traces/<commit-revision>.jsonl` (0700 dirs, 0600 file) + `.context-corvint/traces/.trace-operation.lock` (0 bytes) |
| `cem prepare` | **Yes** | `.corvint/change.cem.json` |

**The Go candidate writes no derived cache anywhere.** `grep -rn "corvint-cache\|CORVINT_CACHE_DIR"` over
`internal/ cmd/ pkg/` returns zero non-test hits, and no cache file ever appeared after a Go-only run.

### 1.2 What the Python oracle writes

| Command | Wrote anything? | Evidence |
|---|---|---|
| `init` / `adopt` (± `--full-receipt`) | **No** | S0, S3 inventories identical; `"mutates":false` |
| `harness event` (same four events) | **No** | S7→S8 identical |
| `migrate-traces --dry-run` | **No** | `"mutates":false` |
| `cem status` | **No** | map sha256 unchanged |
| `query` | **Yes — derived cache only** | `.git/corvint-cache/<tree-revision>-<engineDigest>.json` (S6) |
| `record` | **Yes** | same `.context-corvint/traces/<commit-revision>.jsonl` + `.trace-operation.lock` |
| `cem prepare` | **Yes** | `.corvint/change.cem.json` |

### 1.3 Complete path inventory (union of both runtimes)

| Path | Kind | Written by | Runtime-tagged? | In git? |
|---|---|---|---|---|
| `.context-corvint/traces/<commit-revision>.jsonl` | durable trace store, JSONL, `schema_version: 1` | **both** | No | untracked (subject gitignores it) |
| `.context-corvint/traces/.trace-operation.lock` | 0-byte lock | **both** | No | untracked |
| `.corvint/change.cem.json` | CEM map (`cem/0.2`) | **both** | No | intended to be committed |
| `.git/corvint-cache/<tree-revision>-<engineDigest>.json` | derived index cache | **Python only** | **Yes** | inside `.git`, invisible to `git status` |

No user-level or system-level state was created by either runtime: after every run,
`~/.corvint`, `~/.context-corvint`, `~/.cache/corvint`, `~/Library/Caches/corvint`, and
`~/Library/Application Support/corvint` all remained absent.

All receipts (`init`, `adopt`, `query`, `impact`, `harness event`, `migrate-traces --dry-run`) are
**stdout-only**. Neither runtime persists a receipt to disk, so there is no persisted receipt corpus
that could require migration.

## 2. Format compatibility — byte comparisons

Every comparison below is `cmp` on the two runtimes' raw stdout or on the raw artifact bytes.

| Surface | Result | Size |
|---|---|---|
| `init` receipt | **BYTE-IDENTICAL** | 3557 B |
| `init --full-receipt` | **BYTE-IDENTICAL** | 4577 B |
| `adopt` receipt | **BYTE-IDENTICAL** | — |
| `adopt --full-receipt` | **BYTE-IDENTICAL** | 4618 B |
| `query` receipt (clean store) | **BYTE-IDENTICAL** | 2198 B |
| `harness event --event session-start --startSource startup` | **BYTE-IDENTICAL** | 863 B |
| `harness event --event session-start --startSource compact` | **BYTE-IDENTICAL** | 863 B |
| `harness event --event post-tool` | **BYTE-IDENTICAL** | 816 B |
| `harness event --event stop` | **BYTE-IDENTICAL** | 830 B |
| `harness event` malformed-input refusal (×3 events) | **BYTE-IDENTICAL** | 101 B |
| `migrate-traces --dry-run` over the mixed-runtime store | **BYTE-IDENTICAL** (`plan_digest e1c7c00e…`) | — |
| `.corvint/change.cem.json` written by `cem prepare` | **BYTE-IDENTICAL** (`sha256 64c10850…`) | — |
| `impact` receipt | **DIFFER** — Go refuses in this repo, see §5 | — |

Trace-store rows written by the two runtimes are the same canonical JSON object shape with
identical key ordering, identical `schema_version: 1`, and identical `trace_id` derivation — an
identical invocation under either runtime yields the same `trace_id` (`8fd34994…`, `4d0f64ce…`,
`4387bbe2…`, `15f3997e…`, `98b50175…` all reproduced across runtimes).

## 3. Round-trip results

### 3.1 Forward: Go writes → Python reads → Python writes → Go reads (`/tmp/rb-subject`)

| Step | Command | Observed |
|---|---|---|
| RT-1c | `corvint record --task "improve the greeting helper" …` | Created `.context-corvint/traces/e9dcc2d8….jsonl`, 1 row, 300 B |
| RT-2 | `python3 -m corvint_cli record` — **same arguments** | rc=0, receipt **byte-identical to Go's**, same `trace_id 8fd34994…`; **trace file byte-unchanged** (`cmp` pass). Python recognised the Go-written row and deduplicated it. |
| RT-3 | Python `record` — new task | Appended row 2 in place, existing Go row byte-preserved |
| RT-4 | `corvint record` — **same arguments as the Python row** | rc=0, **trace file byte-unchanged** (`cmp` pass). Go recognised the Python-written row and deduplicated it. |
| RT-5 | `corvint record` — third task | Appended row 3; rows 1–2 byte-preserved |
| READ-2 | Python `query` over the 3-row mixed store | rc=0, `"local_trace_count": 3`, `"local_trace_state": "ready"` — all three rows (Go, Python, Go) consumed |
| READ-3/4 | `migrate-traces --dry-run`, both runtimes | rc=0 both, `"legacy_trace_files": 0`, `"trace_rows": 0`, **identical `plan_digest`** — *neither runtime plans any migration over the mixed-runtime store* |
| CEM-1→CEM-4 | Go `cem prepare` → Python `cem status` → Python `cem prepare` → Go `cem status` | Map sha256 `64c10850…` at every step; each runtime verified the other's map without rewriting it |

**No error, no migration step, no format rewrite was observed at any step.**

### 3.2 Reverse: Python writes → Go reads → Go writes → Python reads (`/tmp/rb-subject2`, virgin store)

| Step | Observed |
|---|---|
| REV-1 | Python `record` created `.context-corvint/traces/e3bb679a….jsonl` on a virgin repo |
| REV-2 | Go `record`, same arguments → rc=0, **store byte-unchanged** (`cmp` pass) |
| REV-3 | Go `record`, new task → appended in place |
| REV-4 | Python `record`, same arguments as the Go row → rc=0, idempotent |

Final store: 2 rows, one authored by each runtime, mutually readable.

### 3.3 Wheel-reinstall rollback

| Step | Observed |
|---|---|
| WHEEL-1 | `pip install context_corvint-0.4.0a0-py3-none-any.whl` into a clean venv, rc=0 |
| WHEEL-2 | `/tmp/rb-venv2/bin/corvint --version` → `Corvint 0.4.0a0` |
| WHEEL-3 | Wheel-installed `corvint query` over the Go-authored state → rc=0, `local_trace_count: 3`, `local_trace_state: "ready"`, `state: "READY"` — **no migration prompt, no conversion, no warning** |
| WHEEL-4 | Wheel-installed `corvint record` with the last Go row's arguments → rc=0, idempotent, trace file unchanged, row count still 3 |
| End state | Wheel-installed `corvint query` output **byte-identical** to the same query taken before any Go write occurred |

Total rollback procedure actually performed: **install the wheel, run `corvint`.** No repository step,
no receipt step, no map step, no trace step, no cache step.

## 4. Runtime-tagged derived cache

Only one derived cache exists: `.git/corvint-cache/<tree-revision>-<engineDigest>.json`, written by
Python (`src/context_corvint_index.py:755-770`, `_cache_file`).

**The tag is proven, not inferred.** The observed filename suffix was
`d69892c4a386cbe6b3570c1956a766cd86fd146f001813b020b1c0e45b8903f8`, and an independent computation of
`sha256("context_corvint_index.py" || bytes || "context_corvint_project.py" || bytes)` over
`/private/tmp/corvint-integration/src` produced exactly that value. Any change to the Python engine
bytes therefore changes the filename, so a different runtime version can only *miss* the cache — it
can never read a foreign one.

| Test | Observed |
|---|---|
| CACHE-1 | `rm -rf /tmp/rb-subject/.git/corvint-cache`, then re-run `query` → **byte-identical output** with and without the cache; cache silently regenerated |
| CACHE-2 | `record` after cache deletion → rc=0, idempotent row still recognised (row count 3) — the cache holds no state the trace store depends on |
| CACHE-3/4 | Fresh repo, Python `query` first (creates the cache), then Go `query` with the cache present → rc=0, **byte-identical to Python**; cache file count unchanged at 1 — **Go neither reads, writes, nor invalidates the Python cache** |

The cache lives inside `.git/`, so it never appears in `git status` and can never be committed by
accident. Deleting it is harmless in both directions.

## 5. Refusals recorded (Go candidate support boundary, not rollback failures)

| Command | Exact refusal | rc |
|---|---|---|
| `corvint query --task "add two integers"` | `{"code": "unsupported-query-intent", "error": "native Go query currently supports only project-operations task orientation", "ok": false}` | 2 |
| `corvint query` with a populated `.context-corvint/` | `{"code": "unsupported-query-trace-state", "error": "native Go authority-start query requires an absent clean-tree local trace store", "ok": false}` | 2 |
| `corvint impact greet.go` | `{"code": "unsupported-impact-repository", "error": "native Go impact requires a slash-qualified Go module", "ok": false}` | 2 |
| `corvint ocm status` (no `--map`) | `{"code": "invalid-arguments", "error": "the following arguments are required: --map", "ok": false}` | 2 |

The second row is the only one that interacts with state: **once any trace row exists, the Go
candidate's `query` refuses while Python's succeeds.** This is a Go-forward capability gap. It does
not impede rollback (rollback is Go→Python, and Python handles the store), but it is noted because it
means Go's `query` cannot be exercised on a repository that has ever been recorded into.

Not exercised in this proof, recorded as **NOT_TESTED**: `feature`, `eval`, `lrf`, `ocm verify`,
`ocm report`, `cem report`, `record`'s `session-end`/`user-prompt`/`file-change` harness events, and
`migrate-traces --apply`.

## 6. Verdict per sentence of GPK-V0-024

### Sentence 1 — "Rollback MUST require no repository, receipt, map, trace, or cache migration."

| Dimension | Verdict | Basis |
|---|---|---|
| **repository** | **PROVEN** | `git status --porcelain` was empty at every snapshot S0–S10 in both subject repos. Neither runtime touched a tracked file, the index, refs, or objects. Rolling back required no repository action. |
| **receipt** | **PROVEN** | No receipt is persisted by either runtime (all `mutates:false` reads emit to stdout only), so there is no receipt corpus to migrate. Separately, all seven compared receipt surfaces are byte-identical across runtimes, so a receipt captured under Go is indistinguishable from one captured under Python. |
| **map** | **PROVEN** | `.corvint/change.cem.json` produced by Go `cem prepare` and by Python `cem prepare` are byte-identical (`sha256 64c10850…`); each runtime's `cem status` verified the other's map at rc=0 and left the bytes unchanged. |
| **trace** | **PROVEN** | Full round trip in both orders (§3.1, §3.2) with zero errors and zero byte changes to pre-existing rows; `migrate-traces --dry-run` under both runtimes reported `legacy_trace_files: 0` with an identical `plan_digest` over the mixed-runtime store, i.e. both runtimes independently conclude no trace migration is needed. |
| **cache** | **PROVEN** | The one derived cache is Python-only and Python-engine-tagged; Go never reads or writes it, and deleting it changed no output (§4). Rollback needs no cache action; the cache is simply already correct. |

**Sentence 1: PROVEN.**

### Sentence 2 — "Reinstalling the last Python wheel or selecting the prior integration executable MUST restore the old runtime."

**PROVEN**, on both alternatives:

- *Reinstalling the wheel*: `context_corvint-0.4.0a0-py3-none-any.whl` was built and installed into a
  clean venv, and the resulting `corvint` console script read Go-authored trace state
  (`local_trace_count: 3`, `state: READY`), recorded idempotently over a Go-written row, and produced
  query output byte-identical to the pre-Go baseline. No conversion step was performed or offered.
- *Selecting the prior executable*: throughout §3, invoking `python3 -m corvint_cli` immediately after a
  Go write, with no intervening step, succeeded at rc=0 in every case.

### Sentence 3 — "Runtime-tagged derived caches may be ignored or deleted."

**PROVEN.** The Python derived cache's filename suffix was shown to equal an independently computed
sha256 of the Python engine source bytes, so it is genuinely runtime-tagged. Deleting it left `query`
output byte-identical and left `record` idempotency intact; the Go candidate ignores it entirely
(zero source references, verified by grep, and byte-identical output with the cache present).

### Sentence 4 — "Any receipt mismatch, release-critical evidence miss, read mutation, panic, unsafe write, process leak, or repeated performance-gate failure stops rollout and restores the previous runtime before investigation."

**NOT_PROVEN — out of scope for this exercise.** This sentence states a rollout *policy trigger*, not
a property of on-disk state, and cannot be proven by a state round trip. What this run *did* observe,
as partial supporting evidence:

- **receipt mismatch**: none observed across 11 compared surfaces (§2). The one divergence is an
  explicit `unsupported-*` refusal, not a mismatch.
- **read mutation**: none observed — every command declaring `mutates:false` left the inventory
  byte-identical (S0→S8).
- **panic**: none observed; the Go candidate never produced a Go panic trace in any run.
- **unsafe write**: none observed; Go's trace-store writes used 0700 directories and a 0600 file, and
  both runtimes refused to write while the tree was dirty (§7).
- **process leak / performance gate**: not measured here.

Proving sentence 4 requires the rollout-control procedure and the performance gate, which are
separate W12 artifacts.

## 7. Where rollback would in fact require a step — findings

1. **No migration is required anywhere.** No case was found in which rolling Go→Python (or Python→Go)
   needed a conversion, rewrite, or repair of any artifact.

2. **One operational precondition, affecting both runtimes equally.** In a subject repository that
   does **not** gitignore `.context-corvint/`, both `record` implementations refuse:
   - Go: `{"error": "repository changed before trace append: repository identity changed", "ok": false}`
   - Python: `{"error": "repository changed while preparing trace", "ok": false}`

   Creating the untracked trace store dirties the worktree, and the post-write stability check then
   sees a changed identity. Adding `.context-corvint/` to `.gitignore` cleared it for both runtimes.
   This is a pre-existing shared property, not a Go regression and not a rollback obstacle — but the
   Python run left a stray `.context-corvint/traces/.trace-operation.lock` behind after its refusal.

3. **Go-forward gap, not a rollback gap:** once any trace row exists, Go `query` refuses
   (`unsupported-query-trace-state`) while Python `query` succeeds. Rolling *back* to Python is
   therefore strictly recovering capability. Rolling *forward* to Go on a repository that has ever
   been `record`ed into loses `query`.

4. **Residue after rollback is inert.** After a Go→Python rollback the repository still holds
   `.context-corvint/traces/*.jsonl` (Python reads it natively), `.context-corvint/traces/.trace-operation.lock`
   (0 bytes), and `.git/corvint-cache/*.json` (Python's own tagged cache). None requires action.
