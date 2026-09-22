# Decision 0164 — Secret screen gains common credential formats with a flag false-positive policy

Date: 2026-09-12. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

`docs/agent-memory/bugs.md` (2026-09-12) confirmed that `internal/secretscreen.Pattern` neither
matched nor redacted Azure `AccountKey=` connection strings, Google `ya29.` access tokens and
`GOCSPX-` client secrets, Shopify and GitLab runner/deploy tokens, command-line credential flags,
`curl -u user:pass`, or a short token used as URL userinfo (`https://ghs_x@github.com`). A related
hypothesis in `docs/agent-memory/ideas.md` was confirmed by a failing test: the credentialed-URL
branch stopped a password at its first raw `@`, so `https://user:p@ssw0rd@host` kept `ssw0rd@`.
Invariant 5 makes each miss a learning-admission leak, but a bare `--token` rule would also refuse
the commit message "add --token flag".

The owner call:

1. Add the formats with distinctive prefixes or shapes to the writer screen only: an
   `account[_-]?key` assignment name; `ya29.`, `GOCSPX-`, `shpat_`/`shpss_`/`shpca_` and
   `glrt-`/`gldt-` tokens with a 20-character floor; a credentialed URL redacted through the last
   `@` before whitespace, `/`, `?`, `#` or a quote (a later `@` past those belongs to another value,
   and crossing it would cut a following secret in half); and passwordless URL userinfo that carries a known vendor-token
   prefix at any length or is at least 32 letters and digits. `StoredV1Pattern` is unchanged.
2. Flag false-positive policy. A bare credential flag redacts only when a value follows as a
   separate argument (or, for `-p`, a separate or `=`-joined argument) that itself looks like a
   credential: it contains at least one letter and at least one digit. A plain word (`flag`,
   `value`), a digit-only port or uid, and a `$VARIABLE` reference stay unmatched. Long and
   single-dash flags use the writer credential vocabulary with an optional `name-`/`name_`
   qualifier (`--password`, `-token`, `--db-password`). Because `-p` also means "patch" to Git and
   "parents" to `mkdir`, it redacts only after `login`, `-u` or `--user` on the same line, and the
   redaction starts at that context word. `curl -u`/`--user` redacts any `user:password` pair,
   since the colon pairing is itself credential syntax; the flag must start an argument, so a `-u`
   inside a hyphenated host before a port (`curl http://svc-users:8080/`) is not the flag.
3. Precedence. The two URL alternatives precede `secretPattern` in `Pattern` because the stored
   credentialed-URL branch matches a shorter span at the same start and Go's regexp is
   leftmost-first (the same reason as `authorizationSchemeAlt`). The vendor and flag alternatives
   are appended after it: none starts where an earlier alternative matches.

Accepted residual false positives: a letter-and-digit argument after a credential flag that is not
a secret (`--token-name v2` does not match, but `--secret build42` does), and a `-p` argument after
an unrelated `-u` flag on the same line. Accepted residual misses: a glued short option
(`mysql -phunter2`), since the call admits only separate or `=`-joined values; a flag value with no
digit (`--password hunter!`); and `=`-joined long flags keep the pre-existing generic assignment
behavior, which matches any value (`--token=flag`).

The same change closes the other three hypotheses in that ideas entry. The observations triage
reader skipped nothing a hand-edited row put in a rendered key, so a degradation containing a line
break forged a `FALSIFICATION` line; triage now skips such a row as malformed (`SOL-V0-005`). The Go
status probe refused a CRLF `.git` pointer that Git itself accepts; it now strips trailing CR and LF
as Git does (`EAF-V0-007`). The envelope hypothesis is refuted as a defect: U+00A0 renders as a
visible space rather than a line break or invisible reordering, so it is outside the `AHI` hidden-
character rule, and a lookalike terminator is one case of unbounded semantic imitation that the
envelope's untrusted-data label, not its byte-exact collision check, addresses. No Go or JavaScript
envelope builder changes.

Consequences: `docs/specs/learned-trace-admission-v0.md` `LTA-V0-004`,
`docs/specs/self-observation-ledger-v0.md` `SOL-V0-005` and
`docs/specs/expert-audit-followup-v0.md` `EAF-V0-007` are amended with their traceability rows. The
analyzer schema moves to the next `corvint-analyzer/` identifier because `internal/secretscreen` and
`internal/gitstatus` are pinned analyzer inputs. The bugs.md entry and the ideas.md hypothesis entry
are removed.

Rollback: revert this decision's commit. That restores the narrower screen, the first-`@` URL
redaction, the unvalidated triage keys, the CRLF pointer refusal, the prior analyzer schema
identifier, and both backlog entries.

## Amendment 2026-09-13: quoted flag values, variable references and the URL user part

A fresh-merge review (`docs/agent-memory/ideas.md`, 2026-09-13) hypothesised four gaps against this
decision's intent, and a scratch probe of the writer screen confirmed all of them. First,
`mysql --password "hunter2 more words"` redacted only up to the first space and kept `more words"`.
Second, in `{"cmd":"x --token abc123","password":"top secret value"}` the flag argument swallowed
`","password":"top`, which cut the following secret in half and leaked `secret value"`. Third,
`--password $DB_PASSWORD2` matched, although point 2 says a `$VARIABLE` reference stays unmatched.
Fourth, `https://u:x#k='p@ss word'` still leaked `ss word'`, because the stored credentialed-URL
branch inside `secretPattern` matched as a fallback once the point 1 alternative declined.

The owner call:

- A credential-flag argument and a `curl -u` pair are shell arguments. A double- or single-quoted
  argument is consumed as one complete lexical string, and an unterminated quote redacts through
  EOF, the same rule as the quoted assignment. A bare argument ends at whitespace or an unescaped
  quote. A bare or double-quoted argument starting with `$` stays unmatched. A single-quoted `$`
  is literal, so it is screened like any other value.
- The writer's credentialed-URL alternative excludes whitespace, `/`, `?`, `#` and quotes from the
  user part as well as the password. This amends point 3: it now replaces the stored branch inside
  `secretPattern` for `Pattern`, instead of preceding it. The passwordless-userinfo alternative
  still precedes `secretPattern`. `StoredV1Pattern` keeps the stored branch unchanged, so the
  writer no longer matches `https://host?x=a:b@c` while the stored v1 reader still does.

New accepted residuals:
- A bare flag argument stops at an embedded unescaped quote: `--token abc1"tail` keeps `"tail`.
- A `$`-leading double-quoted secret literal is not screened.
- A quoted `=`-joined long flag (`--password="abc 123"`) keeps the generic assignment behaviour
  above.

`LTA-V0-004` is amended. `internal/secretscreen/testdata/parity.json` gains baseline and
writer-only rows, and the analyzer schema moves to the next identifier.

Rollback for this amendment: revert its commit. That restores first-space flag arguments, matched
`$` references, the stored URL fallback, the prior `LTA-V0-004` text and parity rows, and the prior
analyzer schema identifier.
