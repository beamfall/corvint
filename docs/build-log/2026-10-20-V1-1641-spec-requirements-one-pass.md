# 2026-10-20 V1-1641: one-pass spec-requirements generation

Ticket: V1-1641 (`make spec-requirements` ran the generator three times, one pass per output).

## Decision

`SRG-V0-018` is added to `docs/specs/spec-requirement-index-generator-v0.md` and accepted under the
owner's standing auto-accept of lane requirements (2026-10-11). `SRG-V0-007` is amended to allow
the one write it names.

- `script/gen-spec-requirements.sh --all DIR` writes `REQUIREMENTS.tsv`, `INDEX.json` and
  `README.md` into DIR from one enumeration and one read of the staged specs. All three are built
  before any is written, so a refused projection writes nothing. `SPEC_GENERATE` in the `Makefile`
  is that one invocation, for both `spec-requirements` and `spec-requirements-check`.
- The default, `--index`, `--readme` and `--lines` modes remain, unchanged byte for byte. The
  conflict test and `script/spec-acceptance-audit.sh` still call them.
- Blob reads go through one `git cat-file --batch --buffer` co-process. A forked writer sends every
  `:<path>` request while the parent reads the replies. Neither side waits on a round trip, and
  neither can fill the other's pipe. The base had already replaced per-file `git cat-file blob`
  forks with one co-process, but it still made one synchronous request per file. A path the batch
  cannot frame (a newline) or serve as a blob falls back to the one-shot read, as before.
- `SRG-V0-001` is unchanged: the input set is `git ls-files` and the bytes are the Git index's.

## Evidence

Byte identity on the live corpus at base `f3ca7224`: the old three-pass generator's default,
`--index`, `--readme` and `--lines` output, the new generator's four modes and the new `--all`
files were compared with `cmp`. All were equal to each other and to the committed files.
`script/gen-spec-requirements_test.sh` case 10 retains the comparison, and the refusal and
fallback paths, as a test.

Timings on the same host (darwin, `/usr/bin/time -l`). The ticket's 34 to 52 s was measured
before the base's single co-process landed. At this base the remaining cost is perl CPU per pass.

| Measurement | Before (base) | After | Host load (`uptime`, 1 min) |
|---|---|---|---|
| `make -s spec-requirements` | 2.68 s real, 2.31 s user, 0.11 s sys | 1.17 s real, 0.87 s user, 0.05 s sys | 29.26 before, 15.60 after |
| `make -s spec-requirements-check` (includes `go test ./internal/specindex`) | 4.54 s real, 3.89 s user, 0.35 s sys | 3.97 s real, 2.48 s user, 0.41 s sys | 27.56 before, 15.60 after |
| generator only, interleaved A/B, 3 runs each | 2.57 to 2.79 s real, 2.38 to 2.46 s user | 0.94 to 0.95 s real, 0.86 to 0.87 s user | 17.19 to 17.29 |

The before and after `make` runs saw different host loads. The interleaved generator-only A/B ran
under one load and gives the comparable figure, about 2.9 times less wall and CPU time.

Checks: `script/gen-spec-requirements_test.sh` and `script/spec-requirements-conflict_test.sh`
pass, and `make spec-requirements-check` passes.

## Rollback

Revert the commit. `SPEC_GENERATE` returns to three invocations. No generated file moves, because
every mode's output is unchanged.
