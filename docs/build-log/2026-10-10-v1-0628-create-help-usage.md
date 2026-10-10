# V1-0628: top-level help no longer implies CREATE takes --target

Ticket V1-0628 (issue 430). Per-verb `ticket create --help` already omits `--target` and
`--expected-revision` and documents optional payload `localToken` (CAL-V0-051, shipped, covered by
`TestCALV0051_CreateHelpAndMeaningfulIDs`, which also exercises BT-002, F0-5 and FL-016.matrix through
show, claim and plan preview, collisions and invalid tokens). The remaining misleading text was the
top-level `help` usage line `ticket <mutation> ... [--target TICKET|LOCAL --expected-revision N]`.

Change: that line is split into a `ticket create` line (names `localToken`, no target) and a line for
the target-taking mutations. Proposed `CAL-V0-211` (pending owner acceptance, V1-0628) records it;
`TestCALV0211_TopLevelHelpCreateUsage` covers it.

Decision: no caller-chosen ID mechanism beyond the existing `localToken`, no alias, no serial or wire
change. The ticket's first option (explicit ID) is already delivered by `localToken`; an alias would
need a new identity contract and is not proposed here.

Not run: `make gate`, the whole suite, live-store check.
