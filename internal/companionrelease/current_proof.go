package companionrelease

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// The current-checkout proofs below take the checkout root as a parameter, so
// this package never locates a repository root itself (V1-0643, PUB-V0-670).
// Their only caller that reads the enclosing checkout's committed Git content is
// the deliberate root-locator test package internal/companionrelease/currentproof.

// ProveCurrentExportedTasksAndVSIX exports the committed tree at root, builds
// corvint-tasks twice from the Tasks subset and the VSIX twice from npmCache, and
// checks the VSIX member set exactly (CRB-V0-017). It returns the proof log.
func ProveCurrentExportedTasksAndVSIX(ctx context.Context, gitPath, root, scratch, npmCache string) (string, error) {
	source, err := exportSource(ctx, gitPath, root, scratch)
	if err != nil {
		return "", err
	}
	taskExport := tasksExport(source)
	staged, err := stageBuildSource(taskExport, filepath.Join(scratch, "tasks"))
	if err != nil {
		return "", err
	}
	built, err := buildComponentTwice(ctx, staged, "./cmd/corvint-tasks", "corvint-tasks", supportedTarget, scratch)
	if err != nil {
		return "", err
	}
	log := fmt.Sprintf("Tasks commit=%s tree=%s binary=%s\n", taskExport.HeadCommit, taskExport.HeadTree, built.SHA256)
	vsix, err := buildVSIXTwice(ctx, source, scratch, npmCache)
	if err != nil {
		return log, err
	}
	members, err := readVSIXMembers(vsix.Data)
	if err != nil {
		return log, err
	}
	if len(members) != len(vsixMembers) {
		return log, fmt.Errorf("members=%d, expected %d", len(members), len(vsixMembers))
	}
	// The count plus every current member makes the set exact, so no legacy member is accepted.
	for _, name := range vsixMembers {
		if _, ok := members[name]; !ok {
			return log, fmt.Errorf("missing %s", name)
		}
	}
	for _, name := range []string{"extension/dist/src/configuration.js", "extension/media/corvint.svg"} {
		if len(members[name]) == 0 {
			return log, fmt.Errorf("empty %s", name)
		}
	}
	if vsix.Path != "extensions/corvint-vscode-0.1.0.vsix" {
		return log, fmt.Errorf("vsix path %s", vsix.Path)
	}
	log += fmt.Sprintf("Core commit=%s tree=%s VSIX=%s members=%d", source.HeadCommit, source.HeadTree, sha256Hex(vsix.Data), len(members))
	return log, nil
}

// ProveCurrentTasksSmokeGenerator exports the committed tree at root and
// generates the Tasks smoke queue and policy from its Tasks subset.
func ProveCurrentTasksSmokeGenerator(ctx context.Context, gitPath, root, scratch string) (string, error) {
	source, err := exportSource(ctx, gitPath, root, scratch)
	if err != nil {
		return "", err
	}
	files, err := generateQueuePolicy(ctx, tasksExport(source), scratch)
	if err != nil {
		return "", err
	}
	if len(files.Queue) == 0 || len(files.Policy) == 0 {
		return "", fmt.Errorf("empty smoke fixture")
	}
	return fmt.Sprintf("Tasks commit=%s tree=%s queue=%s policy=%s", source.HeadCommit, source.HeadTree, sha256Hex(files.Queue), sha256Hex(files.Policy)), nil
}

// ProveCurrentTasksSourceArchiveBuildsOffline exports the committed tree at
// root, extracts the shipped Tasks source archive bytes (not stageBuildSource's
// parallel materialization path) and builds ./cmd/corvint-tasks offline, then
// proves the build fails once a transitive dependency is removed (PUB-V0-012,
// PUB-V0-013). It needs no npm or VSIX cache.
func ProveCurrentTasksSourceArchiveBuildsOffline(ctx context.Context, gitPath, root, scratch string) (string, error) {
	source, err := exportSource(ctx, gitPath, root, scratch)
	if err != nil {
		return "", err
	}
	subset := tasksExport(source)
	for _, name := range []string{"LICENSE", "LICENSE-APACHE-2.0", "LICENSING.md", "PROVENANCE.md"} {
		original, ok := findExportFile(source, name)
		if !ok {
			return "", fmt.Errorf("source notice missing: %s", name)
		}
		retained, ok := findExportFile(subset, name)
		if !ok || original.OID != retained.OID || string(original.Data) != string(retained.Data) {
			return "", fmt.Errorf("notice changed: %s", name)
		}
	}
	if subset.HeadCommit != source.HeadCommit || subset.HeadTree != source.HeadTree {
		return "", fmt.Errorf("subset changed immutable identity")
	}
	extract := func(export Export, dir string) (string, error) {
		files, member, digest, err := assembleModuleSource("corvint-tasks", export)
		if err != nil {
			return "", err
		}
		bundle := &VerifiedRetainedBundle{entries: files}
		if err := bundle.ExtractSourceArchive(member, dir); err != nil {
			return "", err
		}
		if err := verifyStagedTree(export, dir); err != nil {
			return "", err
		}
		return digest, nil
	}
	directory := filepath.Join(scratch, "source")
	archiveDigest, err := extract(subset, directory)
	if err != nil {
		return "", err
	}
	goPath, err := goBinary()
	if err != nil {
		return "", err
	}
	env := closedGoEnv(filepath.Join(scratch, "home"), "")
	built := runWithStdin(ctx, directory, env, buildTimeout, nil, goPath, "build", "./cmd/corvint-tasks")
	if built.err != nil {
		return "", fmt.Errorf("offline archive rebuild: %w", built.err)
	}
	log := fmt.Sprintf("Tasks commit=%s tree=%s archive=%s files=%d; offline go build ./cmd/corvint-tasks PASS", subset.HeadCommit, subset.HeadTree, archiveDigest, len(subset.Files))
	// diagnostic is transitive through contextindex, not a direct Tasks import.
	broken := subset
	broken.Files = nil
	for _, f := range subset.Files {
		if !strings.HasPrefix(f.Path, "internal/diagnostic/") {
			broken.Files = append(broken.Files, f)
		}
	}
	if len(broken.Files) == len(subset.Files) {
		return log, fmt.Errorf("negative control removed no dependency")
	}
	brokenDir := filepath.Join(scratch, "missing-dependency")
	if _, err := extract(broken, brokenDir); err != nil {
		return log, err
	}
	failed := runWithStdin(ctx, brokenDir, env, buildTimeout, nil, goPath, "build", "./cmd/corvint-tasks")
	if failed.err == nil || !strings.Contains(failed.err.Error(), "internal/diagnostic") {
		return log, fmt.Errorf("ineffective missing-dependency control: %v", failed.err)
	}
	return log, nil
}
