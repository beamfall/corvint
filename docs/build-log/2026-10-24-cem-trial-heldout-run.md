# Reviewer-trial held-out run, N = 57 (V1-0836)

Intent: `docs/specs/cem-reviewer-trial-v0.md` (CRT-V0-008, CRT-V0-010, CRT-V0-014, CRT-V0-015,
CRT-V0-016). The owner commissioned this run on 2026-10-24. The owner counts the all-`unknown`
cem/1.0 map that `cem prepare` builds as the shipped profile's artifact for this trial (CRT-V0-015).

**Result: neither hypothesis is supported.** The treatment arm is indistinguishable from control. The
seeded arm missed *more* evidence than control. No Corvint behaviour was changed in response to these
results.

## Registration and selection

| Field | Value |
|---|---|
| Registration | `tools/cem-trial/testdata/heldout-2026-10-24b/registration.json`, commit `bc580295f`, committed 2026-10-24T20:59:18Z |
| Earlier registration | `tools/cem-trial/testdata/heldout-2026-10-24/registration.json`, commit `8423df09d`, committed 2026-10-24T19:48:16Z |
| Scorer sha256 | `f21b8bfac1e9e634d8590968ce272241d2a9a0410d451e7692d1355f9b286ea1` (`cat gold.go main.go register.go score.go select.go`) |
| Manifest | `tools/cem-trial/testdata/heldout-2026-10-24b/tasks.json`, sha256 `0b9c095838e7f8cac80d58566bdc5d4c7610469bc4e70d6e6e89d74958a10dee` |
| Partition / seed | `heldout` / `heldout-2026-10-24` |
| Population | 559, after excluding the pilot and development manifests |
| Pairs | 57, two repeats per arm, arms `control`, `treatment`, `seeded` (342 lanes) |
| Agent | `codex-cli 0.162.0`, `gpt-5.6-sol` at `medium` |
| Corvint | `1.0.0-rc.3 (build 0)`, sha256 `1cc99761720a1ec7bdec7fcc6aed07e1a5bddacc176518540db6422248f55041` |
| Hypotheses | H1: seeded mean miss is lower than control. H2: treatment mean miss differs from control. α = 0.05, primary metric `miss` |

## How the run went

1. **First attempt (invalid).** It was registered at `8423df09d`, and its first lane started at
   19:53:08Z. 83 of the 342 lanes errored, so CRT-V0-010 discards it. The errors:
   - 77 are agent-side failures: 26 websocket `403 Forbidden` from the model endpoint, 7 model-list
     timeouts, 4 local Codex skill or exec errors, and 40 bare `agent exited 1`. The host was under
     memory pressure at the time (fseventsd at 94 GB, swap full).
   - 6 are `cem prepare` refusals where the map's hunk did not match the presented hunk. This is the
     V1-2619 hunk slide below.

   The report is kept unrepaired at
   `benchmarks/results/cem-reviewer-trial-heldout-2026-10-24-attempt1-invalid.json` (sha256
   `a2212b2ea3d6b809fd965c1c5fe340f89b5bdfcdd7de66b7d86230ad846077c7`, manifest `879d19a2…`).
2. **Scorer change (V1-2619).** Before the rerun, `select` was changed to present the canonical diff
   options (`f99c64c6c`): `--no-ext-diff --no-color --src-prefix=a/ --dst-prefix=b/`, with no rename
   or copy detection. The fix stops hunks from sliding when local diff configuration differs. That
   changed the scorer digest, and so the registration (`21e21b860`, then `bc580295f`). The
   re-selection gave the same 57 pairs. Only `c01-5944eab4fd07`'s presented patch changed.
3. **Reuse rule (CRT-V0-016, `3074f8911`).** Lanes may be reused across a re-registration whose
   document differs only in `scorer_sha256`. The final report carries the earlier registration and
   that report's manifest digest as `re_registered`.
4. **Final run.** It used `--reuse` of the first attempt. **259 of the 342 lanes were reused**: each
   had a byte-identical prompt, did not error, and had an observed wall time. Those 259 lanes, and
   the pairs they cover, were **already observed in the invalid first attempt**. 83 lanes ran fresh
   (control 25, treatment 30, seeded 28). The final run had 0 errored lanes and 0 dropped pairs, and
   is not invalid. Each reused lane started after `8423df09d` was committed, and each fresh lane
   after `bc580295f`.

Final report: `benchmarks/results/cem-reviewer-trial-first-run.json`, sha256
`629b839edbe462fa820f82ab15cac033a774ad278ba29c6a1d1f99ca566d652b`. At `bc580295f`, `cem-trial score`
rebuilds it byte-identically. Rescoring is pinned to that tree, because a later scorer change would
change the digest.

## Results

| Arm | Mean miss | Citable material hunks | Cited-not-gold paths | Verifier hard failures |
|---|---|---|---|---|
| control | 0.573 | | 0.893 | |
| treatment | 0.554 | 0.819 (830 of 1014) | 0.892 | 0 of 114 citing lanes |
| seeded | 0.649 | 0.782 (793 of 1014) | 0.911 | 0 of 112 citing lanes |

The stem baseline mean miss is **0.370**, lower than every arm's.

| Contrast | Δ (reference − compared) | BCa 95% interval | McNemar (changes) | Reading |
|---|---|---|---|---|
| treatment vs control (H2) | +0.019 | [−0.048, 0.102] | 5 vs 2, p = 0.45 | consistent with 0 but not with 20% |
| seeded vs control (H1) | −0.076 | [−0.146, −0.004] | 3 vs 3, p = 1.0 | treatment missed more evidence than control |
| seeded vs treatment | −0.095 | [−0.183, −0.028] | lanes 13 vs 4, p = 0.049 | seeded worse |

The seed's suggestions covered 156 of the 210 gold items (0.743). When an item was suggested, the
miss rate was 0.647. When it was not, the miss rate was 0.870.

### Reading

- **H2 is not supported.** The all-`unknown` map does not change what the reviewer finds. Its
  interval excludes the registered 20% reduction.
- **H1 is not supported.** The seeded arm's interval sits wholly on the harmful side. The
  change-level McNemar test is a tie, so how large the harm is remains uncertain, but the direction
  is not toward a benefit.
- **The stem baseline beats every arm.** It names the test files by path stem. That suggests the
  reviewer's citations, not the map's contents, limit the score.

### Limits

- **K1 (decision 0523).** These runs predate the P2 baseline. Under the CRT-V0-015 pre-baseline
  clause, which the 2026-10-24 amendment left in place, this run is not outcome evidence for a
  shipped verifier-supported map. It measures the all-`unknown` cem/1.0 map that the owner counted
  for this trial, and nothing more.
- **Reuse.** 76% of lanes came from an attempt that was later discarded. Their prompts were
  byte-identical and their scoring is recomputed. CRT-V0-016 does not check that the scorer change
  leaves those lanes' scoring unchanged; prompt identity covers the prompts.
- **Single model, single repository.** The run covers `gpt-5.6-sol` at medium effort on Beamfall
  only.
- The README's CEM row is unchanged. No claim of a reviewer benefit is made.

## Review

Codex (one read-only round) reviewed `f99c64c6c` and `3074f8911` and found seven items, filed as
**V1-2624**:
1. the same-commit early return in `reRegister`;
2. a nil current registration that skips `reRegistrationInvalid`;
3. `ReusedFrom` carrying only the manifest digest, so `--reuse` and `--resume` cannot be told apart
   when the manifest does not change;
4. `--resume` dropping a checkpoint's `re_registered`;
5. variable shadowing in `merge`;
6. never-started pending lanes counted by `unstampedLanes`;
7. `canonicalDiffArguments` not neutralising config the way gitauth does (for example
   `diff.noprefix`).

None of them applies to this run. The manifests and the commits differ, nothing was resumed, every
lane in the earlier report was stamped, and the selection host sets no such config. They are left
unfixed here so that the tree stays at the registered scorer. Fixing them changes the digest and
needs a new registration for any later run.

## Not run

- Dogfood binding for this evidence commit: `NOT_PRODUCED` (evidence-only change to a registered
  harness tree).
- A second model or repository.
