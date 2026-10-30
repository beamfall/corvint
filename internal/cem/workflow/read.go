package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
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
	// GuardingTests runs the opt-in producer guarding-test search after a
	// cem/1.0 verify accepts and reports it beside the result (TCQ-V0-062).
	GuardingTests bool
	// MaxDeferredIntents bounds a status or report's recorded intent-scope
	// deferrals; nil is the default bound 0 (CEM-V1-645).
	MaxDeferredIntents *int
	// ScaledOperations opts a cem/1.0 status into the stable verifier's
	// map-scaled Git operation bound instead of the protocol's fixed 1,024
	// (DCW-V0-373).
	ScaledOperations bool
	// NonRegular opts a cem/1.0 status or verify into the additive
	// cem-non-regular/1 profile (CAX-V0-020): non-regular changed paths are
	// left out of the derived patch and reported beside the result as
	// explicit unknowns that status counts include.
	NonRegular bool
	// PathPolicy opts a cem/1.0 status into the additive cem-path-policy/1
	// profile (CAX-V0-060): unknown hunks a base-tree witness path policy
	// declares are classified beside the result, and declared-inert ones are
	// counted apart from unknown.
	PathPolicy bool
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
	var raw []byte
	var err error
	// A CLI report reads an out-of-tree map as status and verify do; the MCP
	// report preview stays repository-relative (CEM-V1-315, CEM-V1-640).
	if filepath.IsAbs(options.MapPath) && (action == "status" || action == "verify" || action == "report") {
		raw, options.MapPath, err = s.readExternalMap(options.MapPath)
	} else {
		raw, err = s.readMapBytes(options.MapPath)
	}
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
// that names the map or its sidecar. Both report profiles share it. An
// absolute map is the resolved out-of-tree path readExternalMap returned; the
// output is compared with it by name in its own directory and by file
// identity (CEM-V1-640).
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
	mapFile, mapRelative, mapDirectory := filepath.Join(s.workRoot.Path(), filepath.FromSlash(options.MapPath)), options.MapPath, outputRoot == s.workRoot
	if filepath.IsAbs(options.MapPath) {
		mapFile, mapRelative = options.MapPath, filepath.Base(options.MapPath)
		mapDirectory = filepath.IsAbs(options.Output) && sameDirAsAny(outputRoot.Path(), []string{filepath.Dir(options.MapPath)})
	}
	// Fold case: on a case-insensitive volume a case variant names the map.
	if mapDirectory && strings.EqualFold(outputRelative, mapRelative) {
		return nil, "", invalidArguments("report and map paths must differ")
	}
	if mapDirectory {
		if err := checkMapSidecarAliasing(mapRelative, outputRelative); err != nil {
			return nil, "", err
		}
	}
	// Folding misses the volume's own aliases (NFD/NFC, locale folds): compare
	// the existing output's file identity with the map's.
	if sameFile(filepath.Join(outputRoot.Path(), filepath.FromSlash(outputRelative)), mapFile) {
		return nil, "", invalidArguments("report and map paths must differ")
	}
	return outputRoot, outputRelative, nil
}

// readExternalMap reads the absolute --map a status or verify names outside the
// repository (CEM-V1-315). The map binds the repository by object IDs, so its
// location carries no authority; it is read once, bounded, and never written.
// The parent directory is resolved and must lie outside the worktree and its
// Git directories by file identity, so a symlinked parent cannot reach a map
// inside them; a map there must be named repository-relative (CEM-V1-316). The
// final component must be a regular file: a symlink is refused, never followed.
// It returns the bytes and the resolved absolute path they were read from.
func (s *Session) readExternalMap(mapPath string) ([]byte, string, error) {
	cleaned := filepath.Clean(mapPath)
	name := filepath.Base(cleaned)
	root, err := publish.OpenRoot(filepath.Dir(cleaned))
	if err != nil || name == string(filepath.Separator) || name == "." {
		return nil, "", cemcode.New(cemcode.MapUnavailable, "map %q has no readable parent directory", mapPath)
	}
	// The verifier, not this check, judges a repository whose Git directories
	// cannot be located; the worktree itself is always protected.
	protected := []string{s.workRoot.Path()}
	if s.openPublicationDirectories() == nil {
		protected = append(protected, s.gitRoot.Path(), s.commonDir)
	}
	if withinAny(root.Path(), protected) {
		return nil, "", invalidArguments("an absolute --map must lie outside the repository and its Git directory; name a map inside them repository-relative")
	}
	if afterExternalCheck != nil {
		afterExternalCheck(root.Path())
	}
	// Read through a held handle of the checked directory, never by pathname
	// again: a parent swapped after the check cannot redirect it (CEM-V1-244).
	raw, err := root.ReadBoundedMember(name, wire.MaxMapBytes, cemcode.MapUnavailable)
	if err != nil {
		return nil, "", err
	}
	return raw, filepath.Join(root.Path(), name), nil
}

// ReadExternalMap is readExternalMap for a consumer outside the Core read,
// such as prove --cem over an out-of-tree map (CEM-V1-641): the same
// containment, symlink and size refusals apply.
func (s *Session) ReadExternalMap(mapPath string) ([]byte, error) {
	raw, _, err := s.readExternalMap(mapPath)
	return raw, err
}

// externalReportOutput admits an absolute report path whose existing parent directory lies
// outside the worktree and the Git directories. A location inside them must be named
// repository-relative, so the metadata and map-aliasing checks apply to it. The caller
// publishes the returned member with publish.PublishMember, through a held handle of the
// checked directory (CEM-V1-244).
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
	if afterExternalCheck != nil {
		afterExternalCheck(root.Path())
	}
	return root, filepath.Base(cleaned), nil
}

// afterExternalCheck is a test seam run after an absolute map's or report's
// directory passed the location check and before it is read or written, so a
// regression can swap a parent directory in that window. Nil in production.
var afterExternalCheck func(dir string)

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
