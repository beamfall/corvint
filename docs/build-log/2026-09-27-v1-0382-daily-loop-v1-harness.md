## 2026-09-27 V1-0382, V1-0369, V1-0161, V1-0162: daily-loop V1 harness revision, unsealed

`benchmarks/daily-loop-v0/harness.py` stays frozen with its preregistered digest and its runs
(decision 0423). The fixes land in a new revision, `benchmarks/daily-loop-v1/harness.py`. The two
directories share no files, so no v0 run or receipt changes.

V0 defects that V1 fixes:

- V1-0162: `run` trusted the `--prereg-sha256` argument. A changed preregistration could run under
  a digest the operator supplied. V1 reads the committed `preregistration.sha256`, computes the
  digest of `preregistration.json`, and refuses `preregistration-digest-mismatch` or
  `preregistration-digest-argument-mismatch`. Without the seal file it refuses
  `preregistration-seal-missing`.
- V1-0382: a loop step inherited the operator's shell. An omitted-input variant could pick up a
  `DOGFOOD_*` value, and host Git configuration reached every clone and commit. V1 strips every
  `GIT_*` variable and points `GIT_CONFIG_GLOBAL` and `GIT_CONFIG_SYSTEM` at the null device. That
  replaces `$HOME/.gitconfig` and the XDG file, so `HOME` itself is kept for the Go build and module
  caches. A failed bind `git add` or `git commit` used to be ignored. It now records its exit status
  as `bindExit`, and the row reads `HARNESS-FAILURE`. A mutation that commits nothing is also
  `HARNESS-FAILURE`, never a measured refusal.
- V1-0161: L3c re-encoded the CEM with the same canonical encoder, so it committed identical bytes
  and passed as a no-op. V1 writes a noncanonical encoding that differs from the committed bytes.
  Each designated loop and CEM case now names the refusal it was designed to provoke. A case whose
  refusal does not match is recorded as not informative. The refusal parser keeps the `fix:` line,
  so a report-drift refusal and a not-complete refusal no longer read the same.
- V1-0369: `agent_cost` claimed savings from any two arms. The arms could hold different cases,
  duplicates, malformed token counts or a failed treatment. V1 names each problem in
  `invalidObservations`. It makes a savings claim only on the same unique cases in both arms,
  well-formed rows and zero treatment human failures. Otherwise the result reads "measured, no
  savings claim". With no observations the metrics stay `NOT_OBSERVED`.
- `docs/build-log/` is excluded from a task's changed paths, like `docs/BUILD-LOG.md`, as decision
  0423 requires of a later revision.

Two further v0 defects came to light during the port, and V1 fixes both:

- An operator-shell `CORVINT_BIN` selected the binary the loop measured. V1 drops every inherited
  `CORVINT_*` variable, so the loop always builds the candidate.
- A reused run identifier silently overwrote the earlier run file. V1 refuses
  `run-id-already-used`, and `run-id-invalid` for an identifier outside `[a-z0-9-]`.

PCCO-V0-015 now names the committed `preregistration.sha256` as the binding seal and states the
input scrub. PCCO-V0-017 now requires matched cases, well-formed rows and no treatment human
failure before a savings claim. Both requirements remain proposed, not accepted.

Evidence: `py_compile` passes. A scratch check (not committed) passed for all of these:

- unmatched arms, a failed treatment, and matched clean arms give the expected savings claim;
- each malformed row class and a duplicate case are named in `invalidObservations`;
- a refusal with a `fix:` line parses to its own code;
- `expect` marks wrong-code and `HARNESS-FAILURE` cases as not informative;
- the noncanonical re-encode differs from the canonical bytes;
- inherited `DOGFOOD_*`, `CORVINT_*` and `GIT_*` values do not reach a step;
- `run` refuses `preregistration-seal-missing` and `run-id-invalid`.

The M1 to M6 expected codes match the v0 run-001 and run-002 CEM results.

Not done: V1 is unsealed. It has no `preregistration.json`, `corpus.json` or
`preregistration.sha256`, and `run` refuses until they are committed. The v0 corpus cases were
observed by run-001 and run-002, so they are development data for V1, not a held-out corpus. The
loop-level expected refusals come from the untouched-repository smokes. Before the seal they need a
V1 smoke on the Corvint `make` path, and the seal then names the rc.1 candidate, the corpus head,
the partition and the digests.

Rollback: delete `benchmarks/daily-loop-v1/`, remove it from the PCCO-V0 row in
`docs/specs/INDEX.json`, and revert the PCCO-V0-015 and PCCO-V0-017 text and the added trace row.
