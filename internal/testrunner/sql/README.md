# Experimental SQL native profiles

These profiles implement the bounded SQL slice of
[`test-runner-execution-v0`](../../../docs/specs/test-runner-execution-v0.md).
They describe native test results; they do not establish database authority,
source Git provenance, dependency closure, or semantic criterion adequacy.

Both require an independently pinned executable and one root-relative script
with a declared input hash. Selection is the whole script: `selectors` is empty
or contains exactly `project`. Execution binds `Input.SourceFile` to that project
as caller-bound metadata. Manual report imports remain caller observations.

## pgTAP

`sql-pgtap` runs a pinned `psql` with fixed quiet, unaligned, tuples-only, no-rc,
no-password-prompt, stop-on-SQL-error options. `project` must end in `.sql`.
The independently hashed `config` file is closed JSON containing only:

```json
{"socketDirectory":"/private/tmp/owned-socket","port":55437,"database":"postgres","user":"corvint"}
```

The config may be absolute or root-relative; symlinks, unknown/duplicate fields,
network hosts, connection strings and credentials are refused. It names an
operator-owned database reachable through one Unix socket. The adapter neither
starts a server nor selects a default database. SQL and psql scripts remain
executable project code; this profile is not a SQL sandbox or permission grant.
Database state, server identity and dependency authority are `NOT_OBSERVED`.

Raw pgTAP stdout supplies one plan and sequential result ordinals. Case identities
combine the caller-bound source file, native ordinal and native description;
`granularity` is `CASE`. Native skip reasons and failure diagnostics are retained.
TAP bailouts, malformed/truncated output, plan mismatches, unqualified directives,
and unclassified stderr cannot produce complete results. Retry information and
failure cause remain `NOT_REPORTED` and `UNKNOWN`, respectively.

psql exit 0 is outcome-neutral: a native `not ok` assertion still exits 0. SQL
errors exit nonzero and remain incomplete; they are never converted to passed
assertions. Empty output and zero tests also remain incomplete.

## Original SQLite sqllogictest

`sql-sqllogictest-sqlite` runs the pinned original SQLite project runner with
fixed `-verify -engine SQLite` options and one hashed `.slt` file. Its connection
file is always `scratch.sqlite` inside the executor's newly created report
directory. This matters because the native runner removes an existing connection
file before opening it. No caller-supplied connection path or extra argv is used.

The native final summary supplies executed and skipped counts. It does not expose
per-query case identities, so the observation contains one `SUITE_ONLY` test
identified by the script, with explicit `executedCount` and `skippedCount`.
Skipped-only suites are `SKIPPED`; empty suites are incomplete. Error counts must
agree with native process status. Missing/duplicate summaries, source mismatch,
unclassified zero-error diagnostics, and a native `halt` remain incomplete.
Failure cause is `UNKNOWN`; retries are `NOT_REPORTED`. No per-query IDs or
semantic assertion links are invented from aggregate counts.

## Native evidence and limits

Captured fixtures in `testdata` came from PostgreSQL 17.11 with pgTAP 1.3.4 and
from the original sqllogictest revision
`db57eba95d7c412bb413da5480c8be24109a8faf`, bundled SQLite 3.53.0. The official
pgTAP archive and pg_prove 3.37 archive were checked against published SHA256
values. Every compiled original sqllogictest source was checked against its
immutable Fossil manifest. Source archives, licenses, manifests, binary hashes,
raw streams and cluster cleanup evidence remain in
`/private/tmp/cem10-build/sql`; `testdata/provenance.json` retains fixture hashes.
No user database or installed service was changed.

The separately evaluated Rust sqllogictest CLI 0.29.1 emits a passed file-level
JUnit testcase even when every SQL record is skipped or the file is empty.
Those reports cannot establish the executed SQL denominator and are not accepted
as a substitute for this original SQLite runner profile.

`TestSQLLiveExecution` is opt-in through explicit `CORVINT_SQL_LIVE_DIR`,
`CORVINT_SQL_CONNECTION`, and `CORVINT_SQL_PSQL` settings. It downloads nothing,
uses already prepared native runtimes and a disposable database, and retains
Build/Execute/Parse/Normalize receipts. The external qualification harness owns
cluster startup and shutdown, including verified signal cleanup. Other database
versions, operating systems, per-query identities for sqllogictest, retries,
database authorization and runtime closure remain unqualified.

Official references:

- [pgTAP](https://pgtap.org/documentation.html)
- [pg_prove](https://pgtap.org/pg_prove.html)
- [Original SQLite sqllogictest](https://www.sqlite.org/sqllogictest/doc/trunk/about.wiki)
- [Rust sqllogictest CLI](https://github.com/risinglightdb/sqllogictest-rs)
