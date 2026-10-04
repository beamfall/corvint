# Optional stable producer prototype: bounded historical join

Status: experimental proposal; independent implementation review and source
admission pending. No stable release, native completion or external adoption.

Gate A reviewed the R1 producer plan with no HIGH and three MED dispositions:
stage independently pinned original snapshot receipt bytes outside `.git`, reuse
the reviewed typed stable encoder/result, and retain receipt authority unknown
without selected-sequence journal verification. The immutable `1e71d5f5` source
and exact sixteen-file reviewed native repair overlay were frozen before the
producer build. Original candidate assembly and all historical packet bytes are
preserved.

The separate optional `assemble-stable` path constructs a new stable document
from genuinely parsed/canonically verified 0.2 or 0.3 input. It carries the full
stable result, preserves witnesses and native outcome bytes, computes the raw
snapshot artifact digest separately from native receipt identity, and regenerates
complete criterion/link IDs. Tasks semantic reads remain outside CEM/assembly at
the independently pinned native read boundary.

Actual historical proof preserved target
`a14f9d0123a923fa11ec411ec293a643856603e5` and base
`474f75da42ffab0f6b04425f26c8b655602d2ed8`. A pre/post-pinned native Tasks verifier
validated the retained capture; original actual Go plan/receipt, three declared
source blobs and native phase report bytes were retained. The producer emitted
stable map SHA256
`07926ddf783521da7a11d6aff9a3b1c89f030b2bba02cf99b30288d6b1061b87`.
Its embedded verification equals Core and portable CLI results across all 21
fields without normalization. The historical source has no 0.3 witnesses;
separate manufactured witness tests retain covered/uncovered, survived/not-run
and an uncompiled residual without claiming execution.

An initial SHA256 producer fixture exposed the existing native literal-config
restriction: ordinary Git initialization places `[extensions]` before `[core]`.
The unchanged stable verifier refuses that valid ordinary repository as
`unsupported-repository-envelope`. Exact generated config, Git version and full
refusal are retained; a separately ordered fixture is only fixture qualification.
Neither the historical repository/config nor the reviewed native overlay was
changed to hide the limit. Broader ordinary-checkout admission is separate work.

Evidence root:
`/private/tmp/cem10-build/stable-next/stable-producer-prototype`.
`gate-a-dispositions.json`, `source-provenance.json`, combined source pins,
`historical-proof/summary.json`, `ordinary-sha256-envelope/summary.json`, focused
test/vet logs and final patch/manifest retain reproducibility. Same-user ABA,
source completeness, execution-at-commit, selected receipt journal authority,
current applicability, criterion adequacy, all-runner qualification and external
consumer/outcome evidence remain unqualified. Rollback removes only the optional
operation; default producers, candidate behavior, historical bytes and native
store history remain intact.
