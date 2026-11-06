<!-- SPDX-License-Identifier: Apache-2.0 -->
# V1-4726 self-review repair: fixture marker carriage returns

The repair review found that the raw-byte fixture-marker precheck could reject
`//corvint:fixture-\rpath` (an actual carriage-return byte) before Go's scanner
removed the carriage return and recognized the comment. The optimization then
retained no literal spans and incorrectly selected a marked test package as a
`PATH_LITERAL_READER`, violating existing AFP-V0-551 and the lane's unchanged
selection-output contract. No requirement or wire contract changes.

Decision: retain literal spans whenever the source contains any carriage return,
in addition to the existing exact-marker check. The scanner still decides whether
a comment is a fixture marker. This is the conservative repair requested by the
review, without a normalized source copy or an additional parser. Rollback is to
remove the span precheck and retain spans unconditionally as the original did;
restoring the broken raw-byte-only precheck would reintroduce the regression.

`TestAFPV0551FixtureMarkerCarriageReturnParity` builds equivalent temporary
repositories with plain and embedded-carriage-return markers. It compares the
selected units and full witnesses, asserts the marked package is exempt, and
keeps an unmarked reader as a positive control. Existing assertions and goldens
are unchanged. Before the fix, the whole Go adapter package fails this test:
the carriage-return form selects both packages while the plain form selects only
the reader. After the fix the whole package passes.

Raw evidence remains outside Git under
`/home/russell/lanes/dryruncpu-evidence/`, with filenames prefixed `repair-`.
`corvint affected --base 28a3ff86a2c3d17662765da12cc9251c06466f5f` selected all
three lane-touched packages. Scope remains UNKNOWN: platform build constraints,
foreign shell invocations, and two web-flow binary consumers remain unresolved.
The installed selector reports `self-binary-stale`; no present-source selection
parity is inferred from its receipt. Native platform qualification and new timing
measurements are NOT_RUN in this narrow correctness repair. Prior performance
measurements are historical evidence, not a measurement of this repaired tree.
Independent review belongs to the orchestrator; no model-backed reviewer ran.

Verification uses Go 1.27.3 with identity variables absent. Each whole-package
command ran through `script/heavy.sh`, one package per command, with
`GOTOOLCHAIN=local go test -count=1 -timeout 30m PACKAGE` and an explicit
`HEAVY_WHOLE_REASON` naming the affected selection and lane requirement.

- `./internal/liveverify/affected/golang/`: PASS, 8.665s.
- `./internal/liveverify/affected/`: PASS, 48.423s.
- `./internal/gateselect/`: PASS, 1.836s.
- `script/check-line-citations.sh --repin`: no moved citation;
  `make -s line-citations-check`: PASS. Specs are unchanged.

Evidence SHA-256:

| File | SHA-256 |
| --- | --- |
| `repair-affected-plan.json` | `78c966411a591bedd9b913b45f52ac6afdcee238263347b7650ea310d26801ce` |
| `repair-test-golang-red.log` | `7887a5459c01db64462467b7d67f27edd420d13663d09352a6492b10c75ac173` |
| `repair-test-golang.log` | `1b64dc47627b5f672ff94ea2f08f716daa332954848915bff9e40b9d6b01b3c2` |
| `repair-test-affected.log` | `71b4c2939a47e3daea5814c2453aaad21b64f3f9db9f016ac074abcda5c8b0a8` |
| `repair-test-gateselect.log` | `9a41ae4a048bec81f7141251b9c9136933f44583c8fc2c008ecbda6d92d015de` |
| `repair-repin.log` | `85be64c6dde7c05243069a8a876a4dc6953564d59a161d92a23e6f348d8c9133` |
| `repair-line-citations.log` | `855a4551b199f407e4e7cec705511746022a39aa0f70383fc13aac9acc748bba` |
