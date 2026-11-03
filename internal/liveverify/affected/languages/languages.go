// Package languages lists every affected-selection language provider, so each caller that builds an
// impact graph walks the same languages. The list is read from the language plugin manifest
// (docs/specs/language-plugins-v0.md, LGP-V0-005).
package languages

import (
	"github.com/Beamfall/corvint/internal/langplugin"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// All returns a fresh provider for every supported language.
func All() []affected.Language { return langplugin.Providers() }
