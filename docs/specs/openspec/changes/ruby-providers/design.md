# Design: ruby-providers

Status: proposed (2026-11-09). Requirement text lives in `specs/ruby-providers/spec.md`.

## Decisions

1. RSpec and Minitest are line-exact runners (owner decision R8 (2026-11-09)); they run at tier
   `file` until the Ruby line evidence passes FIH-V1 (change `line-evidence`), then at `line`.
2. The Ruby lister is `static` (the framework classification in
   `internal/liveverify/affected/ruby`) and runs only under `corvint inventory refresh` (owner
   decision R10 (2026-11-09)).
3. `cmd/corvint-ruby-test-provider` is retired once the compiled-in path lands (owner decision R14
   (2026-11-09); RBR-V1-012, RNC-V1-017). Its output stays byte-compatible until then (RBR-V1-008).
4. Minitest per-test results come from the compiled-in hook `MinitestHook#run` in
   `internal/rubylinecov`, extended with `result.error?` (settled D9). `minitest-reporters` and
   JUnit XML are not used.
5. Rails engines and separate Ruby roots are runner instances (owner decision R15 (2026-11-09)).
6. Kill switch through NMS-V1-008 (owner decision R13 (2026-11-09)).
7. Missing results are `NOT_RUN` and the run `INCOMPLETE`, not `FAILED_MEASUREMENT` (review T-m2).
8. Kind: `type: :system` and `type: :feature` are e2e only with a browser driver; Capybara
   `rack_test` is not e2e (G13, review T-m11).

## Failure modes

- Ruby, Bundler or the framework gem is missing: refuse with `runner-unavailable`, exit 2.
- The selected files run zero examples or tests: the selected tests are `NOT_RUN` and the run is
  `INCOMPLETE` (review T-m2 replaces the earlier `FAILED_MEASUREMENT`).
- The JSON or capture document is oversized, malformed or has unknown members: fail closed.
- `--exec-prefix` container is not running: refuse as infrastructure failure, not as a test result.
- Both RSpec and Minitest claim a file (explicit Minitest require in an RSpec-looking file): follow
  `ruby.go:388` (`explicitMinitest`); if unresolved, the kind is UNKNOWN and both runners run it.

## Acceptance evidence

- Fixture repositories: an RSpec gem, a Rails application with Minitest and system tests, and a
  plain Minitest gem; one Go test per requirement.
- `corvint test` on rubocop/rubocop and on rails/rails `activesupport/` at their PRR-V1 pins.
- NMS-V1 and FIH-V1 reports for both runners.
- `binaries.json` without the Ruby provider after RBR-V1-012.

## Rollback

Revert the change's commits; Ruby selections return to advice only. IDs are retired, not reused.
The kill switch covers a bad selection without a revert. Reverting RBR-V1-012 restores the
`binaries.json` entry.

## Open questions for the owner

- The non-Rails Minitest invocation (RBR-V1-004) is NOT_VERIFIED; the lane records it in the spec
  before code.
