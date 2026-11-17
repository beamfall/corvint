package contextindex

import (
	"github.com/Beamfall/corvint/internal/langplugin/decl"
	"github.com/Beamfall/corvint/internal/langplugin/dialect"
	"github.com/Beamfall/corvint/internal/rubydialect"
	"github.com/Beamfall/corvint/internal/shaderdialect"
	"github.com/Beamfall/corvint/internal/webdialect"
)

var dialects = dialect.NewRegistry(rubydialect.Hooks(), shaderdialect.Hooks(), webHooks())

// webHooks supplies the web lexical hooks for each plugin whose impact dialects
// run the module-import rule: the impact profile for each such dialect and the
// region normalizer for the plugin's region dialects. They stay in core
// because the web lexer is shared with core's registry, setup and evaluation
// readings (LGP-V0-027).
func webHooks() dialect.Hooks {
	hooks := dialect.Hooks{Impacts: map[string]dialect.Impact{}, Regions: map[string]dialect.Region{}}
	for _, language := range decl.Languages() {
		web := false
		for _, d := range language.Dialects {
			if d.Kind == decl.ImpactDialect && d.Rule == decl.ModuleImports {
				hooks.Impacts[d.Name] = webdialect.Impact(flowWebCodeLines)
				web = true
			}
		}
		for _, d := range language.Dialects {
			if web && d.Kind == decl.RegionDialect {
				hooks.Regions[d.Name] = dialect.Region{Normalize: webNormalizedLines}
			}
		}
	}
	return hooks
}
