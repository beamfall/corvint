# Repository-declared extractor profiles and extractor-inventory flow import

Date: 2026-10-23. Lane: claude/extractor-profiles-gh1583 (base 0b7d59ba6). GitHub #1583.
Requirements `DCP-V1-390..399` (`docs/specs/documentation-corpus-v1.md`) and `AFU-V1-122..124`
(`docs/specs/application-flow-understanding-v1.md`), accepted under the owner's standing
auto-accept (2026-10-11).

## Decision

- Profiles are configuration, not Core taxonomy. A closed `corvint-extractor-profile/1` file,
  committed at the revision, names line-oriented RE2 extractors over Git blobs. The subject types
  stay generic (`endpoint`, `job`, `event`, `data_entity`, `ui_surface`, `symbol`); only
  `job` and `event` were added to the provider subject kinds.
- The producer's output, `corvint-extractor-inventory/1`, wraps an ordinary
  `corvint-corpus-provider/1` record with `generated` trust. That record builds unchanged
  through the existing provider admission, so there is no second admission path. Inventory gaps
  stay in the inventory, because a supplied corpus gap must name a subject and an unreadable file
  has none. The record therefore declares gaps `not-collected` and states where they are reported.
- Relations are exact-name joins only: `implements` (handler symbol to endpoint or UI surface,
  when exactly one single-site symbol has that name), `calls` and `depends_on` (produce or
  consume interaction to the declared job or event), and `related_to` (field to entity). Every
  join that fails is a named gap; no join is guessed.
- Framework tool output is admitted only as a committed file whose SHA-256 the profile pins.
  Corvint never runs the tool. A digest mismatch is a gap with no subject.
- The flow relation join (`DCP-V1-093..099`) checks `produces`/`consumes` names against the
  declared job and event subjects. It does so only when the corpus has any of them, so existing
  corpora keep their bytes.
- `flows import --format extractor-inventory` reuses the existing all-or-nothing importer. It
  writes one proposed `api` flow per endpoint and one proposed `ui` flow per UI surface, with
  inferred source links at the exact anchor spans.

## Evidence

Focused tests, run through `/private/tmp/cem10-build/heavy.sh`:

- `go test ./internal/doccorpus -run 'DCPV139|DCPV109|FlowRelation|CorpusAdoption|Provider'`: PASS.
- `go test ./internal/appflows`: PASS.
- `go test ./cmd/corvint -run 'DCPV1399|TestDocsCorpusHelp|TestDocsCorpusOperation|Flows'`: PASS.

Checks: `go vet` on the three touched packages, `gofmt -l`, `make windows-vet`, and
`make spec-requirements-check traceability-tests-check line-citations-check`. The PR records the
results.

## Limits

The extractor is line-oriented: one match per line, and the only multi-line construct is the
bounded entity/field block. It has no language parser and infers no framework convention. Handler
joins are exact names within one profile. A stale committed tool output is the producer's
responsibility. Extracted subjects carry no authority. Real-framework qualification on an adopter
repository is `NOT_OBSERVED`. The full `cmd/corvint` package and `make gate` were not run
(lane policy).

## Rollback

Revert `internal/doccorpus/extractor.go`, the `job`/`event` subject kinds, the
`name_subjects` join check, the `extract` CLI operation, the `extractor-inventory` importer, and
the two spec sections. Corpora and intents produced without these features are unchanged.
