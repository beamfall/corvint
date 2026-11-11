# Runner protocol host and conformance kit

This is the shadow implementation of RSP-V0-025 through RSP-V0-032 and RSP-V0-041.
It adds no production selection, release-manifest entry or core command.

`registry.RunnerPlugin` remains the common interface. `Session.Plugin` returns a
subprocess adapter implementing it. `Reference` is the compiled-in reference Go
adapter; `tools/runner-reference` serves the same implementation. Its native report
parser is the existing compiled-in Go runner parser. Its inventory artifact is the
protocol's `{config_digest, tests:[{id,path}]}` object captured by the core. The
reference always describes a superset command, has no qualification, and returns
explicitly absent native evidence. It is an interoperability fixture, not a
qualified production inventory collector.

The host API sequence is:

1. Read immutable project pins with `ReadProjectPins`, supplying the core's trusted
   object reader and trusted base. CI candidate changes make the union of old and
   new claims FULL. A reader error wrapping `fs.ErrNotExist` means no declared
   plugins; any other read error or malformed file makes `FullAll` nonempty.
   Pins claiming compiled-in runner kinds are refused. The reference twin claims
   `reference-go-test`, while its report parser still uses the Go implementation.
2. Read the operator store's `runner-plugins.json` and/or supply the release's
   declared entries. `ResolvePlan` retains reasons for every unavailable runner.
   Repository executable fields are source labels; they are never launch paths.
3. `Open` one session per run, with an external core-owned run directory. Obtain
   each declared runner's SPI adapter with `Plugin`. The core owns runner command
   admission and tracing; the host never executes returned argv.
4. `Evidence` stages validated native components and returns
   `evidence-pending-session-close`. Always call `Close`, even on failures. Only
   after successful close does the same `Evidence` request return the staged
   components. The host has no record-store writer or qualification authority.
5. Use `FullPlans` after failures. Successful sessions still require independent
   core qualification keyed to the installed executable digest.

`OpenHost` composes steps 2 to 5 for one run (RSP-V0-063): given the required
runner names, the discovery plan, the store and release entries and the core's
pre-plugin paths, it opens at most one session per required pin and its `Close`
returns one plan line per required external runner, always `full`, with `run`
`fail` and `runner-adapter-unavailable` whenever the runner is unpinned,
unresolvable or its plugin failed and no pre-plugin path exists. No production
run path calls it yet.

`Schemas()` exports closed JSON Schemas for all six method payloads and results.
The host and `Serve` validate these same definitions on both sides. Framing also
rejects duplicate members, invalid UTF-8, unknown envelope members, wrong IDs and
trailing output. Optional features must pass `RequireFeature` before use; declaring
a feature cannot extend the closed version-1 schema by itself.

`Conform` applies one captured-artifact corpus to any SPI implementation.
`CompareConformance` requires identical inventories, command descriptions, results
and native evidence. `TestRSPV0025TwinParity` uses that kit over the compiled-in
reference and a digest-installed subprocess of the same code. Requirement-named
negative fixtures cover framing, paths, pins, versions, authority injection,
crashes, timeouts, output bounds, repository mutation and failed publication.
Tests are deliberately NOT_RUN in this build-only lane.

Linux amd64, arm64 and riscv64 use a sealed executable descriptor, verified after
sealing, and execute that same descriptor. Other platforms or refused kernel
operations fail closed. Process-group cleanup uses the existing group reaper;
this is not a privilege or network sandbox. The plugin contract prohibits starting
tests. Installation is an explicit local operator trust decision.

Bounds include the specified protocol limits, discovery 1 MiB, executable 256 MiB,
and repository snapshot 100,000 entries / 1 GiB / 60 seconds. Status is obtained
through the existing isolated Git-status reader, plus content hashes of working
files, including already dirty and untracked files. Common Git config and hooks
are also hashed, including for linked worktrees; objects and packs are excluded. An unavailable snapshot,
special file, symlink escape, unknown platform or cap hit cannot certify success.
Snapshots conservatively include ignored files; core artifacts belong outside the
repository. No caching, exclusion or qualification derives from these snapshots.

Overhead measurement is NOT_RUN. Integration owns conformance execution, independent
review and the final evidence binding. The fixture suite alone does not establish
external interoperability or runner soundness qualification.
