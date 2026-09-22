# GPK-V0-021 — Corvint self-dogfood, output equality (W12)

Scope: **output equality only**. No timing, throughput, or resource measurement was performed in
this run; `GPK-V0-021`'s "private measurement receipt" sentence is owned by the separate
`conformance/perf-v0` timing workstream and is explicitly **not** adjudicated here (see Verdict).

## Identity

| Item | Value |
|---|---|
| Source repo | `/private/tmp/corvint-integration`, branch `integration/go-migration` |
| Source HEAD at run start | `c4b3b255a4c0ef9b6b4736ea8436faf136a51a7c` (moved to `30ddcfd06de8646790f133f03793d2fbff7dc3e8` by concurrent workstreams during the run; the subject copy is pinned and unaffected) |
| Subject repository | `/private/tmp/corvint-self` — `git archive HEAD \| tar -x`, then `git init` + single `pin` commit |
| Subject HEAD | `8ead5940a24193c550cbc503ed20b767686b8f02` |
| Subject tree | `323b3f0610dc35478bf1593d8d8f41ac22e61d16` |
| Subject tracked files | 1256 |
| Subject `git status --porcelain` | empty (`e3b0c442…7852b855` = sha256 of zero bytes) for the whole run |
| Candidate | `go build -o /tmp/corvint-w12 ./cmd/corvint`, sha256 `08a3ad4c43ebf5fe26a5b4ab57823fe88ba6688024affacc02a7ea6c4f9259dd`, reports `Corvint 0.4.0a0` |
| Oracle | `PYTHONPATH=/private/tmp/corvint-integration/src /usr/bin/python3 -m corvint_cli`, Python 3.9.6, reports `Corvint 0.4.0a0` |
| Runner | `run-case.sh` in this directory |
| Raw outputs | `raw/<case>.{oracle,cand}.{stdout,stderr}`, `raw/<case>.argv` |
| Stdin fixtures | `inputs/*.json` |

`src/**` was read but never modified. Nothing outside this results directory was written by this
workstream: no `git add`/`commit`/`checkout`/`stash` was run, and `cmd/corvint` was not created.
The source repo is shared with concurrent workstreams and its `HEAD` moved during this run
(`c4b3b255…` at start, `30ddcfd0…` at finish); that is irrelevant to the comparison, because every
case ran against the pinned copy at `/private/tmp/corvint-self`, whose tree never changed. Other
untracked/modified paths visible in the source repo's `git status` belong to those workstreams, not
to this one.

## 1. Enumerated supported operations

Determined from the candidate's own `--help` output, not from the Python CLI and not by guessing.
Evidence: `/tmp/corvint-w12 --help`, `/tmp/corvint-w12 help <command>`,
`/tmp/corvint-w12 help harness event` — transcripts in `raw/help-*.txt`.

The candidate's `--help` states its own support boundary verbatim:

> This binary implements only the experimental slices listed above; it is not the complete Corvint CLI.

Commands the candidate accepts:

| Command | Read-only? | In this comparison? |
|---|---|---|
| `init` | yes | yes |
| `adopt` | yes | yes |
| `query --task T --limit 1` | yes | yes (authority-start slice) |
| `impact PATH...` | yes | yes |
| `impact --working-tree-untracked` | yes | no — needs a mixed worktree; the subject is clean by construction |
| `impact --base COMMIT` | yes | no — needs a committed range; the subject has one commit |
| `harness event --event session-start` | yes | yes (both `startup` and `compact` start sources) |
| `harness event --event post-tool` | yes | yes |
| `harness event --event stop` | yes | yes |
| `harness event --event user-prompt` | yes | yes |
| `harness event --event file-change` | yes | yes |
| `harness event --event session-end` | yes | yes |
| `lrf --cem MAP` | yes | no — not a context/harness operation |
| `ocm status` / `ocm verify` | yes | no — not a context/harness operation |
| `ocm report` | **writes** | no |
| `cem begin/prepare/cite/mark/status/verify/report` | **writes** (except `status`/`verify`) | no |
| `record` | **writes** | no |
| `migrate-traces --dry-run` / `--apply` | dry-run read-only; `--apply` **writes** | no |
| `feature FEATURE_ID` (undocumented) | yes | yes (case 17) |
| `eval` (undocumented) | yes | yes (case 18) |

Undocumented surface, probed rather than assumed (`raw/help-refused-surface.txt`): `feature` and
`eval` appear in the Python CLI's `--help` and **not** in the candidate's, and neither has a `help`
topic (`help feature` → `unknown help topic`). But both are still dispatched: `corvint feature`
demands its positional `feature_id` and, given one, emits a full context receipt; `corvint eval`
runs and fails on a corpus path. They are therefore reachable but outside the candidate's declared
support boundary, so they are not counted as supported context operations — they are compared below
only as cases 17-18 to record what they do. Within `query`, the candidate's `help query` narrows
further — `--limit 1` is mandatory, and only task text classified
`project-operations` is accepted ("this is not general query"). Within `impact`, `help impact`
restricts paths to `.go` files in a non-root Go package.

Harness-event input schemas were read from the frozen oracle (`src/context_corvint_harness.py:267-352`,
`_validate_input`) so that both runtimes receive byte-identical, schema-valid stdin.

## 2. Per-case equality table

Both runtimes were invoked with byte-identical argv and byte-identical stdin, from cwd
`/private/tmp/corvint-self`, with `--root /private/tmp/corvint-self`. stdout was compared with `cmp`
(byte-for-byte, not JSON-normalised). "status stable" = `git status --porcelain | shasum -a 256` in
the subject repo, sampled three times per case (before oracle, between the two runs, after
candidate) and required identical across all three.

All harness cases share the argv prefix
`--root /private/tmp/corvint-self harness event --host claude-code --host-version 1.0.0 --surface cli --adapter-version 0.1.0 --input -`
(abbreviated `HP` below).

| # | Operation | argv (after `corvint`/`corvint`) | oracle bytes | candidate bytes | byte-equal | status-stable |
|---|---|---|---|---|---|---|
| 1 | harness session-start (`startup`) | `HP --event session-start` | 862 | 862 | **yes** | yes |
| 2 | harness session-start (`compact`) | `HP --event session-start` | 862 | 862 | **yes** | yes |
| 3 | harness post-tool | `HP --event post-tool` | 815 | 815 | **yes** | yes |
| 4 | harness stop | `HP --event stop` | 829 | 829 | **yes** | yes |
| 5 | harness user-prompt (migration orientation) | `HP --event user-prompt` | 4705 | 4705 | **yes** | yes |
| 6 | harness user-prompt (parity-qualified text) | `HP --event user-prompt` | 4678 | 4678 | **yes** | yes |
| 7 | harness session-end | `HP --event session-end` | 967 | 967 | **yes** | yes |
| 8 | harness file-change (Python path) | `HP --event file-change` | 4851 | 3759 | **NO** | yes |
| 9 | harness file-change (Go path) | `HP --event file-change` | 6079 | 6079 | **yes** | yes |
| 10 | query authority-start (parity-qualified text) | `--root … query --task "Identify the active work queue, required workflow gates, and minimum context needed to safely take the next roadmap ticket" --limit 1` | 4298 | 4298 | **yes** | yes |
| 11 | query authority-start (migration orientation) | `--root … query --task "Orient to the Go production kernel migration: identify the active work queue, required workflow gates, and minimum context needed to safely take the next roadmap ticket" --limit 1` | 4380 | 4380 | **yes** | yes |
| 12 | init (sealed summary) | `--root … init` | 13154 | 13154 | **yes** | yes |
| 13 | init (full receipt) | `--root … init --full-receipt` | 428442 | 428442 | **yes** | yes |
| 14 | adopt (sealed summary) | `--root … adopt` | 13197 | 13197 | **yes** | yes |
| 15 | impact, Go source | `--root … impact internal/analyzercap/registry.go` | 5510 | 5510 | **yes** | yes |
| 16 | impact, Python source | `--root … impact src/context_corvint_harness.py` | 4282 | 0 | **NO** | yes |
| 17 | feature, unknown id (undocumented surface) | `--root … feature some-feature` | 769 | 769 | **yes** | yes |
| 18 | eval, no arguments (undocumented surface) | `--root … eval` | 0 | 0 | **yes** | yes |

Exit codes matched on every case (0 on 1–15 and 17; case 16 oracle 0, candidate 2; case 18 both 2).

stderr was also compared byte-for-byte: identical on 17 of 18 cases. Case 18 has identical non-empty
stderr on both sides
(`{"error": "impact paths are not tracked at revision 323b3f06…: internal/auth/auth_test.go", "ok": false}`).
The exception is case 16, where the candidate writes 112 bytes of structured refusal and the oracle
writes none.

Per-case raw rows, including the two `git status` digests, are in `summary.psv`
(`case|oracle_rc|cand_rc|oracle_bytes|cand_bytes|byte_equal|status_stable|status_before|status_after`).
`summary.psv` is an append log; its first two `h-session-end` rows are superseded fixture-repair
attempts where my initial stdin was schema-invalid — both runtimes rejected it with a byte-identical
`invalid-harness-input` error, and the third row is the valid run reported above.

Two case pairs additionally test that the candidate rejects what the oracle rejects: the two
superseded `h-session-end` attempts produced byte-identical stderr
(`{"code": "invalid-harness-input", "error": "explicit outcome requires taskSha256, changedPaths, and verification", "ok": false}`)
and identical exit code 2 from both runtimes.

### Repository-status invariance

Every one of the 18 cases is `status-stable = yes`. The subject repo's `git status --porcelain` was
empty (sha256 `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`) before the oracle
run, between the two runs, and after the candidate run, in all 18 cases. **No case mutated the
repository.** Only read-only operations were exercised; `cem`, `record`, `ocm report`, and
`migrate-traces --apply` were not run.

## 3. Orientation to this migration (using the candidate)

`GPK-V0-021`'s "orient to this migration" sentence was satisfied by running the candidate itself
against the pinned copy of this repo, on migration-specific task text, through both the `query`
authority-start slice (case 11) and the `harness user-prompt` event (case 5).

Candidate `query` result for
*"Orient to the Go production kernel migration: identify the active work queue, required workflow
gates, and minimum context needed to safely take the next roadmap ticket"*
(`raw/q-authority-start-migration.cand.stdout`):

- `tool: query`, `ok: true`, `mutates: false`
- intent classified `project-operations`, confidence `high`, matched terms
  `gate, gates, orient, roadmap, ticket, workflow`
- freshness `fresh`, scope `git`, revision `323b3f06…`, 0 mixed paths
- abstention inactive; coverage `1/1` authoritative, 0 omitted, within budget (packet 4326 B)
- learning: `local_trace_state: absent`, 0 local traces, 0 matched history commits
- single ranked result: `AGENTS.md`, kind `instructions`, score 384, summary
  "`docs/specs/go-production-kernel-migration-v0.md` governs the current Go migration. | Substantive
  Corvint work must follow `docs/DOGFOOD.md`: use Corvint to collect pre-change context, bind … it still
  requires the accepted benchmark/format gate."
- evidence: `AGENTS.md:22` / `:40` / `:16` (`project-instructions`, authoritative), plus
  `instruction-reference` links to `docs/specs/go-production-kernel-migration-v0.md`,
  `docs/DOGFOOD.md`, `docs/BUILD-LOG.md`, `docs/SPEC-DRIVEN-DEVELOPMENT.md`,
  `docs/decisions/0002-future-publication-transition.md`, `docs/specs/README.md`,
  `docs/specs/deployment-neutral-index-platform-v0.md`
- one exclusion disclosed: `internal/contextindex/impact_test.go`, "generated-file header excluded"

The `harness user-prompt` receipt for the same text
(`raw/h-user-prompt-migration.cand.stdout`) returned the same top result (`AGENTS.md`, kind
`instructions`, score 384) with `profile: corvint-harness-event/0`, `support: FALLBACK`,
`mutates: false`, `degradations: ["frontier-authority-unavailable"]`, and a repository binding of
commit `8ead5940…` / tree `323b3f06…` / `worktreeState: clean`.

Both orientation outputs were byte-identical to the oracle's.

The orientation was substantively correct: the candidate routed to `AGENTS.md` and surfaced
`docs/specs/go-production-kernel-migration-v0.md` — the spec that carries `GPK-V0-021` itself — plus
`docs/DOGFOOD.md`, without reading a single source file. It did not enumerate individual open
migration work items; the authority it returned is the instruction file that names the governing
spec, which is what the `project-operations` profile is scoped to return at `--limit 1`.

## 4. Divergences (nothing was repaired)

### D-1 — `harness event --event file-change` on a Python path: candidate omits all `reverse-import` results

Case 8. Raw byte lengths: oracle stdout **4851** bytes, candidate stdout **3759** bytes
(delta 1092 bytes). Exit codes and stderr identical. Pretty-printed unified diff:
`raw/h-file-change.diff` (78 lines).

Ranked results:

| runtime | results |
|---|---|
| Python | `path src/context_corvint_harness.py` 1000; `spec docs/specs/agent-harness-integration-v0.md` 825; `reverse-import src/corvint_cli.py` 700; `reverse-import tests/test_context_corvint_harness.py` 650; `reverse-import tests/test_harness_protocol_equivalence.py` 650 |
| Go | `path src/context_corvint_harness.py` 1000; `spec docs/specs/agent-harness-integration-v0.md` 825 |

The two shared results are identical in every field. The three `reverse-import` results have no
counterpart on the candidate side. The coverage block moves in lockstep:
`authoritative_results`/`included_results`/`requested_results` 5 → 2 and `packet_bytes` 4104 → 3012;
`omitted_results` stays 0 and `within_budget` stays true on both sides, so the candidate does not
report this as an omission — it does not know the results exist.

**Is this the known divergence?** No. `conformance/divergence-register.md` DR-0005 (OPEN,
`spec-gap`) is `harness event --event user-prompt` returning **no `symbol` results** on the Beamfall
corpus, because the query-path build never populates `index.Symbols`. Here `user-prompt` is
byte-equal (cases 5 and 6, 4705 B and 4678 B), and the divergent kind is `reverse-import`, not
`symbol`. `file-change` takes a different code path from `user-prompt`: `harnessIndexedContext`
routes `user-prompt` to `contextindex.BuildQuery` but `file-change` to the full
`contextindex.Build` + `EvalImpact` (`cmd/corvint/harness_context.go:21-45`). **D-1 is a
previously unregistered divergence, adjacent to DR-0005 but not the same case.**

**Likely cause (inferred, not instrumented).** Case 9 is the control: the same event on a Go path
(`internal/analyzercap/registry.go`) is byte-equal at 6079 B, so the candidate's `reverse-import`
relation works for Go and fails for Python. `reverseImporters` (`internal/contextindex/impact.go:463-468`)
builds its match target as `index.Module + "/" + path.Dir(changedPath)` — a Go import path. For
`src/context_corvint_harness.py` that yields `github.com/corvint-context/corvint/src`, which no Python
`import context_corvint_harness` statement can match, so `seen` stays empty. The oracle instead matches
the Python module name, and its own evidence strings say so ("imports context_corvint_harness"). The
candidate's `importEvidenceLine` (`:485-497`) already contains Python-aware handling
(`containsPythonWord`), which suggests the Python case was intended to be reachable and the target
construction is where it is lost. Classification: **candidate-side gap in non-Go reverse-import
target construction** — a `go-defect` candidate under the register's taxonomy, but adjudicating it
against spec text is out of scope for this run and no repair was made.

Confirming this would take a unit test over `reverseImporters` with a Python-module `index.Imports`
entry, or a debug print of the computed `target`.

### D-2 — `impact` on a Python path: documented scope refusal, not a parity divergence

Case 16. Oracle stdout **4282** bytes, exit 0. Candidate stdout **0** bytes, exit 2, stderr 112
bytes:

```
{"code": "unsupported-impact-path", "error": "native Go impact currently supports only .go paths", "ok": false}
```

This is the boundary the candidate's own `help impact` declares ("One or more normalized
repository-relative `.go` paths in a non-root Go package"), emitted as a structured refusal rather
than an approximation. It matches the prior W12 Beamfall row `impact-refuses-python`. It is **not**
a case where the two runtimes claim to answer the same question and disagree; it is the candidate
declining a question outside its declared slice. No repair.

## 5. Verdict on GPK-V0-021, per sentence

> `GPK-V0-021`: Corvint self-dogfood MUST first use the candidate to orient to this migration, run its
> supported context/harness operations, preserve a private measurement receipt, and compare exact
> outputs with Python. Once CEM/OCM are ported, the final committed Go migration diff MUST be
> prepared, verified, reported, and obligation-checked by the Go `corvint`; before then Python may
> bind the experimental slice but that does not close Go parity.

| Clause | Verdict | Reason |
|---|---|---|
| "use the candidate to orient to this migration" | **PROVEN** | Cases 5 and 11 ran the candidate on migration-specific task text against a pinned copy of this repo. It classified `project-operations` with high confidence, bound revision `323b3f06…`, and returned `AGENTS.md` with `instruction-reference` evidence pointing at `docs/specs/go-production-kernel-migration-v0.md` and `docs/DOGFOOD.md`. Output recorded in §3 and in `raw/q-authority-start-migration.cand.stdout`. |
| "run its supported context/harness operations" | **PROVEN** | All six `harness event` events the candidate declares (`session-start` in both start sources, `post-tool`, `stop`, `user-prompt`, `file-change`, `session-end`) plus `query`, `init`, `adopt`, and `impact` were run — 16 cases, with 2 further cases covering the undocumented `feature`/`eval` dispatch. The two unexercised `impact` profiles (`--working-tree-untracked`, `--base`) require a mixed worktree and a committed range respectively, neither of which a single-commit clean archive can provide; that is a limitation of this subject, named here rather than glossed. |
| "preserve a private measurement receipt" | **NOT_PROVEN** *(by this evidence set)* | Deliberately out of scope. No timing, resource, or `conformance/perf-v0` receipt was produced by this run, by instruction, because concurrent load would invalidate the timing workstream's measurement. This sentence must be closed by that workstream, not by this document. |
| "compare exact outputs with Python" | **PROVEN as executed; the comparison's result is NOT clean** | 18 byte-for-byte `cmp` comparisons were run on identical argv and stdin against one pinned subject revision. 16 of 18 are byte-equal on stdout and exit code (17 of 18 on stderr). Two are not: D-1 (`file-change` on a Python path, 4851 vs 3759 bytes — an unregistered candidate-side `reverse-import` gap) and D-2 (`impact` on a Python path — the candidate's declared scope refusal). |
| "Once CEM/OCM are ported, the final committed Go migration diff MUST be prepared, verified, reported, and obligation-checked by the Go `corvint`" | **NOT_PROVEN** *(not attempted; not yet due)* | No `cem` or `ocm` operation was run — all of `cem begin/prepare/cite/mark/report` and `ocm report` write local artifacts, and this run was restricted to read-only cases in a throwaway subject. This clause is conditioned on CEM/OCM being ported and governs the final committed migration diff, which this evidence run is not. |
| "before then Python may bind the experimental slice but that does not close Go parity" | **Consistent with observation; no claim made** | Nothing here binds a slice with Python. D-1 is a live example of the clause's point: 16 green comparisons do not close parity while an unadjudicated divergence stands. |

**Overall: NOT_PROVEN for `GPK-V0-021` as a whole.** Two of its four leading obligations are proven
by this run and the exact-output comparison found a real, previously unregistered divergence (D-1)
that must be adjudicated in `conformance/divergence-register.md` before the `harness` command's
`file-change` event can claim parity. The measurement-receipt sentence is owned elsewhere.

Per the register's own standing caveat, a byte-equal row here means "the two runtimes agree and no
divergence is known," which is weaker than "the spec is satisfied." This document does not make the
stronger claim.
