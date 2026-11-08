# clicat2 introductory MCP discovery regression

Independent review required regression coverage for the corrected introductory
offline discovery guidance (P3, V1-4846, CAP-V0-8800). Existing inventory tests
inspect gated tables; routing tests only establish pointers to the catalog.

`TestCAPV08800IntroductoryMCPDiscovery` now reads the real capability catalog's
introduction, normalizes whitespace, and requires the offline `corvint help
--json` to `mcpTools` route, its discovery fields, the CLI-help restriction,
live `tools/list` schema/availability authority, and the offline availability
qualification. Restoring the previous unavailable paragraph removes required
claims and fails this test. Table text cannot satisfy an introductory claim.
This is missing documentation regression coverage, not an analyzer soundness
gap: no accepted syntax or analysis allowlist is changed. No product output,
golden, existing assertion, or governing requirement changes.

Verification: `corvint affected --base
6339f980e13e36c8ff51a593568da3d935ffae77` selected the checker package, retaining
`scope=UNKNOWN`, 76 LANGUAGE_FRONTIER unknowns and `self-binary-stale`. The full
`tools/capabilities-check` package passed with `-count=1 -timeout 30m` through
`script/heavy.sh`, without a run filter or exported Git identity: 10.658 seconds,
wrapper exit 0. Raw evidence is outside the repository in
`/home/russell/lanes/clicat2-evidence/independent-review-capabilities-check.log`.
Its SHA-256 is
`1d00d262a111d674a89c63abc4fc973b2676466f2c4dc7891b1311f8bd5f831a`.

The existing pre-change capture is reused. Selected self-development routes are
affected planning and final CEM binding/checking; broader selection, independent
review and native ticket completion remain with the host. Retrieval/learning
evaluations and live protocol qualification are not applicable to this test-only
repair. Rollback removes this test and record; the corrected guidance remains.
