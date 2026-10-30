# CodeQL quoting, hashing and URL-scheme boundaries — V1-3179

The owner-requested security triage reviews 21 CodeQL alerts on main at
21fea70cb (analysis 2060084653): go/unsafe-quoting #27-#43, go/command-injection
#99, go/weak-sensitive-data-hashing #100/#101 and go/incomplete-url-scheme-check
#102. Seven are repaired at the sink with a hostile-input regression test; fourteen
are reasoned false positives. Alert state on GitHub is unchanged; resolution needs
the delivered-head CodeQL run.

## Repairs

- #27, #38 (`cmd/corvint` invalidChoice, unknownHelpTopic) and #28 (`internal/cem/cli`
  invalidChoice): the value and choices are rendered with Python `repr`, matching
  argparse's `%r`. Ordinary values keep their exact bytes; a quote or newline is escaped.
- #31 (`jstestprovider` boundarySensitiveProperty): a bare `"` inside a `'` or template
  property literal made the JSON decode fail, leaving escapes undecoded so an escaped
  action name evaded the sensitive-input classifier. Bare quotes are now escaped first.
- #32 (`tasks/cli` quoteField): refused `--fields` names are `strconv.Quote`d.
- #39 (`corpusbridge` frameCost): the path is costed as the JSON the row writes, not a
  Go-quoted string; costs for ordinary paths are unchanged.
- #102 (`appmap` skExternal, SKH-V0-111): `data:` and `vbscript:` targets, and any
  letter case of the listed schemes, are external and never in-app routes.

## False positives

- #29, #30, #34, #42: the concatenated quote is a search needle, the guard itself, a
  value already refused for quotes, or English prose.
- #33, #40, #41 (mobile WDIO config): every value is `json.Marshal`ed, a complete JS
  string literal; pinned by `TestJSLiteralEscapesHostileStrings`.
- #35-#37 (diagnostic envelope): every value passes through `pythonJSONString`.
- #43 (tracerecordrepo): Python `CalledProcessError` text with `repr`-escaped elements.
- #99: console form values become discrete argv after fixed flags of an operator-set
  binary, exec'd without a shell; the Tasks parser takes the next token as a value.
- #100/#101: SHA-1/SHA-256 recompute the Git object id that cat-file reported; the
  algorithm is fixed by the Git object format, not a secret-hashing choice.

## Rollback

Revert the lane commit; no wire format, store or Git semantics changed.
