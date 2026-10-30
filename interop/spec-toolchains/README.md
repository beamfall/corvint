# Spec toolchain pins

`versions.json` names the only OpenSpec and Spec Kit releases Corvint supports (`STA-V0-001`,
`docs/specs/spec-toolchain-adapters-v0.md`); every other version is untested. The values come
from the npm registry and the GitHub release, tag and archive of `github/spec-kit`.

- `npm/` pins the OpenSpec CLI and its dependency tree with sha512 integrity. Regenerate with
  `npm install --package-lock-only --ignore-scripts` in that directory.
- `speckit/` pins `specify-cli` to the release commit's archive plus hashed dependencies, and the
  hashed `hatchling` build backend. Regenerate with
  `uv pip compile --universal --python-version 3.11 --generate-hashes --no-header requirements.in -o requirements.txt`
  (and the same for `build-requirements.in`).
- `fixtures/` are the committed intent-form fixtures the forms' tests and the conformance job read.

`make spec-toolchain-conformance` (`STA-V0-011`) installs these pins with checksum verification,
runs the upstream validators and compares Corvint's derived obligations with upstream's own view.
`script/spec-toolchain-latest.sh` (`STA-V0-012`) reports whether a newer release exists. A move to
a new release changes `versions.json`, these lock files and the spec trace in one change, and needs
a green `spec-toolchain-conformance` run.
