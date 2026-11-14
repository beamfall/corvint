// SPDX-License-Identifier: Apache-2.0
package sessiontest

import (
	"slices"
	"strings"
	"testing"
)

// JST-V1-001, JST-V1-006: Jest needs no provider. A `providers.jest`
// declaration is refused (the provider path admits only Playwright and
// Vitest, STR-V0-8701), and with no declared command a Jest instance's
// full command is the compiled-in `npx jest`, run from the instance root.
func TestJSTV1001JestNeedsNoProvider(t *testing.T) {
	got := ParseCommands([]byte(`{"profile": "corvint-test-commands/0", "providers": {"jest": ["corvint-js-test-provider", "jest"]}}`))
	if !got.Invalid || !strings.Contains(got.Unreadable, `unsupported runner "jest"`) {
		t.Fatalf("providers.jest admitted: %#v", got)
	}
	root := writeTree(t, map[string]string{"apps/web/jest.config.js": "module.exports = {}\n"})
	argv, dir, source := BuiltInDeclared().InstanceCommand("jest@apps/web", root)
	if !slices.Equal(argv, []string{"npx", "jest"}) || dir != "apps/web" || source != CommandDefault {
		t.Fatalf("jest@apps/web: %q %q %q", argv, dir, source)
	}
}
