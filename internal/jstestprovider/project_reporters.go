package jstestprovider

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

// Keep-reporters mode (PWP-V0-010..013) appends the provider reporter to the
// project's own reporter list instead of replacing it. The default
// replace-only controlled config and receipt bytes are unchanged.

const (
	keepReportersOption     = ", keepReporters:true"
	projectReportersInvalid = "project-reporters-invalid"
	maxProjectReporters     = 32
)

// builtinPlaywrightReporters is the closed set Playwright resolves by name
// rather than as a module; every other name is a module path.
var builtinPlaywrightReporters = []string{"blob", "dot", "github", "html", "json", "junit", "line", "list", "null"}

var reporterDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type reportedProjectReporter struct {
	Name          string `json:"name"`
	Options       string `json:"options"`
	OptionsDigest string `json:"optionsDigest,omitempty"`
}

// keptReportersConfig is the controlled-config prelude that normalizes the
// original reporter value. Module names and the built-in reporters' output
// paths resolve from the original config directory, as Playwright would
// resolve them there. The file-writing built-ins also receive that directory
// as configDir, unless the project set one, so default and environment-named
// outputs do not land in the provider's scratch directory. Other option values
// pass through unchanged.
func keptReportersConfig() string {
	builtin, _ := json.Marshal(builtinPlaywrightReporters)
	return "const keptBuiltin = " + string(builtin) + ";\n" +
		"const keptInvalid = () => { throw new Error('" + projectReportersInvalid + "'); };\n" +
		"const keptName = name => keptBuiltin.includes(name) ? name : name.startsWith('.') || require('node:path').isAbsolute(name) ? resolve(name) : (() => { try { return require.resolve(name, {paths:[base]}); } catch { return name; } })();\n" +
		"const keptOptions = (name, options) => { if (!['blob', 'html', 'json', 'junit'].includes(name) || (options !== undefined && (options === null || typeof options !== 'object' || Array.isArray(options)))) return options; const result = {configDir: base, ...options}; for (const key of ['outputFile', 'outputFolder', 'outputDir']) if (typeof result[key] === 'string') result[key] = resolve(result[key]); return result; };\n" +
		"const keptReporters = value => (value === undefined ? [] : typeof value === 'string' ? [value] : Array.isArray(value) ? value : keptInvalid()).map(entry => { const [name, options] = typeof entry === 'string' ? [entry] : Array.isArray(entry) && entry.length >= 1 && entry.length <= 2 && typeof entry[0] === 'string' ? entry : keptInvalid(); const resolved = keptName(name); const kept = keptOptions(resolved, options); return kept === undefined ? [resolved] : [resolved, kept]; });\n"
}

// bindProjectReporters binds the reporter-observed kept entries after the
// config inputs are bound, so a module is "bound" only when its exact source
// digest is already a rechecked config input. A missing or invalid list leaves
// the entries unobserved (nil).
func bindProjectReporters(r *Receipt, reported []reportedProjectReporter) error {
	if reported == nil {
		return errors.New("project-reporters-unobserved")
	}
	binding := &ProjectReporters{Entries: make([]ProjectReporter, 0, len(reported)), Effects: "unknown"}
	if len(reported) > maxProjectReporters {
		return errors.New(projectReportersInvalid)
	}
	for _, entry := range reported {
		kept := ProjectReporter{Name: entry.Name, Module: "unknown", Options: entry.Options, OptionsDigest: entry.OptionsDigest}
		if builtinReporter(entry.Name) {
			kept.Module = "builtin"
		} else if digest := r.Identity.ConfigInputDigests[entry.Name]; digest != "" {
			kept.Module, kept.ModuleDigest = "bound", digest
		}
		binding.Entries = append(binding.Entries, kept)
	}
	candidate := *r
	external := *r.External
	external.ProjectReporters = binding
	candidate.External = &external
	if err := projectReportersShapeError(candidate); err != nil {
		return err
	}
	r.External.ProjectReporters = binding
	return nil
}

// keptConfig reports whether a retained controlled config was generated in
// keep-reporters mode: the provider reporter's closing options are fixed.
func keptConfig(override string) bool {
	return strings.HasSuffix(override, keepReportersOption+"}]]};\n")
}

func builtinReporter(name string) bool {
	for _, builtin := range builtinPlaywrightReporters {
		if name == builtin {
			return true
		}
	}
	return false
}

// projectReportersShapeError validates the closed keep-reporters binding. A
// binding must agree with the retained controlled config in both directions.
func projectReportersShapeError(r Receipt) error {
	x := r.External
	if x == nil {
		return nil
	}
	kept := keptConfig(x.ConfigOverride)
	if x.ProjectReporters == nil {
		if kept {
			return errors.New(projectReportersInvalid)
		}
		return nil
	}
	p := x.ProjectReporters
	if !kept || !isExternalProfile(r.Profile) || p.Effects != "unknown" || len(p.Entries) > maxProjectReporters {
		return errors.New(projectReportersInvalid)
	}
	if p.Entries == nil {
		// Unobserved entries are retained only beside a run-level infrastructure failure.
		if r.Infrastructure == nil {
			return errors.New(projectReportersInvalid)
		}
		return nil
	}
	for _, entry := range p.Entries {
		if entry.Name == "" || len(entry.Name) > 4096 {
			return errors.New(projectReportersInvalid)
		}
		switch entry.Module {
		case "builtin":
			if !builtinReporter(entry.Name) || entry.ModuleDigest != "" {
				return errors.New(projectReportersInvalid)
			}
		case "bound":
			if builtinReporter(entry.Name) || entry.ModuleDigest == "" || entry.ModuleDigest != r.Identity.ConfigInputDigests[entry.Name] {
				return errors.New(projectReportersInvalid)
			}
		case "unknown":
			if builtinReporter(entry.Name) || entry.ModuleDigest != "" {
				return errors.New(projectReportersInvalid)
			}
		default:
			return errors.New(projectReportersInvalid)
		}
		switch entry.Options {
		case "bound":
			if !reporterDigestPattern.MatchString(entry.OptionsDigest) {
				return errors.New(projectReportersInvalid)
			}
		case "absent", "unknown":
			if entry.OptionsDigest != "" {
				return errors.New(projectReportersInvalid)
			}
		default:
			return errors.New(projectReportersInvalid)
		}
	}
	return nil
}
