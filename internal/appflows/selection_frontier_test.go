package appflows

import (
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TJAA-V0-052: a located TypeScript frontier entry (`reason:path:line` with a token, or a
// `reason:+N` count) is the same E2E frontier the profile answers itself as the bare reason.
func TestLocatedTypeScriptFrontierKeepsE2EAnswer_TJAAV0052(t *testing.T) {
	frontier := func(detail string) affected.Unknown {
		return affected.Unknown{Reason: affected.UnknownLanguageFrontier, Detail: detail}
	}
	for _, detail := range []string{
		"typescript:e2e-runtime-dependency",
		"typescript:e2e-runtime-dependency:tests/a.spec.ts:1 `@playwright/test`",
		"typescript:e2e-runtime-dependency:+4",
		"typescript:executable-config-unresolved:playwright.config.ts:3 `testDir`",
	} {
		if unbounded(frontier(detail)) {
			t.Fatalf("%q left the closure unbounded", detail)
		}
	}
	for _, detail := range []string{
		"typescript:path-alias-unresolved:tests/a.spec.ts:2 `lib/x`",
		"typescript:dynamic-import",
		"typescript:e2e-runtime-dependency-extra:x",
	} {
		if !unbounded(frontier(detail)) {
			t.Fatalf("%q was answered by the profile", detail)
		}
	}
}
