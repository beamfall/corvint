package dynamic

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/rubytestprovider"
	"github.com/Beamfall/corvint/internal/secretscreen"
	tr "github.com/Beamfall/corvint/internal/testrunner"
)

// RSpec exec prefix, path mapping and the Corvint formatter
// (docs/specs/test-runner-execution-v0.md TRE-V0-081..086).
const (
	// RSpecFormatterCorvint is the one admitted Request.Formatter value: the
	// Corvint-shipped formatter of the Ruby provider (TRE-V0-084).
	RSpecFormatterCorvint = rubytestprovider.FormatterCorvint
	rspecCorvintReport    = "rspec-corvint.json"
	rspecCorvintFormatter = "corvint_rspec_formatter.rb"
	maxExecPrefixWords    = 64
	maxPathMappings       = 8
	maxInclusionFrames    = 64
)

// rspecPathMapper admits a request's ExecPrefix, PathMap and Formatter and
// returns the function that maps an absolute host path to the path the
// prefix's runtime sees (TRE-V0-081, TRE-V0-082). Without a PathMap the map
// is the identity, so the historical argv is unchanged.
func rspecPathMapper(r tr.Request) (func(string) (string, error), error) {
	if r.Formatter != "" && r.Formatter != RSpecFormatterCorvint {
		return nil, fmt.Errorf("unknown RSpec formatter %q", r.Formatter)
	}
	if len(r.ExecPrefix) > maxExecPrefixWords {
		return nil, fmt.Errorf("exec prefix exceeds %d words", maxExecPrefixWords)
	}
	for _, w := range r.ExecPrefix {
		if w == "" || len(w) > 4096 || strings.ContainsAny(w, "\x00\r\n") {
			return nil, fmt.Errorf("invalid exec prefix word")
		}
	}
	if secretscreen.MatchArgv(r.ExecPrefix) {
		return nil, fmt.Errorf("exec prefix is secret-shaped")
	}
	if len(r.PathMap) == 0 {
		return func(p string) (string, error) { return p, nil }, nil
	}
	if len(r.ExecPrefix) == 0 {
		return nil, fmt.Errorf("path map requires an exec prefix")
	}
	if len(r.PathMap) > maxPathMappings {
		return nil, fmt.Errorf("path map exceeds %d entries", maxPathMappings)
	}
	for i, m := range r.PathMap {
		if !filepath.IsAbs(m.Host) || filepath.Clean(m.Host) != m.Host || m.Host == string(filepath.Separator) ||
			!path.IsAbs(m.Container) || path.Clean(m.Container) != m.Container || m.Container == "/" ||
			strings.ContainsAny(m.Host+m.Container, "\x00\r\n") {
			return nil, fmt.Errorf("path mapping %d is not two absolute clean non-root paths", i)
		}
		for _, o := range r.PathMap[:i] {
			if _, ok := under(o.Host, m.Host, filepath.Separator); ok {
				return nil, fmt.Errorf("overlapping host path mappings")
			}
			if _, ok := under(m.Host, o.Host, filepath.Separator); ok {
				return nil, fmt.Errorf("overlapping host path mappings")
			}
			if _, ok := under(o.Container, m.Container, '/'); ok {
				return nil, fmt.Errorf("overlapping container path mappings")
			}
			if _, ok := under(m.Container, o.Container, '/'); ok {
				return nil, fmt.Errorf("overlapping container path mappings")
			}
		}
	}
	mapPath := func(p string) (string, error) {
		if filepath.Clean(p) != p {
			return "", fmt.Errorf("host path %q is not clean", p)
		}
		for _, m := range r.PathMap {
			if rest, ok := under(m.Host, p, filepath.Separator); ok {
				return path.Join(m.Container, filepath.ToSlash(rest)), nil
			}
		}
		return "", fmt.Errorf("host path %q is outside every path mapping", p)
	}
	if _, e := mapPath(filepath.Clean(r.ReportDir)); e != nil {
		return nil, fmt.Errorf("report directory: %w", e)
	}
	return mapPath, nil
}

// under reports whether p is base or lies beneath it, and the remainder.
func under(base, p string, sep byte) (string, bool) {
	if p == base {
		return "", true
	}
	if strings.HasPrefix(p, base) && len(p) > len(base) && p[len(base)] == sep {
		return p[len(base)+1:], true
	}
	return "", false
}

// buildRSpec is the fixed rspec argv. With no ExecPrefix, PathMap or
// Formatter it is byte-identical to the historical profile (TRE-V0-086).
func buildRSpec(r tr.Request, v *tr.Invocation) error {
	mapPath, err := rspecPathMapper(r)
	if err != nil {
		return err
	}
	mapped := func(p string) (string, error) {
		if !filepath.IsAbs(p) {
			return p, nil
		}
		return mapPath(p)
	}
	argv := append([]string{}, r.ExecPrefix...)
	if r.Formatter == RSpecFormatterCorvint {
		v.Files[rspecCorvintFormatter] = rubytestprovider.CorvintFormatterSource()
		v.ReportPaths = append(v.ReportPaths, rspecCorvintReport)
		formatter, e := mapPath(filepath.Join(r.ReportDir, rspecCorvintFormatter))
		if e != nil {
			return e
		}
		out, e := mapPath(filepath.Join(r.ReportDir, rspecCorvintReport))
		if e != nil {
			return e
		}
		argv = append(argv, "--require", formatter, "--format", rubytestprovider.FormatterClass, "--out", out)
	} else {
		v.ReportPaths = append(v.ReportPaths, "rspec.json")
		out, e := mapPath(filepath.Join(r.ReportDir, "rspec.json"))
		if e != nil {
			return e
		}
		argv = append(argv, "--format", "json", "--out", out)
	}
	if r.Config != "" {
		config, e := mapped(r.Config)
		if e != nil {
			return fmt.Errorf("config: %w", e)
		}
		argv = append(argv, "--options", config)
	}
	for _, s := range r.Selectors {
		sel, e := mapped(s)
		if e != nil {
			return fmt.Errorf("selector: %w", e)
		}
		argv = append(argv, sel)
	}
	v.Argv = append(v.Argv, argv...)
	return nil
}

// inclusionLocations admits one example's shared_group_inclusion_backtrace
// under the Corvint formatter (TRE-V0-085): present, bounded, and every frame
// a non-empty single-line inclusion location. The member is decoded only
// here, so the stock report's parse never depends on it (TRE-V0-086).
func inclusionLocations(id string, raw json.RawMessage) ([]string, error) {
	var frames *[]struct {
		Location *string `json:"inclusion_location"`
	}
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &frames); err != nil {
			return nil, fmt.Errorf("RSpec example %q has an invalid shared_group_inclusion_backtrace", id)
		}
	}
	if frames == nil {
		return nil, fmt.Errorf("RSpec example %q lacks shared_group_inclusion_backtrace", id)
	}
	if len(*frames) > maxInclusionFrames {
		return nil, fmt.Errorf("RSpec example %q exceeds %d inclusion frames", id, maxInclusionFrames)
	}
	var out []string
	for _, f := range *frames {
		if f.Location == nil || *f.Location == "" || len(*f.Location) > 4096 || strings.ContainsAny(*f.Location, "\x00\r\n") {
			return nil, fmt.Errorf("RSpec example %q has an invalid inclusion location", id)
		}
		out = append(out, *f.Location)
	}
	return out, nil
}
