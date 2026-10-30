# Decision 0031 — siblings sharing a basename term with the subject rank first

Date: 2026-09-02. Status: accepted. Authority: repository owner, verbatim instruction "do 1 then 2"
(2026-09-02), where 2 was the recommendation to stop the packet's twenty rows going to its weakest
slot after the beamfall-apple reading showed thirteen missed gold paths in the subject's own
directory.

## What is decided

1. The `sibling` slot orders its candidates by the number of basename terms (camelCase split,
   lowercased, extension dropped) shared with the subject's basename, then by the identifier
   evidence it already used (TCP-V0-004 amended). `PlaybackPlanningTests.swift` and
   `PlaybackState.swift` come before an unrelated file in the same directory.
2. The alternative the recommendation named, admitting structural overflow above a lexical floor
   of five rows, is not adopted: offline on beamfall-apple it added 104 sibling and co-change rows
   for no new gold and displaced two lexical gold rows (64 against 66 gold rows in the top 20).
   The missed siblings were ranked out, not capped out.

## Evidence

Offline gold-placement probe (`corvint context`, limit 20, each task's base commit checked out):
corvint v2 (63 tasks) 232 to 242 gold rows, sibling gold 16 to 25; beamfall-apple (40) 66 to 68;
beamfall (60) 95 to 95 with top-five 61 to 64. No set lost a row.

## What is not decided

Whether the same term overlap should weight the lexical fill's path term; the lexical slot is the
term listing the trial's grep arm approximates and changing it changes what the packet is compared
against.
