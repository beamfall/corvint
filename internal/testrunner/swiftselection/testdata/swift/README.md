# Swift selection fixtures

`xctest-pass.txt`, `xctest-mixed.txt` and `swift-testing.jsonl` are byte-for-byte
extractions of `swift-xctest-pass.txt`, `swift-xctest-three.txt` and `swift.jsonl`
from `../../../platform/testdata/native-report-fixtures.json`. Extraction checked
each archive SHA-256 before writing. Their original collection provenance stays
with that existing platform fixture archive; this lane ran no native tools.

The two `*-list.txt` files and `coverage.lcov` are synthetic input fixtures, not
live discovery, qualification, native coverage or Git-content attestations. The
list maps explicitly relate SwiftPM display names to terminal identities and
source paths. The source map and blob IDs are core-supplied fixture inputs.
Negative variants are derived inside tests only after their positive fixture
initializes successfully. No existing report or golden is modified.
