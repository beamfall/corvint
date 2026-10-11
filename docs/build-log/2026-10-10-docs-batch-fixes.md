# Documentation batch fixes (V1-1127, V1-0155, V1-0265, V1-0225, V1-0267, V1-0176, V1-0377)

Documentation and citation corrections only. No behavior change and no new or amended requirement.

- V1-1127: the stage-slot race entry now names racer `gate-a` and the `GateRun` unlocked-read path at `30e306ec`.
- V1-0155: `docs/DOGFOOD.md` states that a linked worktree is supported and why (per-worktree absolute Git directory; shared `info/exclude`).
- V1-0265: restored the decision index header row. Coverage and duplicate enforcement already existed (`deffd56e`); 382 files, 382 unique rows.
- V1-0225: `docs/GLOSSARY.md` repinned to `mcp-server-2026-07-28-v0.md:243@f4047e06`, verified by `line-citations-check`.
- V1-0267: both `GPK-V0-046`/`GPK-V0-053` citations in `conformance/divergence-register.md` repinned (526-530, 598-600). The file stays outside the scanned set, with the reason recorded at the end of `script/check-line-citations.sh`: scanning it would exceed the unpinned ceiling.
- V1-0176: `script/dogfood-change_test.sh` now asserts that `docs/DOGFOOD.md` names `.corvint/change.ocm.001.json` in the reviewer report command.
- V1-0377: `docs/INSTALL.md` states the two private ledgers, their write conditions and that they are not ranking, evidence, authority or trace writes.
