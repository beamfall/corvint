## 2026-09-27 V1-0292: Swift reverse-import corroboration no longer accepts `init` or a member

The pre-1.0 panel found that the Swift `reverse-import` rule (decision 0030) accepted any symbol the
subject defines as corroboration, with no length floor and no rarity floor. A subject's `init` or a
member property such as `width` is spelled by nearly every Swift file, so nearly every importer of
the module was admitted. The Go reference slot already requires four bytes and at most 50 sources
(`contextMaxDefiners`).

Decision: a corroborating Swift symbol is now a type or function name at least four bytes long that
at most 50 sources name. It is never `init`, `deinit` or `subscript`, which name themselves, and
never a `var` or `let`. The Swift extractor emits those both as members and at the top level, and a
symbol records no enclosing declaration, so a member property cannot be told from a top-level
constant, and both are dropped. For the same reason a method name still counts. Telling members
apart would need a new index field, and that is not part of this fix. TCP-V0-004 states the rule.

Evidence: `TestTaskContextSwiftImporterNeedsMoreThanAnInitializerOrAMember` admits an importer that
names the subject's type and refuses one that spells only `init` and `width`. The decision 0030
test still admits the same two importers. The beamfall-apple corpus that decision 0030 measured is
outside this repository and was not rerun, so the effect on its 17-in-127 result is not measured.

Rollback: revert the change. Corroboration again accepts every subject symbol.
