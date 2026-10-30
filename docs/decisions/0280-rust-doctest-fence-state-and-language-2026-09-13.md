# Decision 0280 — Rust doctest anchors track fence state and a closed language set

Date: 2026-09-13. Status: accepted. Authority: repository owner call in the assigned leaf task.

The Rust affected-test adapter classified each fence-looking doc line as an opener before consulting
its current fence state. A closing line for a non-Rust block therefore looked like a bare runnable
Rust fence and made the file a doctest anchor. The info parser also treated `ignore` as non-Rust,
accepted any token beginning with `edition`, and accepted a known Rust token even beside an unknown
language token.

The call: track an opened non-Rust fence before testing later lines as openers. A bare fence is Rust;
an annotated fence is Rust only when every comma- or space-separated token is `rust`, `ignore`,
`no_run`, `compile_fail`, `should_panic`, or `edition20xx`. Any other token makes the fence non-Rust.
Its closing fence closes that state and never becomes a bare opener. An unterminated bare or Rust
fence remains a conservative doctest anchor.

Consequence: `RUST-AFFECTED-V0-003` and its detection prose are amended. No wire or frontier code
changes. `TestFencedDoctestLanguageAndClosingFence` pins the Rust token set, non-Rust exclusions,
closing-fence behavior, and conservative unterminated Rust anchors.

Rollback: revert the commit. That restores the documented false-positive closing-fence anchor and
the false-negative `ignore` classification.
