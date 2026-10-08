# 2026-10-08: Go JSON PASS event screening (V1-0687)

Human-owned intent: ticket V1-0687 (P1 bug), proposed `LTA-V0-015` in
`docs/specs/learned-trace-admission-v0.md`.

## Finding

`corvint dogfood verify` of an unchanged `go test -json` check exited 0 but recorded
`secretScreened=true`, `qualified=false`, and replaced both logs with `log-secret-screened`. The
writer screen already neutralised the bare `PASS:` field of a complete plain
`--- PASS: Name (0.00s)` line (`LTA-V0-004`), but `go test -json` carries the same marker inside an
`Output` string, `{"Action":"output",...,"Output":"--- PASS: TestAlpha (0.00s)\n",...}`. The plain
marker is anchored at line start, so the JSON copy stayed a `pass` credential assignment whose value
was the test name. Every PASS event in every `go test -json` run matched.

## Change

- `internal/secretscreen/secretscreen.go`: `goJSONPassMarker` recognises one complete
  test2json output event, in cmd/test2json's field order, with optional `Time`, `Package` and
  `"OutputType":"frame"`, and no other field. Apart from the Output's structural `\t`, `\r` and
  `\n` escapes, no string in it may hold a quote, a backslash escape or an unescaped control
  character, the Output must be exactly one PASS marker line ending in `\n`, and the Test
  field must equal the marker's test name. `maskGoPassMarkers` (shared with the plain marker)
  replaces only `PASS:` with a same-length word at unchanged offsets; `Pattern` then screens the
  whole text, including the test name, the Test and Package values, and every other line.
- `Pattern`'s source, `StoredV1Pattern` and the `LTA-V0-014` prefilter are unchanged. The
  analyzer schema moves to `corvint-analyzer/112` because `internal/secretscreen` is a pinned
  input of `TestAnalyzerSchemaInputs`; extracted facts change only for text holding a recognised
  event.

## Evidence

Failing before (base 722904f9, fix reverted), passing after:

- `TestLTAV0015GoJSONPassMarker`: before, the seven clean-event cases failed (real transcript,
  top-level, subtest, indented, no OutputType, no Time/Package, CRLF) and the seventeen
  credential/malformed cases already matched; after, all 26 pass (two raw-control-character cases
  added after review), and recognition is asserted directly (masked exactly for the recognised
  events).
- `TestActualVerificationAndSecretRefusal/go-json-pass-log`: before, FAIL
  (`selected-check-unverified:test`); after, PASS with the stored stdout byte-identical.
  `go-json-pass-log-with-embedded-secret` (a `pass=synthetic123` subtest name) is screened both
  before and after.
- Real output: `go test -json ./internal/testacceptance ./internal/secretscreen` (3,575 lines,
  845 PASS events, including the `TestPTFV0EarlyStaleWithoutRow` package from the original
  receipt) matched on 845 lines before and on 0 lines after.
- Live qualification, go1.27.1, binaries built from this tree with and without the fix, fresh
  committed fixture modules, plan argv `["go","test","-json","./..."]` unchanged:

  | fixture / binary | exit | secretScreened | qualified |
  |---|---|---|---|
  | clean / base | 0 | true | false |
  | clean / fixed | 0 | false | true |
  | `pa`+`ss=synthetic123` subtest / base | 0 | true | false |
  | `pa`+`ss=synthetic123` subtest / fixed | 0 | true | false |

  The review-fixed binary (control characters excluded) repeated both fixed rows on fresh copies.
  The fixed clean run retained the real JSON stdout with three PASS events; the credential run
  retained only `log-secret-screened`. Both fixture worktrees stayed clean.

## Independent review

Codex (`gpt-6-astra`, read-only) security review of the screening change found no P0/P1 and no
credential-screening bypass. Two P2 findings were fixed: the Package and name classes admitted
unescaped control characters (invalid JSON), now excluded with raw-tab and raw-NUL negative cases;
and the `\u` escape case held a literal space, now a real `\u0078` escape. Spec wording on the
permitted Output escapes and on recognised credential-bearing events was clarified.

## Non-goals

No JSON decoding of arbitrary output, no exemption for other test2json events (`--- FAIL`,
`--- SKIP` and `=== RUN` never matched), no change to stored-v1 detection, no global allow list,
and no rewriting of the check's argv or recorder.

## Failure modes

- A future toolchain that adds a field, reorders fields or changes `OutputType` makes the event
  unrecognised, so the check is screened again (fail closed, a false positive, not a leak).
- A test name containing a quote, backslash or non-ASCII escaped by the encoder is unrecognised
  and screened.
- Like the plain marker, any process can print a recognised line; the only bytes neutralised are
  the `PASS:` field name, whose value (the test name) is still screened by every other branch.

## Rollback

Remove `goJSONPassMarker` and its loop in `maskGoPassMarkers` and restore `corvint-analyzer/111`
with its prior audited SHA; nothing stored depends on the recognition.

## Not run

`make gate` and `go test ./...` (lane policy: shared host). The original frozen V1-0541 plan and
its prior failed receipts were not touched; this lane did not re-run that plan, so its
qualification on the Corvint repository remains `NOT_RUN` here.
