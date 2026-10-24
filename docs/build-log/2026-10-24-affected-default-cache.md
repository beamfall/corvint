# Affected default graph cache under the user cache directory

Date: 2026-10-24. Lane: claude/affected-default-cache-1024 (base ec41e4e9c). GitHub #1601, tickets
V1-2473 and V1-2572. Requirements `AFP-V0-432..435` in `docs/specs/affected-plan-v0.md` are
proposed and experimental. Owner decision 2026-10-24 (`docs/build-log/2026-10-24-owner-queue-decisions.md`,
PR #1718).

## Decision

Without `--cache` or `--no-cache`, every plan form of `corvint affected` now keeps its graph cache at
`os.UserCacheDir()/corvint/affected`. That is `~/Library/Caches` on darwin, and `$XDG_CACHE_HOME`
or `~/.cache` on linux. The default is held to every AFP-V0-390 placement and handle rule and
keyed exactly as an explicit `--cache` entry. `--cache DIR` moves it and `--no-cache` turns it off;
the two together are refused.

A default location that cannot be used is not an error. That covers a lookup that fails or is not
absolute, a placement `--cache` would refuse (for example `HOME` inside the repository), and a
directory that is unwritable or not private. In each case the call runs uncached, prints the same
bytes and writes nothing to stderr.

AGENTS.md invariant 4 now names this cache as a bounded, private user-cache write outside the
repository. The AFP-V0-390..397 non-goal "no default cache location" is removed, and AFP-V0-390
and AFP-V0-431 are amended.

## On-disk encoding

There is no new encoding. Entries keep the `corvint-affected-graph-cache/0` profile, file name and
key of AFP-V0-391. An entry written by an older binary at an explicit `--cache` stays readable
under the same identity rules: a different executable never reuses it and rebuilds instead. The
new read-only report `affected-cache-status/0` (AFP-V0-435) is printed only by
`corvint affected cache` and is never stored.

## Doctor visibility

There is no `corvint doctor` command. Visibility goes through `corvint affected cache`, which prints
JSON. `--format doctor-plugin` prints one `AFFECTED_CACHE` TQD-V0-010 item, which a
`corvint-tasks doctor --plugins DIR` executable can wrap. This matches the existing
`corvint changes behind --format doctor-plugin` pattern.

## Test isolation

The `cmd/corvint` TestMain replaces the user-cache lookup with one that fails, so no in-process test
touches the real user cache. The default tests inject a temporary directory.
`script/gate-affected_test.sh` runs its built planner with `HOME` and `XDG_CACHE_HOME` under the
test root. `tools/corvint-pr-tests` already sets `HOME` to a runtime directory.
`internal/companionrelease` sets `HOME` to the repository root, so the default there degrades to
uncached by AFP-V0-434.

## Measurement (darwin)

- Host: darwin arm64, under load from concurrent lanes (load average 25 on 12 cores).
- Repository: this repository at 8b4bc545c, clean.
- Binary: one locally built binary, run as
  `affected --path cmd/corvint/affected.go` with a fresh scratch `HOME`.

| call | wall time |
| --- | --- |
| `--no-cache`, first run | 2.09 s |
| `--no-cache`, second run | 1.20 s |
| default, cold (stores the entry) | 1.32 s |
| default, warm (three runs) | 0.42 s, 0.42 s, 0.43 s |

The warm time includes hashing the 44 MB executable for the entry identity. The three receipts were
byte-identical (`cmp`). Afterwards `affected cache` reported `PRESENT`, 1 entry, 4,757,378 bytes.
Linux and WSL2 wall times were not measured: `NOT_OBSERVED`.

## Open risk

Entries of deleted roots, such as removed lane worktrees, accumulate at one per root, because there
is no eviction. `affected cache` reports the size. Pruning is filed as V1-2610.
