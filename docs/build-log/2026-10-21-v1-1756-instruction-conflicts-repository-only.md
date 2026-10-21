# V1-1756: `make instruction-conflicts` gives a repository-layer verdict without host files

Date: 2026-10-21. Lane: claude/batch-dogfood-flow (base 5cf75daf0). Follows V1-0721
(`docs/build-log/2026-10-04-v1-0721-instruction-conflicts-doctor.md`). The doctor is local and
read-only, and it is never a gate step.

Before this change, `make instruction-conflicts` with no `INSTRUCTION_HOST_FILES` ran the script
with no `--host`. The script then printed `UNKNOWN host layer not observed` and exited 3 even when
the project files were clean, so the target never returned a usable repository verdict.

Decision (delegated, conservative): the scope is declared explicitly, not inferred from a missing
argument. The script gains `--repository-only`:

- It prints `NOT_OBSERVED host layer: --repository-only; ...` and `result: NO_CONFLICT_FOUND
  (repository layer; host NOT_OBSERVED)`, and exits 0 when the project files are clean.
- A conflict between project files still exits 1.
- An absent or unreadable project file is still `UNKNOWN` and exits 3.
- `--repository-only` together with `--host` is a usage error (exit 2), so a host file is never
  silently ignored.
- The script's default is unchanged: a direct call with no `--host` and no `--repository-only` still
  exits 3 with `UNKNOWN`.

The make target passes `--repository-only` only when `INSTRUCTION_HOST_FILES` is empty, and
otherwise passes `--host` once for each named file, as before.

Evidence: `script/check-instruction-conflicts_test.sh` covers both modes:

- case 5b: clean, conflict, absent project file and the `--host` usage error under
  `--repository-only` (the new case fails against the previous script);
- the existing case 5: the default without `--host` stays `UNKNOWN`.

The target's expansion was observed rather than pinned:

- `make -s -n instruction-conflicts` prints `--repository-only`;
- with `INSTRUCTION_HOST_FILES='a b'` it prints `--host a --host b`.

A pinned `make -n` case would make the shell test read the repository root, which
`unbounded-readers-check` refuses. Run on this repository, the target exits 0.

Rollback: revert the script, the test and the Makefile hunk.
