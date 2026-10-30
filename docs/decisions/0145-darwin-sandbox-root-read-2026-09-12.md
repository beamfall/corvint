# Decision 0145 — The Darwin analyzer sandbox may read the root directory entry list

Date: 2026-09-12. Status: accepted. Authority: repository owner instruction, 2026-09-12.

On macOS 26.6.2 the ACC-V0-002 Darwin profile aborted every contained payload with SIGABRT
(exit 134, empty stderr) before it ran: a trivial C read/write program and the Go echo analyzer
both aborted in a 0700 staging directory with `Dir=/` and an empty environment. Under the 100 ms
plan cap the abort surfaced as a timeout that tests skipped as a host limit. Bisecting the profile
showed that only `(allow file-read-data (literal "/"))` lets both payloads run; metadata,
existence, or xattr reads on `/`, and Cryptexes or `/private/var/db/dyld` reads, do not.

The owner call: the profile gains exactly that rule. It permits listing the root directory's
entries and nothing beneath it: `/System` and `/System/Volumes/Data` stay denied first, repository
paths stay unreadable, and Mach lookup/registration stays denied. ACC-V0-002 now states the
allowance and forbids any `/` subpath rule; `TestSandboxProfileDeniesDataMountAndAmbientMachIPC`
requires the literal rule and refuses a subpath form, and
`TestNewStaticCoreUsesContainedNativeInvocation` runs a payload under the production profile.

Disclosed residue: the entry list of `/` names top-level directories, which is not repository
content. The payload now runs but that test still times out at the 100 ms invocation cap on a
loaded Darwin host; the cap is a separate defect (agent-memory bugs).

Rollback: remove the rule, its test expectation, and the ACC-V0-002 sentence. Contained Darwin
payloads then abort again and the contained-invocation test fails.
