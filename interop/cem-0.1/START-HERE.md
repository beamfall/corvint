# CEM 0.1 external consumer start

Status: local experimental packet; not yet a release archive.

This is a protocol-falsification experiment, not an endorsement or integration request. In ten
minutes, a volunteer should be able to verify the frozen fixture packet, run its local preflight,
create an owner-only observation, and start a clean-room consumer. Implementing the complete strict
verifier may take up to one engineer-day; ten minutes is the start target, not a completion claim.

The runner uses the Python standard library and local Git. It invokes a consumer only through the
process ABI in `ADAPTER.md`; it does not import, link, translate, or subprocess Corvint. It uses only
the synthetic fixture repository and makes no network request. It is not an operating-system network
sandbox for an untrusted implementation, so run an unknown executable inside your normal local
sandbox or offline environment.

The runner reads the executable entry point once and invokes a fresh private read/execute-only copy
of those exact bytes for every case. It starts each child in a new process group and terminates
members that remain in that group on completion, timeout, SIGINT, or SIGTERM. A child can detach into
a new session or process group and can still make host or network syscalls; containing either
behavior requires an OS sandbox or equivalent external supervisor.

`runner.py` is the packet's historical non-fixture executable, changed only by erratum 1's manifest
pins, and is not normative CEM
behavior. Active executions use the native Go command at `../../tools/cem-interop-runner`; retaining
`runner.py` preserves old packet identities and does not make old receipts current. The raw
manifest, algorithms, schema, and fixtures remain the implementation-neutral contract.

## Ten-minute start

Use POSIX, Go 1.27 or newer, Git with SHA-1 and SHA-256 repository support, and a private output
directory that already exists. The output path and its parents must not be symlinks; on macOS use
`/private/tmp/...` rather than the `/tmp` symlink when placing the observation in temporary storage:

```console
go run ../../tools/cem-interop-runner doctor
mkdir -m 700 private-observation
go run ../../tools/cem-interop-runner start --lane consumer \
  --output private-observation/cem-interop.json
```

`doctor` first verifies the frozen raw manifest SHA-256, captures all 51 manifest-declared fixtures
through bounded descriptor reads, and reconstructs the exact SHA-1, SHA-256, and six drift
repositories from that snapshot. It also reports `packetSha256`, `corvintCommit`, and `corvintTreeState`.
`start` refuses to replace an existing observation. A persistent owner-only
`.cem-observation.lock` sibling file serializes cooperating runner transactions with a five-second
acquisition deadline; keep it beside the observation.

Build a single executable entry point in a separate repository using only this packet, standard
language documentation/libraries, and local Git. Do not inspect Corvint source, tests, history, or the
Corvint-authored Go portability probe. Then run:

```console
go run ../../tools/cem-interop-runner consumer --implementation /absolute/path/to/consumer \
  --observation private-observation/cem-interop.json
go run ../../tools/cem-interop-runner inspect \
  --observation private-observation/cem-interop.json
```

The first completed 32-case run is retained. `--retry` is rejected until that run exists. One
explicit repair run is then permitted; a third run is refused. Concurrent cooperating runners are
serialized, and replacement compares the source observation digest before writing. The observation
contains fixture case names, decisions, durations, exit codes, failure classes, and digests. It
never stores source or patch bodies, stdout/stderr, an implementation path, an absolute temporary
path, command arguments, prompts, or environment contents. A runner-level abort leaves the prior
observation unchanged and prints only a bounded terminal error; it does not persist partial case
progress. Review the observation before sharing anything.

Observation profile `cem-external-interop-observation/1` records `publication: COMMITTED`, the
framed digest of the public contract/harness/schema packet, and the Corvint checkout commit/tree state.
A packet digest covers `ADAPTER.md`, `ALGORITHMS.md`, `IMPLEMENTATIONS.md`, `START-HERE.md`,
`SUBMIT.md`, `manifest.json`, `observation.schema.json`, and `runner.py`; the manifest pins the 51
fixture bytes. Entries are sorted and framed by four-byte filename length, filename, eight-byte
content length, and content before SHA-256.
A post-link or post-replacement cleanup/fsync fault is reported as success only when the exact
committed bytes can be read back. `COMMITTED` describes observable destination bytes, not guaranteed
crash durability after a directory-fsync failure. An independent matrix submission requires a clean
public commit whose recomputed packet digest matches the observation.
SIGINT or SIGTERM during publication follows the same rule: exact new destination bytes reconcile as
`COMMITTED`; any unverifiable publication attempt fails explicitly as
`observation-commit-uncertain`, never as a misleading plain interruption.

## Passing this packet means only

- 7/7 valid cases accepted;
- 19/19 invalid cases rejected;
- 6/6 drift decisions and exact records reproduced;
- SHA-1 and SHA-256 both supported; and
- no required case returned operationally unsupported.

It does not prove semantic evidence quality, usefulness, independent authorship, product value, or
standard status. The public drift fixtures each contain one evidence item; multi-evidence ordering is
not exercised and remains a versioned next-suite gap. Read `SUBMIT.md` before proposing a matrix
entry.

The manifest-derived implementation used by Corvint's own runner tests is harness plumbing only. It
branches on public fixture digests and is never independent semantic or conformance evidence.

Producer execution and sealed anti-copying qualification are `NOT_BUILT`. A consumer PASS may be
reviewed for `C1` or `C2`; this runner cannot fill `P1`.
