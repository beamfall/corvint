# rc.2 preparation: urfave preregistration and release policy published

Human-owned intent: on 2026-10-05 the owner answered "as recommended. get to rc2" to the proposal to
publish the urfave/cli selection and the rc.2 signing and support policy that were prepared on
2026-09-29 but never reached `main`.

## Decision

- The 2026-09-29 commits `b436fadd` (urfave preregistration) and `96a69bbd` (rc.2 and stable
  policy) are carried onto `main` by this change, once merged, with their content unchanged except numbering. They were prepared
  as decisions 0427 and 0428, and `main` has since accepted different records with those numbers, so
  they are published as 0432 and 0433. The sealed `urfave/preregistration.json` and
  `urfave/admission.json` keep the label `0427` and are not edited; decision 0432 records the mapping.
- Decision 0432 also supersedes the naming of spf13/cobra in decision 0427 item 4(a). The rc.2
  untouched-repository run uses urfave/cli, Corvint at loop target `a5a7412c` and beamfall/core.

## Evidence

- Harness `benchmarks/untouched-repository-v1/harness.py` on `main` is unchanged:
  sha256 `b67cc95abd92ffffeef9bf1bccfce4d0316e270716d6cd065948e802994f9eb2`.
- A fresh, non-shallow clone of `https://github.com/urfave/cli.git` has commit `7389061e` with tree
  `7b3d3d69`. `harness.py corpus` over it reproduces `urfave/corpus.json` byte for byte
  (sha256 `b9cf1407...`). `preregistration.json` hashes to its seal `eedf799f...`.
- `git grep -i urfave` over `origin/main` and every fetched `origin` branch's `benchmarks/` and
  `docs/build-log/` finds only V1-0019's own owner-disposition note.

## Limits

- No candidate exists and no case ran; every qualification row stays `NOT_RUN`. The independent
  preregistration review ran after the seal, not before it; decision 0432 records that sequencing
  deviation and the owner's acceptance of the unchanged seal. The bounded
  provenance search cannot observe private or unrecorded use.
