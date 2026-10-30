# Corvint Pulse snapshot P0-A conformance

This directory freezes only the proposed `corvint-pulse-snapshot/0` P0-A stdio actor surface:
initialization, capability reporting, workspace-generation invalidation, explicit unsupported
snapshot/overlay operations, protocol/session/sequence rejection, and clean shutdown.

Self-check the dependency-free corpus without a product binary:

```sh
python3 conformance/pulse-snapshot-v0/run.py
GOTOOLCHAIN=local go run ./conformance/pulse-snapshot-v0
```

Run the frozen transcripts against an explicitly supplied executable command:

```sh
python3 conformance/pulse-snapshot-v0/run.py --command \
  /absolute/path/to/corvint-pulse --root /absolute/path/to/disposable-repository serve --stdio
GOTOOLCHAIN=local go run ./conformance/pulse-snapshot-v0 --command \
  /absolute/path/to/corvint-pulse --root /absolute/path/to/disposable-repository serve --stdio
```

Both independent runners invoke the argument vector directly without a shell, start a fresh process
for every case, send the case on stdin, and require exact exit, stdout, and empty-stderr bytes. The Go
runner embeds and pins the corpus root, bounds both output streams and line counts, applies a
five-second per-case deadline, and kills the owned process group on POSIX completion, timeout,
SIGINT, or SIGTERM. It hashes an explicitly supplied `--root` and any linked-worktree Git directory
before and after all cases to reject repository mutation, and replaces the executed `--root` value
with that same resolved directory. Snapshot reads are descriptor-relative through `os.Root`: regular
files are opened nonblocking and identity-checked, symlinks are hashed without following them, and
FIFOs, devices, sockets, or platforms without descriptor-safe roots fail closed. `$SESSION` in frozen
expected lines is replaced only by the fresh session token observed in that case's `initialized`
response. It is not accepted in requests or any other field, and a session token may not be reused
across case processes.

Success establishes only transcript conformance for P0-A. It does not establish full WSI capture,
overlays, `VALIDATED_AT`, `CURRENT`, WEI, results, providers, execution, test selection, language or
operating-system qualification, network absence, product delivery, licensing terms, or any LPCV
promotion gate. The Go runner's current-run containment and repository check do not qualify other
operating systems or prove that a child could not deliberately detach from the owned group.
