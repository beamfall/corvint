# Heading navigation integration evidence

The experimental heading-navigation branch is integrated with public main
`29274889ac6cf78a2c2029d64b1a6601f537441d` at merge
`f1d46cfccae088922b152bf7fd81b49fc11ab1dc`.

Git's automatic merge conflicted only in `docs/specs/INDEX.json`: concurrent final
entries represented documentation CI and the heading-navigation experiment. The
reviewed resolution retains both objects, synchronizes the HNE claim with its
spec/README, and preserves all other paths byte-for-byte from the automatic merge.
The assembled spec-index and heading-benchmark focused checks passed before the
integration evidence slice. The experiment remains proposed and experimental;
these checks do not promote retrieval behavior or establish external utility.

The original feature CEM `f9512e58c700602436e59c5b71a6ac9db49cc9998` and the
claim-repair CEM `95e8a25978ec07903273bcd60a9afc3581310b0e` remain sealed.
Rebinding main-to-merge correctly refused `sealed-cem-in-change`; its refusal and
merge-tree comparison are retained in the private qualification packet. This
slice uses the merge as baseline and binds this evidence entry only. Its enrolled
focused checks assess the assembled tree; this CEM does not rebind original source.
The existing retrieval-comparability contract supplies review context, while the
new HNE intent is proposed and does not gain acceptance from integration.

Rollback reverts the integration merge and this evidence slice without deleting
prior seals. Full repository qualification and external outcome evidence remain
separate, unclaimed gates.
