# Secret Screen Writer V0

Owner: Russell Lewis
Date: 2026-11-11
Status: accepted
Intent status: accepted (decision 0058; moved from the learned-trace spec by decision 0597)
Delivery status: experimental
Requirement prefix: `LTA-V0`
Authoritative inputs: `docs/decisions/0058-learning-loop-evaluation-gate-and-retention-2026-09-05.md`;
`docs/decisions/0164-secretscreen-credential-formats-2026-09-12.md`;
`docs/decisions/0597-context-proven-only-2026-11-11.md`;
`docs/specs/learned-trace-admission-v0.md` (retired; its history).

## Agent digest
- Claim: `internal/secretscreen.Pattern` and its prefilter redact credential-shaped text, with a prefilter that returns exactly the complete pattern's matches.
- Status: accepted (decision 0058; moved from the learned-trace spec by decision 0597) / experimental
- Exists: `internal/secretscreen` (`Pattern`, the writer prefilter in `prefilter.go`, the Go verbose and JSON PASS markers in `secretscreen.go`), consumed live by `cmd/corvint/runs.go`, `internal/appflows`, `internal/appmap`, `internal/gotestrun`, `internal/doccorpus`, `internal/diagnostic` and others.
- Blocked on: nothing; these requirements keep governing live code after the trace writer and reader that first consumed them were deleted (decision 0597).
- Read next: `LTA-V0-004`, then `LTA-V0-014`.

## Why this spec exists

Decision 0597 retired `learned-trace-admission-v0.md` and deleted `internal/trace` and its record
admission. Requirements `LTA-V0-004` and `LTA-V0-014` to `LTA-V0-020` govern the secret screen and its
prefilter, which are still consumed by live code, so they move here with their IDs, text and test
citations unchanged. `LTA-V0-018` (slot-learning splits) stayed in the retired spec: its code,
`internal/slotlearn`, was deleted. Where the carried text names `record`, the `query` history
screen, `internal/trace` or `internal/contextindex` as the screen's consumers, those consumers are
gone; the screen's live consumers are the packages named in the digest, and the requirement
text states what the screen itself must match.

## Requirements

- `LTA-V0-004`: The current writer screen (`internal/secretscreen.Pattern`, consumed by `record`
  admission and by `query` history-candidate screening) MUST also match, in addition to the shapes
  above: a bare assignment of the credential vocabulary whose value is single- or double-quoted,
  consuming the complete lexical string under the same whole-value and unterminated-through-EOF
  rules as the quoted JSON property; the bare `pass` vocabulary stem only when followed by an
  assignment operator, except for the structural bare `PASS:` prefix on a complete Go verbose-test marker line shaped as
  `[whitespace]--- PASS: TestName (seconds)`; all other text on that line, including the test name, remains screened. `whsec_`, `hf_`, `dop_v1_` and `xapp-` tokens at or above their length
  floors, and Slack incoming-webhook URLs; an AWS access-key ID immediately followed by its
  40-character secret, redacted together; and an `authorization` assignment whose value is an HTTP
  scheme word (`basic`, `bearer`, `digest`, `negotiate`, `ntlm`, `token`) followed by a
  space-separated credential, redacted together. Decision 0164 adds: an `account[_-]?key`
  assignment (Azure `AccountKey=`); Google `ya29.` and `GOCSPX-`, Shopify `shpat_`/`shpss_`/`shpca_`
  and GitLab `glrt-`/`gldt-` tokens at or above a 20-character floor; a credentialed URL whose
  password contains a raw `@`, redacted through the last `@` before whitespace, `/`, `?`, `#` or a
  quote, so the match never ends inside a later secret (the user part stops at the same characters,
  so `https://host?x="a:b@c d"` is not a credentialed URL); URL userinfo with no password that carries a known vendor-token prefix at any length or is at least 32 letters
  and digits (plain user names such as `git@` stay unmatched); a credential-vocabulary flag
  (`--password`, `-token`, `--db-password`) followed by a separate argument, where a single- or
  double-quoted argument is consumed as one complete lexical string (through EOF when unterminated)
  and a bare argument ends at whitespace or an unescaped quote, so a following JSON property is never
  cut in half; a bare or double-quoted argument starting with `$` is a variable reference and MUST
  NOT match; a `-p` separate or
  `=`-joined argument after `login`, `-u` or `--user` on the same line; and `curl -u`/`--user`
  with a bare or whole quoted `user:password` pair, where the flag starts an argument (a `-u` inside a hyphenated host
  such as `svc-users:8080` MUST NOT match). A flag argument counts only when it contains both a letter and a
  digit, so commit prose such as `add --token flag`, `docker login -p value` and git's
  `log -p HEAD~3` MUST NOT match. `record` MUST refuse such input and `query` MUST
  drop such a history candidate. `StoredV1Pattern` and the dashboard readers MUST NOT gain these shapes. The
  retired Python oracle's narrower screen is not a compatibility target (decision 0092).

### Proposed amendment: writer-screen prefilter (2026-10-04, ticket V1-0722, owner-directed)

- `LTA-V0-014`: The writer screen MAY skip a top-level branch of `secretscreen.Pattern` for a text
  only when a condition necessary for any substring of that text to match the branch is false, and
  MUST then return exactly the matches `Pattern` returns for the same text, in the same order and
  at the same byte offsets. A text holding a non-ASCII rune that case-folds to an ASCII letter
  (U+212A, U+017F) MUST be screened with the complete pattern. The prefilter MUST NOT change
  `Pattern`'s source text, `StoredV1Pattern`, or what `MatchString`, `Screen` and `MatchArgv`
  report. It is an execution strategy only; a detector change still needs its own requirement.

Acceptance: `TestLTAV0014PrefilterReturnsPatternMatches`, `TestLTAV0014GeneratedBranchTextsKeepTheirBranchLive`, `TestLTAV0014PatternSourceIsUnchanged`,
`TestLTAV0014TextWithoutCandidatesSkipsThePattern`, the package `TestMain` observer, which
compares every text any `internal/secretscreen` test screens with `Pattern` and with each branch
alone, and `BenchmarkScreen` for the retained before and after.

### Proposed amendment: Go JSON PASS event marker (2026-10-08, ticket V1-0687)

- `LTA-V0-015`: The writer screen MUST treat the structural `PASS:` prefix inside one complete
  `go test -json` output event line as it treats the plain `LTA-V0-004` Go verbose marker, and only
  there. The line MUST be exactly `{`, an optional `"Time"` string of RFC 3339 characters,
  `"Action":"output"`, an optional nonempty `"Package"` string, a `"Test"` string, and an
  `"Output"` string holding optional leading spaces or `\t` escapes, `--- PASS: `, a test name, a
  space, a parenthesised seconds value and a final `\n` (optionally preceded by `\r`), then an
  optional `"OutputType":"frame"` and `}`, in that field order with no other field. Apart from
  those structural Output escapes, no string in the line may hold a quote, a backslash escape or an
  unescaped control character, the test names may not hold a space, and the Test field MUST equal
  the marker's test name. Only the four `PASS` bytes and colon are neutralised, at unchanged byte
  offsets; every other byte, including the test name, the Test and Package values, and every other
  line, remains screened, so credential-shaped test names, arbitrary JSON values, other `Action` or
  `OutputType` values, extra or reordered fields, escaped content and trailing Output text are
  still matched. `Pattern`'s source text, `StoredV1Pattern` and the `LTA-V0-014` prefilter are
  unchanged; there is no global exemption, so a local-completion check whose `go test -json` output
  carries a credential stays `secretScreened` and unqualified.

Acceptance: `TestLTAV0015GoJSONPassMarker` (a real go1.27.1 `go test -json` transcript and
structural variants screen clean; credential-bearing events stay recognised but still match;
malformed-structure cases are not recognised and still match), `TestActualVerificationAndSecretRefusal` subtests `go-json-pass-log` and
`go-json-pass-log-with-embedded-secret`, plus a recorded live `corvint dogfood verify` of an
unchanged `go test -json ./...` check (build log 2026-10-08-go-json-pass-screening).

- `LTA-V0-016`: The writer screen's JWT-shaped branch MUST require compact-JWS header structure:
  a word boundary, then a case-exact `ey` or `ew` (base64url of `{` followed by a quote or by
  whitespace), at least eight more base64url characters, and two further dot-separated base64url
  segments of at least ten characters each. A dotted identifier whose first segment is not
  case-exact `ey`/`ew` (for example an adopter obligation alias such as
  `CHECKOUT-REGRESSION.PAYMENT-FLOW.SAVED-CARD-RETRY-PATH-V2.R00001`, a lowercase dotted path, or a
  word ending in `ey` such as `monkey-business.abcdefghij.abcdefghij`) MUST NOT match that branch.
  Every real compact JWS (bare, in JSON, in a URL fragment, in prose, after a hyphen, and with a
  spaced or newline header) MUST still be redacted, and a five-part JWE MUST still be hit.
  `StoredV1Pattern` is unchanged, so schema-v1 rows keep their recorded classification, and the
  `LTA-V0-014` dotted-triple prefilter stays a necessary condition (V1-1704).

Acceptance: `TestLTAV0016JWTBranchRequiresHeaderStructure` (positives fully redacted, five-part JWE
hit, negatives unmatched, the dotted alias still matched by `MatchStoredV1String`),
`TestLTAV0014PatternSourceIsUnchanged` (re-pinned writer digest), and
`TestTOLV0055`..`TestTOLV0059` in `internal/tasks/cli` with a dotted adopter alias.

### Proposed amendment: sk- branch hyphenated words (2026-10-22, ticket V1-2418)

- `LTA-V0-017`: The writer screen's `sk-` vendor branch MUST require, after a word boundary and
  `sk-`, at least 20 consecutive characters from `[a-z0-9_]` (case-insensitive) within the
  following `[a-z0-9_-]` run, and MUST still redact that whole run. A run of short hyphenated words
  such as the build-log path `2026-10-22-sk-1022p-site-knowledge-setup.md` MUST NOT match that
  branch; legacy, `sk-proj-` and `sk-ant-` shaped keys MUST still be redacted. Every writer match of
  the branch is also a match of the previous branch with the same span, and `StoredV1Pattern` is
  unchanged, so schema-v1 rows keep their recorded classification.

Acceptance: `TestLTAV0017SKBranchNeedsAnUnhyphenatedRun` (key shapes redacted in prose and paths,
hyphenated word runs unmatched, `MatchStoredV1String` still matching the original shape) and
`TestLTAV0014PatternSourceIsUnchanged` (re-pinned writer digest).

### Proposed amendment: unterminated private keys and linear prefilter (2026-10-28, V1-2993, V1-2997)

- `LTA-V0-019`: The writer screen's private-key branches MUST redact from a
  `-----BEGIN ... PRIVATE KEY-----` or `-----BEGIN PGP PRIVATE KEY BLOCK-----` header through the end
  of the screened value, so a block without its footer never leaves its base64 body unredacted.
  Redacting through the end is the conservative choice: the screen cannot know where an unterminated
  body stops. `StoredV1Pattern` keeps the header-only branches and is unchanged, so schema-v1 rows
  keep their recorded classification; the writer source digest is re-pinned.
- `LTA-V0-020`: The prefilter's credential-name scan MUST scan the lowered text's field-byte runs
  once, test each run for the credential stems present in the text, and call the boundary check once
  per run, so its cost is linear in the input rather than in input times stem occurrences. Its
  candidate decisions MUST equal the previous per-occurrence scan's on every input.

Acceptance: `TestLTAV0019UnterminatedPrivateKeyRedactsThroughEnd` (RSA, EC, OPENSSH, plain,
ENCRYPTED, PGP and lowercase headers redact through the end with no body left; a terminated block
followed by a footerless one keeps the text between them; the stored-v1 pattern still matches the
header), `TestLTAV0020CredentialNameScanMatchesReference` (equal decisions to the per-occurrence
reference over the differential corpus and adversarial cases),
`TestLTAV0020RepeatedCredentialWordsScaleLinearly` (four times the input stays within eight times the
time), and `TestLTAV0014PatternSourceIsUnchanged` (re-pinned writer digest).

## Acceptance evidence and testing matrix

| Requirement | Deterministic acceptance evidence |
|---|---|
| `LTA-V0-014` | every parity fixture, its case, fold, split, JSON and base64 variants and every ordered fixture pair return `Pattern`'s matches, as do texts generated from each branch's own expression; each branch's necessary condition holds wherever that branch alone matches; a text with U+212A or U+017F uses the complete pattern; the writer pattern's source digest is pinned |
| `LTA-V0-016` | real compact JWS forms (bare, JSON, URL fragment, prose, after a hyphen, spaced and newline headers) are fully redacted and a five-part JWE is hit; a dotted adopter alias, lowercase dotted text, a word ending in `ey`, an uppercased header, a module-like path, two segments and a short header do not match; the stored-v1 pattern still matches the alias |
| `LTA-V0-015` | a recorded go1.27.1 `go test -json` transcript, top-level, subtest, indented, CRLF, time-less and package-less PASS events screen clean; a credential in the test name, Package value or a later line is still redacted while the marker stays recognised; a mismatched Test field, a non-`output` Action, a non-`frame` OutputType, an extra or reordered field, continued or prefixed Output, an escaped quote or `\u` sequence, a raw tab or NUL, leading text, another JSON value and an unterminated line are not recognised and still match; `dogfood verify` qualifies the clean event and screens the credential-bearing one |
| `LTA-V0-017` | legacy, `sk-proj-` and `sk-ant-` shaped keys are redacted bare, in prose and in a path; hyphenated word runs such as `sk-1022p-site-knowledge-setup` and a 19-character run do not match; the stored-v1 pattern still matches the hyphenated run |
| `LTA-V0-004`, `LTA-V0-019`, `LTA-V0-020` | the acceptance paragraphs above and the traceability tests below |

## Traceability

| Requirement | Implementation | Evidence |
|---|---|---|
| `LTA-V0-004` | `internal/secretscreen.Pattern`, consumed by `internal/trace` record admission and `internal/contextindex` `containsSecret` | `TestSecretPatternParityCorpus` (writer-only rows, stored-v1 non-match, and length-floor, bare-`pass` and hyphenated-host curl non-matches), `TestGoVerbosePassMarkerBoundary`, `TestScreenConsumesWholeQuotedAssignmentValue`, `TestScreenRedactsAWSSecretAdjacentToItsKeyID`, `TestScreenRedactsCredentialAfterAuthorizationScheme`, `TestScreenRedactsWholePasswordContainingAtSign`, `TestCredentialedURLPasswordStopsAtQueryFragmentOrQuote`, `TestLTAV0004RecordRefusesWriterOnlySecretShapes` (removed), `TestSecretPatternMatchesHistorySecretShapes` (removed) |
| `LTA-V0-014` | `internal/secretscreen/prefilter.go` | `TestLTAV0014PrefilterReturnsPatternMatches`, `TestLTAV0014PatternSourceIsUnchanged`, `TestLTAV0014TextWithoutCandidatesSkipsThePattern`, `TestLTAV0014BareKeyFieldIsNoCandidate`, `FuzzLTAV0014PrefilterReturnsPatternMatches`, `BenchmarkScreen` |
| `LTA-V0-016` | `internal/secretscreen/prefilter.go` (`writerJWTAlt`) | `TestLTAV0016JWTBranchRequiresHeaderStructure`, `TestLTAV0014PatternSourceIsUnchanged`, `TestTOLV0055_AliasLimit128` (removed) |
| `LTA-V0-017` | `internal/secretscreen/prefilter.go` (`writerSKAlt`) | `TestLTAV0017SKBranchNeedsAnUnhyphenatedRun`, `TestLTAV0014PatternSourceIsUnchanged` |
| `LTA-V0-019` | `internal/secretscreen/prefilter.go` (`writerPrivateKeyHeaderAlt`, `writerPGPPrivateKeyHeaderAlt`) | `TestLTAV0019UnterminatedPrivateKeyRedactsThroughEnd`, `TestLTAV0014PatternSourceIsUnchanged` |
| `LTA-V0-020` | `internal/secretscreen/prefilter.go` (`afterCredentialName`, `runHoldsStem`) | `TestLTAV0020CredentialNameScanMatchesReference`, `TestLTAV0020RepeatedCredentialWordsScaleLinearly` |
| `LTA-V0-015` | `internal/secretscreen/secretscreen.go` (`goJSONPassMarker`, `maskGoPassMarkers`), consumed by `internal/localcompletion` verify | `TestLTAV0015GoJSONPassMarker`, `TestActualVerificationAndSecretRefusal` (removed) (`go-json-pass-log`, `go-json-pass-log-with-embedded-secret`) |

## Rollback

Revert the commit that moves these requirements; the code is unchanged by this spec. For
`LTA-V0-014`, make `writerMatches` call `Pattern` directly; no stored state depends on the prefilter. For
`LTA-V0-015`, remove `goJSONPassMarker` and its loop in `maskGoPassMarkers`; `go test -json` PASS output is
screened again and nothing stored depends on the recognition. For `LTA-V0-016`, restore the
previous last writer alternative and the prior source digest; no stored state depends on it. For
`LTA-V0-017`, drop the `writerSKAlt` substitution and restore the prior source digest; no stored
state depends on it. For `LTA-V0-019`, drop the two private-key writer substitutions and restore the
prior digest. For `LTA-V0-020`, restore the per-occurrence scan; nothing stored depends on either.
