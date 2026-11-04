// SPDX-License-Identifier: Apache-2.0

// Package jobinputs evaluates the project-owned input declarations of non-test
// CI jobs over a change and decides RUN or SKIP for each job (AFP-V0-2340 to
// AFP-V0-2346, docs/specs/affected-plan-reuse-v0.md). A job is SKIP only when
// no changed path is one of its declared inputs; any doubt, an unreadable
// declaration, a failed Go listing, an UNKNOWN plan scope for a test job,
// runs it.
package jobinputs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// Profiles.
const (
	DeclarationProfile = "corvint-ci-job-inputs/0"
	PlanProfile        = "corvint-ci-job-plan/0"
	// DeclarationPath is the default declaration, relative to the root.
	DeclarationPath = ".corvint/ci-job-inputs.json"
)

// Decisions.
const (
	DecisionRun  = "RUN"
	DecisionSkip = "SKIP"
)

// Reasons and witness kinds (AFP-V0-2342). A RUN job names the first reason
// that holds and every witness of every reason that holds, up to MaxWitnesses.
const (
	ReasonWorkflowChanged    = "WORKFLOW_CHANGED"
	ReasonDeclarationChanged = "DECLARATION_CHANGED"
	// ReasonCheckoutAttributesChanged: a changed .gitattributes, at any
	// depth, can rewrite the checkout bytes or the exported tree of any
	// declared input without changing that input's blob.
	ReasonCheckoutAttributesChanged = "CHECKOUT_ATTRIBUTES_CHANGED"
	ReasonInputPathChanged          = "INPUT_PATH_CHANGED"
	ReasonGoModuleChanged           = "GO_MODULE_CHANGED"
	ReasonGoPackageDirChanged       = "GO_PACKAGE_DIRECTORY_CHANGED"
	ReasonGoTestSelected            = "GO_TEST_SELECTED"
	ReasonPlanScopeUnknown          = "PLAN_SCOPE_UNKNOWN"
	ReasonIrregularTreeEntry        = "IRREGULAR_TREE_ENTRY"
	ReasonOversizedTreeEntry        = "OVERSIZED_TREE_ENTRY"
	ReasonTreeListingLarge          = "TREE_LISTING_LARGE"
	ReasonEvaluationError           = "EVALUATION_ERROR"
	ReasonNoDeclaredInputChange     = "NO_DECLARED_INPUT_CHANGED"
	WitnessInputsEvaluated          = "INPUTS_EVALUATED"
)

// Bounds.
const (
	MaxWitnesses     = 16
	maxDeclaration   = 1 << 20
	maxJobs          = 64
	maxEntries       = 256
	maxBlobBytes     = 128 << 20
	maxListingBytes  = 4 << 20
	commandTimeout   = 5 * time.Minute
	maxGoListOutput  = 128 << 20
	minimumPlatforms = 1
)

var goModuleFiles = []string{"go.mod", "go.sum", "go.work", "go.work.sum"}

// GoPackages declares the packages a job builds and the platforms it builds
// them for.
type GoPackages struct {
	Packages  []string `json:"packages"`
	Platforms []string `json:"platforms"`
}

// Job is one declared job.
type Job struct {
	Workflow   string      `json:"workflow"`
	Job        string      `json:"job"`
	Paths      []string    `json:"paths,omitempty"`
	GoPackages *GoPackages `json:"goPackages,omitempty"`
	GoTests    []string    `json:"goTests,omitempty"`
	// ExportedTree marks a job that exports every tracked file, so any
	// changed entry that is not a regular blob within the export's bounds,
	// or a listing near the export's bound, runs it.
	ExportedTree bool `json:"exportedTree,omitempty"`
}

// Declaration is the corvint-ci-job-inputs/0 document.
type Declaration struct {
	Profile string `json:"profile"`
	Jobs    []Job  `json:"jobs"`
}

// Parse validates a declaration strictly.
func Parse(body []byte) (Declaration, error) {
	var declaration Declaration
	if len(body) > maxDeclaration {
		return declaration, fmt.Errorf("declaration exceeds %d bytes", maxDeclaration)
	}
	if err := json.Unmarshal(body, &declaration, json.RejectUnknownMembers(true)); err != nil {
		return declaration, err
	}
	if declaration.Profile != DeclarationProfile || len(declaration.Jobs) == 0 || len(declaration.Jobs) > maxJobs {
		return declaration, fmt.Errorf("profile must be %s with 1 to %d jobs", DeclarationProfile, maxJobs)
	}
	seen := map[string]bool{}
	for _, job := range declaration.Jobs {
		key := job.Workflow + "#" + job.Job
		switch {
		case !strings.HasPrefix(job.Workflow, ".github/workflows/") || !affected.ValidRelativePath(job.Workflow):
			return declaration, fmt.Errorf("job %q: workflow %q is not a path under .github/workflows/", job.Job, job.Workflow)
		case job.Job == "" || strings.ContainsAny(job.Job, " \t\n#/"):
			return declaration, fmt.Errorf("job name %q is invalid", job.Job)
		case seen[key]:
			return declaration, fmt.Errorf("job %s is declared twice", key)
		case len(job.Paths) == 0 && job.GoPackages == nil && len(job.GoTests) == 0:
			return declaration, fmt.Errorf("job %s declares no input", key)
		case len(job.Paths) > maxEntries || len(job.GoTests) > maxEntries:
			return declaration, fmt.Errorf("job %s declares more than %d entries", key, maxEntries)
		}
		seen[key] = true
		previous := ""
		for _, entry := range job.Paths {
			if !affected.ValidReadScopeEntry(entry) || entry <= previous {
				return declaration, fmt.Errorf("job %s: path %q is not a sorted repository path or directory/", key, entry)
			}
			previous = entry
		}
		for _, pattern := range job.GoTests {
			if !validPackagePattern(pattern, false) {
				return declaration, fmt.Errorf("job %s: go test package %q is not ./DIR", key, pattern)
			}
		}
		if job.GoPackages != nil {
			if len(job.GoPackages.Packages) == 0 || len(job.GoPackages.Platforms) < minimumPlatforms {
				return declaration, fmt.Errorf("job %s: goPackages needs packages and platforms", key)
			}
			for _, pattern := range job.GoPackages.Packages {
				if !validPackagePattern(pattern, true) {
					return declaration, fmt.Errorf("job %s: go package %q is not ./DIR or ./DIR/...", key, pattern)
				}
			}
			for _, platform := range job.GoPackages.Platforms {
				goos, goarch, ok := strings.Cut(platform, "/")
				if !ok || !lowerWord(goos) || !lowerWord(goarch) {
					return declaration, fmt.Errorf("job %s: platform %q is not GOOS/GOARCH", key, platform)
				}
			}
		}
	}
	return declaration, nil
}

func lowerWord(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func validPackagePattern(pattern string, wildcard bool) bool {
	rest, ok := strings.CutPrefix(pattern, "./")
	if !ok {
		return false
	}
	if wildcard {
		if rest == "..." {
			return true
		}
		rest = strings.TrimSuffix(rest, "/...")
	}
	return affected.ValidRelativePath(rest) && !slices.Contains(strings.Split(rest, "/"), "...")
}

// TreeEntry is one HEAD tree entry.
type TreeEntry struct {
	Mode string
	Size int64
}

// Evidence is what a job plan is evaluated over. The caller fills it from
// one observation of the checkout.
type Evidence struct {
	// Changed holds the changed paths of the range and worktree.
	Changed []string
	// Tree holds HEAD's entries; ListingBytes is the size of the full
	// `git ls-tree -r -z --full-tree` listing.
	Tree         map[string]TreeEntry
	ListingBytes int
	// GoDirectories returns, for one job's goPackages, each repository
	// directory of a non-standard package any platform builds, and the
	// go.mod and go.sum paths of each such package's effective module, each
	// mapped to "IMPORTPATH PLATFORM".
	GoDirectories func(GoPackages) (map[string]string, error)
	// PlanScope and Selected come from the affected plan of the same
	// range: Selected maps a selected unit id to its witness kind.
	PlanScope string
	Selected  map[string]string
	PlanError string
	// Module is the root module path, for goTests unit ids.
	Module string
}

// Witness is one machine-checkable reason.
type Witness struct {
	Kind     string `json:"kind"`
	Path     string `json:"path,omitempty"`
	Input    string `json:"input,omitempty"`
	Package  string `json:"package,omitempty"`
	Platform string `json:"platform,omitempty"`
	Unit     string `json:"unit,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

// JobResult is one job's decision.
type JobResult struct {
	Workflow  string    `json:"workflow"`
	Job       string    `json:"job"`
	Decision  string    `json:"decision"`
	Reason    string    `json:"reason"`
	Witnesses []Witness `json:"witnesses"`
	// Omitted counts witnesses beyond MaxWitnesses.
	Omitted int `json:"omitted,omitempty"`
}

// Plan is the corvint-ci-job-plan/0 document.
type Plan struct {
	Profile           string      `json:"profile"`
	Tool              string      `json:"tool"`
	OK                bool        `json:"ok"`
	Mutates           bool        `json:"mutates"`
	Base              string      `json:"base"`
	Revision          string      `json:"revision"`
	Declaration       string      `json:"declaration"`
	DeclarationDigest string      `json:"declarationDigest,omitempty"`
	DeclarationError  string      `json:"declarationError,omitempty"`
	Changed           int         `json:"changed"`
	PlanScope         string      `json:"planScope,omitempty"`
	Jobs              []JobResult `json:"jobs"`
}

// DigestBytes names a declaration's content.
func DigestBytes(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func within(path, entry string) bool {
	if strings.HasSuffix(entry, "/") {
		return strings.HasPrefix(path, entry) || path == strings.TrimSuffix(entry, "/")
	}
	return path == entry
}

// Evaluate decides every declared job (AFP-V0-2341, AFP-V0-2342).
func Evaluate(declaration Declaration, declarationPath string, evidence Evidence) []JobResult {
	changed := affected.NormalizePaths(evidence.Changed)
	var results []JobResult
	for _, job := range declaration.Jobs {
		result := JobResult{Workflow: job.Workflow, Job: job.Job, Witnesses: []Witness{}}
		var witnesses []Witness
		reason := ""
		hold := func(kind string, witness Witness) {
			if reason == "" {
				reason = kind
			}
			witness.Kind = kind
			witnesses = append(witnesses, witness)
		}
		for _, path := range changed {
			if path == job.Workflow {
				hold(ReasonWorkflowChanged, Witness{Path: path})
			}
			if path == declarationPath {
				hold(ReasonDeclarationChanged, Witness{Path: path})
			}
			if path == ".gitattributes" || strings.HasSuffix(path, "/.gitattributes") {
				hold(ReasonCheckoutAttributesChanged, Witness{Path: path})
			}
		}
		for _, path := range changed {
			for _, entry := range job.Paths {
				if within(path, entry) {
					hold(ReasonInputPathChanged, Witness{Path: path, Input: entry})
					break
				}
			}
		}
		goDirectories := 0
		if job.GoPackages != nil || len(job.GoTests) != 0 {
			for _, path := range changed {
				if slices.Contains(goModuleFiles, path) {
					hold(ReasonGoModuleChanged, Witness{Path: path})
				}
			}
		}
		if job.GoPackages != nil {
			if evidence.GoDirectories == nil {
				hold(ReasonEvaluationError, Witness{Detail: "no Go listing"})
			} else if directories, err := evidence.GoDirectories(*job.GoPackages); err != nil {
				hold(ReasonEvaluationError, Witness{Detail: "go list: " + err.Error()})
			} else {
				goDirectories = len(directories)
				// A path anywhere below a package directory runs the job: an
				// embed pattern may name a subtree, and Go keeps every embedded
				// file inside its package directory.
				for _, path := range changed {
					// A closure module's manifest or checksum (the root's are
					// held above) governs how its packages compile.
					if base := filepath.Base(path); (base == "go.mod" || base == "go.sum") && !slices.Contains(goModuleFiles, path) {
						if owner, ok := directories[path]; ok {
							importPath, platform, _ := strings.Cut(owner, " ")
							hold(ReasonGoModuleChanged, Witness{Path: path, Package: importPath, Platform: platform})
							continue
						}
					}
					for directory := filepath.ToSlash(filepath.Dir(path)); ; directory = filepath.ToSlash(filepath.Dir(directory)) {
						if owner, ok := directories[directory]; ok {
							importPath, platform, _ := strings.Cut(owner, " ")
							hold(ReasonGoPackageDirChanged, Witness{Path: path, Package: importPath, Platform: platform})
							break
						}
						if directory == "." || directory == "/" {
							break
						}
					}
				}
			}
		}
		if len(job.GoTests) != 0 {
			switch {
			case evidence.PlanError != "":
				hold(ReasonEvaluationError, Witness{Detail: "affected plan: " + evidence.PlanError})
			case evidence.Module == "":
				hold(ReasonEvaluationError, Witness{Detail: "no root module path"})
			default:
				for _, pattern := range job.GoTests {
					unit := "go:" + evidence.Module + "/" + strings.TrimPrefix(pattern, "./")
					if kind, ok := evidence.Selected[unit]; ok {
						hold(ReasonGoTestSelected, Witness{Unit: unit, Detail: kind})
					}
				}
				if evidence.PlanScope != affected.ScopeBounded {
					hold(ReasonPlanScopeUnknown, Witness{Detail: "plan scope " + evidence.PlanScope})
				}
			}
		}
		if job.ExportedTree {
			if evidence.ListingBytes > maxListingBytes {
				hold(ReasonTreeListingLarge, Witness{Detail: fmt.Sprintf("%d listing bytes over %d", evidence.ListingBytes, maxListingBytes)})
			}
			for _, path := range changed {
				entry, present := evidence.Tree[path]
				switch {
				case !present:
				case entry.Mode != "100644" && entry.Mode != "100755":
					hold(ReasonIrregularTreeEntry, Witness{Path: path, Detail: "mode " + entry.Mode})
				case entry.Size > maxBlobBytes:
					hold(ReasonOversizedTreeEntry, Witness{Path: path, Detail: fmt.Sprintf("%d bytes", entry.Size)})
				}
			}
		}
		if reason == "" {
			result.Decision, result.Reason = DecisionSkip, ReasonNoDeclaredInputChange
			result.Witnesses = []Witness{{Kind: WitnessInputsEvaluated, Detail: fmt.Sprintf("changed=%d paths=%d goPackageDirectories=%d goTests=%d exportedTree=%t", len(changed), len(job.Paths), goDirectories, len(job.GoTests), job.ExportedTree)}}
		} else {
			result.Decision, result.Reason = DecisionRun, reason
			if len(witnesses) > MaxWitnesses {
				result.Omitted = len(witnesses) - MaxWitnesses
				witnesses = witnesses[:MaxWitnesses]
			}
			result.Witnesses = witnesses
		}
		results = append(results, result)
	}
	return results
}

// GoDirectories lists, for every platform, the packages the patterns build
// and maps each repository directory, and the go.mod and adjacent go.sum of
// each package's effective module, to "IMPORTPATH PLATFORM" (the first
// platform that builds it). A package outside root that is not standard, or
// any listing error, is an error. So is a non-standard package with cgo or
// assembly sources (cgo-input, assembly-input), whose includes may name
// files outside every listed directory, and one whose module metadata is
// missing or whose module manifest is outside root or not its directory's
// go.mod (module-input): those closures are not bounded, so the job runs.
func GoDirectories(ctx context.Context, root, goExecutable string, env []string, packages GoPackages) (map[string]string, error) {
	if goExecutable == "" {
		var err error
		if goExecutable, err = exec.LookPath("go"); err != nil {
			return nil, err
		}
	}
	if env == nil {
		env = os.Environ()
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	directories := map[string]string{}
	platforms := append([]string{}, packages.Platforms...)
	sort.Strings(platforms)
	// Each platform is listed with cgo off and on, so a file or import
	// either setting selects is in the union whichever the job builds with.
	type variant struct{ platform, cgo string }
	var variants []variant
	for _, platform := range platforms {
		variants = append(variants, variant{platform, "0"}, variant{platform, "1"})
	}
	for _, v := range variants {
		platform := v.platform
		goos, goarch, _ := strings.Cut(platform, "/")
		listEnv := []string{}
		for _, entry := range env {
			name, _, _ := strings.Cut(entry, "=")
			if name == "GOOS" || name == "GOARCH" || name == "GOPROXY" || name == "CGO_ENABLED" {
				continue
			}
			listEnv = append(listEnv, entry)
		}
		listEnv = append(listEnv, "GOOS="+goos, "GOARCH="+goarch, "GOPROXY=off", "CGO_ENABLED="+v.cgo)
		ctx, cancel := context.WithTimeout(ctx, commandTimeout)
		command := exec.CommandContext(ctx, goExecutable, append([]string{"list", "-e", "-deps", "-json=ImportPath,Dir,Standard,Incomplete,Error,DepsErrors,Module,CgoFiles,SFiles"}, packages.Packages...)...)
		command.Dir = resolved
		command.Env = listEnv
		var stdout, stderr bytes.Buffer
		command.Stdout, command.Stderr = &stdout, &stderr
		err := command.Run()
		cancel()
		if err != nil {
			return nil, fmt.Errorf("%s: %w: %s", platform, err, strings.TrimSpace(stderr.String()))
		}
		if stdout.Len() > maxGoListOutput {
			return nil, fmt.Errorf("%s: go list output exceeds %d bytes", platform, maxGoListOutput)
		}
		listed, err := decodeList(stdout.Bytes())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", platform, err)
		}
		for _, p := range listed {
			switch {
			case p.Error != nil:
				return nil, fmt.Errorf("%s: %s: %s", platform, p.ImportPath, p.Error.Err)
			case len(p.DepsErrors) != 0 || p.Incomplete:
				return nil, fmt.Errorf("%s: %s: incomplete listing", platform, p.ImportPath)
			case p.Standard:
				continue
			}
			relative, ok := inside(resolved, p.Dir)
			if !ok {
				return nil, fmt.Errorf("%s: %s is outside the repository", platform, p.ImportPath)
			}
			switch {
			case len(p.CgoFiles) != 0:
				return nil, fmt.Errorf("%s: cgo-input: %s uses cgo, whose includes go list does not name", platform, p.ImportPath)
			case len(p.SFiles) != 0:
				return nil, fmt.Errorf("%s: assembly-input: %s uses assembly, whose includes go list does not name", platform, p.ImportPath)
			case p.Module == nil:
				return nil, fmt.Errorf("%s: module-input: %s has no module metadata", platform, p.ImportPath)
			}
			moduleDirectory, directoryOK := inside(resolved, p.Module.Dir)
			manifest, manifestOK := inside(resolved, p.Module.GoMod)
			if !directoryOK || !manifestOK || manifest != path.Join(moduleDirectory, "go.mod") {
				return nil, fmt.Errorf("%s: module-input: %s has unmodeled module inputs", platform, p.ImportPath)
			}
			owner := p.ImportPath + " " + platform
			for _, key := range []string{relative, manifest, path.Join(moduleDirectory, "go.sum")} {
				if _, seen := directories[key]; !seen {
					directories[key] = owner
				}
			}
		}
	}
	return directories, nil
}

type listedPackage struct {
	ImportPath string
	Dir        string
	Standard   bool
	Incomplete bool
	Error      *struct{ Err string }
	DepsErrors []*struct{ Err string }
	Module     *struct{ Dir, GoMod string }
	CgoFiles   []string
	SFiles     []string
}

// inside returns target relative to root in slash form, or false when it is
// empty or outside root.
func inside(root, target string) (string, bool) {
	if target == "" {
		return "", false
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

func decodeList(body []byte) ([]listedPackage, error) {
	var listed []listedPackage
	decoder := jsontext.NewDecoder(bytes.NewReader(body))
	for {
		var p listedPackage
		if err := json.UnmarshalDecode(decoder, &p); err != nil {
			if errors.Is(err, io.EOF) {
				return listed, nil
			}
			return nil, err
		}
		listed = append(listed, p)
	}
}
