# Decision 0242 — The Swift/Apple candidate binds UNKNOWN_FIELD to a safe canonical extension

Date: 2026-09-13. Status: accepted. Authority: repository owner delegation to make owner calls and
record them (orchestration thread, 2026-09-12).

Decision 0222 applied the canonical-extension rule to every ACP candidate except Swift/Apple and
left `SAC-002` open. `internal/analyzerswift` returned the fixed `NONCANONICAL_REQUEST` sentinel
whenever its closed envelope grammar failed, including for a safe extension.

The call: the rule applies to `SAC-002`. The evidence is in the specs themselves:

- `SAC-002` defines the request as "the frozen experimental envelope", which is the section of
  `analyzer-candidate-profiles.md` that states the rule.
- That section lists `swift-apple` as an envelope family.
- The Swift spec cites `analyzer-candidate-profiles.md` as an authoritative input.
- Where the profile excludes Swift, it names Swift explicitly (`ACP-012`). No such exclusion
  exists for the extension rule.

The three fixed inputs narrow the schema values; they do not remove the extension rule.

Decision 0221 (a) placement and decision 0222 precedence apply unchanged:

- A request is the bound `UNKNOWN_FIELD` rejection when its only defect is extension members that
  follow the root, `target`, or an `inputs` element's schema fields, and its remaining bytes pass
  the closed Swift envelope grammar.
- Any other request that fails the grammar is still the fixed sentinel.
- Content decoding and digest checks run only on an extension-free request, so an extension takes
  precedence over a later scoped reason.

Consequences: `TestSafeCanonicalExtensionBindsUnknownField` (`internal/analyzerswift`) failed
before the change. The candidate reuses the Kotlin/Android `withoutExtensions` shape. Its ACP-009
reachable production source went from 41,215 to 44,081 bytes of 65,536 (21,455 bytes headroom).

Rollback: revert the commit. That restores the fixed sentinel for every unknown member and the
pre-amendment `SAC-002` text.
