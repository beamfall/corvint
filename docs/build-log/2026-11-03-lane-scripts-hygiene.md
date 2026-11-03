<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-4350, V1-4351, V1-4352: lane-scripts suite hygiene and a stray-backup gate

Base: `030c7b570ee95398a00b22e070cef2da33365b34` (`origin/main`). Branch `claude/cloud-lane-hygiene`,
a cloud session spending spare credit on self-contained 1.0 work. Requirements `ASD-V0-2370..2372`
in `docs/specs/corvint-self-development-v0.md` (block `ASD-V0-2370..2379`, unused on the base: no
`ASD-V0-237x` row in `docs/specs/REQUIREMENTS.tsv` nor anywhere in the tree). No `.github/` or
`.taskman/` file changed; no Tasks state was written.

## Decisions

`ASD-V0-2370` (V1-4352). `internal/jstestprovider/node_identity_test.go.bak` is removed, and
`script/check-stray-backups.sh` refuses any indexed path ending in `.bak` or `.orig`, in any letter
case, with no allow list. It reads the index, not the worktree, as the binary-blob gate does, so an
untracked editor leftover never fails a lane. It joins `DOC_GATE_TARGETS`. CI's doc-gates job runs
`make -s doc-gates`, and `script/batch-precheck.sh` derives its doc-gate rows from that target
(`doc_gate_expansion`), so both run the new gate with no `.github/` edit and no CI patch is needed.
It is also a member of `STATIC_GATES`, the seconds-long fail-fast phase, with its own `fix:` line.

`ASD-V0-2371` (V1-4350). The suite already created `$tmp` under the caller's `TMPDIR` but left
`TMPDIR` itself pointing at the caller's directory. Two kinds of fixture inherit it: `heavy.sh`
runs, which create their private `heavy-run.*` directory under the inherited `TMPDIR`
(`ASD-V0-734`) and leave it when a fixture kills the wrapper, and runs that derive
`corvint-go-build-cache` from it. The fix exports `TMPDIR=$tmp/tmpdir` right after the root is
made, rather than chasing each fixture. HUP, INT and TERM now exit through the EXIT trap, which
removes the root.

`ASD-V0-2372` (V1-4351). Every group already ran with `HEAVY_TMP_FSTYPE=disk`, so the probe itself
(`stat -f -c %T /tmp` on Linux, nothing on macOS) never ran and the tmpfs branch ran only through
the override. A new group unsets the override and puts a fake `stat` and `uname` first on `PATH`.
The fake `stat` answers the probe with `tmpfs` and then `ext2/ext3` and passes every other call
through. The fake `uname -s` answers `Linux`, so a macOS host probes too. The group then asserts
each branch and that the probe was consulted. The fakes follow the suite's existing stubs for
`go`, `ssh`, `sysctl`, `df` and `git`. The rules are documented in the test header.

A host difference was not reproduced here. On this Linux host, a full run with an ext2/ext3
`TMPDIR` and one with a tmpfs `TMPDIR` (`/dev/shm`) gave identical group results before the change.
The hsh-w48 split is therefore attributed to the unexercised probe and to scratch reaching the
caller's `TMPDIR`. That attribution is unconfirmed: wave 1103's own logs were not available to this
session.

## Evidence

- Leak, before (base): `TMPDIR=<fresh dir> bash script/lane-scripts_test.sh` left
  `corvint-go-build-cache`, `heavy-run.7gwhBB` and `heavy-run.kh7hnj`. The same three kinds of
  entry were left on the tmpfs run and on a non-root run of the base.
- Leak, after: a `find`-snapshot of a fresh `TMPDIR`, and an `ls -A` of `/tmp`, taken before and
  after a full run, are identical. A run sent TERM mid-run (exit 143), and one whose process group
  was sent INT, each left the `TMPDIR` empty.
- `bash script/lane-scripts_test.sh`: 54 groups passed, 5 failed as root. The new
  `ASD-V0-2372` group passes. The 5 failures appear identically on the base under root. Four
  (unreadable-file, idle-check unreadable, lane-clean unresolved root, read-only `TMPDIR`) pass as
  a non-root user. The fifth, `lane-setup: REFUSE unknown-base origin/main`, and the landing-watch
  reconcile group fail on the base too as a non-root user from a local clone. They are not caused by
  this change. As a non-root user: 57 passed, 2 failed, against 56 passed and the same 2 failed on
  the base.
- `bash script/check-bash32_test.sh`: PASS. `BASH32_INCLUDE_UNTRACKED=1 script/check-bash32.sh`: ok.
- `make stray-backups-check`: PASS. Before the `.bak` removal it fails, naming that path.
  `make stray-backups-test`: PASS.
- `make spec-requirements` and `make spec-requirements-check`: PASS (obligation-count warnings only;
  the self-development spec's `## Requirements` holds 247 of 256).
- `make doc-gates`: the first run failed `line-citations-check` (three specs cite `Makefile` lines
  that the two new `GATE_STEPS` lines moved), `shell-foreign-callers-check` (a comment in the new
  script named `script/batch-precheck.sh`) and `selection-baseline-check` (a shallow clone without
  the remote branch that holds a baseline head). The citations were repinned with
  `script/check-line-citations.sh --repin --changed --base 030c7b57...`, the comment was reworded,
  and the branch fetched. Each of the three then passed alone. `script/batch-precheck.sh --list`
  includes `make stray-backups-check`.
- `corvint affected --base 030c7b57...`: `scope=UNKNOWN dirty=9 selected=167 unknown=80`
  (`LANGUAGE_FRONTIER=76`, `UNOWNED_DIRTY_PATH=2`, `UNSELECTED_BINARY_CONSUMER=2`). The Makefile and
  spec edits fan out widely. No Go source changed, so the selected Go units and `make windows-vet`
  were not run; `make gate` was not run, per the owner.

## Limits

Pre-change Corvint context was not collected before the first edit. `corvint affected` ran after
implementation. The dogfood CEM bind, check and seal outcome is recorded in the pull request.
