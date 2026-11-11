// SPDX-License-Identifier: Apache-2.0

package noncodemap

import (
	"path"
	"strings"
)

// Never-narrow categories (NCM-V1-005). CategoryRunnerConfig is the one a
// deletion turns into a removed runner (NCM-V1-001, R32).
const (
	CategoryLockfile     = "lockfile or module file"
	CategoryToolchain    = "toolchain pin"
	CategoryCI           = "CI configuration"
	CategoryMake         = "make file"
	CategoryRunnerConfig = "runner configuration"
	CategoryRelease      = "release configuration"
	CategoryGenerator    = "generator input"
	CategoryEmbed        = "go:embed input"
	CategorySnapshot     = "snapshot update path"
	CategoryCorvint      = "Corvint selection declaration"
	CategorySecret       = "secret or environment configuration"
)

// neverBase maps a base name, matched at any depth, to its category.
var neverBase = map[string]string{
	"go.mod": CategoryLockfile, "go.sum": CategoryLockfile, "go.work": CategoryLockfile, "go.work.sum": CategoryLockfile,
	"package-lock.json": CategoryLockfile, "npm-shrinkwrap.json": CategoryLockfile, "yarn.lock": CategoryLockfile,
	"pnpm-lock.yaml": CategoryLockfile, "bun.lock": CategoryLockfile, "bun.lockb": CategoryLockfile,
	"Gemfile.lock": CategoryLockfile, "poetry.lock": CategoryLockfile, "Pipfile.lock": CategoryLockfile,
	"uv.lock": CategoryLockfile, "pdm.lock": CategoryLockfile, "Cargo.lock": CategoryLockfile, "composer.lock": CategoryLockfile,

	".go-version": CategoryToolchain, ".tool-versions": CategoryToolchain, ".nvmrc": CategoryToolchain,
	".node-version": CategoryToolchain, ".python-version": CategoryToolchain, ".ruby-version": CategoryToolchain,
	"rust-toolchain": CategoryToolchain, "rust-toolchain.toml": CategoryToolchain, "mise.toml": CategoryToolchain,
	".mise.toml": CategoryToolchain,

	"Makefile": CategoryMake, "makefile": CategoryMake, "GNUmakefile": CategoryMake,

	"package.json": CategoryRunnerConfig, "tsconfig.json": CategoryRunnerConfig, "pytest.ini": CategoryRunnerConfig,
	"pyproject.toml": CategoryRunnerConfig, "setup.cfg": CategoryRunnerConfig, "tox.ini": CategoryRunnerConfig,
	"conftest.py": CategoryRunnerConfig, "Gemfile": CategoryRunnerConfig, ".rspec": CategoryRunnerConfig,
	"Rakefile": CategoryRunnerConfig,

	".goreleaser.yml": CategoryRelease, ".goreleaser.yaml": CategoryRelease,

	"buf.yaml": CategoryGenerator, "buf.gen.yaml": CategoryGenerator, "sqlc.yaml": CategoryGenerator,
	"sqlc.yml": CategoryGenerator, "gqlgen.yml": CategoryGenerator,

	".env": CategorySecret, ".envrc": CategorySecret,
}

// neverPath maps a whole repository-relative path to its category.
var neverPath = map[string]string{
	"tools/release-binary/binaries.json": CategoryRelease,
	Path:                                 CategoryCorvint,
	".corvint/test-read-scopes.json":     CategoryCorvint,
	"config/credentials.yml.enc":         CategorySecret,
	"config/master.key":                  CategorySecret,
	"config/secrets.yml":                 CategorySecret,
}

// runnerConfigStems are the base-name stems whose every extension is runner
// configuration: `vitest.config.ts`, `jest.config.cjs` and so on.
var runnerConfigStems = []string{"vitest.config.", "vitest.workspace.", "vite.config.", "jest.config.", "playwright.config."}

// NeverNarrowed reports whether no map entry may narrow p, and why
// (NCM-V1-005). Whole paths and base names come from fixed tables; `.github/`
// and `__snapshots__` match as directories, `*.mk`, `*.proto`, `*.snap` and
// `*.golden` by extension, `tsconfig.*.json` and `.env.*` by base-name form.
// A go:embed input needs the Go index and is decided by the caller
// (Input.Embedded).
func NeverNarrowed(p string) (string, bool) {
	if category, ok := neverPath[p]; ok {
		return category, true
	}
	if strings.HasPrefix(p, ".github/") {
		return CategoryCI, true
	}
	base := path.Base(p)
	if category, ok := neverBase[base]; ok {
		return category, true
	}
	for _, stem := range runnerConfigStems {
		if strings.HasPrefix(base, stem) && len(base) > len(stem) {
			return CategoryRunnerConfig, true
		}
	}
	switch {
	case strings.HasPrefix(base, "tsconfig.") && strings.HasSuffix(base, ".json"):
		return CategoryRunnerConfig, true
	case strings.HasPrefix(base, ".env."):
		return CategorySecret, true
	}
	switch path.Ext(base) {
	case ".mk":
		return CategoryMake, true
	case ".proto":
		return CategoryGenerator, true
	case ".snap", ".golden":
		return CategorySnapshot, true
	}
	if p == "__snapshots__" || strings.HasPrefix(p, "__snapshots__/") || strings.Contains(p, "/__snapshots__/") {
		return CategorySnapshot, true
	}
	return "", false
}
