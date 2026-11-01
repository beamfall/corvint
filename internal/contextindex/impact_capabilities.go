package contextindex

import (
	"sort"
	"strings"
)

// ImpactCapabilitiesProfile names the machine-readable impact admission
// listing (GPK-V0-084).
const ImpactCapabilitiesProfile = "corvint-impact-capabilities/0"

// ImpactCapabilities lists what path impact admits, derived from the same
// tables the admission and rule predicates read (GPK-V0-084, GPK-V0-085):
// textSuffixes through ImpactPathAdmitted, and the GPK-V0-027 rule suffixes
// through ImpactRuleNamed, and the Swift, Kotlin and C# module-membership
// readings through ModuleRuleLanguage (NGI-V0-986). It reads no repository.
func ImpactCapabilities(defaultLimit int) map[string]any {
	ruled, unruled, caseSensitive, moduled := []string{}, []string{}, []string{}, []string{}
	for suffix := range textSuffixes {
		probe := "probe" + suffix
		if !ImpactPathAdmitted(probe) {
			continue
		}
		if ModuleRuleLanguage(probe) != "" {
			moduled = append(moduled, suffix)
		}
		if !ImpactRuleNamed(probe) {
			unruled = append(unruled, suffix)
			continue
		}
		ruled = append(ruled, suffix)
		if !ImpactRuleNamed("probe" + strings.ToUpper(suffix)) {
			caseSensitive = append(caseSensitive, suffix)
		}
	}
	sort.Strings(ruled)
	sort.Strings(unruled)
	sort.Strings(caseSensitive)
	sort.Strings(moduled)
	return map[string]any{
		"suffixes": map[string]any{
			"withReverseImportRule": ruled,
			"admittedWithoutRule":   unruled,
			"ruleCaseSensitive":     caseSensitive,
			// NGI-V0-986: admitted suffixes without a GPK-V0-027 rule whose
			// reverse references a module-membership reading computes.
			"withModuleReferenceRule":  moduled,
			"admissionCaseInsensitive": true,
			"unadmitted":               "unsupported-impact-path-suffix",
		},
		"limits": map[string]any{
			"pathsMin":     1,
			"pathsMax":     maxImpactPaths,
			"pathCharsMax": maxPathChars,
			"limitMin":     1,
			"limitMax":     maxLimit,
			"limitDefault": defaultLimit,
		},
	}
}
