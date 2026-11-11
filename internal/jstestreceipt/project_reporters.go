package jstestreceipt

import (
	"errors"
	"regexp"
	"strings"
)

const (
	KeepReportersOption     = ", keepReporters:true"
	ProjectReportersInvalid = "project-reporters-invalid"
	MaxProjectReporters     = 32
	// KeptReportersAfterProvider places the kept entries after the provider
	// reporter (PWP-V0-014), so the provider observes each callback first.
	KeptReportersAfterProvider = ", ...keptReporters(original.reporter)"
	// LegacyKeptConfigSuffix closes the retired provider-last form
	// (PWP-V0-010 as first proposed); its retained receipts stay readable and
	// never project passing.
	LegacyKeptConfigSuffix = KeepReportersOption + "}]]};\n"
	KeptConfigSuffix       = KeepReportersOption + "}]" + KeptReportersAfterProvider + "]};\n"
)

// BuiltinPlaywrightReporters is the closed set Playwright resolves by name
// rather than as a module; every other name is a module path.
var BuiltinPlaywrightReporters = []string{"blob", "dot", "github", "html", "json", "junit", "line", "list", "null"}

var ReporterDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// KeptConfig reports whether a retained controlled config was generated in
// keep-reporters mode, in either order: the closing bytes are fixed.
func KeptConfig(override string) bool {
	return ProviderFirstConfig(override) || strings.HasSuffix(override, LegacyKeptConfigSuffix)
}

// ProviderFirstConfig reports the keep-reporters config whose provider
// reporter runs before every kept entry (PWP-V0-014).
func ProviderFirstConfig(override string) bool {
	return strings.HasSuffix(override, KeptConfigSuffix)
}

func BuiltinReporter(name string) bool {
	for _, builtin := range BuiltinPlaywrightReporters {
		if name == builtin {
			return true
		}
	}
	return false
}

// ProjectReportersShapeError validates the closed keep-reporters binding. A
// binding must agree with the retained controlled config in both directions.
func ProjectReportersShapeError(r Receipt) error {
	x := r.External
	if x == nil {
		return nil
	}
	kept := KeptConfig(x.ConfigOverride)
	if x.ProjectReporters == nil {
		if kept {
			return errors.New(ProjectReportersInvalid)
		}
		return nil
	}
	p := x.ProjectReporters
	if !kept || !IsExternalProfile(r.Profile) || p.Effects != "unknown" || len(p.Entries) > MaxProjectReporters {
		return errors.New(ProjectReportersInvalid)
	}
	// A carried qualification is a closed qualified record and only beside the
	// provider-first config (PWP-V0-016); whether it matches is a projection
	// question, never a shape question.
	if q := p.Qualification; q != nil && (q.Verdict != KeepReportersQualified || KeepReportersQualificationError(*q) != nil || !ProviderFirstConfig(x.ConfigOverride)) {
		return errors.New(ProjectReportersInvalid)
	}
	if p.Entries == nil {
		// Unobserved entries are retained only beside a run-level infrastructure failure.
		if r.Infrastructure == nil {
			return errors.New(ProjectReportersInvalid)
		}
		return nil
	}
	for _, entry := range p.Entries {
		if !validProjectReporter(entry) || (entry.Module == "bound" && entry.ModuleDigest != r.Identity.ConfigInputDigests[entry.Name]) {
			return errors.New(ProjectReportersInvalid)
		}
	}
	return nil
}

// validProjectReporter checks one entry's closed shape. A bound module's digest
// must additionally equal its config input where the receipt binds one.
func validProjectReporter(entry ProjectReporter) bool {
	if entry.Name == "" || len(entry.Name) > 4096 {
		return false
	}
	switch entry.Module {
	case "builtin":
		if !BuiltinReporter(entry.Name) || entry.ModuleDigest != "" {
			return false
		}
	case "bound":
		if BuiltinReporter(entry.Name) || !ReporterDigestPattern.MatchString(entry.ModuleDigest) {
			return false
		}
	case "unknown":
		if BuiltinReporter(entry.Name) || entry.ModuleDigest != "" {
			return false
		}
	default:
		return false
	}
	switch entry.Options {
	case "bound":
		return ReporterDigestPattern.MatchString(entry.OptionsDigest)
	case "absent", "unknown":
		return entry.OptionsDigest == ""
	}
	return false
}
