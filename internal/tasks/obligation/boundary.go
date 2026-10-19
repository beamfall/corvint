package obligation

// The test-boundary detector (TOL-V0-079, TOL-V0-083, GitHub #1180): which
// pending obligations cite only code in repositories with no runnable test
// provider, and which out-of-scope decisions a newly appeared provider
// makes worth revisiting. It is pure: the caller lists each repository's
// tracked paths and feeds them to a BoundaryInventory.
import (
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Boundary verdicts and reasons.
const (
	BoundaryTestable       = "TESTABLE"
	BoundaryNoTestBoundary = "NO_TEST_BOUNDARY"
	BoundaryUnknown        = "UNKNOWN"

	// BoundaryNoCitations: the entry records no cites, so its boundary is
	// unknown, never assumed (product invariant 2).
	BoundaryNoCitations = "NO_CITATIONS"
	// BoundaryRepositoryUnavailable: a cite names a repository that could
	// not be listed.
	BoundaryRepositoryUnavailable = "REPOSITORY_UNAVAILABLE"
	// BoundaryNoTrackedCode: no cited path is tracked in its repository.
	BoundaryNoTrackedCode = "NO_TRACKED_CODE"
	// BoundaryNoRunnableTests: cited code is tracked, but only in
	// repositories without one runnable test file.
	BoundaryNoRunnableTests = "NO_RUNNABLE_TESTS"

	// MaxBoundaryExamples bounds the test files a repository inventory
	// names as examples.
	MaxBoundaryExamples = 5
)

// BoundaryTestKind classifies a tracked path as a runnable test file of a
// closed set of providers by naming convention, or "" when it is not one.
// It is a naming heuristic: a matching file that a runner never executes
// still counts, and a provider outside the set does not.
func BoundaryTestKind(p string) string {
	base := path.Base(p)
	dir := "/" + path.Dir(p) + "/"
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	switch ext {
	case ".go":
		if strings.HasSuffix(stem, "_test") {
			return "go"
		}
	case ".py":
		if strings.HasPrefix(stem, "test_") || strings.HasSuffix(stem, "_test") {
			return "python"
		}
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
		if strings.HasSuffix(stem, ".test") || strings.HasSuffix(stem, ".spec") {
			return "javascript"
		}
	case ".rb":
		if strings.HasSuffix(stem, "_spec") || strings.HasSuffix(stem, "_test") {
			return "ruby"
		}
	case ".java", ".kt":
		if strings.HasSuffix(stem, "Test") || strings.HasSuffix(stem, "Tests") {
			return "jvm"
		}
	case ".rs":
		if strings.Contains(dir, "/tests/") {
			return "rust"
		}
	case ".cs":
		if strings.HasSuffix(stem, "Test") || strings.HasSuffix(stem, "Tests") {
			return "dotnet"
		}
	case ".swift":
		if strings.HasSuffix(stem, "Tests") {
			return "swift"
		}
	case ".php":
		if strings.HasSuffix(stem, "Test") {
			return "php"
		}
	case ".exs":
		if strings.HasSuffix(stem, "_test") {
			return "elixir"
		}
	}
	return ""
}

// BoundaryInventory is one repository's test inventory and which cited
// paths it tracks. Repository is nil for the store's own repository.
type BoundaryInventory struct {
	Repository *string
	// Commit is the listed commit; Reason names why the repository could
	// not be listed, and the inventory is then UNKNOWN.
	Commit    string
	Reason    string
	TestFiles int64
	Kinds     map[string]int64
	Examples  []string
	// Patterns are the TOL-V0-103 declared test-file patterns that extend
	// the closed naming set; nil keeps BoundaryTestKind alone.
	Patterns []intent.ObligationTestFilePattern
	cited    map[string]bool
	tracked  map[string]bool
}

// NewBoundaryInventory starts the inventory of repo for the cited paths.
func NewBoundaryInventory(repo *string, cited []string) *BoundaryInventory {
	b := &BoundaryInventory{Repository: repo, Kinds: map[string]int64{}, cited: map[string]bool{}, tracked: map[string]bool{}}
	for _, c := range cited {
		b.cited[boundaryCitePath(c)] = true
	}
	return b
}

// Visit records one tracked path: its test kind, and each cite it matches
// as a file or as a directory prefix. It always returns false so it can be
// passed as a keep func that retains nothing.
func (b *BoundaryInventory) Visit(p string) bool {
	if k := BoundaryTestKindWith(p, b.Patterns); k != "" {
		b.TestFiles++
		b.Kinds[k]++
		if len(b.Examples) < MaxBoundaryExamples {
			b.Examples = append(b.Examples, p)
		}
	}
	for q := p; q != "." && q != "/" && q != ""; q = path.Dir(q) {
		if b.cited[q] {
			b.tracked[q] = true
		}
	}
	return false
}

// Known reports whether the repository was listed.
func (b *BoundaryInventory) Known() bool { return b.Reason == "" }

func (b *BoundaryInventory) Value() wire.Value {
	o := wire.NewObject().Set("repository", wire.StringOrNull(b.Repository))
	if !b.Known() {
		return wire.ObjectValue(o.Set("status", wire.String(BoundaryUnknown)).Set("reason", wire.String(b.Reason)))
	}
	kinds := make([]string, 0, len(b.Kinds))
	for k := range b.Kinds {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	kv := make([]wire.Value, 0, len(kinds))
	for _, k := range kinds {
		kv = append(kv, wire.ObjectValue(wire.NewObject().Set("kind", wire.String(k)).Set("files", wire.String(string(wire.CountOf(b.Kinds[k]))))))
	}
	ex := make([]wire.Value, 0, len(b.Examples))
	for _, e := range b.Examples {
		ex = append(ex, wire.String(e))
	}
	o.Set("status", wire.String("KNOWN")).Set("commit", wire.String(b.Commit)).
		Set("testFiles", wire.String(string(wire.CountOf(b.TestFiles)))).Set("kinds", wire.Array(kv...)).Set("examples", wire.Array(ex...))
	// TOL-V0-103: an inventory classified with declared patterns says so; one
	// without keeps its exact earlier bytes.
	if len(b.Patterns) > 0 {
		o.Set("declaredPatterns", wire.String(string(wire.CountOf(int64(len(b.Patterns))))))
	}
	return wire.ObjectValue(o)
}

// BoundaryRepoKey is the inventory map key of a cite's repository: "" for
// the store's own repository, else the declared name.
func BoundaryRepoKey(repo *string) string {
	if repo == nil {
		return ""
	}
	return *repo
}

// BoundaryPending reports whether the detector covers e: every entry that
// is neither WITNESSED nor DEFERRED.
func BoundaryPending(e ticket.ObligationEntry) bool {
	return e.State != ticket.ObligationWitnessed && e.State != ticket.ObligationDeferred
}

// BoundaryVerdict classifies e against the inventories keyed by
// BoundaryRepoKey. One cite tracked in a repository with a runnable test
// makes it TESTABLE; an unlisted repository leaves it UNKNOWN; otherwise it
// is NO_TEST_BOUNDARY with NO_TRACKED_CODE or NO_RUNNABLE_TESTS.
func BoundaryVerdict(e ticket.ObligationEntry, inv map[string]*BoundaryInventory) (verdict, reason string) {
	if len(e.Cites) == 0 {
		return BoundaryUnknown, BoundaryNoCitations
	}
	unknown, tracked := false, false
	for _, c := range e.Cites {
		b := inv[BoundaryRepoKey(c.Repository)]
		if b == nil || !b.Known() {
			unknown = true
			continue
		}
		if b.tracked[boundaryCitePath(c.Path)] {
			if b.TestFiles > 0 {
				return BoundaryTestable, ""
			}
			tracked = true
		}
	}
	switch {
	case unknown:
		return BoundaryUnknown, BoundaryRepositoryUnavailable
	case tracked:
		return BoundaryNoTestBoundary, BoundaryNoRunnableTests
	}
	return BoundaryNoTestBoundary, BoundaryNoTrackedCode
}

// BoundaryCiteTracked reports whether c is tracked: true or false in a
// listed repository, ok false in an unlisted one.
func BoundaryCiteTracked(c ticket.ObligationCite, inv map[string]*BoundaryInventory) (tracked, ok bool) {
	b := inv[BoundaryRepoKey(c.Repository)]
	if b == nil || !b.Known() {
		return false, false
	}
	return b.tracked[boundaryCitePath(c.Path)], true
}

// boundaryCitePath is the key a cite matches tracked paths under: a
// directory cite may end in "/", which tracked paths never do.
func boundaryCitePath(p string) string { return strings.TrimSuffix(p, "/") }

// BoundaryRescopeCandidate reports whether e is out of scope by an
// accepted decision and now TESTABLE (TOL-V0-083), so the decision is
// worth revisiting.
func BoundaryRescopeCandidate(e ticket.ObligationEntry, verdict string) bool {
	return e.Scope != nil && e.Scope.Status == ticket.ObligationScopeAccepted && !e.Scope.Core && !e.Core && verdict == BoundaryTestable
}

// BoundaryTestKindWith is BoundaryTestKind extended by the TOL-V0-103
// declared patterns: the closed naming set answers first, then the first
// declared pattern that matches gives its kind. A pattern without "/" is
// matched against the base name, one with "/" against the whole path; "*"
// never crosses "/" (path.Match).
func BoundaryTestKindWith(p string, patterns []intent.ObligationTestFilePattern) string {
	if k := BoundaryTestKind(p); k != "" {
		return k
	}
	base := path.Base(p)
	for _, tp := range patterns {
		subject := p
		if !strings.Contains(tp.Pattern, "/") {
			subject = base
		}
		if ok, _ := path.Match(tp.Pattern, subject); ok {
			return tp.Kind
		}
	}
	return ""
}
