# Selection correctness: resumed verification

Owner intent: finish the existing selection correctness lane against
`6f12f4704235dc8f9bea19fc12995bfb68c746c3`, retaining its correct implementation and measurements.
The original measurement record is `2026-11-03-selbugs-selection-correctness.md`.

## Remaining manifest refusal

V1-4358 / ASD-V0-2401 still reproduced when `TMPDIR` named the linked worktree's Git directory:
heavy.sh suggested a `corvint-heavy-selection-*.json` path under that directory, which
`corvint test` refuses. The manifest helper now resolves the temporary directory and checks the
physical checkout, Git directory and common Git directory. A match uses `/tmp`.

The spec amendment preceded the implementation commit. The `tools` and `policy` groups of
`script/lane-scripts_test.sh` pass (21 groups), including linked Git directories and outside
symlinks to them. The suggested `/tmp` path was accepted by a real `corvint test --dry-run`.
A proposed missing-directory fixture was removed: heavy.sh's preflight creates that directory,
so it is no longer unresolvable when the manifest helper runs.

## Proving checks

- AFP-V0-2400/2401 and CIN-V0-2320: a private clone with `python3 -c 1` appended to
  `script/batch-regen.sh` selected 15 of 45 shell-test steps, including
  `no-python-runtime-dependency-test`; that check failed, naming the injected line 189.
  The unchanged checkout passed the same self-test. The shell declaration, shell-step selector
  and CI workflow packages passed their tests.
- AFP-V0-2402/2403: the focused CLI run (including use-case/help tests and whole-package
  witnesses) and selection-baseline unit tests passed. Historical measurements and failed-CI
  controls remain in the original build-log entry.
- AFP-V0-2404/2405/2406: the affected package's complete test run passed in 135.744 seconds,
  including plugin-local frontier-site determinism and fixture-frontier controls.
- ASD-V0-2400/2403: offload fixtures passed, including new-package failure propagation and a
  failing Darwin half; native Darwin/WSL execution remains NOT_PRODUCED on this Linux host.
- ASD-V0-2402: batch-precheck self-tests passed, including allowed unknowns, injected golden
  drift and forbidden/malformed unknown controls.

## Evidence limits and friction

Private command outputs are retained in `/home/russell/.cache/selbugs-resume/`.
Ticket reads returned `MALFORMED` / `NOT_OBSERVED`: this host has no queue journal and the tickets
are absent from its committed projection. The charter supplied the intent; ticket writes,
publication, independent review and sealing remain with the orchestrator.

The host default heavy lock directory `/home/russell/bench/rt/.locks` is outside the writable
roots; checks used the existing `/home/russell/.cache/corvint-heavy/slots` with explicit log and
cache overrides. Commits succeed while Git also reports that
`/home/russell/projects/corvint/.git/packed-refs.lock` is read-only; no permission change was made.

The capability-catalog target expanded to one direct multi-package Go test command and passed;
that invocation violated the lane's wrapper rule. Subsequent Make test commands use a private
adapter invoking heavy.sh once per package. This does not change the repository Makefile.
