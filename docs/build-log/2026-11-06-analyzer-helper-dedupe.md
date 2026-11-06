# Analyzer envelope helper dedupe (V1-4753, V1-4747 slice 3a)

The V1-4747 scope review found byte-identical candidate-envelope helpers in
`internal/analyzerruby/analyzer.go` and `internal/analyzershell/analyze.go` and asked to start with
the decoded length, the duplicate-member scan and safe extension stripping. This entry records
only that move.

## Scope

- New stdlib-only package `internal/analyzerenvelope` holds `DecodedLength`,
  `DuplicateJSONName(raw, maxDepth, maxTokens)` and `WithoutExtensions` (with its private schema
  field table). The bodies are the former copies with exported names; the only edit is that the
  duplicate scan takes the depth and token bounds as parameters, and both callers pass their own
  `MaxJSONDepth` and `MaxJSONTokens` (8 and 4,096), so each candidate's constants stay authoritative.
- `analyzerruby` and `analyzershell` call the shared helpers and drop their copies and the
  `encoding/json/jsontext` import. Refusal precedence and the order of checks in `envelope` and the
  input loop are unchanged.
- `TestHardAllocationOutputAndSourceRatchets` in `analyzershell` now counts the shared package in
  the shell CLI's production source, as ACP-009's reachable-closure rule requires.
  `.corvint/test-read-scopes.json` declares the new directory for `analyzercap` and `analyzershell`.
- The Ruby and Shell rows of the implementation table in
  `docs/specs/analyzer-candidate-profiles.md` name the shared package.

Non-goals: the other identical blocks (`sentinel`, `bound`, `echoes`, `inputLess`, `factLess` and
the output-size helpers) stay per package because they are bound to each package's own `Request`,
`Input` and `Fact` types and `Profile`/`Family` constants. Sharing them would need shared types or
generics, a larger API than the duplication. `overBound` is a one-line comparison used throughout
both packages and stays local. `validPreEnvelope`, `pathOK`, `exactTarget` and the evidence code
differ between the two packages (path alphabet, refusal reasons, target matrix, evidence caching)
and are not shared. Rust, Python, .NET, Go, JS, Kotlin, shader and Swift carry related copies.
They are out of scope here (the ticket defers Rust) and were not compared.

## Evidence

Base `8fc0cf37642e7012f4c26671c9ed97a8706d6e09`, Go 1.27.3.

- Identity (base line numbers): `diff` of `analyzerruby/analyzer.go:195-263` against `analyzershell/analyze.go:152-220`
  (decoded length, `overBound`, duplicate scan) and of `analyzerruby/analyzer.go:301-340` against
  `analyzershell/analyze.go:258-297` (extension table and stripping) printed nothing.
- Differential: a temporary test, since removed, compared the pre-move copy with the shared
  helpers over 43,642 inputs. The inputs were six request-shaped seeds with every truncation,
  single-byte deletion and single-byte insertion from a JSON punctuation alphabet, plus every
  string of up to nine characters over `A=/` for the decoded length. There were no divergences.
- Maintained table tests in `internal/analyzerenvelope/envelope_test.go` pin each helper's
  results, including the bound edges and the bytes a refused extension scan returns.
- `go list -deps` of both candidate commands gains exactly `internal/analyzerenvelope`. Its own
  closure is the standard library only.
- ACP-009: reachable production source is 55,791/65,536 for Ruby and 40,833/65,536 for shell
  (`TestCandidateSourceCeilingPerExecutable`). The Ruby allocation sample is 33,700 B/op and
  244 allocs/op, within 35,000 and 1,000.
- Tests via `script/heavy.sh` with Corvint's selection manifest, all passing:
  `./internal/analyzerenvelope`, `./internal/analyzerruby`, `./internal/analyzershell`,
  `./cmd/corvint-analyzer-ruby`, `./cmd/corvint-analyzer-shell`, `./internal/analyzercap`,
  `./internal/langplugin`, `./internal/gateselect`, `./tools/test-read-scopes-check`,
  `./tools/unbounded-readers`. `make test-read-scopes-check` passes. The full selection was 79
  units, widened mostly by the read-scope file edit. The rest of it was not run locally.

## Rollback

Revert the commit. The helpers return to both candidate packages and the new package disappears.
There is no persistent state, wire or frame change to unwind.
