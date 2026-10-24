# Reviewer-trial cem/1.0 arm builder and ten-pair development run (V1-0836)

Intent: `docs/specs/cem-reviewer-trial-v0.md`. The 2026-10-24 amendments to CRT-V0-005,
CRT-V0-008 and CRT-V0-015 are accepted under the owner's standing lane-requirement auto-accept.
Base: `ec41e4e9c901cb3546e95a82fc665bb8bc6c58e9`. Scope:
- build the narrowest cem/1.0 builder for the `treatment` and `seeded` arms;
- run the ten-pair development set;
- freeze the held-out pair count.

The held-out selection and run are `NOT_RUN`, and the README's CEM row is unchanged.

## Builder (CRT-V0-005)

cem/0.x is retired, so `cem begin` is gone. For each change, the harness:
1. clones the prep clone at `C^` into a private target clone;
2. applies the presented patch with `git apply --index`;
3. commits it as a synthetic target `T` (fixed dates, so `T` is stable across builds);
4. runs `cem prepare --base C^ --target T`.

What this guarantees:
- **Leak control holds.** `T` is not `C` and exists only in the target clone, which is removed when the change ends. The prep clone stays clean and cannot resolve `T`.
- **Hunk numbering matches.** The map is refused unless its hunks match the patch's hunks in count, order, path and start lines.
- **Artefacts are stable.** The artefacts are the map, `cem status` and `cem report`. The target clone path is replaced by `<target-clone>`, so prompts are byte-identical apart from the prologue.
- **Citations score against a clean map.** They are replayed with `cem cite` into a pristine copy of the map, restored before each lane.

Registered runs still refuse both arms (CRT-V0-015): an all-`unknown` worklist is not the shipped profile's artifact. Whether it could be is left to the owner.

The first slice errored all 60 lanes with `repository-object-unavailable`. Cause: the stable verifier refuses a root reached through a symlink, and macOS `TMPDIR` is one. This product bug is already open as V1-2519. The harness now works under the symlink-resolved path. `TestMapArmsCarryAPreparedCEM10Map` sets a symlinked `TMPDIR`; without the fix it fails on linux/WSL2 with the same code.

## Development run

| Field | Value |
|---|---|
| Manifest | `tools/cem-trial/testdata/development-2026-10-24/tasks.json` |
| Manifest sha256 | `83ee379a67557cc0a9d97ec0df790909abb4c744ee5e0ea53f92bcf81aaa6030` |
| Partition | `development` |
| Seed | `dev-2026-10-24` |
| Population | 569, after excluding the pilot |
| Pairs | 10 |
| Agent | `codex-cli 0.162.0`, `gpt-5.6-sol` at `medium` |
| Repeats | 2 |
| Lane timeout | 8 min |
| Workers | 10 |
| Corvint | `1.0.0-rc.3 (build 0)` built from this branch |
| Report | `benchmarks/results/cem-reviewer-trial-development-2026-10-24.json`, sha256 `15d4634fe014ce31c6c308937c62ff5b48751896833e54e98789c3b712614adc` |

The report is committed unrepaired. `cem-trial score` rebuilds it byte-identically.

The run went in three slices, each kept under 10 minutes with `timeout -s TERM 555`:

| Slice | Result |
|---|---|
| 1 | Symlink failure. Discarded and not reused. |
| 2 | 38 lanes finished. 22 were canceled by the slice cut. |
| 3 | `--reuse` of slice 2. All 60 lanes finished. |

Ten of the canceled lanes read `codex: context canceled after 8m0s`. Every one of them had run for under 3 minutes: the suffix names the configured timeout, not elapsed time (CRT-V0-011's existing message). The final report has 0 errored lanes, 0 dropped pairs and is not invalid.

| Arm | Mean miss | Citable | Cited-not-gold paths |
|---|---|---|---|
| control | 0.540 | | 0.882 |
| treatment | 0.607 | 0.8125 (130 of 160) | 0.879 |
| seeded | 0.647 | 0.70 (112 of 160) | 0.884 |

The stem baseline mean miss is 0.232. In the treatment arm, 20 lanes cited; in the seeded arm, 17.

| Contrast | Δ | BCa 95% interval | Reading |
|---|---|---|---|
| treatment vs control | −0.067 | [−0.25, 0.033] | consistent with 0 but not with 20% |
| seeded vs control | −0.107 | [−0.30, 0.037] | consistent with 0 but not with 20% |

There were no verifier hard failures. These are development numbers and are never outcome evidence.

## Frozen held-out pair count (CRT-V0-008)

The delta for each change is control miss minus seeded miss, with each change's two repeats averaged first:

`0.10, 0.333, 0, 0, −0.5, 0, −0.5, 0, −0.5, 0`

That gives a mean of −0.107, a standard deviation σ_d of 0.290 and a variance of 0.0842.

The target is Δ = 0.20 × 0.54 = 0.108. With α = 0.05 two-sided and 80% power:

n = (1.95996 + 0.84162)² × 0.0842 / 0.108² = **56.6, so 57 pairs at two repeats**.

That is consistent with the planned ~55. Caveats:
- **Uncertain variance.** Its χ² interval over 9 degrees of freedom gives n between about 27 and 190.
- **Rough normal approximation.** The deltas are mostly 0 or ±0.5, so the formula is only an approximation.
- **Other contrast.** The treatment contrast would need 39 pairs, from σ_d 0.238.

Any held-out selection must pass both the pilot manifest and this one to `--exclude`.

## Not run

- Held-out selection and run.
- Registration.
- A second dogfood-change attempt. The first refused (NOT_PRODUCED: not-complete, no diff and no `DOGFOOD_INTENTS_FILE`).
