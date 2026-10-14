package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/publish"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// ReadOptions configure status, verify, and report.
type ReadOptions struct {
	MapPath      string
	PatchGiven   bool // whether --patch appeared at all; cem/1.0 refuses it
	ExpectedBase string
	Target       string
	Limits       PolicyLimits
	Output       string // report only
	Format       string // report only: empty/markdown publishes; json is read-only
	OCMPath      string // optional explicitly supplied obligation map
	OCMGiven     bool   // whether --ocm appeared at all, even empty
	// ArtifactRoot is the explicit absolute artifact root a cem/1.0 status or
	// verify passes to the stable verifier (CEM-V1-017).
	ArtifactRoot   string
	ArtifactsGiven bool // whether --artifacts appeared at all
	// Citation adds the map's citation class beside a cem/1.0 status result
	// (CEM-V1-095, CEM-V1-106).
	Citation bool
	// HunkRevert runs the opt-in go test hunk-revert falsifier after a
	// cem/1.0 verify accepts and reports it beside the result (CEM-V1-088).
	HunkRevert bool
}

// ActionReportPreview renders the report without publishing it: the
// read-only MCP projection of `cem report` (MCPV0-025).
const ActionReportPreview = "report-preview"

// Read runs one of the verification-consuming commands. The action is
// "status", "verify", "report", or ActionReportPreview; rendering is not a
// second trust path. cem/1.0 is the only wire: a map that does not declare
// it is refused by the stable parser, a retired cem/0.x map with
// unsupported-spec and the migration fix (CEM-V1-200, CEM-V1-210, CEM-V1-215).
func (s *Session) Read(ctx context.Context, action string, options ReadOptions) (map[string]any, error) {
	if action == "report" && options.Format == "json" && options.Output != "" {
		return nil, invalidArguments("JSON review is read-only and does not accept --output")
	}
	if options.Format != "" && options.Format != "markdown" && options.Format != "json" {
		return nil, invalidArguments("report format must be markdown or json")
	}
	raw, err := s.readMapBytes(options.MapPath)
	if err != nil {
		return nil, err
	}
	if !wire.DeclaresStable(raw) {
		if _, err := wire.ParseStable(raw); err != nil {
			return nil, err
		}
	}
	return s.readStable(ctx, action, raw, options)
}

const reportWarning = "Paths and content-derived identifiers or digests can disclose repository information. Keep this report local unless the repository owner approves sharing it."

// reportOutput resolves where a published report goes and refuses an output
// that names the map or its sidecar. Both report profiles share it.
func (s *Session) reportOutput(options ReadOptions) (*publish.Root, string, error) {
	outputRoot, outputRelative := s.gitRoot, defaultReportRelative
	if filepath.IsAbs(options.Output) {
		root, relative, err := s.externalReportOutput(options.Output)
		if err != nil {
			return nil, "", err
		}
		outputRoot, outputRelative = root, relative
	} else if options.Output != "" {
		outputRoot, outputRelative = s.workRoot, options.Output
	}
	// Fold case: on a case-insensitive volume a case variant names the map.
	if outputRoot == s.workRoot && strings.EqualFold(outputRelative, options.MapPath) {
		return nil, "", invalidArguments("report and map paths must differ")
	}
	if outputRoot == s.workRoot {
		if err := checkMapSidecarAliasing(options.MapPath, outputRelative); err != nil {
			return nil, "", err
		}
	}
	// Folding misses the volume's own aliases (NFD/NFC, locale folds): compare
	// the existing output's file identity with the map's.
	if sameFile(filepath.Join(outputRoot.Path(), filepath.FromSlash(outputRelative)),
		filepath.Join(s.workRoot.Path(), filepath.FromSlash(options.MapPath))) {
		return nil, "", invalidArguments("report and map paths must differ")
	}
	return outputRoot, outputRelative, nil
}

// externalReportOutput admits an absolute report path whose existing parent directory lies
// outside the worktree and the Git directories. A location inside them must be named
// repository-relative, so the metadata and map-aliasing checks apply to it.
func (s *Session) externalReportOutput(output string) (*publish.Root, string, error) {
	cleaned := filepath.Clean(output)
	root, err := publish.OpenRoot(filepath.Dir(cleaned))
	if err != nil {
		return nil, "", err
	}
	protected := []string{s.workRoot.Path(), s.gitRoot.Path(), s.commonDir}
	if withinAny(root.Path(), protected) {
		return nil, "", invalidArguments("an absolute --output must lie outside the repository and its Git directory; name a location inside them repository-relative")
	}
	return root, filepath.Base(cleaned), nil
}

// withinAny reports whether dir or one of its ancestors is the same directory as any protected
// one. Comparing file identity, not spelling, also catches case and normalization aliases.
func withinAny(dir string, protected []string) bool {
	for current := dir; ; current = filepath.Dir(current) {
		if sameDirAsAny(current, protected) {
			return true
		}
		if filepath.Dir(current) == current {
			return false
		}
	}
}

func sameDirAsAny(dir string, candidates []string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}
	for _, candidate := range candidates {
		other, err := os.Stat(candidate)
		if err == nil && os.SameFile(info, other) {
			return true
		}
	}
	return false
}

// sameFile reports whether both paths name one existing file.
func sameFile(first, second string) bool {
	firstInfo, firstErr := os.Lstat(first)
	secondInfo, secondErr := os.Lstat(second)
	return firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo)
}
