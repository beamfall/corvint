# Owner queue decisions (2026-10-24)

Date: 2026-10-24. The owner delegated the open owner queue to the orchestrator ("do the 'waiting
on you' for me"). Each entry below is the recorded decision, its reason and what implements it.
Each one can be reversed by a later owner decision that replaces the entry.

## V1-2564: corpus artifact reuse across a Go toolchain or engine change

**Decision: rejected.** An artifact's records are reused only when both the derivation and the
engine string match, as PR #1693 already does (DCP-V1-532/533). A Go toolchain bump keeps doing a
full rebuild.

**Reason:** the engine string is part of the admitted derivation. A toolchain can change Unicode
tables, regexp, sort and strconv behaviour without a derivation bump, so treating those records as
equal would be a certainty that nothing checks (invariant 2). The measured cost is a 5.0 s full
rebuild at 9,500 files on WSL2, paid once per toolchain bump. The ticket closes as decided, with no
code change.

## V1-2572: default `corvint affected --cache` location

**Decision: accepted, with these bounds.**
- The default cache lives under `os.UserCacheDir()/corvint/affected/`. It is never inside the
  repository, its Git directory or `.corvint/`.
- Each root has one entry, created 0700. The entry is keyed exactly as `--cache` is keyed today:
  executable identity, root and the stat pins of the walked files.
- `--cache DIR` overrides the location and `--no-cache` turns it off. `corvint doctor` reports the
  location and its size.
- The plan is byte-identical with the cache, without it, and from a cold cache. The cache is never
  an input to ranking, evidence or authority.
- Product invariant 4 in `AGENTS.md` is amended to name this cache as a bounded, private user-cache
  write outside the repository, admitted by its AFP-V0 requirement block.

**Reason:** an uncached `--path` call takes 9–11 s, and a warm one takes 1.8 s (GH #1601). Agents do
not pass `--cache` on their own, so making it opt-in leaves the measured speed-up unused. Keeping
the cache outside the repository leaves repository and trace state untouched. Implemented by lane
`claude/affected-default-cache-1024`.

## V1-1666: CCE-V0 owner questions

The CCE-V0 intent was accepted on 2026-10-22. The open questions in
`docs/specs/change-compiled-e2e-v0.md` § Owner questions are answered as follows.

1. **Input and fixture declarations:** keep them in `.corvint/e2e-blocks.json` for V0. Folding them
   into AFU-V1 flow intents is revisited in V1, after the block vocabulary is stable.
2. **`PRESERVES` and refactors:** `PRESERVES` does not count toward parity for refactors beyond what
   CCE-V0-037 already allows (its killed seeded mutants count). Only `DISCRIMINATES` counts toward
   usefulness.
3. **Snapshotting a `DISCRIMINATES` scenario:** yes. Once snapshotted it is an ordinary hosted test
   with normal review. Corvint never commits it itself.
4. **Frozen gate values:** accepted as written: 30 changes, 100% per-candidate parity, 95% total
   parity, zero escapes, and candidate minimums of 5 changes run and 1 seeded regression caught.
5. **Next runner after Playwright:** deferred until the Playwright V0 route is delivered.
   Nightwatch is a candidate, not a commitment.
6. **Reframing:** yes. V0 sells a per-change proof to the agent and reviewer, and retirement is the
   V2 payoff.
7. **`CHANGED_IN_DIFF` / `NEW_IN_DIFF` oracles:** these never count before the V1 acceptance
   ledger exists. A reviewer approving the PR does not make them count.
8. **Statistical gate:** the 5% Holm-corrected upper bound is accepted as written: at least 59
   caught seeded regressions with zero misses. No looser bound applies to a first retirement.

V1-1666 stays open on its remaining acceptance: the 30-change adopter shadow window needs an adopter
Playwright repository. Corvint itself is not one, so that item stays NOT_RUN.

## V1-2580 and the #1702/#1712 `cce-live` job: CI wiring

**Decision: accepted.** One `.github` PR adds two jobs:
- `cem-release-gate` on linux/amd64 and macOS arm64. Each builds `corvint` and
  `interop/cem01-go` from the same checkout and runs `make cem-release-gate`.
- `cce-live` on macos-15 arm64, as proposed in #1702 and #1712. It is path-filtered to
  `internal/changee2e/**`, and the pinned tuple is Playwright 1.64.0, Node 24.11.1 and headless
  shell 1248.

The `ci-control-plane` status is posted on the reviewed head after one clean Codex round and green
CI. Implemented by lane `claude/ci-cem-cce-jobs-1024`.

## #1689: opt-in self-hosted shard runner

**Decision: approved as code. The runtime switch stays with the owner.** Routing reads
`CORVINT_SELF_HOSTED` and is off unless that variable is `on`. Fork PRs and pushes to `main` stay
on hosted runners. Setting the variable is a repository setting the owner flips; the orchestrator
does not. A merge-queue entry can carry code from a fork PR that a maintainer queued, so the owner
should only turn the variable on while queued PRs come from the repository itself. Approval is
given on the exact reviewed head, after its failing shard is repaired.

## V1-2525: DCP-V1 spec over the 256-obligation bound

**Decision: split the spec. The OCM bound stays where it is.** The newest, least-cited requirement
blocks of `documentation-corpus-v1.md` move to companion specs with new prefixes, following the
leases precedent (`corvint-tasks-lease-gate-latency-v0.md`). This continues until every spec holds
256 or fewer. The old spec keeps a stub for each moved section, with an old-to-new ID crosswalk,
so GitHub issue references stay searchable.

**Reason:** changing the OCM map to carry only touched obligations would change what a CEM proves,
during the CEM 1.0 release window. Splitting the spec is wire-neutral. Implemented by lane
`claude/dcp-spec-split-1024`.

## V1-2203: heartbeat fsync floor

**Decision: accepted.** Heartbeat and renew receipts may be committed without their own fsync.
They become durable with the next synced write, or with a bounded group sync (at most 1 s). Claim,
submit, gate, complete, release and every authority-bearing write keep their full fsync.

**Reason:** a heartbeat or renew lost in a host crash can only make a lease look older, so the lease
expires earlier. That fails toward safety: the holder crashed with the host anyway. It never grants
a lease or loses an authority decision. Recovery must treat a missing tail heartbeat as an older
heartbeat, and a crash-injection test proves this. Implemented by lane
`claude/tasks-heartbeat-durability-1024`. The target is still a median of 50 ms or less on a quiet
host.
