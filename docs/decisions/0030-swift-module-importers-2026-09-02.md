# Decision 0030 — a Swift subject's module importers are `reverse-import` rows when they name its symbols

Date: 2026-09-02. Status: accepted. Authority: repository owner, verbatim instruction "do 1 then 2"
(2026-09-02), on the reading of the unseen beamfall-apple set (40 Swift tasks) that recommended
"a Swift import rule for `pair` and `reverse-import`" as the retrieval slice that set names.

## What is decided

1. The task-context packet's `reverse-import` slot admits, for a Swift subject in a SwiftPM
   `Sources/<module>/` or `Tests/<module>/` layout, every Swift source in another module that has an
   `import <module>` (or `@testable import <module>`) line and mentions a symbol the subject defines
   as a whole word (TCP-V0-004 amended). The row is `syntax`, medium, its evidence line the import.
2. The rule lives in the packet compiler only. `impact` and its receipt keep refusing `.swift`
   (`GPK-V0-027`, decision 0007 D2): those are oracle-parity surfaces and the packet is not.
3. The symbol corroboration is part of the rule, not a ranking. On the beamfall-apple corpus the
   module edge alone reaches 22 gold paths in 351 rows; with the corroboration 17 in 127, the
   co-change slot's precision, and a packet of twenty rows cannot afford the former.

## What is not decided

Whether the same shape (module directory plus symbol mention) should serve Kotlin, Rust, and C#,
which are indexed for symbols and have no import rule either. Each needs its own layout rule and
its own measurement; none has an unseen set yet.
