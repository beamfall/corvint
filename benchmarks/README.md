# Benchmark contract

Corvint benchmarks compare Corvint with a deterministic exact-text-search baseline. Optional competitor
adapters may add public tools, but Corvint never vendors them.

Each case freezes:

- repository URL and commit;
- natural-language task and mode;
- critical, required, relevant, and forbidden evidence selectors;
- output-byte budget;
- expected abstention state;
- independently reviewed rationale.

## Partitions and first-observation rule

- **Development** cases may guide implementation and regression repair.
- **Held-out** cases must be authored and frozen without access to Corvint output, then preserve their
  first run. Once anyone uses a result to change Corvint, that case is development forever.
- **Challenge** cases may target a later product version. Their first run diagnoses the architecture
  but is not merged into an earlier version's release score.

Version promotion requires both a green development corpus and a fresh untouched partition targeted
to that version. A repaired observed case proves regression coverage, not generalization. Corpus
manifests must retain `partition`, `target_version`, and first-observation provenance; a report must
not silently average mixed target versions into a release claim.

The runner reports corpus-level critical misses, recall, serialized-result-byte-weighted precision,
top-five success, abstention, budget compliance, total evaluation latency, and packet bytes. It does
not yet report time to first correct file. Training traces from the same task or outcome commit must
be removed before scoring.

Corvint's current five-repository development result is strong, but the untouched blind-v2 first run
is intentionally preserved as a challenge failure: two cases target V4 and three target V6. The
mixed set is useful architectural evidence and invalid as a single V4 pass/fail number. The dated
metrics and their interpretation are in the release notes (`docs/RELEASE-NOTES.md`).

Untouched blind-v3 targets the advertised V4 contract and is the binding generalization failure:
8/10 critical misses, 0.181818 recall, 0.258638 serialized-byte-weighted precision, 0.6 top-five
success, 0.0 abstention and epistemic-state accuracy, and 1.0 budget compliance
(`benchmarks/results/blind-v3-first-run.json`). It stays unchanged. The failure motivates the
sequence inversion toward CEM interoperability and typed proof obligations; it must not be repaired
into a held-out pass or hidden inside the development aggregate.

The CEM first-run evidence preserves two separate closed profiles: the immutable `/0` Beamfall
offline-install failure and the `/0.1` repaired pass. Their raw metadata-only artifacts are
`benchmarks/results/cem-first-run-beamfall-failed-v0.json` and
`benchmarks/results/cem-first-run-beamfall-pass-v0.1.json`; interpretation and schema locations are
in `docs/specs/cem-pilot-kit.md`. The 15.715100875-second pass is one experienced-operator
observation, not a first-use median or an adoption result.

The Agent Retrieval Bench control arm (Corvint beside a grep baseline over the bench's frozen
snapshots) is `tools/retrieval-bench`; its protocol and kill-criterion reading are in that
directory's README. Its first run over a released subset is preserved under `benchmarks/results/`
as first-observation evidence.

Run against exact local checkouts:

```console
go run ./benchmarks/runner \
  --checkout beamfall=/path/to/beamfall \
  --checkout flask=/path/to/flask \
  --checkout cobra=/path/to/cobra \
  --checkout zod=/path/to/zod \
  --checkout execa=/path/to/execa \
  --output benchmarks/results/v4-development.json \
  --enforce-v4
```

`--prepare` fetches missing public repositories into `.corvint-benchmark-cache/`. Beamfall remains an
explicit local checkout until it is public. `--enforce-v4` exits nonzero when any binding V4 product
threshold fails; it never changes the thresholds or drops a failing case.

The runner evaluates through the native index and evaluation packages and independently replays the
baseline and learned-arm accounting before reporting it. `--repos ID[,ID...]` restricts the run to a
subset of manifest repository ids; the report's `aggregate.skipped_repository_ids` records what was
left out, and `v4_release.ready` is forced false whenever anything was skipped, so a subset run can
never read as a release run.

## Corvint-on-Corvint dogfood measurement

`go run ./benchmarks/dogfood-measure` records subprocess performance and output identity without treating
packet bytes as tokens or inferring agent behavior. It uses Go's standard library and bounded native process supervision. Output uses
`corvint-dogfood-measure/1`; historical Python `/0` receipts remain unchanged. Keep its protocol and result private when paths or repository
identity are sensitive:

```console
$ corvint_git_dir=$(git rev-parse --absolute-git-dir)
$ go run ./benchmarks/dogfood-measure \
    --root "$PWD" \
    --protocol "$corvint_git_dir/corvint/measurements/protocol.json" \
    --output "$corvint_git_dir/corvint/measurements/result.json"
```

The closed protocol profile is `corvint-dogfood-measure-protocol/0`:

```json
{
  "profile": "corvint-dogfood-measure-protocol/0",
  "expectedHead": "FULL_GIT_OBJECT_ID",
  "expectedDirtyPaths": ["cmd/corvint/ocm.go"],
  "inputs": [".corvint/change.cem.json", "cmd/corvint/ocm.go"],
  "workloads": [
    {
      "id": "query",
      "argv": ["/absolute/path/corvint", "--root", "{root}", "query", "--task", "compose Change Frontier from CEM OCM LRF and TCQ while preserving native verifier error precedence and one raw-copy boundary", "--limit", "10", "--budget-bytes", "8000"],
      "cacheMode": "corvint-index",
      "expectedExit": 0,
      "timeoutSeconds": 300,
      "maxP95WallMs": 1000,
      "maxStdoutBytes": 8000
    },
    {
      "id": "impact",
      "argv": ["/absolute/path/corvint", "--root", "{root}", "impact", "cmd/corvint/ocm.go", "cmd/corvint/lrf.go", "cmd/corvint/tcq.go", "--limit", "10", "--budget-bytes", "8000"],
      "cacheMode": "corvint-index",
      "expectedExit": 0,
      "timeoutSeconds": 300,
      "maxStdoutBytes": 8000
    },
    {
      "id": "lrf",
      "argv": ["/absolute/path/corvint", "--root", "{root}", "lrf", "--cem", ".corvint/change.cem.json", "--expected-base", "6b5b0e00bc1ee16f77dc879f8d3d55b726fbfed0", "--target", "fd3b9db9b81c4abc5f59320159d36652bd4fde3e"],
      "cacheMode": "none",
      "expectedExit": 0,
      "timeoutSeconds": 30
    }
  ]
}
```

`{root}` is the supported substitution. `argv` executes directly; the retired `{python}`
placeholder is refused. Use an explicit native binary path. Prepare CEM/OCM inputs with the
native CLI before measuring, and list each frozen input under `inputs`. Synthetic zero-claim
controls remain structural controls, not representative linked-claim performance evidence.

The runner performs one unmeasured prime, then 30 rotating samples per workload in each phase by
default. `corvint-cache-cold` uses a unique empty `CORVINT_CACHE_DIR` for every sample and is labelled
`COLD_UNIQUE`; `primed-dirty` reuses the primed directory and is labelled `PRIMED_SHARED` even when
the repository is dirty. These labels describe the harness cache topology, not an inferred internal
hit. Cache-isolation regressions must separately prove that revision-index workloads avoid rebuilds.
Workloads with no Corvint index cache are `NOT_APPLICABLE`.

Every path in `inputs` must already be an existing regular non-symlink file. Missing paths,
directories, and symlinks fail before the prime; every admitted file is fingerprinted before and
after each workload.

Every sample records wall, user CPU, system CPU, maximum RSS, exit code, timeout state, and complete
stdout/stderr byte counts and SHA-256 digests. Distributions retain every sample and report
nearest-rank p50/p95, maximum, and median absolute deviation—never a mean. Input drift, output
nondeterminism, timeout, unexpected exit, or any stderr invalidates the artifact. A failed latency or
byte threshold leaves artifact integrity `valid:true` but makes `checksState:FAIL` and the CLI
non-successful. The p95 latency threshold applies only to `primed-dirty` and is `NOT_RUN` below 30
samples per phase; output-byte thresholds apply to both phases. Any `FAIL` wins over `NOT_RUN`, and
either state makes the CLI exit `1` after writing the complete artifact.

Stdout and stderr are drained through nonblocking pipes into separate bounded buffers. Crossing the
8 MiB per-stream limit immediately kills and reaps the dedicated process group and emits no partial
hash. The limit does not constrain legitimate workload data or Corvint cache files.

Tokens, source opens, broad searches, widenings, and reviewer misses are always
`NOT_OBSERVED`. Only an external agent-harness dispatcher may supply those session measurements;
this subprocess runner cannot turn them into zero or derive them from packet size.

## CEM reviewer trial

`tools/cem-trial` is the external agent-harness dispatcher for V4's CEM gate, specified in
`docs/specs/cem-reviewer-trial-v0.md`. It presents one Beamfall commit's source hunks to one agent
twice — bare (`control`) and with the cem/0.1 map, `cem status` worklist, and `cem report`
(`treatment`) — withholding the test-or-spec files the same commit touched as that change's gold, and
reports the paired mean difference in missed-evidence findings with a BCa interval, the citable
material-hunk fraction, and the rate of incorrect verifier hard failures. The partitions and the
first-observation rule above govern it: `tools/cem-trial/testdata/pilot` is a `pilot` set whose
results may tune prompts and are then development forever, and the judged report will be written once
to `benchmarks/results/cem-reviewer-trial-first-run.json` from a `heldout` selection. Thirty pairs is
an estimation run, not a test: `CRT-V0-008` fixes the wording under which V4's "at least 20% fewer"
may be called met.

The first pilot, `benchmarks/results/cem-reviewer-trial-pilot.json` (seed `pilot-2026-09-03`,
`gpt-5.6-sol`), is **invalid** and carries no estimate: the agent's budget ran out mid-run, 31 of 50
lanes exited non-zero, and no pair scored. It stands as published evidence of the harness defect it
exposed, not as a measurement of CEM.

## Blind-v4 (frozen 2026-09-02, unrun)

`benchmarks/blind-v4-manifest.json` freezes 31 V4-targeted cases across the same five repositories
at the blind-v3 pins (beamfall at `5dbd4ed6e`). They were authored by five independent sessions that
never ran Corvint or read its source, each `ground_truth` line was verified with `git show` at the
pin, and every file passed `validate_cases` before its digest was recorded. `first_observed_at` is
null: the partition has not been run. Its first run is the V4 generalization result; after that
run it is development forever. Run it once, with `--engine corvint`, and preserve the report.

## Workflow baseline (AT-01 context/recovery freeze)

`benchmarks/workflow-baseline-v0.md` and `benchmarks/workflow-baseline-v0.json` are the AT-01
preregistration for `start`/`investigate`/`resume`/`change-review` workflow tasks: task-class
definitions and solved criteria, the three comparison arms (current admitted Corvint, unrestricted
native tools plus structured notes, and the AT-05/06/07 candidate), the exact `dogfood_workers.py`
receipt metrics plus completion/correctness/latency fields, and the scorer edge rules a run must
obey before AT-08 can score it. It is independent of `docs/specs/compat-trial-v0.md`'s
compatibility-detection trial and freezes no held-out task list and no savings claim; both remain
AT-08 obligations.

## Agent Retrieval Bench: development status, folds, baseline ladder, registration

Every positive sample of the ARB v2 releases (`v2_code2test`, `v2_comment2context`, `v2_edit2ripple`,
`v2_trace2code`; 345 positives over 25 repositories) has been observed under several `context`
configurations and one of those observations chose a ranking parameter (decision 0066), so under
the first-observation rule above all four positive subsets are **development** partitions. No
unobserved positive partition remains in v2. `v2_abstention` (82 samples: 50 natural no-gold, 32
counterfactual) is also **development**: the base BUILD-LOG already records its use for two
withdrawn rules. Wave 1 found the earlier unobserved claim stale; decision 0078 preserves the
correction and current registered comparison. A held-out claim requires a future unobserved
partition or sealed blind-v4 after its own release prerequisites and preregistration. The frozen
query/eval endpoint does not measure context-only ranking flags.

Because the positives are burned, the strongest internal check is cross-fitting by repository.
The fold map is frozen (its SHA-256 over sorted `fold TAB repo` lines is written into every report's
`registration.fold_map_sha256`, currently `8d439eb6f7cb2471e90ee905684a0054bce49462859d52db01031f7cabc8f004`):

- Fold A (14): `HypothesisWorks/hypothesis`, `astral-sh/ruff`, `caddyserver/caddy`, `gin-gonic/gin`,
  `huggingface/diffusers`, `ipython/ipython`, `microsoft/playwright`, `mockito/mockito`,
  `numpy/numpy`, `pytest-dev/pytest`, `python/mypy`, `scrapy/scrapy`, `vitejs/vite`, `vuejs/core`
  (positives: code2test 49, comment2context 31, edit2ripple 30, trace2code 64).
- Fold B (11): `clap-rs/clap`, `eslint/eslint`, `etcd-io/etcd`, `fastapi/fastapi`,
  `huggingface/transformers`, `pallets/click`, `pydantic/pydantic`, `pypa/pip`,
  `spring-projects/spring-boot`, `tokio-rs/tokio`, `tox-dev/tox`
  (code2test 57, comment2context 49, edit2ripple 28, trace2code 37).

Fold rule: a parameter or mechanism choice is made on one fold only, declared in its decision record
before the other fold is run, and the other fold's first run is preserved unrepaired. Both folds
are always reported; a difference that changes sign between folds is not promotable. `b=0.3` in
TCP-V0-014 already saw both folds and is frozen as-is on v2. Every report says "development,
cross-fitted" until a registered unobserved partition passes.

Baseline ladder (`tools/retrieval-bench/README.md`): `grep` (the unchanged control), `grep-ident`
(whole-word identifiers from the query's values), `bm25:all` and `bm25:ident` (whole-file BM25,
k1 1.2, b 0.75, never tuned). Every report carries all four beside `context`; a "beats grep" claim
is judged against the strongest baseline in each cell by the paired bootstrap interval in the
report's `paired` section, never by point estimate. Minimum detectable paired difference at the
current n (95% half-width of the observed per-sample differences) is roughly 0.08–0.13 at
recall@5 and 0.06–0.10 at recall@20 per task; a 0.05 gain is not resolvable on any single task.

Registration: before a first run record the samples file SHA-256, the `corvint` SHA-256, the fold
map SHA-256, the arms, and the date; the report's `registration` section carries the first four so
a rerun after a code change is recognisable as a new registration. Per-arm wall time follows the
cache-state vocabulary above: `COLD_UNIQUE` is observed (no `.corvint/index` in the copy before the
call), `PRIMED_SHARED` when one exists, nearest-rank p50/p95/max and MAD, `NOT_RUN` below 30
samples, never a mean. "Faster than ripgrep" may only be claimed from primed cells whose p95 ratio
is below one on every repository-size bucket; the cold cell is the amortised cost, never the headline.
