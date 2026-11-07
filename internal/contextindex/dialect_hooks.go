package contextindex

import (
	"github.com/Beamfall/corvint/internal/langplugin/decl"
	"github.com/Beamfall/corvint/internal/langplugin/dialect"
	"github.com/Beamfall/corvint/internal/webdialect"
	// The Ruby plugin registers its impact and region dialects on link; every
	// binary that links this package links it (LGP-V0-025).
	_ "github.com/Beamfall/corvint/internal/rubydialect"
)

// init registers the web lexical hooks for each plugin whose impact dialects
// run the module-import rule: the impact profile for each such dialect and the
// region normalizer for the plugin's region dialects. They stay in core
// because the web lexer is shared with core's registry, setup and evaluation
// readings (LGP-V0-027).
func init() {
	for _, language := range decl.Languages() {
		web := false
		for _, d := range language.Dialects {
			if d.Kind == decl.ImpactDialect && d.Rule == decl.ModuleImports {
				dialect.RegisterImpact(d.Name, webdialect.Impact(flowWebCodeLines))
				web = true
			}
		}
		for _, d := range language.Dialects {
			if web && d.Kind == decl.RegionDialect {
				dialect.RegisterRegion(d.Name, dialect.Region{Normalize: webNormalizedLines})
			}
		}
	}
}
