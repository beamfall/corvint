# pk-affected verifier repair

Owner intent: the 2026-11-05 repair charter for V1-4392 and V1-4279, against
`6c33df33bc5670131482f5263b410cb980580657`, starting at
`9c6bd9bd646864452db366411b178a704df25a21`. Restore frozen outputs, repair Windows
type checking, qualify both failing packages, and bind/check without sealing.

The previous handback and host verifier log are the original failure evidence.
Raw evidence for this round lives under `/home/russell/lanes/pk-affected-evidence/`.
The external handback retains command results, digests, final identity and limitations.

## Causes and decisions

- The context fixture's expected commit is `9ab69ef47d196e1a757a3de3319527eb3686ff8b`.
  Hashing the same commit content with Russell's author/committer identity produces
  `05d3142624a106eafdac3a4f644b6f44eacaf3eb`, exactly the verifier's observed value.
  The verifier inherited seal identity while testing. Context and genesis/Core fixtures
  pass with identity confined to bind/commit commands. Their golden bytes are retained.
- Strict/coverage receipts acquired `viaLength` on ordinary full witnesses through the
  lane's `Witness.MarshalJSON`. Remove that extra member and restore the full committed-range
  golden exactly to BASE. TSP-V0-7303's `anyDirtyPath:true` remains on unbounded witnesses.
  TSP-V0-7311's earlier uniform-length proposal yields to the charter's frozen full profile:
  full witnesses retain `via`; compact/by-path forms carry `viaLength`. The shared decoder
  recovers chain length from `via`. Exact projection comparisons remain, with new explicit
  assertions that compact length matches the chain and full witnesses omit the extra field.
  The two exact ordinary-witness expectations are restored to their pre-lane values.
- Windows vet cannot see `sessionFixture`, which is declared in a Darwin/Linux test file.
  Move its calling dry-run projection test into that same file; keep every test assertion.
- The live selection gate failed again before the performance repair: normalized p95
  150.84, 134.56 and 134.36 ms, with all three rounds over the unchanged 100 ms bound.
  A CPU profile attributed 53.80% of sampled CPU cumulatively to `Graph.pathTokens`.
  Prepare those tokens once in `Graph.finish`, before publishing the immutable graph.
  Selections only read the prepared slices. Graph-cache restore uses the same construction;
  cache bytes, graph digest inputs, plan witnesses and selection rules are unchanged.
  AFP-V0-021/549 parity compares complete plans against per-selection preparation for six
  dirty sets, including concurrent reads. The full affected package then passes: raw p95
  10.026378 ms, normalized p95 13.355637 ms, 542 units and 160 samples in its first round.
- Scratch fixtures discover the sandbox's enclosing `/home/russell/lanes/.git` without
  an explicit Git discovery boundary. Qualify the CLI package with
  `GIT_CEILING_DIRECTORIES=/home/russell/lanes`; the mandated TMPDIR stays unchanged.
  Negative-fixture assertions and production diagnostics are preserved.

## Evidence and limits

The unchanged `TestNewLeafPackageWithoutShellCallerIsBounded` verifies V1-4392 under
AFP-V0-160/511/560/581. `TestUnboundedWitnessSaysAnyDirtyPath` verifies V1-4279 under
TSP-V0-7303. Both pass in the whole affected package, together with
`TestPreparedPathTokensPreserveSelection` and the unmodified live latency gate.
Windows vet passes after the platform repair and token preparation change.

CPU profile SHA256: `23e75029de4ee4d2b1b325d3c30a1e64ce6b32852999fedd47a21f44fba51774`.
Whole affected log SHA256: `74fce8f86fd3645efce3e2a6e68fb41a4bb2859423892364b1f818acea57e56d`.
These observations establish the named Linux run; a quiet-host/platform qualification
campaign and the orchestrator's independent review remain separate.

Rollback removes the prepared-token field and fast path, restoring per-selection preparation.
Frozen full-wire restoration and fixture/platform repairs stand independently of that optimization.
