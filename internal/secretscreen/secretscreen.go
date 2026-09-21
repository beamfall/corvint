// Package secretscreen holds the secret-shaped-text detector Corvint's local
// writers share plus immutable stored-format compatibility matchers. Before
// this package existed, internal/trace and
// internal/contextindex each carried a byte-for-byte-identical copy of the
// same pattern with no parity test between them; see docs/agent-memory/fixes.md
// 2026-09-01 "observations ledger has no secret screen; secret regex
// duplicated in two packages".
package secretscreen

import (
	"regexp"
	"strings"
)

// pythonWhitespace mirrors the Python runtime's str.strip() whitespace set,
// expressed as a regexp character-class body.
const pythonWhitespace = `\t\n\v\f\r\x1c-\x20\x{0085}\x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}`

// Placeholder replaces every secret-shaped match Screen redacts.
const Placeholder = "[REDACTED]"

const storedV1AssignmentNames = `api[_-]?key|access[_-]?key|authorization|password|private[_-]?key|secret|token`

const writerAssignmentNames = `api[_-]?key|access[_-]?key|account[_-]?key|authorization|credential|credentials|passphrase|passwd|password|private[_-]?key|secret|token`

// Bare pass must end the field name: allowing its arbitrary suffix would
// classify ordinary status text such as "passed: all checks" as a credential.
const writerAssignmentFields = `[a-z0-9_.-]*(?:(?:` + writerAssignmentNames + `)[a-z0-9_.-]*|pass)`

// writerQuotedAssignmentAlt matches a bare (non-JSON-property) assignment
// name whose value is a single- or double-quoted lexical string, consuming
// it in full (including internal whitespace and backslash escapes, and
// extending through absolute EOF when unterminated) so a value like
// `password = "correct horse battery"` cannot be truncated at the first
// internal space the way the generic non-whitespace value branch in
// secretPattern would truncate it. It must appear before
// secretPattern(writerAssignmentFields, ...) in Pattern's alternation: Go's
// regexp uses leftmost-first alternation, so the shorter generic branch
// would otherwise win and reproduce the truncation. This is writer-only
// and deliberately not part of secretPattern, so StoredV1Pattern cannot
// gain it.
const writerQuotedAssignmentAlt = writerAssignmentFields + `[` + pythonWhitespace + `]*[:=][` + pythonWhitespace + `]*(?:"(?:[^"\\]|\\[\s\S])*(?:"|\\?\z)|'(?:[^'\\]|\\[\s\S])*(?:'|\\?\z))`

// awsAccessKeyIDWithSecretAlt matches an AWS access-key ID immediately
// followed by what is shaped like its paired 40-character secret access
// key, so the secret is redacted even with no nearby assignment name
// (secretPattern's bare `akia|asia` branch only ever covers the key ID).
// Writer-only for the same leftmost-first-precedence and StoredV1Pattern
// reasons as writerQuotedAssignmentAlt.
const awsAccessKeyIDWithSecretAlt = `(?:akia|asia)[a-z0-9]{16}[^a-z0-9]{1,3}[a-z0-9/+]{40}\b`

// authorizationSchemeAlt matches an authorization assignment whose value is
// an HTTP authentication scheme word, consuming the space-separated credential
// that follows it. The generic assignment branch in secretPattern stops at
// the scheme word, so Screen would otherwise redact `Authorization: Bearer`
// and keep the credential. Writer-only for the same leftmost-first-precedence
// and StoredV1Pattern reasons as writerQuotedAssignmentAlt.
const authorizationSchemeAlt = `[a-z0-9_.-]*authorization[a-z0-9_.-]*[` + pythonWhitespace + `]*[:=][` + pythonWhitespace + `]*(?:basic|bearer|digest|negotiate|ntlm|token)[ \t]+[^` + pythonWhitespace + `]+`

// writerCredentialedURLAlt matches a credentialed URL whose password itself
// contains a raw `@`, consuming through the last `@` before whitespace, a
// path slash, `?`, `#` or a quote. Neither the user nor the password crosses
// `?`, `#` or a quote: `?` and `#` end a URL authority, and crossing them or a
// quote would end the match inside a later secret. The stored branch stops
// the password at the first `@` and lets both parts cross `?`, so Pattern
// passes this branch to secretPattern in its place rather than beside it;
// StoredV1Pattern keeps the stored branch unchanged.
const writerCredentialedURLAlt = `[a-z][a-z0-9+.-]*://[^` + pythonWhitespace + `/:?#"']+:[^` + pythonWhitespace + `/?#"']+@`

// storedV1CredentialedURLAlt is the credentialed-URL branch of the stored
// schema-v1 detector.
const storedV1CredentialedURLAlt = `[a-z][a-z0-9+.-]*://[^` + pythonWhitespace + `/:]+:[^` + pythonWhitespace + `/@]+@`

// writerURLTokenUserinfoAlt matches URL userinfo with no password that is
// itself a token: a known vendor prefix at any length (`https://ghs_x@host`)
// or a run of at least 32 letters and digits. Plain user names such as
// `git@` or `deploy@` stay unmatched. Writer-only, ahead of secretPattern.
const writerURLTokenUserinfoAlt = `[a-z][a-z0-9+.-]*://(?:(?:gh[pousr]_|github_pat_|gl(?:pat|rt|dt)-|shp(?:at|ss|ca)_|ya29\.|gocspx-|npm_|hf_|xox[a-z]-|sk-|[rs]k_(?:live|test)_|dop_v1_)[^` + pythonWhitespace + `/:@]*|[a-z0-9]{32,})@`

// Units of a command-line argument. A bare argument is a whitespace-free run
// that ends at an unescaped quote, so a flag value inside a JSON string stops
// at the string's closing quote instead of swallowing the next property; a
// backslash pair (`\"`) is one unit, so a JSON-escaped quoted value stays whole.
// A double- or single-quoted argument is the complete lexical string under
// writerQuotedAssignmentAlt's escape and unterminated-through-EOF rules.
const (
	bareArgumentUnit        = `(?:[^` + pythonWhitespace + `"'\\]|\\[^` + pythonWhitespace + `])`
	bareArgumentFirst       = `(?:[^` + pythonWhitespace + `"'$\\]|\\[^` + pythonWhitespace + `])`
	doubleQuotedUnit        = `(?:[^"\\]|\\[\s\S])`
	doubleQuotedFirst       = `(?:[^"\\$]|\\[\s\S])`
	singleQuotedUnit        = `(?:[^'\\]|\\[\s\S])`
	quotedArgumentTerminate = `\\?\z`
)

// letterAndDigit matches a run of units that contains at least one letter and
// at least one digit; when the run does not start with that letter or digit,
// its first unit must match first.
func letterAndDigit(first, unit string) string {
	return `(?:` + first + unit + `*)?(?:[a-z]` + unit + `*[0-9]|[0-9]` + unit + `*[a-z])` + unit + `*`
}

// credentialArgument is a command-line value that contains at least one
// letter and at least one digit. It is the "looks like a credential" test for
// bare flags (decision 0164): a plain word such as `flag` or `value`, a
// digit-only port, and a `$VARIABLE` reference (bare or double-quoted, with
// or without a digit) stay unmatched. A single-quoted value is literal to a
// shell, so a leading `$` there does not exempt it.
var credentialArgument = `(?:"` + letterAndDigit(doubleQuotedFirst, doubleQuotedUnit) + `(?:"|` + quotedArgumentTerminate + `)` +
	`|'` + letterAndDigit(singleQuotedUnit, singleQuotedUnit) + `(?:'|` + quotedArgumentTerminate + `)` +
	`|` + letterAndDigit(bareArgumentFirst, bareArgumentUnit) + `)`

// curlUserPassword is a `user:password` pair: a double- or single-quoted
// string containing a colon, or a bare pair made of bareArgumentUnit runs.
var curlUserPassword = `(?:"` + doubleQuotedUnit + `*:` + doubleQuotedUnit + `*(?:"|` + quotedArgumentTerminate + `)` +
	`|'` + singleQuotedUnit + `*:` + singleQuotedUnit + `*(?:'|` + quotedArgumentTerminate + `)` +
	`|(?:[^` + pythonWhitespace + `:"'\\]|\\[^` + pythonWhitespace + `])+:` + bareArgumentUnit + `+)`

// writerCredentialFlagAlt matches command-line credentials: a long or
// single-dash credential-vocabulary flag followed by a separate credential
// argument (`--password hunter2`, `--db-token abc123`); a `-p` separate or
// `=`-joined credential argument after `login`, `-u` or `--user` on the same
// line (`docker login -p hunter2`), consumed from that context word so git's
// `log -p <revision>` stays unmatched; and `curl -u`/`--user` with a
// `user:password` pair, where the flag must start an argument so a `-u` inside
// a hyphenated host (`svc-users:8080`) is not read as the flag. `=`-joined long flags are already matched by the
// generic assignment branch. Appended after secretPattern: none of these
// shapes starts where an earlier alternative can match.
var writerCredentialFlagAlt = `\B--?(?:[a-z0-9]+[_-])*(?:` + writerAssignmentNames + `|pass)[ \t]+` + credentialArgument + `|` +
	`(?:\blogin\b|\B-u)[^\n]*?[ \t]-p(?:[ \t]+|=)` + credentialArgument + `|` +
	`\bcurl[ \t](?:[^\n]*?[ \t])?(?:-u[ \t]*|--user[ \t]+)` + curlUserPassword

// writerOnlyVendorAlt matches vendor token and endpoint shapes with no
// pre-existing shorter alternative to out-race, so it is appended after
// secretPattern rather than needing precedence: Svix/Stripe-style webhook
// signing secrets (`whsec_`), Hugging Face tokens (`hf_`), DigitalOcean
// personal access tokens (`dop_v1_`), Slack app-level tokens (`xapp-`),
// Google OAuth access tokens (`ya29.`) and client secrets (`GOCSPX-`),
// Shopify access tokens (`shpat_`, `shpss_`, `shpca_`), GitLab runner and
// deploy tokens (`glrt-`, `gldt-`), and Slack incoming-webhook URLs (which
// carry no `@`, so the credentialed-URL branch in secretPattern never sees
// them). Writer-only so StoredV1Pattern does not gain new vendor shapes.
const writerOnlyVendorAlt = `whsec_[a-z0-9+/=_-]{20,}|hf_[a-z0-9]{20,}|dop_v1_[a-f0-9]{20,}|xapp-[a-z0-9-]{10,}|` +
	`ya29\.[a-z0-9_-]{20,}|gocspx-[a-z0-9_-]{20,}|shp(?:at|ss|ca)_[a-z0-9]{20,}|gl(?:rt|dt)-[a-z0-9_-]{20,}|` +
	`https?://hooks\.slack\.com/services/[a-z0-9]+/[a-z0-9]+/[a-z0-9]+`

// Pattern is the current secret-shaped-text detector: key=value/key: value,
// quoted credential values (bare assignment or double-quoted JSON property,
// whole lexical strings, including escaped quotes and whitespace, without
// JSON escape decoding; unterminated values extend through absolute EOF,
// including a dangling backslash),
// credential assignments, the credential after an authorization scheme word,
// private-key headers, credentialed URLs (including a raw `@` in the
// password and token-shaped userinfo), command-line credential flags, an AWS
// access-key ID paired with its adjacent secret, and known vendor token
// shapes (GitHub, GitLab, Slack, AWS, Azure account keys, Google, npm,
// Stripe, PyPI, SendGrid, Shopify, Svix/Stripe webhook secrets, Hugging
// Face, DigitalOcean, bearer tokens, and JWT-shaped strings).
var Pattern = regexp.MustCompile(`(?i)` + writerQuotedAssignmentAlt + `|` + awsAccessKeyIDWithSecretAlt + `|` + authorizationSchemeAlt + `|` +
	writerURLTokenUserinfoAlt + `|` +
	secretPattern(writerAssignmentFields, writerCredentialedURLAlt) + `|"` + writerAssignmentFields + `"[` + pythonWhitespace + `]*:[` + pythonWhitespace + `]*(?:"(?:[^"\\]|\\[\s\S])*(?:"|\\?\z)|[^` + pythonWhitespace + `]+)|` +
	writerOnlyVendorAlt + `|` + writerCredentialFlagAlt)

// StoredV1Pattern preserves the detector used when schema-version 1 trace
// rows were written. It is intentionally narrower than the current writer
// screen so a detector expansion cannot invalidate immutable stored rows.
var StoredV1Pattern = regexp.MustCompile(secretPattern(`[a-z0-9_.-]*(?:`+storedV1AssignmentNames+`)[a-z0-9_.-]*`, storedV1CredentialedURLAlt))

var goVerbosePassMarker = regexp.MustCompile(`(?m)^[ \t]*--- PASS: [^\r\n ]+ \([0-9]+(?:\.[0-9]+)?s\)\r?$`)

func writerMatches(text string) [][]int {
	protected := goVerbosePassMarker.FindAllStringIndex(text, -1)
	matches := Pattern.FindAllStringIndex(text, -1)
	kept := matches[:0]
	for _, match := range matches {
		insideMarker := false
		for _, marker := range protected {
			if match[0] >= marker[0] && match[1] <= marker[1] {
				insideMarker = true
				break
			}
		}
		if !insideMarker {
			kept = append(kept, match)
		}
	}
	return kept
}

func secretPattern(assignmentFields, credentialedURLAlt string) string {
	return `(?i)` + assignmentFields + `[` + pythonWhitespace + `]*[:=][` + pythonWhitespace + `]*[^` + pythonWhitespace + `]+|` +
		`-----BEGIN [A-Z ]*PRIVATE KEY-----(?s:.*?)-----END [A-Z ]*PRIVATE KEY-----|` +
		`-----BEGIN [A-Z ]*PRIVATE KEY-----|` +
		`-----BEGIN PGP PRIV[A]TE KEY BLOCK-----(?s:.*?)-----END PGP PRIV[A]TE KEY BLOCK-----|` +
		`-----BEGIN PGP PRIV[A]TE KEY BLOCK-----|` +
		credentialedURLAlt + `|` +
		`gh[pousr]_[a-z0-9]{20,}|github_pat_[a-z0-9_]{20,}|` +
		`glpat-[a-z0-9_-]{20,}|xox[a-z]-[a-z0-9-]{10,}|` +
		`(?:akia|asia)[a-z0-9]{16}|aiza[a-z0-9_-]{30,}|npm_[a-z0-9]{20,}|` +
		`\bsk-[a-z0-9_-]{20,}|(?:sk|rk)_(?:live|test)_[a-z0-9]{16,}|` +
		`pypi-ageichlwasi5vcmc[a-z0-9_-]{20,}|` +
		`sg\.[a-z0-9_-]{16,}\.[a-z0-9_-]{32,}|sk[0-9a-f]{32}|` +
		`bearer[` + pythonWhitespace + `]+[a-z0-9][a-z0-9_.~+/-]{15,}|` +
		`[a-z0-9_-]{10,}\.[a-z0-9_-]{10,}\.[a-z0-9_-]{10,}`
}

// MatchString reports whether text contains secret-shaped content for new
// writes and Git-history candidate filtering. Stored traces use the v1 matcher.
func MatchString(text string) bool {
	return len(writerMatches(text)) != 0
}

// MatchStoredV1String reports whether text matches the immutable schema-v1
// trace detector. New writes must use MatchString instead.
func MatchStoredV1String(text string) bool {
	return StoredV1Pattern.MatchString(text)
}

// Screen redacts every secret-shaped match in text with Placeholder and
// reports whether any redaction occurred.
func Screen(text string) (redacted string, hit bool) {
	matches := writerMatches(text)
	if len(matches) == 0 {
		return text, false
	}
	var screened strings.Builder
	start := 0
	for _, match := range matches {
		screened.WriteString(text[start:match[0]])
		screened.WriteString(Placeholder)
		start = match[1]
	}
	screened.WriteString(text[start:])
	return screened.String(), true
}
