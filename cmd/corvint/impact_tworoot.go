package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"runtime"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/appmap"
	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/rootalias"
)

// impactTwoRootSchema is the document `impact --app-root DIR --test-root ALIAS=DIR` writes
// (ANL-V0-064).
const impactTwoRootSchema = "impact-two-root/0"

// impactTwoRootReport is the two-root impact answer: the test-repository spec files an
// application diff reaches, with every binding it was computed against. Verdict is
// full-suite-required, with Reasons and Unknown lines, whenever the report cannot show the
// selection covers the change; a change never yields an empty targeted selection (ANL-V0-064,
// ANL-V0-065).
type impactTwoRootReport struct {
	Schema          string                  `json:"schema"`
	App             string                  `json:"app"`
	Authority       string                  `json:"authority"`
	AppRoot         impactTwoRootAppBinding `json:"app_root"`
	TestRoot        appmap.RootPin          `json:"test_root"`
	MapDigest       string                  `json:"map_digest"`
	ImpactDigest    string                  `json:"impact_digest"`
	ChangedPaths    int                     `json:"changed_paths"`
	IgnoredPaths    int                     `json:"ignored_paths"`
	ReportedVerdict string                  `json:"reported_verdict"`
	Verdict         string                  `json:"verdict"`
	Reasons         []string                `json:"reasons"`
	SpecFiles       []string                `json:"spec_files"`
	TestKeys        []string                `json:"test_keys"`
	Unknown         []string                `json:"unknown"`
}

// impactTwoRootAppBinding names the application range the diff was taken over. Stated marks an
// --at revision read from committed objects only; at HEAD, Dirty counts uncommitted application
// paths the committed diff cannot have seen.
type impactTwoRootAppBinding struct {
	Base   string `json:"base"`
	Head   string `json:"head"`
	Stated bool   `json:"stated,omitempty"`
	Dirty  int    `json:"dirty_paths"`
}

// runEvalOrTwoRootImpact routes `impact --app-root ... --test-root ...` to the two-root impact
// (ANL-V0-064) and every other invocation to runEvalInvocation unchanged.
func runEvalOrTwoRootImpact(ctx context.Context, arguments []string, stdout, stderr io.Writer) (int, bool) {
	args, ok := twoRootImpactArguments(arguments)
	if !ok {
		return runEvalInvocation(ctx, arguments, stdout, stderr)
	}
	if err := runTwoRootImpact(ctx, args, runtime.GOOS, stdout); err != nil {
		emitError(stderr, err)
		return 2, true
	}
	return 0, true
}

// twoRootImpactArguments returns the impact arguments when the invocation is `[--root V] impact`
// naming --app-root or --test-root before any `--`; a --root is kept so it refuses typed.
func twoRootImpactArguments(arguments []string) ([]string, bool) {
	index := 0
	if index < len(arguments) && arguments[index] == "--root" {
		index += 2
	} else if index < len(arguments) && strings.HasPrefix(arguments[index], "--root=") {
		index++
	}
	if index >= len(arguments) || arguments[index] != "impact" {
		return nil, false
	}
	for _, argument := range arguments[index+1:] {
		if argument == "--" {
			break
		}
		if name, _, _ := strings.Cut(argument, "="); name == "--app-root" || name == "--test-root" {
			rest := append([]string(nil), arguments[index+1:]...)
			if index > 0 {
				rest = append(rest, "--root")
			}
			return rest, true
		}
	}
	return nil, false
}

func twoRootImpactLayout(message string) error {
	return &gokernel.Error{Code: "unsupported-impact-layout", Message: message}
}

const twoRootImpactUsage = "two-root impact requires --app-root DIR, --test-root ALIAS=ABSOLUTE_ROOT, --manifest FILE and --base COMMIT; " +
	"optional --manifest-repo ALIAS, --at REV, --test-at REV and --ledger FILE; it takes no --root, --revision or PATH"

// runTwoRootImpact builds the application map with the tests read from the aliased test root,
// computes the impact of the application diff base..at, and writes impact-two-root/0. It reads
// committed objects only and writes nothing else (ANL-V0-064..067).
func runTwoRootImpact(ctx context.Context, args []string, platform string, out io.Writer) error {
	if !nativePlatformQualified(platform) {
		return impactPlatformRefusal(platform)
	}
	f := flag.NewFlagSet("impact", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	appRoot := f.String("app-root", "", "application checkout the diff is taken in")
	testRoot := f.String("test-root", "", "ALIAS=ABSOLUTE_ROOT of the test repository")
	manifest := f.String("manifest", "", "manifest path")
	manifestRepo := f.String("manifest-repo", "", "alias of the root holding the manifest")
	base := f.String("base", "", "application base commit")
	at := f.String("at", "", "application revision read in place of HEAD")
	testAt := f.String("test-at", "", "test revision read in place of the test root's HEAD")
	ledger := f.String("ledger", "", "application-edge-ledger/0 file")
	f.Bool("root", false, "refused: the roots are --app-root and --test-root")
	f.Bool("revision", false, "refused: the application revision is --at")
	if err := f.Parse(args); err != nil || f.NArg() != 0 || isFlagSet(f, "root") || isFlagSet(f, "revision") {
		return twoRootImpactLayout(twoRootImpactUsage)
	}
	alias, testDir, aliased := rootalias.Split(*testRoot)
	if *appRoot == "" || !aliased || testDir == "" || *manifest == "" || *base == "" ||
		(*manifestRepo != "" && *manifestRepo != alias) {
		return twoRootImpactLayout(twoRootImpactUsage)
	}
	appDir, appHead, reason := appflows.OpenCheckout(ctx, ".", *appRoot)
	if reason != "" {
		return &gokernel.Error{Code: "appmap-root-unavailable", Message: "--app-root: " + reason}
	}
	// --at is resolved once, so the map and the diff read the same commit (ANL-V0-066).
	revision := "HEAD"
	var dirty, testDirty []string
	if *at != "" {
		var err error
		if revision, err = appflows.ResolveRevision(ctx, appDir, *at); err != nil {
			return &gokernel.Error{Code: "appmap-invalid-revision", Message: "--at " + *at + " is not a commit of --app-root"}
		}
	} else {
		var err error
		if dirty, err = affected.DirtyPaths(ctx, gitstatus.Executable(), appDir); err != nil {
			return &gokernel.Error{Code: "appmap-root-unavailable", Message: "--app-root: worktree status is unreadable"}
		}
	}
	if *testAt == "" {
		var err error
		if testDirty, err = affected.DirtyPaths(ctx, gitstatus.Executable(), testDir); err != nil {
			return &gokernel.Error{Code: "appmap-root-unavailable", Message: "--test-root: worktree status is unreadable"}
		}
	}
	roots := appmap.Roots{Repos: map[string]string{alias: testDir}, ManifestRepo: *manifestRepo}
	if *testAt != "" {
		roots.Revisions = map[string]string{alias: *testAt}
	}
	m, err := appmap.BuildRoots(ctx, appDir, *manifest, revision, roots)
	if err != nil {
		return err
	}
	var l *appmap.EdgeLedger
	if *ledger != "" {
		if l, err = appmap.LoadEdgeLedger(*ledger); err != nil {
			return err
		}
	}
	data, err := appmap.Impact(ctx, m, l, *base, appmap.Options{Root: appDir, Revision: revision, Full: true})
	if err != nil {
		return err
	}
	var r appmap.ImpactReport
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	if r.TestRoot == nil || r.TestRoot.Repo != alias {
		return twoRootImpactLayout("the manifest's tests.repo must name the --test-root alias " + alias + "; for tests inside the application repository use `corvint flows appmap impact`")
	}
	if err := twoRootImpactDrift(ctx, appDir, appHead, revision, dirty, *at == "", &r, testDir, testDirty); err != nil {
		return err
	}
	report := twoRootImpactVerdict(r, len(dirty), *at != "")
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	_, err = out.Write(append(raw, '\n'))
	return err
}

// twoRootImpactVerdict derives the effective verdict: unknown or ignored application paths, an
// uncommitted application worktree, an omitted spec file, or a change that selected no spec file
// all fail closed to full-suite-required with an explicit unknown line; nothing here narrows the
// reported selection (ANL-V0-065).
func twoRootImpactVerdict(r appmap.ImpactReport, dirty int, stated bool) impactTwoRootReport {
	out := impactTwoRootReport{Schema: impactTwoRootSchema, App: r.App, Authority: appmap.AuthorityLearned,
		AppRoot:  impactTwoRootAppBinding{Base: r.Base, Head: r.Head, Stated: stated, Dirty: dirty},
		TestRoot: *r.TestRoot, MapDigest: r.MapDigest, ImpactDigest: r.Digest, ChangedPaths: r.Changed, IgnoredPaths: r.Ignored,
		ReportedVerdict: r.Verdict, Verdict: r.Verdict, Reasons: []string{}, SpecFiles: append([]string{}, r.SpecFiles...),
		TestKeys: append([]string{}, r.TestKeys...), Unknown: []string{}}
	for _, u := range r.Unknown {
		line := "UNBOUND_APPLICATION_PATH: " + u.Path + ": " + u.Reason
		if u.Detail != "" {
			line += ": " + u.Detail
		}
		out.Unknown = append(out.Unknown, line)
	}
	if len(r.Unknown) > 0 || r.Omitted["unknown"] > 0 {
		out.Reasons = append(out.Reasons, "impact-unknown-paths")
	}
	if r.Ignored > 0 {
		out.Reasons = append(out.Reasons, "impact-ignored-paths")
	}
	if dirty > 0 {
		out.Reasons = append(out.Reasons, "app-worktree-not-covered")
	}
	if r.Omitted["spec_files"] > 0 {
		out.Reasons = append(out.Reasons, "spec-files-over-bound")
	}
	if r.Changed > 0 && len(r.SpecFiles) == 0 && r.Verdict != appmap.VerdictFullSuite {
		out.Reasons = append(out.Reasons, "change-selected-no-spec-files")
	}
	if len(out.Reasons) > 0 {
		out.Verdict = appmap.VerdictFullSuite
		out.Unknown = append(out.Unknown, affectedAppNavigationUnknown+": "+r.App+": "+strings.Join(out.Reasons, ","))
	}
	return out
}

// twoRootImpactDrift reads both roots again after the report was computed: an application HEAD or
// worktree read at HEAD, or a test root HEAD or worktree read at its HEAD, that moved refuses rather
// than publish a report over inputs nobody observed together; a stated --at must be the commit both
// the map and the diff read (ANL-V0-066).
func twoRootImpactDrift(ctx context.Context, appDir, appHead, revision string, dirty []string, appAtHead bool, r *appmap.ImpactReport, testDir string, testDirty []string) error {
	drift := func(which string) error {
		return &gokernel.Error{Code: "impact-root-drift", Message: which + " changed while impact was computed"}
	}
	pin := r.TestRoot
	if !appAtHead && (r.MapRevision != revision || r.Head != revision) {
		return drift("--at " + revision)
	}
	if appAtHead {
		if r.MapRevision != appHead || r.Head != appHead {
			return drift("--app-root HEAD")
		}
		if head, err := appflows.ResolveRevision(ctx, appDir, "HEAD"); err != nil || head != appHead {
			return drift("--app-root HEAD")
		}
		if again, err := affected.DirtyPaths(ctx, gitstatus.Executable(), appDir); err != nil || !equalStringSlices(dirty, again) {
			return drift("--app-root worktree")
		}
	}
	if !pin.Stated {
		if head, err := appflows.ResolveRevision(ctx, testDir, "HEAD"); err != nil || head != pin.Revision {
			return drift("--test-root " + pin.Repo + " HEAD")
		}
		if again, err := affected.DirtyPaths(ctx, gitstatus.Executable(), testDir); err != nil || !equalStringSlices(testDirty, again) {
			return drift("--test-root " + pin.Repo + " worktree")
		}
	}
	return nil
}

func isFlagSet(f *flag.FlagSet, name string) bool {
	set := false
	f.Visit(func(fl *flag.Flag) { set = set || fl.Name == name })
	return set
}
