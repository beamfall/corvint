# Playwright loader names, tag-closure aliases, and template suffixes

Date: 2026-10-24. GitHub #1660 and #1663. Requirements `AFP-V0-417..419` in
`docs/specs/affected-plan-v0.md` and `IDX-SNAP-V0-109` in `docs/specs/index-snapshot-v0.md`, each
accepted under the owner's standing auto-accept (2026-10-11).

## #1660: loader names in string text

AFP-V0-386 counted any `global` or `globalThis` token in the config as a writer. That included a
project or dependency named `'global-setup'`, so a config whose executablePath read a recorded
variable still kept the EXECUTION unknown. The writer now lexes the config with the
`playwrightLex` code mask that the tag-closure check already uses. A loader name counts in code,
in a template substitution, or as a string literal used as a computed member key. Inside other
string, template or regex text it is data. That exemption applies only while the config tokenizes
and has no reflective channel. Otherwise every occurrence still counts, as before. The detail now
names the matched name and its `CONFIG:LINE` site. `globalSetup` is unchanged.

## #1660: tsconfig aliases in the tag closure

The AFP-V0-409 closure followed only relative imports. So a spec importing `lib/fixtures` through
tsconfig `paths` or `baseUrl` was never verified. The closure now resolves a bare specifier
through the nearest bound `tsconfig.json`, using `parseTypeScriptAliases`, `nearestTypeScriptAliases`
and `typeScriptAliases.resolve`, the same resolver the dependency graph uses (TJAA-V0-012/037).

- Only a target that is one bound repository file is followed.
- An invalid tsconfig fails closed. That covers unreadable, malformed and `extends` files.
- An ambiguous or missing target fails closed.
- A config naming `tsconfig` fails closed, because Playwright may read another file.

The unverified-omission detail now appends the first stop for the first unit: the file and the
specifier, or the non-literal construct. To avoid a receipt or profile change, the stop is
recomputed at plan time from the bound sources.

## #1663: template and Rake suffixes

The fix is a fixed allowlist, not a repository-declared suffix map.

- `.rake` joins the Ruby row. It uses the Ruby extractor, `#` markers and the Ruby region language.
- `.erb`, `.html`, `.htm`, `.haml`, `.slim`, `.hbs`, `.vue` and `.svelte` are searchable text with
  no extractor.

The allowlist follows the IDX-SNAP-V0-108 precedent. A declared map would add a project-authority
configuration surface and cache keying, which needs its own owner intent. The golden analyzer
corpus gained `lib/tasks/seed.rake` and `app/views/home.html.erb`. The output digest therefore
moved, and `script/analyzer-schema.sh bump` moved the schema to `corvint-analyzer/148`.
Template-embedded code still yields no symbols.
