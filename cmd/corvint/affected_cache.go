package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedPathsFromMaxBytes bounds one --paths-from file (AFP-V0-395): the
// --path bound of 4096 canonical paths, each well under 1 KiB.
const affectedPathsFromMaxBytes = 4 << 20

// affectedProgramMaxBytes bounds the executable read for the cache identity.
const affectedProgramMaxBytes = 1 << 30

func setAffectedOnce(field *string, name, value string) error {
	if *field != "" || value == "" {
		return argumentError(name + " requires exactly one nonempty value")
	}
	*field = value
	return nil
}

// resolveAffectedBatchAndCache reads --paths-from into the --path set and
// resolves --cache, or else the default cache location unless --no-cache,
// both after --root is known.
func resolveAffectedBatchAndCache(invocation *affectedInvocation) error {
	if invocation.Cache != "" {
		resolved, gitDirectories, err := resolveAffectedCache(invocation.Root, invocation.Cache)
		if err != nil {
			return err
		}
		invocation.Cache, invocation.CacheExcluded = resolved, gitDirectories
	} else if !invocation.NoCache && !invocation.CacheStatus && invocation.Snapshot == "" && invocation.PlaywrightConfig == "" {
		// The default location is held to every --cache placement rule; one
		// that fails any of them, or that cannot be named, is not used and
		// the call runs uncached with the same bytes (AFP-V0-436, AFP-V0-438).
		if location, _ := affectedDefaultCacheLocation(); location != "" {
			if resolved, gitDirectories, err := resolveAffectedCache(invocation.Root, location); err == nil {
				invocation.Cache, invocation.CacheExcluded = resolved, gitDirectories
			}
		}
	}
	if invocation.PathsFrom != "" {
		paths, err := readAffectedPathsFrom(invocation.Root, invocation.PathsFrom)
		if err != nil {
			return err
		}
		for _, value := range paths {
			if err := addAffectedPath(invocation, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// readAffectedPathsFrom reads one bounded regular file, relative to root
// unless absolute, of LF-separated --path values; blank lines are skipped and
// every other line must be a canonical repository-relative path (AFP-V0-395).
// --cache is resolved first, so a refused cache placement reads nothing. A
// file with no path is refused rather than planning the worktree instead.
// The file is opened nonblocking and only the open descriptor is held to a
// regular file, so a FIFO put in its place cannot hang the call.
func readAffectedPathsFrom(root, name string) ([]string, error) {
	if !filepath.IsAbs(name) {
		name = filepath.Join(root, name)
	}
	affectedPathsFromBeforeOpen(name)
	file, err := os.OpenFile(name, os.O_RDONLY|affectedOpenNonblocking, 0)
	if err != nil {
		return nil, argumentError("--paths-from must name a readable regular file")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, argumentError("--paths-from must name a readable regular file")
	}
	if info.Size() > affectedPathsFromMaxBytes {
		return nil, argumentError(fmt.Sprintf("--paths-from is over the %d-byte bound", affectedPathsFromMaxBytes))
	}
	body, err := io.ReadAll(io.LimitReader(file, affectedPathsFromMaxBytes+1))
	if err != nil || len(body) > affectedPathsFromMaxBytes {
		return nil, argumentError("--paths-from must name a readable regular file within its bound")
	}
	if strings.HasPrefix(string(body), "\ufeff") {
		return nil, argumentError("--paths-from must not begin with a byte order mark")
	}
	paths := []string{}
	for _, line := range strings.Split(string(body), "\n") {
		if line == "" {
			continue
		}
		if strings.HasSuffix(line, "\r") {
			return nil, argumentError("--paths-from lines end in LF, not CRLF; write --path for a name that ends in a carriage return")
		}
		if !affected.ValidRelativePath(line) {
			return nil, argumentError("--paths-from line must be a repository-relative canonical path, got " + pythonRepr(line))
		}
		paths = append(paths, line)
	}
	if len(paths) == 0 {
		return nil, argumentError("--paths-from names no path")
	}
	return paths, nil
}

// resolveAffectedCache makes the --cache directory absolute, resolving the
// symlinks of its nearest existing ancestor, and refuses one at or under the
// repository root, under any .git directory, or at or under the repository's
// actual Git and common directories, which `--separate-git-dir` or a linked
// worktree can put anywhere: a read command writes no repository content
// (AFP-V0-390, AGENTS.md invariant 4). It returns the resolved directory and
// the Git directories, which the cache rechecks on the directory it opens.
func resolveAffectedCache(root, value string) (string, []string, error) {
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", nil, argumentError("invalid --cache")
	}
	resolved := resolveExistingAncestor(absolute)
	for _, component := range strings.Split(filepath.ToSlash(resolved), "/") {
		if strings.EqualFold(component, ".git") {
			return "", nil, argumentError("--cache must name a directory outside the repository and outside any .git directory")
		}
	}
	if within(resolved, root) || underSameFile(resolved, root) {
		return "", nil, argumentError("--cache must name a directory outside the repository root: a read command writes no repository content")
	}
	gitDirectories, err := affectedGitDirectories(root)
	if err != nil {
		return "", nil, err
	}
	for _, directory := range gitDirectories {
		if within(resolved, directory) || underSameFile(resolved, directory) {
			return "", nil, argumentError("--cache must name a directory outside the repository's Git directory: a read command writes no repository content")
		}
	}
	return resolved, gitDirectories, nil
}

// affectedGitDirectories asks Git for the repository's Git directory and
// common directory, each resolved like --cache. A repository whose Git
// directory cannot be named refuses --cache rather than risk a write into it.
func affectedGitDirectories(root string) ([]string, error) {
	refusal := argumentError("--cache needs the repository's Git directory, which git rev-parse could not name")
	gitExecutable, err := exec.LookPath("git")
	if err != nil {
		return nil, refusal
	}
	ctx, cancel := context.WithTimeout(context.Background(), affectedRevisionDeadline)
	defer cancel()
	output, err := hermeticGitCommand(ctx, gitExecutable, root, "rev-parse", "--absolute-git-dir", "--git-common-dir").Output()
	if ownership := gitExitOwnershipRefusal(err); ownership != nil {
		return nil, ownership
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	if err != nil || len(lines) != 2 {
		return nil, refusal
	}
	directories := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			return nil, refusal
		}
		if !filepath.IsAbs(line) {
			line = filepath.Join(root, line)
		}
		directories = append(directories, resolveExistingAncestor(filepath.Clean(line)))
	}
	return directories, nil
}

// underSameFile reports whether root is the same directory as candidate or
// one of its existing ancestors, which a textual comparison misses on a
// case-insensitive file system or through a firmlink.
func underSameFile(candidate, root string) bool {
	rootInfo, err := os.Stat(root)
	if err != nil {
		return false
	}
	for current := candidate; ; current = filepath.Dir(current) {
		if info, err := os.Stat(current); err == nil && os.SameFile(info, rootInfo) {
			return true
		}
		if filepath.Dir(current) == current {
			return false
		}
	}
}

// resolveExistingAncestor resolves the symlinks of the deepest existing
// ancestor of absolute and rejoins the components that do not exist yet.
func resolveExistingAncestor(absolute string) string {
	rest := []string{}
	current := absolute
	for {
		if resolved, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(append([]string{resolved}, rest...)...)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return absolute
		}
		rest = append([]string{filepath.Base(current)}, rest...)
		current = parent
	}
}

func within(candidate, root string) bool {
	relative, err := filepath.Rel(root, candidate)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

// affectedProcessStarted is taken at package initialization, the earliest
// point this process can observe the clock.
var affectedProcessStarted = time.Now()

// affectedProgramIdentity is the SHA-256 of the running executable, the
// cache key's program member: an entry written by any other build of corvint,
// whose plugins may observe differently, is never trusted (AFP-V0-391). It is
// empty when the executable cannot be read, which disables the cache.
var affectedProgramIdentity = sync.OnceValue(func() string {
	name, running := affectedExecutable()
	return affectedFileIdentity(name, running, affectedProcessStarted)
})

// affectedPathsFromBeforeOpen runs just before the --paths-from file is
// opened; tests replace it to stage a replacement race (AFP-V0-395).
var affectedPathsFromBeforeOpen = func(string) {}

// affectedFileIdentity hashes the executable at name. Unless name opens the
// running image itself (linux's /proc/self/exe), the pathname may by now name
// a replacement installed after this process started, whose hash would key
// this process's observation to another build. A file whose change or
// modification time is not strictly before started is therefore refused, as
// is one that changes while it is hashed: no identity, so no cache.
func affectedFileIdentity(name string, running bool, started time.Time) string {
	if name == "" {
		return ""
	}
	file, err := os.OpenFile(name, os.O_RDONLY|affectedOpenNonblocking, 0)
	if err != nil {
		return ""
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return ""
	}
	if !running && (!affectedChangeTime(before).Before(started) || !before.ModTime().Before(started)) {
		return ""
	}
	hash := sha256.New()
	copied, err := io.Copy(hash, io.LimitReader(file, affectedProgramMaxBytes+1))
	if err != nil || copied > affectedProgramMaxBytes {
		return ""
	}
	after, err := file.Stat()
	if err != nil || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || !affectedChangeTime(after).Equal(affectedChangeTime(before)) {
		return ""
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

// affectedCacheObserved receives every cache outcome; tests replace it.
var affectedCacheObserved = func(affected.CacheOutcome) {}

// affectedBuildGraph builds the plan graph, through the verified cache when
// --cache named one (AFP-V0-390 to AFP-V0-393).
func affectedBuildGraph(ctx context.Context, invocation affectedInvocation) (*affected.Graph, error) {
	if invocation.Cache == "" {
		return affected.Build(ctx, invocation.Root, affectedLanguages()...)
	}
	graph, outcome, err := affected.BuildCachedExcluding(ctx, invocation.Root, invocation.Cache, invocation.CacheExcluded, affectedProgramIdentity(), affectedLanguages()...)
	if err == nil {
		affectedCacheObserved(outcome)
	}
	return graph, err
}

// affectedUserCacheDir names the user cache directory the default graph cache
// lives under (AFP-V0-436). The package's TestMain replaces it, so no test
// reads or writes the real user cache; a test of the default injects its own.
var affectedUserCacheDir = os.UserCacheDir

// affectedDefaultCacheLocation is os.UserCacheDir()/corvint/affected, or
// empty with the reason there is none (AFP-V0-436).
func affectedDefaultCacheLocation() (string, string) {
	base, err := affectedUserCacheDir()
	if err != nil {
		return "", "no user cache directory: " + err.Error()
	}
	if base == "" || !filepath.IsAbs(base) {
		return "", "the user cache directory is not an absolute path"
	}
	return filepath.Join(filepath.Clean(base), "corvint", "affected"), ""
}

// affectedCacheStatusMaxEntries bounds the directory listing `affected cache`
// reads (AFP-V0-439); a larger directory is reported INCOMPLETE.
const affectedCacheStatusMaxEntries = 4096

// parseAffectedCacheStatusOptions reads `affected cache [--format
// json|doctor-plugin]`, the option at most once (AFP-V0-439).
func parseAffectedCacheStatusOptions(rest []string) (affectedInvocation, error) {
	invocation := affectedInvocation{CacheStatus: true}
	for index := 0; index < len(rest); index++ {
		name, value, inline := strings.Cut(rest[index], "=")
		if name != "--format" {
			return affectedInvocation{}, argumentError("unrecognized arguments: " + rest[index])
		}
		if !inline {
			if index+1 >= len(rest) {
				return affectedInvocation{}, argumentError("--format requires exactly one value")
			}
			index++
			value = rest[index]
		}
		if invocation.CacheStatusFormat != "" || (value != "json" && value != "doctor-plugin") {
			return affectedInvocation{}, argumentError("argument --format: affected cache accepts json or doctor-plugin once")
		}
		invocation.CacheStatusFormat = value
	}
	return invocation, nil
}

// affectedCacheStatus is the `affected-cache-status/0` report (AFP-V0-439).
type affectedCacheStatus struct {
	Bytes    int64  `json:"bytes"`
	Entries  int    `json:"entries"`
	Location string `json:"location"`
	Mutates  bool   `json:"mutates"`
	OK       bool   `json:"ok"`
	Profile  string `json:"profile"`
	Reason   string `json:"reason"`
	State    string `json:"state"`
}

// inspectAffectedDefaultCache reports where the default graph cache of root
// is and how much it holds, without creating, opening an entry of, or
// writing anything: PRESENT, ABSENT (not yet created), UNAVAILABLE (no user
// cache directory, a placement the AFP-V0-390 rules refuse, or not a
// readable directory, so calls run uncached) or INCOMPLETE (the bounded
// listing stopped early). The directory is opened nonblocking and held to a
// directory by its descriptor, so a FIFO in its place cannot hang the call.
func inspectAffectedDefaultCache(root string) affectedCacheStatus {
	status := affectedCacheStatus{OK: true, Profile: "affected-cache-status/0", State: "UNAVAILABLE"}
	location, reason := affectedDefaultCacheLocation()
	if location == "" {
		status.Reason = reason
		return status
	}
	status.Location = location
	resolved, gitDirectories, err := resolveAffectedCache(root, location)
	if err != nil {
		status.Reason = strings.Replace(err.Error(), "--cache", "the default cache", 1)
		return status
	}
	status.Location = resolved
	if _, err := os.Lstat(resolved); errors.Is(err, fs.ErrNotExist) {
		status.State, status.Reason = "ABSENT", "not created yet; the first cached call creates it"
		return status
	}
	directory, err := os.OpenFile(resolved, os.O_RDONLY|affectedOpenNonblocking, 0)
	if err != nil {
		status.Reason = "the cache directory is unreadable"
		return status
	}
	defer directory.Close()
	if info, err := directory.Stat(); err != nil || !info.IsDir() {
		status.Reason = "the cache location is not a directory"
		return status
	}
	if reason := affected.InspectCacheDirectory(resolved, root, gitDirectories); reason != "" {
		status.Reason = reason + "; calls run uncached"
		return status
	}
	status.State = "PRESENT"
	read := 0
	for {
		batch, err := directory.ReadDir(256)
		for _, entry := range batch {
			if read == affectedCacheStatusMaxEntries {
				status.State, status.Reason = "INCOMPLETE", fmt.Sprintf("the listing stopped at its %d-entry bound", affectedCacheStatusMaxEntries)
				return status
			}
			read++
			info, infoErr := entry.Info()
			if infoErr != nil || !info.Mode().IsRegular() {
				continue
			}
			status.Bytes += info.Size()
			if strings.HasPrefix(entry.Name(), "affected-graph-") && strings.HasSuffix(entry.Name(), ".json") {
				status.Entries++
			}
		}
		if err == io.EOF || (err == nil && len(batch) == 0) {
			return status
		}
		if err != nil {
			status.State, status.Reason = "INCOMPLETE", "the cache directory could not be listed in full"
			return status
		}
	}
}

// runAffectedCacheStatus prints the default cache's status (AFP-V0-439) as
// one canonical JSON report or, with --format doctor-plugin, a one-item
// TQD-V0-010 plugin array; it never mutates and exits 0 whatever the state.
func runAffectedCacheStatus(invocation affectedInvocation, stdout, stderr io.Writer) int {
	status := inspectAffectedDefaultCache(invocation.Root)
	var report any = status
	if invocation.CacheStatusFormat == "doctor-plugin" {
		who := status.Location
		if who == "" {
			who = "affected-cache"
		}
		detail := fmt.Sprintf("%s: %d entries, %d bytes", status.State, status.Entries, status.Bytes)
		if status.Reason != "" {
			detail += "; " + status.Reason
		}
		report = []map[string]string{{
			"detail": detail,
			"kind":   "AFFECTED_CACHE",
			"remedy": "delete the directory to clear the cache; `corvint affected --no-cache` disables it and `--cache DIR` moves it",
			"who":    who,
		}}
	}
	encoded, err := gokernel.CanonicalJSON(report)
	if err == nil {
		_, err = stdout.Write(append(encoded, '\n'))
	}
	if err != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write affected cache status"})
		return 2
	}
	return 0
}
