// Package languages lists every affected-selection language provider, so each caller that builds an
// impact graph walks the same languages.
package languages

import (
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/dotnet"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/kotlin"
	"github.com/Beamfall/corvint/internal/liveverify/affected/python"
	"github.com/Beamfall/corvint/internal/liveverify/affected/ruby"
	"github.com/Beamfall/corvint/internal/liveverify/affected/rust"
	"github.com/Beamfall/corvint/internal/liveverify/affected/swift"
	"github.com/Beamfall/corvint/internal/liveverify/affected/typescript"
)

// All returns a fresh provider for every supported language.
func All() []affected.Language {
	return []affected.Language{
		dotnet.New(), golang.New(), kotlin.New(), python.New(),
		ruby.New(), rust.New(), swift.New(), typescript.New(),
	}
}
