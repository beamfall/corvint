# OpenCode 2.0 qualification range

Issue 363 found that the native qualification producer hard-coded OpenCode 2.0.18 even though the
package declaration admitted compatible 2.0 releases. An installation running 2.0.19 could never
produce a matching qualification record and `corvint_status` therefore remained `UNQUALIFIED` with
`qualification-record-unavailable`.

Adapter 0.7.3 admits qualification campaigns for `>=2.0.18 <2.1.0`. The producer reads the actual
host version from the executable in an isolated home, refuses versions outside that range, and
requires the live plugin setup callback to report the same value. The resulting record remains bound
to the exact host version, host executable digest, adapter package bytes, Corvint executable, OS and
architecture. No record is inferred from the range and status remains read-only. Missing or drifted
records return the supported range and the action needed to qualify the exact installation.

The initially installed host was OpenCode 2.0.18. The official
`@opencode/cli-darwin-arm64@2.0.19` archive was then retained in task scratch from its npm release:
archive SHA-1 `719c472fcb3453c78f3900f28635813debd60485`, binary SHA-256
`4d05fc8d3592e4499299e9f6c3411e6f2d1773f61e380da08550ab036d613cd7`, Mach-O arm64, and isolated
`--version` output `opencode v2.0.19`. The focused producer/consumer regression constructs and
validates an exact 2.0.19 tuple.

After independent source review, clean commit
`7523632787df1d423dd85261c375659baee271f0` ran the complete credential-free native campaign against
that executable. The exact `(OpenCode 2.0.19, adapter 0.7.3, darwin, arm64)` tuple passed all fourteen
conformance groups. Query p95 was 207.622 ms, lifecycle p95 was 120.754 ms, critical recall was 1/1,
and injected context was 2,460 bytes against a 14,362-byte manual baseline. The interruption witness
observed descendants and found none surviving before supervisor cleanup. The exact record SHA-256 is
`b883de19a64c189b052d7dcdee4dafab9c57b270074495e7b30bcc0609bceabc`; its Corvint executable SHA-256
is `7c791817403816146bb982e4305e71b2ac225bd23f0acd673184ac67995a0350`. The record remains a private,
ignored installation artifact; cloning the repository does not transfer it. This is exact tuple
evidence, not a claim that all future 2.0 releases were live-tested.

Execution authority remains `NONE`, Frontier remains `UNAVAILABLE`, and legacy lifecycle receipts
remain `FALLBACK`. Dirty or mixed Corvint source checkouts still refuse qualification before a record
can be published. Roll back by reverting adapter 0.7.3 and its AHI-032 amendment; existing exact
records then remain invalid because their package bytes no longer match.
