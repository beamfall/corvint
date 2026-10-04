## 2026-10-04 V1-0721: a local doctor for instruction conflicts with host hooks and memory

Human-owned intent: on 2026-10-04 the owner asked for V1-0640's follow-ups to be done, starting
with the V1-0721 spike. Two contradictions between instruction layers had reached agents on this
machine before anyone saw them. AGENTS.md required draft pull requests while a PreToolUse hook
refused the draft flag (V1-0640). AGENTS.md required approval to merge while a saved Claude Code
memory armed auto-merge by default and `~/.codex/AGENTS.md` preauthorized merging. Agents met
each conflict at run time and either picked a side silently or asked.

### Decision

A local, read-only doctor: `script/check-instruction-conflicts.sh`, run as `make
instruction-conflicts INSTRUCTION_HOST_FILES="..."`. It compares the repository's `AGENTS.md` and
`CLAUDE.md` with only the host files the owner names, and reports `CONFLICT`,
`NO_CONFLICT_FOUND`, `NO_CLAIM` or `UNKNOWN` per topic. Its fixture test,
`script/check-instruction-conflicts_test.sh`, is a gate step (`instruction-conflicts-test`); the
doctor itself is not.

Rejected alternatives:

- **A repository test with a table of known host settings.** Host files are not in the repository
  or in CI. A table in the repository would copy one owner's machine into project files, go stale
  when that machine changes, and pass while the real hook disagreed.
- **Extending V1-0413 or the Instruction Doctor (`IID-V0`).** `IID-V0-001` forbids home, config
  and session discovery for that diagnostic. Host hooks and memory are owner-local state outside
  the default product boundary (invariant 7), and they would need their own accepted profile.
- **No check.** Both contradictions happened, each cost a session to find, and a narrow check
  costs one script.

### How host findings stay local, uncertain and non-authoritative

- No discovery: the doctor reads only the paths passed to it. With no `--host`, it reports the host
  layer as `UNKNOWN` instead of passing.
- Every output starts with a provenance line: host files are not Git-pinned (invariant 1), and a
  conflict does not decide precedence. Project-owned files govern this repository (invariant 3);
  the doctor shows a disagreement, and the owner resolves it.
- An absent or unreadable input, anything that is not a regular file, a missing default
  `AGENTS.md`, or a host directory with no `*.md` files reports `UNKNOWN` and exits 3 when no
  conflict is found, never 0 (invariant 2).
- Its only write is a private temporary list of inputs, removed on exit, and it feeds nothing:
  not ranking, evidence, authority, learning or the gate.

### Method and limits

Claims come from a closed table of three topics: `pr-draft` (draft-required or draft-refused),
`pr-auto-merge` (allowed or needs-approval) and `pr-manual-merge` (needs-approval or
preauthorized). Each topic is matched by case-insensitive English patterns per sentence or
semicolon clause, with the lines of a paragraph joined first so wrapped prose still matches. A general "merging requires
approval" rule also counts against arming auto-merge, while "merging by hand" does not. A negated
sentence such as "do not arm auto-merge on a stacked PR" only withholds the allowed claim, because
in practice it states a scoped limit, not an approval rule. A negation or refusal around "draft"
("never open draft pull requests", "draft pull requests are not allowed") counts as refusing
drafts, a sentence that makes arming auto-merge need approval is not also an allowance, and "do
not merge without asking" is not a preauthorization.

Limits: a paraphrase outside the table is missed, so `NO_CONFLICT_FOUND` means only that the table
found none. A sentence split on an abbreviation's period may be misread. Topics are added by
extending the table, with a fixture for each new pair.

### Evidence

- The fixture reproduces both 2026-10-04 contradictions from their original wording: the
  pre-resolution AGENTS.md text, the hook as one JSON line, the auto-merge memory and the global
  merge preauthorization. It reports three conflicts: draft against the hook, merge approval
  against the memory, and merge approval against the global file. The resolved wording reports
  none. The absent-file, unreadable-file, empty-directory and no-host cases report `UNKNOWN`, a
  conflict still exits 1 beside an unknown, and usage errors exit 2. Negative fixtures keep negated
  and refusing wording from flipping polarity, directory inputs and an absent default `AGENTS.md`
  report `UNKNOWN`, and these cases fail against the first draft of the doctor.
- The test passed under macOS awk 20200816 with `/bin/sh`, Debian mawk with dash (`golang:1.27.1`
  image), and Alpine busybox, run as a non-root user so the unreadable-file case bound. shellcheck
  reported nothing.
- A live run on 2026-10-04 against the owner's `~/.claude/settings.json`, the Corvint project
  memory directory, `~/.claude/CLAUDE.md` and `~/.codex/AGENTS.md` reported one conflict:
  `pr-manual-merge`. AGENTS.md says merging by hand still requires explicit approval, and
  `~/.codex/AGENTS.md` (2026-10-03) preauthorizes merging task-owned pull requests across
  projects. That file also says project contracts govern their repositories, so in this
  repository the AGENTS.md rule applies. The wording is left for the owner to reconcile. The
  draft and auto-merge topics found no conflict. A first pass also flagged two scoped "do not arm
  auto-merge" memory sentences as approval rules; the negation rule was narrowed to remove those
  false positives, and the fixture keeps one of those sentences.
- One independent review requested changes: negated draft wording, "arming auto-merge requires
  approval" and "do not merge without asking" flipped polarity, and a directory input aborted awk
  instead of reporting `UNKNOWN`. Each is fixed with a fixture. The re-run live check gave the
  same single `pr-manual-merge` conflict. Its approval noted that a negation before a semicolon
  still flipped the next clause; clauses now split on semicolons too, with a fixture.

Rollback: delete the two scripts and the Makefile block, and remove `instruction-conflicts-test`
from `GATE_STEPS`. No state, format or product behavior depends on them.
