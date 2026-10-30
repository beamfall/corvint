# Security residuals V1-3197..3199: JS escapes, escaped row budget, URL scheme normalisation

The independent security review of the V1-3179 CodeQL batch filed three Low or informational
residuals. Each fix narrows a classifier toward the conservative side, and none widens what Corvint
admits.

- V1-3197 (`internal/jstestprovider`): bracketed property names and argument candidates now decode
  with the full ECMAScript string-escape grammar (`\xHH`, and `\uHHHH` or `\u{..}`, whose BMP code units pair into surrogates in either form,
  `\0`, `\v`, identity escapes such as the backtick, and line continuations over LF, CRLF, CR,
  U+2028 and U+2029). It replaces the JSON-rule decode, which failed on JS-only escapes and left
  the raw text in the comparison. A property name that cannot be decoded (a trailing backslash, a
  malformed or out-of-range escape, a lone surrogate, or a legacy octal) fails closed and is
  classified as sensitive, so the receipt is refused as unsupported syntax. Argument candidates
  still keep the raw text and add the decoded value only when decoding succeeds. Evidence:
  `TestSensitiveInputPropertyJSEscapes` has one case per escape plus the fail-closed cases and
  negative controls.
- V1-3198 (`internal/mcp/corpusbridge`): the structured half of `frameCost` counts the JSON-escaped
  path bytes instead of `len(path)`, the same encoding the batch row writes (DCP-V1-200..202).
  The enveloped-text half is measured on that same escaped row, where it used `strconv.Quote`
  (`\x01`, 4 bytes, where JSON writes `\u0001`, 6 bytes). The up-front reserve for unresolved rows
  counts both encodings of the escaped path. Evidence: `TestFrameCostCountsEscapedPath` builds the
  row with the batch writer for a 1024-byte control-character path; the previous accounting fails
  it, at 11360 bytes against the 13398 written.
- V1-3199 (`internal/appmap`, SKH-V0-111): `skExternal` normalises a target as a URL parser does.
  It strips leading and trailing C0 controls and spaces, removes tabs and newlines, and treats a
  backslash as a slash. It then treats any scheme, with or without `//`, as leaving the
  application. A template attribute value is also checked after HTML character references
  decode, so `&#x6A;avascript:` and `javascript&colon;` are skipped too. The spec text for SKH-V0-111 records the rule. Evidence:
  `TestSKHV0_111_ObfuscatedSchemesAreNotInApp` covers template and Ruby links.

An independent review of the first commit found three gaps: the text-half and reserve
under-count, the `\u{D800}` surrogate form decoding to U+FFFD, and the HTML character-reference
bypass. All three are fixed here, each with a test.
