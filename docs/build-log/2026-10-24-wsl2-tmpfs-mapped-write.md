# 2026-10-24 V1-2554: the first mapped write moves no time on tmpfs

`TestCALV0070_MutateRefusesMappedWriteAfterMergedAudit` failed both subtests on the WSL2 offload
host at unmodified origin/main `0949876eb`: `change watch on the first mapped write: <nil>`. The
PSR-V0-223 assertion expects the first write through a new shared mapping to update the
modification and change times the Linux change guard re-reads. The test passes on Darwin, where it
expects nothing before `msync`.

## Diagnosis

The host is Ubuntu 26.04 on WSL2 (Linux 6.18.33.2-microsoft-standard-WSL2), with root on ext4.
Its `/tmp` is tmpfs (`findmnt -T /tmp`), and `TMPDIR` is unset, so `t.TempDir()` and the store
fixture live on tmpfs. The eight self-hosted runners `corvint-box-1..8` run on the same host.

A probe outside Go mapped a 100-byte file shared and writable, set its times an hour back, read the
last byte, wrote it and re-read the stat:

| directory | filesystem | first write moved mtime | moved ctime | second write moved mtime |
| --- | --- | --- | --- | --- |
| `/tmp` | tmpfs | no | no | no |
| `/dev/shm` | tmpfs | no | no | no |
| `$HOME` | ext4 | yes | yes | no |

ext4 hooks the write fault (`page_mkwrite`), which calls `file_update_time`; tmpfs's shared
mappings have no such hook, so no time moves. The cause is the filesystem. Watch and inotify
behaviour, timing and Landlock are not involved: the guard reads the stat identity correctly, and
the same test at origin/main passes on the same host with `TMPDIR` on ext4 (log line 142 refusals
`JOURNAL_FORKED` and `INTENT_DIVERGED`, both subtests PASS).

## Decision

PSR-V0-226 amends PSR-V0-223's shared-mapping limit: on tmpfs the stat identity does not see even
the first mapped write, and only CAL-V0-070's content digests do. The test now detects tmpfs by the
store file's statfs type (`TMPFS_MAGIC`), logs it, and there requires the first write to go unseen.
On every other Linux filesystem it still requires `SNAPSHOT_MOVED`. It is not skipped: the
later-write and `Mutate` refusal assertions run on every host. The test passes in the GitHub-hosted
`ubuntu-24.04` shards at origin/main, so their temporary directory hooks the write fault and CI
keeps the PSR-V0-223 first-write assertion (inferred; that runner's filesystem type was not
observed in this change).
No product code changed.

## Evidence

All runs on the WSL2 host (linux/amd64, go1.27.3):

- origin/main, `/tmp` tmpfs: FAIL, both subtests at the first-write assertion (`change watch on the first mapped write: <nil>`).
- origin/main, `TMPDIR` on ext4: PASS.
- this change (`c7f31defc` tree), `/tmp` tmpfs: PASS; both subtests log the PSR-V0-226 tmpfs line.
- this change, `TMPDIR` on ext4: PASS, with the first write seen as `SNAPSHOT_MOVED`.
- this change, the whole `./internal/tasks/store` package: PASS (455 s).
- this change under the CI `go test -exec test-confine` wrapper (Landlock ABI 7, `-race -p 1`, `/tmp`
  tmpfs): the test alone PASS, the whole package PASS (757 s).
