# cem-trial

The external agent-harness dispatcher for `docs/specs/cem-reviewer-trial-v0.md`: V4's CEM gate
("30 Beamfall CEM lanes versus 30 controls") run as a paired, within-change estimation.

One **change** is one non-merge Beamfall commit `C`; its lane runs at `C^`. The harness presents only
the change's **source** hunks and withholds the test-or-spec files the same commit touched — those
that exist at `C^` become the **gold** the agent is scored against, and a test or spec the change
adds is recorded as `gold_unreachable` and never scored, because no arm can cite a path absent at
the base revision (CRT-V0-004, amended 2026-10-01). Every arm sees byte-identical patch bytes, the
same numbered hunk list, the same model, timeout, and read-only clone. The `treatment` arm
additionally receives the cem/0.1 map from `cem begin` over those exact bytes (every hunk
`unknown`), the `cem status` worklist, and `cem report`; the `seeded` arm receives the same three
artefacts plus the evidence `corvint context --subject PATH` retrieves at `C^` for each presented
source file, labelled as suggestions and never cited. Nothing else differs, the control prompt never
names Corvint, CEM, a map or a suggestion, and `treatment` is the format control between `control`
and `seeded`: the 2026-09-04 pilot ran `control` against `treatment` alone, so its treatment carried
no evidence and could only measure the worklist's wording.

## Verbs

```sh
cem-trial select --repo DIR --output DIR --seed S --corvint PATH \
  [--name N] [--since DATE] [--partition pilot|heldout] [--exclude FILES] [--limit N]
cem-trial run --tasks tasks.json --history DIR --corvint PATH \
  [--arms control,treatment,seeded] [--repeats N] [--agent codex|script] [--agent-command PATH] \
  [--model M] [--effort E] [--timeout D] [--workers N] [--seed S] [--output FILE] [--resume|--reuse FILE]
cem-trial score --report FILE [--output FILE]
```

- **select** freezes the set. Its rule is written verbatim into the manifest and its patches are
  digest-pinned; `run` recomputes both and refuses a manifest whose bytes have moved. A candidate is
  rejected when any presented hunk names a gold path, so the answer is never readable from the
  question. The manifest also records the mechanical **stem baseline** — the test path a naming
  convention alone would guess from each source file — as the floor the arms are read against.
  `--exclude` takes a comma-separated list of prior manifests — typically the pilot's `tasks.json` —
  and drops every change they name before the rule selects, so a `heldout` population is disjoint
  from the pilot's (CRT-V0-009). The summary and the manifest's `population` are the pool **after**
  exclusion: `population N, excluded M` means N candidates remain once M were dropped, so N never
  counts the M. Refusals, all exit 2: an exclude file that cannot be read, that names no change, or
  that names an identifier other than a full 40-character lowercase commit; an explicitly empty
  `--exclude`; an `--exclude` none of whose changes is among the candidates (a manifest from another
  or a rebased repository, which would otherwise drop nothing and still exit 0); and a post-exclusion
  pool with no candidate left. A pool short of `--limit` is selected in full with a stderr warning.
  `excluded 0` is reported only when no `--exclude` was passed.
- **run** clones the history at `C^` by fetching exactly that commit (never a shared clone, which
  would carry the change's own objects), asserts `git cat-file -e C` fails there, scrubs `.corvint/`,
  gives each arm its own lane clone, and dispatches the agent once per arm per `--repeats`. For the
  treatment and seeded arms it then replays the reply's citations into the map with `cem cite` and
  records the resulting `cem status` counts, `cem verify` result, and its own independent replay.
  The default `--arms` stays `control,treatment` so a pilot-shaped invocation is unchanged; the
  judged run names all three. The seeded arm's suggestions are computed once per change in the
  prep clone (which holds no object at or after `C`) and recorded on each seeded lane as `suggested`.
- **score** rebuilds every derived field from the raw ones and reproduces a report's bytes unchanged.

## Reply grammar

Every reply must end with one fenced JSON block:

```json
{"citations":[{"hunk":"N","path":"...","lines":"S:E","relation":"specification|decision|test-claim|implementation|call-site|dependency|incident","confidence":"certain|likely|unsure"}],"unknown":["N"]}
```

No block is `ABSENT`, an unparseable one `MALFORMED`; both score as "cited nothing" and are counted,
never dropped.

## Metrics

`miss` is the fraction of a change's gold paths the arm did not cite. For each compared arm the
estimate against `control` is the paired mean difference `Δ`, its relative size `R`, and a 95% BCa
bootstrap interval over changes (10,000 resamples from the recorded seed); the secondary is exact
McNemar on "missed anything at all", reported both over changes (the pilot's reading) and over
lanes (`mcnemar_lanes`, pairing the control and compared lane of the same change and repeat, because
averaging repeats first turns every change both arms sometimes missed into a tie). The report's top
level is the `treatment` contrast under the pilot's keys; `summary.arms[arm]` carries each arm's
block. `citable` is `supported / (total − mechanical)` from the arm's own `cem status` counts. A
verifier **hard failure** is a citing lane where `cem verify` reports `ok:false`; it is
**incorrect** when the harness's own Git-only replay says the citation was valid. An errored lane
drops its pair whole, so the arms always cover the same changes.

A seeded run adds `summary.seed_coverage` (how much of the gold the suggestions carried, and the
miss rate of suggested against unsuggested gold) and `summary.contrasts.seeded_vs_treatment`
(CRT-V0-012): a seeded result is read as retrieval times adoption, never as one number, and a
`fraction` below 0.5 is retrieval-limited and supports no claim about the map.

Thirty pairs is an **estimation run, not a test** (CRT-V0-008): the 20% gate is declared met only when
the point estimate reaches 0.20 relative *and* the interval's lower bound excludes 0. Otherwise the
verdict reads "consistent with 20% and with 0". The proposed 2026-10-01 amendment sizes the
decision-grade three-arm run at about 55 pairs for a 20% relative reduction from a corrected
control miss of 0.50, and reads the gate as "the interval excludes 0 and does not exclude 20%"; the
scorer keeps the pilot's reading until the owner accepts it.

## Pilot

`testdata/pilot/tasks.json` is five Beamfall changes selected with seed `pilot-2026-09-03` from a
population of 620 candidates. It is a `pilot` partition: under `benchmarks/README.md`'s
first-observation rule its results may tune prompts and timeouts and are then development forever.
It predates the reachable-gold rule: its gold includes files the changes added (three of its five
changes scored miss 1.0 in both arms on exactly those), and it is kept as history, not rescored.
A new selection with the amended rule records such files as `gold_unreachable`.

`testdata/fake-agent.sh` proves the pipeline without a model — it cites the first hunk's own source
file plus the stem-convention test file when the working copy holds one:

```sh
go run ./tools/cem-trial run --tasks tools/cem-trial/testdata/pilot/tasks.json \
  --history /path/to/beamfall --corvint ./corvint \
  --agent script --agent-command tools/cem-trial/testdata/fake-agent.sh \
  --repeats 1 --seed pilot-2026-09-03 --workers 2 --output /tmp/dry.json
```
