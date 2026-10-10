// Package update implements the optional, operator-invoked release updater.
package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/gitstatus"
	"github.com/Beamfall/corvint/internal/procgroup"
)

const metadataLimit = 2 << 20
const archiveLimit = 120 << 20
const extractedLimit = 300 << 20

type Config struct {
	Component, BinDir, StateDir string
	AllowNetwork                bool
	// RefreshRoots are checkouts whose index snapshot a Core switch refreshes
	// unconditionally; WorkDir's checkout is refreshed only when it already
	// has a snapshot store (UPD-V0-008).
	RefreshRoots []string
	WorkDir      string
}
type Result struct {
	Component        string `json:"component"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	InstalledBuild   int    `json:"installedBuild,omitempty"`
	AvailableTag     string `json:"availableTag,omitempty"`
	// AvailableBuild is the build a Tasks release declares in its archive
	// manifest. Relation compares installed with available on RelationBasis
	// (build or version), or is UNKNOWN with RelationReason (UPD-V0-009).
	AvailableBuild    int    `json:"availableBuild,omitempty"`
	Relation          string `json:"relation,omitempty"`
	RelationBasis     string `json:"relationBasis,omitempty"`
	RelationReason    string `json:"relationReason,omitempty"`
	Freshness         string `json:"freshness"`
	Action            string `json:"action"`
	Qualification     string `json:"qualification"`
	PublisherIdentity string `json:"publisherIdentity"`
	Receipt           string `json:"receipt,omitempty"`
	// Removed names every state path apply or rollback deleted under the
	// retention bound, and Left every transaction entry it kept because it
	// could not classify or remove it, as "path: reason" (proposed UPD-V0-007).
	Removed []string `json:"removed,omitempty"`
	Left    []string `json:"left,omitempty"`
	// IndexRefresh is the outcome of each snapshot refresh after a Core
	// switch (UPD-V0-008).
	IndexRefresh []IndexRefresh `json:"indexRefresh,omitempty"`
}

// IndexRefresh is one checkout's snapshot refresh: State is built, fresh,
// skipped or failed, and Reason says why for the last two.
type IndexRefresh struct {
	Root   string `json:"root"`
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}
type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}
type release struct {
	Tag        string    `json:"tag_name"`
	Body       string    `json:"body"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Published  time.Time `json:"published_at"`
	Assets     []asset   `json:"assets"`
}
type receipt struct{ Component, Destination, PreviousDigest, CandidateDigest string }
type engine struct {
	config   Config
	client   *http.Client
	api      string
	platform string
	inspect  func(string, string) error
	// refreshCtx bounds the post-switch snapshot refresh; nil means none.
	refreshCtx context.Context
}

func Run(ctx context.Context, command string, c Config) (Result, error) {
	e := engine{config: c, client: &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("redirect limit")
		}
		return allowedURL(req.URL)
	}}, api: "https://api.github.com/repos/beamfall/corvint/releases", platform: runtime.GOOS + "_" + runtime.GOARCH}
	return e.execute(ctx, command)
}

// execute runs command under the updater's own bound; a Core switch's
// snapshot refresh keeps the caller's context instead (UPD-V0-008).
func (e engine) execute(ctx context.Context, command string) (Result, error) {
	e.refreshCtx = ctx
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	return e.run(runCtx, command)
}

// switched refreshes index snapshots after a successful Core switch. run
// calls it while it still holds the binary-directory lock, so every refresh
// runs the binary this run installed (UPD-V0-008).
func (e engine) switched(r *Result) {
	c := e.config
	if c.Component != "core" {
		return
	}
	ctx := e.refreshCtx
	if ctx == nil {
		ctx = context.Background()
	}
	r.IndexRefresh = refreshIndexes(ctx, filepath.Join(c.BinDir, "corvint"), c.RefreshRoots, c.WorkDir)
}

// refreshTimeout bounds one checkout's foreground `index --if-stale`.
const refreshTimeout = 2 * time.Minute

// refreshIndexes runs the switched Core binary's `index --if-stale` in the
// foreground for each explicit root and for workDir's checkout when it is
// already indexed. A snapshot is keyed by the executable's digest, so every
// switch strands the old ones, and hooks may not rebuild them (IDX-SNAP-V0-012).
// A failed refresh never fails the switch; its outcome is reported (UPD-V0-008).
func refreshIndexes(ctx context.Context, binary string, explicit []string, workDir string) []IndexRefresh {
	var out []IndexRefresh
	seen := map[string]bool{}
	add := func(root string, requireStore bool) {
		key := root
		if resolved, err := filepath.EvalSymlinks(root); err == nil {
			key = resolved
		}
		if seen[key] {
			return
		}
		seen[key] = true
		if requireStore && !indexed(root) {
			out = append(out, IndexRefresh{Root: root, State: "skipped", Reason: "no index snapshot store"})
			return
		}
		out = append(out, refreshIndex(ctx, binary, root))
	}
	for _, root := range explicit {
		abs, err := filepath.Abs(root)
		if err != nil {
			out = append(out, IndexRefresh{Root: root, State: "failed", Reason: err.Error()})
			continue
		}
		add(abs, false)
	}
	if root := checkoutRoot(workDir); root != "" {
		add(root, true)
	}
	return out
}

// checkoutRoot is the nearest directory at or above dir holding a `.git`
// entry, or empty when there is none.
func checkoutRoot(dir string) string {
	if dir == "" {
		return ""
	}
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// indexed says whether root already has a snapshot store: the shared one under
// the Git common directory or the worktree fallback (IDX-SNAP-V0-025).
func indexed(root string) bool {
	stores := []string{filepath.Join(root, ".corvint", "index")}
	if common, err := gitstatus.CommonDirectory(root); err == nil {
		stores = append(stores, filepath.Join(common, "corvint", "index"))
	}
	for _, store := range stores {
		if info, err := os.Lstat(store); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func refreshIndex(ctx context.Context, binary, root string) IndexRefresh {
	o := procgroup.Run(ctx, procgroup.Spec{Argv: []string{binary, "--root", root, "index", "--if-stale"}, Dir: root, Env: os.Environ(), Timeout: refreshTimeout, ShutdownTimeout: 2 * time.Second, OutputLimit: 64 << 10, ObserveDescendants: true})
	if o.Err != nil || o.ExitStatus != 0 || !o.OwnedProcessGroupCleanup {
		reason := fmt.Sprintf("exit %d", o.ExitStatus)
		if o.Err != nil {
			reason = o.Err.Error()
		} else if line, _, _ := strings.Cut(strings.TrimSpace(string(o.Stderr)), "\n"); line != "" {
			reason += ": " + line
		}
		return IndexRefresh{Root: root, State: "failed", Reason: reason}
	}
	var receipt struct {
		State   string `json:"state"`
		Mutates bool   `json:"mutates"`
	}
	if err := json.Unmarshal(o.Stdout, &receipt); err != nil {
		return IndexRefresh{Root: root, State: "failed", Reason: "unreadable index receipt"}
	}
	if receipt.State == "fresh" && !receipt.Mutates {
		return IndexRefresh{Root: root, State: "fresh"}
	}
	return IndexRefresh{Root: root, State: "built"}
}
func (e engine) run(ctx context.Context, command string) (r Result, err error) {
	c := e.config
	r = Result{Component: c.Component, Freshness: "UNKNOWN", Action: command, Qualification: "release evidence only; no broad qualification; atomic visibility, no power-loss durability guarantee", PublisherIdentity: "NOT_VERIFIED"}
	name, err := binaryName(c.Component)
	if err != nil {
		return r, err
	}
	if command != "check" && command != "apply" && command != "rollback" {
		return r, errors.New("expected check, apply or rollback")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return r, errors.New("unsupported host")
	}
	if err = validateDirs(c.BinDir, c.StateDir); err != nil {
		return r, err
	}
	if command != "rollback" && !c.AllowNetwork {
		return r, errors.New("network permission required: --allow-network; freshness UNKNOWN")
	}
	destination := filepath.Join(c.BinDir, name)
	var unlock func()
	if command != "check" {
		if err = os.MkdirAll(c.BinDir, 0700); err != nil {
			return r, err
		}
		unlock, err = lockDirectory(c.BinDir)
		if err != nil {
			return r, err
		}
		defer unlock()
		if err = os.MkdirAll(c.StateDir, 0700); err != nil {
			return r, err
		}
		if err = privateDir(c.StateDir); err != nil {
			return r, err
		}
		// The state lock serialises updaters that share this state; the
		// incomplete sweep also requires an age no live run can reach.
		unlockState, stateErr := lockDirectory(c.StateDir)
		if stateErr != nil {
			return r, fmt.Errorf("state directory: %w", stateErr)
		}
		defer unlockState()
		r.Removed, r.Left = sweepIncomplete(c.StateDir, time.Now())
	}
	if command == "rollback" {
		r.Receipt, err = e.rollback(ctx, destination)
		if err == nil {
			e.switched(&r)
		}
		return r, err
	}
	if !c.AllowNetwork {
		return r, errors.New("network permission required: --allow-network; freshness UNKNOWN")
	}
	oldDigest, err := digest(destination)
	if err != nil {
		return r, fmt.Errorf("installed binary: %w", err)
	}
	// Check uses the existing directory and HOME: no staging or receipts are created.
	r.InstalledVersion, r.InstalledBuild, err = version(ctx, destination, "", c.Component)
	if err != nil {
		return r, err
	}
	selected, err := e.discover(ctx)
	if err != nil {
		return r, err
	}
	r.AvailableTag = selected.Tag
	if command == "check" {
		r.Freshness = "RELEASE_OBSERVED"
		e.relate(ctx, selected, &r)
		return r, nil
	}
	stage, err := os.MkdirTemp(c.StateDir, "transaction-")
	if err != nil {
		return r, err
	}
	prepared := false
	defer func() {
		if !prepared {
			os.RemoveAll(stage)
		}
	}()
	archiveName := name + "_" + e.platform + ".tar.gz"
	sums, err := e.asset(ctx, selected, "SHA256SUMS", metadataLimit)
	if err != nil {
		return r, err
	}
	archive, err := e.asset(ctx, selected, archiveName, archiveLimit)
	if err != nil {
		return r, err
	}
	entries, err := parseSums(sums)
	if err != nil {
		return r, err
	}
	expected, ok := entries[archiveName]
	if !ok || hash(archive) != expected {
		return r, errors.New("release archive checksum mismatch or missing entry")
	}
	if err = os.WriteFile(filepath.Join(stage, "release-SHA256SUMS"), sums, 0600); err != nil {
		return r, err
	}
	evidence, _ := json.MarshalIndent(selected, "", "  ")
	if err = os.WriteFile(filepath.Join(stage, "release.json"), evidence, 0600); err != nil {
		return r, err
	}
	archivePath := filepath.Join(stage, "archive.tar.gz")
	if err = os.WriteFile(archivePath, archive, 0600); err != nil {
		return r, err
	}
	extracted := filepath.Join(stage, "extracted")
	if err = os.Mkdir(extracted, 0700); err != nil {
		return r, err
	}
	if err = extract(archivePath, extracted); err != nil {
		return r, err
	}
	candidate, manifestBuild, err := verify(extracted, c.Component, e.platform)
	if err != nil {
		return r, err
	}
	inspect := e.inspect
	if inspect == nil {
		inspect = inspectPlatform
	}
	if err = inspect(candidate, e.platform); err != nil {
		return r, err
	}
	qualificationName := "verification-report.json"
	if c.Component == "tasks" {
		qualificationName = "build-verification.json"
	}
	qualification, err := e.asset(ctx, selected, qualificationName, metadataLimit)
	if err != nil {
		return r, err
	}
	if err = verifyQualification(qualification, extracted, c.Component, e.platform, hash(archive), candidate); err != nil {
		return r, err
	}
	if err = os.WriteFile(filepath.Join(stage, qualificationName), qualification, 0600); err != nil {
		return r, err
	}
	if c.Component == "core" {
		r.Qualification += "; Core source archive and release manifest: NOT_PROVIDED"
	}
	if err = os.Chmod(candidate, 0700); err != nil {
		return r, err
	}
	home := filepath.Join(stage, "smoke-home")
	if err = os.Mkdir(home, 0700); err != nil {
		return r, err
	}
	candidateVersion, build, err := version(ctx, candidate, home, c.Component)
	if err != nil {
		return r, err
	}
	if c.Component == "core" && !strings.HasPrefix(candidateVersion, "Corvint "+strings.TrimPrefix(selected.Tag, "v")+" ") {
		return r, errors.New("Core release/version identity mismatch")
	}
	if manifestBuild != 0 && manifestBuild != build {
		return r, errors.New("manifest/version build mismatch")
	}
	helpArg := "--help"
	if c.Component == "tasks" {
		helpArg = "help"
	}
	if err = smoke(ctx, candidate, home, helpArg); err != nil {
		return r, err
	}
	candidateDigest, err := digest(candidate)
	if err != nil {
		return r, err
	}
	if build < r.InstalledBuild {
		return r, errors.New("refusing downgrade")
	}
	if build == r.InstalledBuild {
		if candidateDigest != oldDigest {
			return r, errors.New("equal-build digest divergence")
		}
		current, digestErr := digest(destination)
		if digestErr != nil {
			return r, digestErr
		}
		if current != oldDigest {
			return r, errors.New("destination digest conflict before unchanged result")
		}
		if err = ctx.Err(); err != nil {
			return r, err
		}
		r.Qualification += "; downloaded evidence verified but not retained for unchanged binary"
		if c.Component == "tasks" {
			r.Qualification += "; supplied Tasks source/manifest verified but not retained"
		}
		r.Action = "unchanged"
		r.Freshness = "CURRENT"
		return r, nil
	}
	previous, err := readRegular(destination, archiveLimit)
	if err != nil {
		return r, err
	}
	if hash(previous) != oldDigest {
		return r, errors.New("installed binary changed during preparation")
	}
	if err = os.WriteFile(filepath.Join(stage, "previous"), previous, 0700); err != nil {
		return r, err
	}
	rec := receipt{c.Component, destination, oldDigest, candidateDigest}
	encoded, _ := json.MarshalIndent(rec, "", "  ")
	// The durable recovery description precedes the only destination switch. A
	// prepared receipt is intentionally usable even if no completion write runs.
	if err = os.WriteFile(filepath.Join(stage, "receipt.json"), encoded, 0600); err != nil {
		return r, err
	}
	prepared = true
	r.Qualification += "; exact release evidence retained: " + filepath.Join(stage, qualificationName)
	if c.Component == "tasks" {
		r.Qualification += "; supplied Tasks source/manifest retained: " + extracted
	}
	r.Receipt = filepath.Join(stage, "receipt.json")
	err = activate(ctx, candidate, destination, oldDigest)
	if err == nil {
		r.Action = "applied"
		r.InstalledVersion = candidateVersion
		r.InstalledBuild = build
		r.Freshness = "CURRENT"
		removed, left := retainCommitted(c.StateDir, stage, candidate, rec)
		r.Removed, r.Left = append(r.Removed, removed...), append(r.Left, left...)
		e.switched(&r)
	}
	return r, err
}

// transactionName matches the directories apply creates with
// os.MkdirTemp(state, "transaction-"); any other name is not the updater's.
var transactionName = regexp.MustCompile(`^transaction-[0-9]+$`)

// incompleteStaleAfter is how old a transaction without a receipt must be
// before it is swept. Run bounds a whole command at five minutes, so a
// directory last modified longer ago than this cannot belong to a live run,
// including one from an updater that predates the state lock.
const incompleteStaleAfter = 30 * time.Minute

// sweepIncomplete removes every updater transaction directory without
// receipt.json that is older than incompleteStaleAfter: an apply killed before
// its receipt was written, which rollback can never use, whatever component it
// was for (proposed UPD-V0-007).
func sweepIncomplete(state string, now time.Time) (removed, left []string) {
	entries, err := os.ReadDir(state)
	if err != nil {
		return nil, []string{state + ": " + err.Error()}
	}
	for _, entry := range entries {
		if !transactionName.MatchString(entry.Name()) {
			continue
		}
		dir := filepath.Join(state, entry.Name())
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() {
			left = append(left, dir+": not a real directory")
			continue
		}
		_, err = os.Lstat(filepath.Join(dir, "receipt.json"))
		if err == nil {
			continue
		}
		if !os.IsNotExist(err) {
			left = append(left, dir+": "+err.Error())
			continue
		}
		if now.Sub(info.ModTime()) < incompleteStaleAfter {
			left = append(left, dir+": incomplete transaction younger than "+incompleteStaleAfter.String())
			continue
		}
		if err = os.RemoveAll(dir); err != nil {
			left = append(left, dir+": "+err.Error())
			continue
		}
		removed = append(removed, dir)
	}
	return removed, left
}

// retainCommitted enforces the retention bound after a successful activation:
// the committed transaction drops its download, smoke home and candidate copy
// (now the installed bytes), keeping its receipt, previous executable and
// release evidence; every other transaction bound to the same component and
// destination is superseded and removed. A transaction whose receipt cannot be
// read is left and named, never guessed (proposed UPD-V0-007).
func retainCommitted(state, stage, candidate string, committed receipt) (removed, left []string) {
	for _, p := range []string{filepath.Join(stage, "archive.tar.gz"), filepath.Join(stage, "smoke-home"), candidate} {
		if _, err := os.Lstat(p); err != nil {
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			left = append(left, p+": "+err.Error())
			continue
		}
		removed = append(removed, p)
	}
	entries, err := os.ReadDir(state)
	if err != nil {
		return removed, append(left, state+": "+err.Error())
	}
	for _, entry := range entries {
		dir := filepath.Join(state, entry.Name())
		if !transactionName.MatchString(entry.Name()) || dir == stage {
			continue
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() {
			left = append(left, dir+": not a real directory")
			continue
		}
		b, err := readRegular(filepath.Join(dir, "receipt.json"), metadataLimit)
		if errors.Is(err, os.ErrNotExist) {
			continue // incomplete: sweepIncomplete owns it
		}
		if err != nil {
			left = append(left, dir+": "+err.Error())
			continue
		}
		var other receipt
		if err = json.Unmarshal(b, &other); err != nil {
			left = append(left, dir+": unreadable receipt")
			continue
		}
		if other.Component != committed.Component || other.Destination != committed.Destination {
			continue
		}
		if err = os.RemoveAll(dir); err != nil {
			left = append(left, dir+": "+err.Error())
			continue
		}
		removed = append(removed, dir)
	}
	return removed, left
}
func binaryName(component string) (string, error) {
	switch component {
	case "core":
		return "corvint", nil
	case "tasks":
		return "corvint-tasks", nil
	}
	return "", errors.New("component must be core or tasks")
}
func allowedURL(u *url.URL) error {
	if u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return errors.New("HTTPS official release URL required")
	}
	switch u.Hostname() {
	case "api.github.com", "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		return nil
	}
	return errors.New("unapproved download host")
}
func (e engine) get(ctx context.Context, address string, limit int64) ([]byte, error) {
	u, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	if err = allowedURL(u); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("release HTTP status %d", res.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if int64(len(data)) > limit {
		return nil, errors.New("download size limit exceeded")
	}
	return data, err
}
func (e engine) discover(ctx context.Context) (release, error) {
	var matches []release
	name, _ := binaryName(e.config.Component)
	for page := 1; page <= 5; page++ {
		data, err := e.get(ctx, fmt.Sprintf("%s?per_page=100&page=%d", e.api, page), metadataLimit)
		if err != nil {
			return release{}, err
		}
		var rows []release
		if err = json.Unmarshal(data, &rows); err != nil {
			return release{}, err
		}
		if len(rows) > 100 {
			return release{}, errors.New("release page exceeds bound")
		}
		for _, r := range rows {
			prefix := "v"
			if e.config.Component == "tasks" {
				prefix = "tasks-dev-"
			}
			if r.Draft || !strings.HasPrefix(r.Tag, prefix) || r.Published.IsZero() {
				continue
			}
			for _, a := range r.Assets {
				if a.Name == name+"_"+e.platform+".tar.gz" {
					matches = append(matches, r)
					break
				}
			}
		}
		if len(rows) < 100 {
			if len(matches) == 0 {
				return release{}, errors.New("no matching release; freshness UNKNOWN")
			}
			sort.Slice(matches, func(i, j int) bool { return matches[i].Published.After(matches[j].Published) })
			return matches[0], nil
		}
	}
	return release{}, errors.New("release pagination cap reached; freshness UNKNOWN")
}

// relate states how the selected release compares with the installed build
// (UPD-V0-009). A Tasks release declares its build in build-verification.json,
// bound to the archive digest in SHA256SUMS; Core release metadata declares no
// build, so Core compares the tag with the installed version by semver
// precedence. Neither is fetched past metadataLimit or written to disk.
func (e engine) relate(ctx context.Context, rel release, r *Result) {
	r.Relation = "UNKNOWN"
	var cmp int
	if e.config.Component == "tasks" {
		build, err := e.declaredTasksBuild(ctx, rel)
		if err != nil {
			r.RelationReason = err.Error()
			return
		}
		r.AvailableBuild, r.RelationBasis = build, "build"
		cmp = r.InstalledBuild - build
	} else {
		installed := semverRE.FindString(r.InstalledVersion)
		available := strings.TrimPrefix(rel.Tag, "v")
		if installed == "" || semverRE.FindString(available) != available {
			r.RelationReason = "installed version or release tag is not a semantic version"
			return
		}
		r.RelationBasis = "version"
		cmp = compareSemver(installed, available)
	}
	switch {
	case cmp < 0:
		r.Relation = "UPDATE_AVAILABLE"
	case cmp == 0:
		r.Relation = "CURRENT"
	default:
		r.Relation, r.RelationReason = "LOCAL_NEWER", "the available release is older than the installed build; apply would be a downgrade and is refused"
	}
}

func (e engine) declaredTasksBuild(ctx context.Context, rel release) (int, error) {
	sumsData, err := e.asset(ctx, rel, "SHA256SUMS", metadataLimit)
	if err != nil {
		return 0, err
	}
	sums, err := parseSums(sumsData)
	if err != nil {
		return 0, err
	}
	b, err := e.asset(ctx, rel, "build-verification.json", metadataLimit)
	if err != nil {
		return 0, err
	}
	var m struct{ Profile, Target, Build, ArchiveSha256 string }
	if err = json.Unmarshal(b, &m); err != nil {
		return 0, errors.New("unreadable Tasks build declaration")
	}
	archive := sums["corvint-tasks_"+e.platform+".tar.gz"]
	build, err := strconv.Atoi(m.Build)
	if err != nil || build <= 0 || m.Profile != "corvint-tasks-archive/0" || m.Target != strings.ReplaceAll(e.platform, "_", "/") || archive == "" || m.ArchiveSha256 != archive {
		return 0, errors.New("Tasks build declaration does not match the release archive")
	}
	return build, nil
}

var semverRE = regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?`)

// compareSemver orders two semantic versions by precedence, ignoring build
// metadata: negative when a is lower, zero when equal, positive when higher.
func compareSemver(a, b string) int {
	ac, ap, _ := strings.Cut(a, "-")
	bc, bp, _ := strings.Cut(b, "-")
	if c := compareIdentifiers(strings.Split(ac, "."), strings.Split(bc, ".")); c != 0 {
		return c
	}
	switch {
	case ap == bp:
		return 0
	case ap == "":
		return 1
	case bp == "":
		return -1
	}
	return compareIdentifiers(strings.Split(ap, "."), strings.Split(bp, "."))
}

func compareIdentifiers(a, b []string) int {
	numeric := func(s string) (int, error) {
		if strings.Trim(s, "0123456789") != "" {
			return 0, strconv.ErrSyntax
		}
		return strconv.Atoi(s)
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		an, aErr := numeric(a[i])
		bn, bErr := numeric(b[i])
		switch {
		case aErr == nil && bErr == nil && an != bn:
			return an - bn
		case aErr == nil && bErr != nil:
			return -1
		case aErr != nil && bErr == nil:
			return 1
		case aErr != nil && a[i] != b[i]:
			return strings.Compare(a[i], b[i])
		}
	}
	return len(a) - len(b)
}

func (e engine) asset(ctx context.Context, r release, name string, limit int64) ([]byte, error) {
	var found string
	for _, a := range r.Assets {
		if a.Name == name {
			if found != "" {
				return nil, errors.New("duplicate release asset")
			}
			found = a.URL
		}
	}
	if found == "" {
		return nil, fmt.Errorf("missing release asset %s", name)
	}
	return e.get(ctx, found, limit)
}
func hash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func readRegular(p string, limit int64) ([]byte, error) {
	info, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", p)
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	now, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, now) {
		return nil, errors.New("file changed while opening")
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if int64(len(b)) > limit {
		return nil, errors.New("file size limit")
	}
	return b, err
}
func digest(p string) (string, error) {
	b, err := readRegular(p, archiveLimit)
	if err != nil {
		return "", err
	}
	return hash(b), nil
}

var buildRE = regexp.MustCompile(`build[. =:]([0-9]+)`)

func version(ctx context.Context, binary, home, component string) (string, int, error) {
	arg := "--version"
	if component == "tasks" {
		arg = "version"
	}
	o := runProcess(ctx, binary, home, arg)
	if o.Err != nil || o.ExitStatus != 0 || !o.OwnedProcessGroupCleanup {
		return "", 0, fmt.Errorf("version smoke failed: %v", o.Err)
	}
	s := strings.TrimSpace(string(o.Stdout))
	if component == "tasks" {
		var err error
		if s, err = tasksVersion(s); err != nil {
			return "", 0, err
		}
	}
	m := buildRE.FindStringSubmatch(s)
	if m == nil {
		return s, 0, errors.New("unknown installed/candidate numeric build")
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return s, 0, errors.New("invalid numeric build")
	}
	if component == "core" && !strings.Contains(strings.ToLower(s), "corvint") {
		return s, 0, errors.New("unexpected Core version identity")
	}
	return s, n, nil
}

// tasksVersion reduces the `corvint-tasks version` command result to the one
// version string it reports, so the identity reads like Core's.
func tasksVersion(s string) (string, error) {
	var env struct {
		Profile string
		Items   []struct{ Version string }
	}
	if json.Unmarshal([]byte(s), &env) != nil || env.Profile != "taskman-command-result/0" || len(env.Items) != 1 || env.Items[0].Version == "" {
		return "", errors.New("unexpected Tasks version identity")
	}
	return env.Items[0].Version, nil
}
func runProcess(ctx context.Context, binary, home, arg string) procgroup.Observation {
	dir := filepath.Dir(binary)
	env := []string{"PATH=/usr/bin:/bin", "LANG=C"}
	if home != "" {
		dir = home
		env = append(env, "HOME="+home, "TMPDIR="+home, "XDG_CONFIG_HOME="+home, "XDG_CACHE_HOME="+home, "XDG_DATA_HOME="+home)
	}
	return procgroup.Run(ctx, procgroup.Spec{Argv: []string{binary, arg}, Dir: dir, Env: env, Timeout: 10 * time.Second, ShutdownTimeout: 2 * time.Second, OutputLimit: 64 << 10, ObserveDescendants: true})
}
func smoke(ctx context.Context, binary, home, arg string) error {
	o := runProcess(ctx, binary, home, arg)
	if o.Err != nil || o.ExitStatus != 0 || !o.OwnedProcessGroupCleanup {
		return fmt.Errorf("help smoke failed: %v", o.Err)
	}
	return nil
}
func validateDirs(bin, state string) error {
	if !filepath.IsAbs(bin) || !filepath.IsAbs(state) || filepath.Clean(bin) != bin || filepath.Clean(state) != state {
		return errors.New("bin-dir and state-dir must be clean absolute paths")
	}
	binCompare, stateCompare := bin, state
	if runtime.GOOS == "darwin" {
		binCompare = strings.ToLower(bin)
		stateCompare = strings.ToLower(state)
	}
	if binCompare == stateCompare || strings.HasPrefix(binCompare, stateCompare+string(os.PathSeparator)) || strings.HasPrefix(stateCompare, binCompare+string(os.PathSeparator)) {
		return errors.New("bin/state paths overlap")
	}
	for _, p := range []string{bin, state} {
		for p != "/" {
			i, err := os.Lstat(p)
			if err == nil {
				if !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
					return fmt.Errorf("unsafe directory %s", p)
				}
			} else if !os.IsNotExist(err) {
				return err
			}
			p = filepath.Dir(p)
		}
	}
	return nil
}
func privateDir(p string) error {
	i, err := os.Stat(p)
	if err != nil {
		return err
	}
	if i.Mode().Perm()&0077 != 0 {
		return errors.New("state directory must be private (0700)")
	}
	return nil
}
func activate(ctx context.Context, source, destination, old string) error {
	b, err := readRegular(source, archiveLimit)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(destination), ".corvint-update-")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err = f.Chmod(0755); err == nil {
		_, err = f.Write(b)
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	current, err := digest(destination)
	if err != nil {
		return err
	}
	if current != old {
		return errors.New("destination digest conflict before activation")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return os.Rename(temp, destination)
}
func (e engine) rollback(ctx context.Context, destination string) (string, error) {
	current, err := digest(destination)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(e.config.StateDir)
	if err != nil {
		return "", err
	}
	var matches []string
	previousDigest := ""
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "transaction-") {
			continue
		}
		p := filepath.Join(e.config.StateDir, entry.Name(), "receipt.json")
		b, err := readRegular(p, metadataLimit)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		var r receipt
		if err = json.Unmarshal(b, &r); err != nil {
			return "", err
		}
		if r.Component == e.config.Component && r.Destination == destination && r.CandidateDigest == current {
			prev := filepath.Join(filepath.Dir(p), "previous")
			d, err := digest(prev)
			if err != nil || d != r.PreviousDigest {
				return "", errors.New("rollback previous digest conflict")
			}
			if previousDigest != "" && previousDigest != r.PreviousDigest {
				return "", errors.New("ambiguous rollback history: different prior digests")
			}
			previousDigest = r.PreviousDigest
			matches = append(matches, p)
		}
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("rollback requires a matching receipt, found %d", len(matches))
	}
	p := matches[0]
	return p, activate(ctx, filepath.Join(filepath.Dir(p), "previous"), destination, current)
}
func parseSums(b []byte) (map[string]string, error) {
	m := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if len(line) < 67 || line[64:66] != "  " {
			return nil, errors.New("invalid checksum record")
		}
		sum, name := line[:64], line[66:]
		decoded, err := hex.DecodeString(sum)
		if err != nil || len(decoded) != 32 || !safeName(name) {
			return nil, errors.New("invalid checksum name/digest")
		}
		if _, ok := m[name]; ok {
			return nil, errors.New("duplicate checksum record")
		}
		m[name] = strings.ToLower(sum)
	}
	return m, nil
}
func safeName(n string) bool {
	return n != "" && n != "." && !strings.Contains(n, "\\") && !strings.ContainsAny(n, "\x00\r\n") && !strings.HasPrefix(n, "/") && path.Clean(n) == n && n != ".." && !strings.HasPrefix(n, "../")
}
func extract(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	seen := map[string]bool{}
	spellings := map[string]string{}
	var size int64
	for count := 0; ; count++ {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if count >= 4096 {
			return errors.New("archive file count limit")
		}
		n := strings.TrimSuffix(h.Name, "/")
		if !safeName(n) {
			return errors.New("unsafe archive path")
		}
		for ancestor := n; ancestor != "."; ancestor = path.Dir(ancestor) {
			key := strings.ToLower(ancestor)
			if spelling, ok := spellings[key]; ok && spelling != ancestor {
				return errors.New("case-alias archive parent")
			}
			spellings[key] = ancestor
		}
		key := strings.ToLower(n)
		if seen[key] {
			return errors.New("duplicate/case-alias archive path")
		}
		seen[key] = true
		if h.Typeflag != tar.TypeDir && h.Typeflag != tar.TypeReg {
			return errors.New("unsupported archive entry type")
		}
		size += h.Size
		if h.Size < 0 || size > extractedLimit {
			return errors.New("archive expansion limit")
		}
		p := filepath.Join(dest, filepath.FromSlash(n))
		if h.Typeflag == tar.TypeDir {
			if err = os.MkdirAll(p, 0700); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return err
		}
		out, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, tr)
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	_, err = io.Copy(io.Discard, io.LimitReader(gz, 1))
	return err
}
func verify(root, component, platform string) (string, int, error) {
	name, _ := binaryName(component)
	base := root
	binary := filepath.Join(base, "bin", name)
	required := []string{"MANIFEST.json", "README.md", "notices/corvint-tasks/LICENSE", "notices/corvint-tasks/LICENSING.md", "notices/corvint-tasks/PROVENANCE.md", "notices/corvint-tasks/THIRD-PARTY-NOTICES.txt", "source/corvint-tasks-src.tar.gz"}
	if component == "core" {
		base = filepath.Join(root, name+"_"+platform)
		binary = filepath.Join(base, name)
		required = []string{"LICENSE", "LICENSE-APACHE-2.0", "LICENSING.md", "PROVENANCE.md"}
	}
	b, err := readRegular(filepath.Join(base, "SHA256SUMS"), metadataLimit)
	if err != nil {
		return "", 0, err
	}
	sums, err := parseSums(b)
	if err != nil {
		return "", 0, err
	}
	rel, _ := filepath.Rel(base, binary)
	if _, ok := sums[filepath.ToSlash(rel)]; !ok {
		return "", 0, errors.New("binary absent from internal checksums")
	}
	for n, want := range sums {
		got, err := digest(filepath.Join(base, filepath.FromSlash(n)))
		if err != nil || got != want {
			return "", 0, fmt.Errorf("internal checksum mismatch: %s", n)
		}
	}
	for _, n := range required {
		b, err := readRegular(filepath.Join(base, n), archiveLimit)
		if err != nil || len(b) == 0 {
			return "", 0, fmt.Errorf("missing evidence/notice %s", n)
		}
	}
	if component == "core" {
		allowed := map[string]bool{"corvint": true, "SHA256SUMS": true, "LICENSE": true, "LICENSE-APACHE-2.0": true, "LICENSING.md": true, "PROVENANCE.md": true}
		err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(base, p)
			if err != nil || !allowed[rel] {
				return errors.New("unexpected Core archive file")
			}
			return nil
		})
		if err != nil {
			return "", 0, err
		}
		return binary, 0, nil
	}
	err = filepath.WalkDir(base, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(base, p)
		if rel == "SHA256SUMS" {
			return nil
		}
		if _, ok := sums[filepath.ToSlash(rel)]; !ok {
			return fmt.Errorf("unlisted Tasks file %s", rel)
		}
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	b, err = readRegular(filepath.Join(base, "MANIFEST.json"), metadataLimit)
	if err != nil {
		return "", 0, err
	}
	var m struct {
		Profile, Target, Build, Qualification string
		Component                             struct {
			Name                                   string
			BinaryPath                             string
			BinarySha256                           string
			BinarySizeBytes                        int64
			SourceArchivePath, SourceArchiveSha256 string
		}
	}
	if err = json.Unmarshal(b, &m); err != nil {
		return "", 0, err
	}
	build, err := strconv.Atoi(m.Build)
	if err != nil || build <= 0 || m.Profile != "corvint-tasks-archive/0" || m.Target != strings.ReplaceAll(platform, "_", "/") || m.Component.Name != name || m.Component.BinaryPath != "bin/"+name || m.Component.BinarySha256 != sums["bin/"+name] || m.Qualification == "" || m.Component.SourceArchivePath != "source/corvint-tasks-src.tar.gz" || m.Component.SourceArchiveSha256 != sums[m.Component.SourceArchivePath] {
		return "", 0, errors.New("Tasks manifest identity mismatch")
	}
	info, err := os.Stat(binary)
	if err != nil || info.Size() != m.Component.BinarySizeBytes {
		return "", 0, errors.New("Tasks manifest binary size mismatch")
	}
	return binary, build, nil
}

func inspectPlatform(binary, platform string) error {
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		return fmt.Errorf("candidate Go build identity: %w", err)
	}
	settings := map[string]string{}
	for _, s := range info.Settings {
		settings[s.Key] = s.Value
	}
	if settings["GOOS"]+"_"+settings["GOARCH"] != platform {
		return errors.New("candidate build platform mismatch")
	}
	return nil
}
func verifyQualification(b []byte, root, component, platform, archiveDigest, binary string) error {
	d, err := digest(binary)
	if err != nil {
		return err
	}
	if component == "core" {
		var report struct {
			Schema, Verdict string
			Targets         []struct {
				GOOS, GOARCH, ArchiveName       string
				RetainedBinary, RetainedArchive struct{ SHA256 string }
			}
		}
		if err = json.Unmarshal(b, &report); err != nil {
			return err
		}
		if report.Schema != "corvint.release-go-archive-report.v2" || report.Verdict != "PASS" {
			return errors.New("invalid Core qualification")
		}
		matches := 0
		for _, target := range report.Targets {
			if target.GOOS+"_"+target.GOARCH == platform {
				matches++
				if target.ArchiveName != "corvint_"+platform+".tar.gz" || target.RetainedBinary.SHA256 != d || target.RetainedArchive.SHA256 != archiveDigest {
					return errors.New("Core qualification digest mismatch")
				}
			}
		}
		if matches != 1 {
			return errors.New("Core qualification target missing/duplicate")
		}
		return nil
	}
	internal, err := readRegular(filepath.Join(root, "MANIFEST.json"), metadataLimit)
	if err != nil {
		return err
	}
	var a, z map[string]json.RawMessage
	if err = json.Unmarshal(internal, &a); err != nil {
		return err
	}
	if err = json.Unmarshal(b, &z); err != nil {
		return err
	}
	for _, key := range []string{"profile", "target", "build", "component", "qualification"} {
		var left, right any
		if json.Unmarshal(a[key], &left) != nil || json.Unmarshal(z[key], &right) != nil {
			return errors.New("missing Tasks qualification field")
		}
		lb, _ := json.Marshal(left)
		rb, _ := json.Marshal(right)
		if !bytesEqual(lb, rb) {
			return fmt.Errorf("Tasks qualification mismatch: %s", key)
		}
	}
	return nil
}
func bytesEqual(a, b []byte) bool { return string(a) == string(b) }
