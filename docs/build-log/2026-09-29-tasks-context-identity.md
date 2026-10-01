# Supervised context preserves canonical ticket identity

V1-0495 follows diagnostic V1-0494. The supervisor previously sent only the title to
Core before constructing its richer stage prompt. On clean Core source
6dc8ed1bceaa563c4e2cddb505b5891741bbce5a, the actual Oscillux prepared tree returned
OUT_OF_SCOPE for the unchanged title but relevant READY syntax evidence for that title
plus its canonical ticket ID. Original refused packets and all 16 current-source probes
are retained privately under `/tmp/oscillux-context-current`.

The initial query now contains the exact title, a space and the admitted canonical ID.
It appends no paths, parses no prose into authority and truncates nothing. Invalid UTF-8
or more than 8,000 combined runes refuses before invocation. Executable pins, timeout,
output bound, READY/fresh/exact-tree requirements and raw packet uncertainty are unchanged.

Focused workflow tests exercise the real pinned subprocess argv/cwd, metacharacters as
inert data, ASCII/multibyte boundaries and invalid evidence. An opt-in actual Core replay
uses `CORVINT_CONTEXT_TEST_CORE`, `CORVINT_CONTEXT_TEST_REPO` and optionally
`CORVINT_CONTEXT_TEST_ABSENT_REPO`; it checks original title refusal, repaired relevant
retrieval with uncertainty, and refusal when the prepared source is absent. Ordinary
unit runs explicitly skip that external replay unless its inputs are supplied.

Actual replay passed with scratch Core SHA256
64354c84e5cad667236f9e1ea9be7cef057fef1572f149cb9486f41a4c8a3c1e,
prepared Oscillux tree0c5dc301fbc19c277f1c01ef53b3859d76605e3b and absent-source primary
treeee9a25b982365d94cb2ae75c359276fbcaa629cd. No model dispatch was involved. The packets
retain zero authoritative results, syntax/advisory limitations and incomplete coverage;
READY is not task qualification. The scratch executable is unsigned and unqualified.

The scoped store/supervisor tests and docs checks passed. The canonical context-focused
check and independent post-change review are recorded separately in private delivery
evidence. No full gate was requested for this slice during the separately frozen
coordinator work. No #354/A6 inputs, runtime policy, Core ranking or Oscillux source were
changed. Integration and native completion remain required; this source preparation does
not complete V1-0495 or its diagnostic predecessor.
