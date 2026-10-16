# V1-1509: escape invisible Unicode literals in Go sources

Date: 2026-10-16. Ticket V1-1509 (P3 CHORE, found by Codex review on V1-0414, PR #983).

## Decision

Literal U+2028, U+2029, U+202E, U+200B and U+200E code points inside Go string and rune
literals hide intent from reviewers. They are replaced with Go escapes (` ` and so on)
so the compiled strings are unchanged. No requirement is added or amended (no spec governs
test-fixture source layout), so no requirement IDs apply.

## Scan

Tracked `*.go` scanned for U+200B-200F, U+2028-202E, U+2060-2064, U+2066-2069, U+FEFF, U+061C:
11 files hit, all fixed. The two named in the ticket plus nine more (all U+2028/U+2029 line and
paragraph separators in canonicalisation and separator tests): `internal/appmap/plan.go`,
`internal/attest/canonical.go`, `internal/attest/fuzz_test.go`, `internal/cem/mdreport/codespan.go`,
`internal/contextindex/webimports_test.go`, `internal/doccompiler/canonical_test.go`,
`internal/liveverify/affected/typescript/{playwright_example_app_test,typescript_test}.go`,
`internal/liveverify/jsresolve/jsresolve_test.go`. After the change the same scan finds 0 hits.

`internal/attest/fuzz_test.go` held the character inside a raw (backquoted) string, where
` ` would not be an escape; it is split as raw + `" "` + raw.

## Evidence

- Byte identity: a scratch go/parser program hashed the unquoted value of every string and rune
  literal in source order, on the base and the changed file. Hashes and literal counts are equal
  for 10 of 11 files; `fuzz_test.go` differs only in literal count (15 vs 17) because of the
  raw/escape split, and its concatenated value is the same by construction.
- Tests (focused, go1.27.2): attest, doccompiler, contextindex, liveverify/affected/typescript,
  liveverify/jsresolve, tasks/ticket pass; integrations/testfixture has no test files (compiled by build);
  `internal/appmap` full package hit the default 600s timeout under host load (load average ~40),
  so the plan-related subset (`-run 'Plan|Comment|Safe'`) was run and passed.
  windows/amd64 vet of changed test packages is clean.
