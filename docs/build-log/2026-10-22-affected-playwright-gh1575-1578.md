# Playwright/TypeScript narrowing for GitHub #1575-#1578

Date: 2026-10-22. Lane: claude/affected-pw-gh1575-1578 (base 0b7d59ba6). Tickets: V1-2455..V1-2458.
Requirements `TJAA-V0-034..037` in `docs/specs/typescript-javascript-affected-adapter-v0.md` are
accepted under the owner's standing auto-accept (2026-10-11).

## Decisions

- #1575 (`TJAA-V0-034`): the regex `import\s*\(` was replaced by the existing literal-aware code
  scanner. Literal text and `x.import(` are skipped. In TypeScript sources, `import('<literal>')`
  followed by a type-style member chain, or after `typeof`, is an import type. It records the
  literal as a static edge instead of raising `typescript:dynamic-import`. Because the specifier
  is a literal, the edge still covers a value-position `import('./x').Y`. A computed argument,
  `.then`, a call on the chain, `await`, and every JavaScript source stay dynamic.
- #1576 (`TJAA-V0-035`): `process.env.NAME` and the numeric conversions are admitted and proven
  primitive. The proof is token-occurrence soundness, the same shape as the `devices` and
  const-literal roots: every occurrence of the root must be the exact read or call. A
  `globalThis`/`global`/`window`/`self` token turns both admissions off.
  - Array literals accept one trailing comma.
  - A computed `launchOptions.executablePath`, when it is the only computed launch option, becomes
    the execution-axis unknown `playwright:browser-executable-unresolved`. The identity key list
    shared with `internal/jstestprovider` is unchanged.
- #1577 (`TJAA-V0-036`): `dependencies: NAME` resolves through a single top-level `const` whose
  initializer is a literal array, a parenthesized one, or a conditional of them. It resolves to
  the branch union, so neither discovery nor selection needs `--env`. Any other use of the name
  still refuses.
- #1578 (`TJAA-V0-037`): the general adapter now demotes a Playwright-owned module outside the
  default `testMatch` to a source. Under a config with unmodelled membership markers, a
  conventional test path stays a test. That config already raises
  `typescript:executable-config-unresolved`. The general adapter resolves tsconfig aliases only
  when Playwright is the sole runner identity and no Playwright config names a `tsconfig`.
  Another runner's loader need not follow tsconfig `paths`.

## Residuals

- An imported module or runtime preload that replaces `process.env`, `Number` or a `devices`
  descriptor is outside the static model.
- Demotion is Playwright-only. Other runners keep their import-based test candidacy.
- Evidence is synthetic fixtures only. Recall on the reporter's repository is `NOT_OBSERVED`.
