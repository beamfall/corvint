<!-- SPDX-License-Identifier: Apache-2.0 -->
# Ratchetsel mainline merge-forward round 2

The owner requested merging mainline `8fc0cf37642e7012f4c26671c9ed97a8706d6e09`
into lane head `d0ac88e645ce5fbcfc285abfd0440a93c617950f`, preserving both sides
and binding against that mainline. The merge is
`087a6f5eaa2064257cc2a0bcc2b26c53bf527076`.

The six conflicts were the use-case ledger and change-consequence implementation
receipt, capability catalog, affected-zero-unknown spec, compatibility-freeze
spec and falsifiable-packet spec. The receipt pair took mainline's pins before
repinning the merged affected implementation. Capability prose retains the lane's
text ownership guidance and mainline's local dry-run summary/explicit JSON modes.
The affected spec retains both ownership witnesses and mainline's driver repair
witnesses. The remaining two conflicts cite unchanged content at moved lines;
`check-line-citations.sh --repin` relocates their anchors to 369 and 1448–1461.

No requirement IDs collided. REQUIREMENTS.tsv and INDEX.json took mainline's
versions before `make spec-requirements`, the owning target for both files and
the README projection, restored AZU-V0-9900 through AZU-V0-9902. INDEX.json
needed no further change. No Go file required manual resolution or regeneration.
Automatic merges retain ownership selection and fixture classification alongside
mainline's scratch diagnostics, unbounded witness labeling and driver traversal.

The first focused check passed spec requirements but failed the two moved line
citations; the unchanged spans were inspected before repinning. The initial
`dogfood-change` refused `reused-worktree-state`: three ignored OCM sidecars
predated the new base. The prescribed recovery moved all three, without deleting
evidence, to this worktree's private Git evidence directory, `stale-ocm-round2/`.
Final evidence and exit codes are retained in the round's external handback.

Per the owner's round scope, validation is limited to the spec generator's
checks and `make -s spec-requirements-check line-citations-check`; there is no
manually resolved or regenerated Go package to test. No doc-gates, corvint test,
gate-affected, whole selection, repository gate, Tasks write, publication or seal
is authorized in this round. Host verification and batch CI remain subsequent
integration evidence. Stop after a passing dogfood-check against the new base.
