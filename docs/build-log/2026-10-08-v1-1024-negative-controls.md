# 2026-10-08: Step-level negative controls implementation (V1-1024)

## Intent

Ticket V1-1024 (GitHub #682) implements `LPCV-V0-057` to `LPCV-V0-070` in
`docs/specs/live-proof-carrying-verification-v0.md` as `corvint test-validity negate`. The owner
accepted these requirements in chat on 2026-10-08, and a separate branch records that acceptance.
This change edits only the spec's delivery status, its digest and the traceability row. It does
not change the intent-status word.

## Shape

- **`internal/stepnegation`** holds the provider-neutral core: trace step-tree and inventory
  parsing, network and DOM fault derivation, the single-step and joint pass rules, the closed
  canonical `corvint-step-negation/0` codec, the aggregate over the inventory, and retention.
  Retention uses `.corvint/strength-evidence/<sha256 of the test key>.json`, written atomically
  with mode 0600, under a per-key lock and confined to the worktree. It merges a new run into
  the stored entries only when the binding and inventory are equal, and then sets `planReused`.
- **`internal/jstestprovider`'s `RunNegate`** owns admission and every run. Admission happens
  before any run and covers profiles `/0` and `/1` on an external server, the qualified tuple,
  exactly one selected test, `--max-runs` (which includes baselines) and `--baseline-repeat`.
  The runs reuse the existing external-server path through one unexported `negation` hook.
  That hook adds trace recording and the Corvint-owned injection module, whose bytes the
  document binds, and forces the controlled config overlay. `qualified-reporter.cjs` is
  unchanged, because the V1-1028 lane owns it.
- **`corvint-js-test-provider negate`** prints the canonical document, or exits 2 with a closed
  `corvint-step-negation-refusal/0` document.
- **`corvint test-validity negate`** is in the core. It execs only the named absolute provider
  file and forwards SIGINT and SIGTERM. Provider output is bounded to 4 MiB. The core prints the
  run's document and a stderr summary, where a step needing a hand-authored control reads
  `UNPROVEN`, then retains the merged document. Exit codes:
  - 2 for a refusal, with no stdout;
  - 1 for a retention failure or incomplete cleanup, after the document is printed;
  - 0 otherwise.
- **`testvaliditydoc.JoinStepNegation`** implements the `LPCV-V0-068` join for `--receipt` and
  `--discover`. A join requires all of the following to match the receipt:
  - the test key, browser and device;
  - the runner and Node versions, on a candidate tuple;
  - the application: the `/0` screened label, or the `/1` attested repository and instance;
  - the spec and config digests, which must also equal the current worktree bytes.

## Decisions

- **The join is compiled but gated off.** `stepNegationJoinQualified` is `false`, so the join
  reads nothing and every `test-validity` output byte is unchanged. The test
  `TestStepNegationJoinGatedOffIsByteIdentical` proves this. The gate stays off until the
  `LPCV-V0-070` matrix is retained on a `PWP-V0-008` qualified tuple.
- **The core locks before the run only when the full identity is known.** With `--spec`,
  `--test` and `--project` all given, a concurrent writer refuses with `negate-evidence-busy`
  before the baseline. Without `--project`, the core learns the identity from the provider
  document, so a busy lock is reported after the run as a retention failure (exit 1, document
  already printed). The lock key uses the screened title, so it equals `Binding.Test.Key()`.
- **The core has one implementation refusal code, `negate-provider-failed`.** It covers a
  provider that cannot start, exits without a closed refusal document, overruns 4 MiB or names
  a different test identity than the pre-locked one. The traceability row names it. A numbered
  requirement that owns it is left for the intent owner.

## Evidence

All of the following pass with `GOMAXPROCS=3 go test -p 1 -count=1`:

- `internal/stepnegation`, at 89.9% statement coverage;
- `internal/jstestprovider`'s negate tests;
- `cmd/corvint-js-test-provider`;
- the `cmd/corvint` negate tests;
- `internal/testvaliditydoc`.

The traceability row names each test.

`TestNegateLiveDiagnostic` ran the synthetic fixture matrix (12 subtests) with Playwright 1.63.0
installed under the lane's TMPDIR. The local Node is v22.23.3, which `PWP-V0-008` does not
qualify, so that run is diagnostic only.

`NOT_RUN`:

- the `LPCV-V0-070` qualified-tuple matrix;
- the owner sample comparison;
- the cost measurement.

Until those run, no negate result joins the strength axis.
