package companionrelease

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// TasksArchiveReport qualifies only the named Tasks binary and its source archive.
// It never promotes the full workflow bundle or a task queue's execution authority.
type TasksArchiveReport struct {
	Profile       string            `json:"profile"`
	Target        string            `json:"target"`
	Build         string            `json:"build"`
	Component     ComponentManifest `json:"component"`
	Toolchain     Toolchain         `json:"toolchain"`
	Version       json.RawMessage   `json:"version,omitempty"`
	ArchivePath   string            `json:"archivePath,omitempty"`
	ArchiveSHA256 string            `json:"archiveSha256,omitempty"`
	Qualification string            `json:"qualification"`
}

// RunTasksArchive reuses the companion pipeline for a standalone native Tasks
// download. The closed target retains the existing macOS arm64 qualification boundary.
func RunTasksArchive(ctx context.Context, opts Options) (*TasksArchiveReport, error) {
	native, err := admitTasksTarget(opts.Target, runtime.GOOS+"/"+runtime.GOARCH)
	if err != nil {
		return nil, err
	}
	if err := validateOutputParent(ctx, opts.OutputParent, opts.CorvintRoot); err != nil {
		return nil, err
	}
	if err := validateBundleName(opts.BundleName); err != nil {
		return nil, err
	}
	if err := validateScratch(ctx, opts.Scratch, opts.CorvintRoot); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(opts.Scratch, 0o700); err != nil {
		return nil, err
	}
	toolDir := filepath.Join(opts.Scratch, "toolchain")
	if err := mkdirScratchDir(toolDir); err != nil {
		return nil, err
	}
	tc, err := resolveToolchain(ctx, toolDir)
	if err != nil {
		return nil, err
	}
	if err := requireCleanTree(ctx, tc.GitPath, opts.CorvintRoot, opts.Scratch); err != nil {
		return nil, err
	}
	source, err := exportSource(ctx, tc.GitPath, opts.CorvintRoot, opts.Scratch)
	if err != nil {
		return nil, err
	}
	resources, err := tasksArchiveResources(source)
	if err != nil {
		return nil, err
	}
	subset := tasksExport(source)
	root, err := stageBuildSource(subset, filepath.Join(opts.Scratch, "tasks-build-src"))
	if err != nil {
		return nil, err
	}
	build, err := sourceBuildNumberAt(ctx, tc.GitPath, opts.CorvintRoot, opts.Scratch, source.HeadCommit)
	if err != nil {
		return nil, err
	}
	binary, err := buildComponentTwiceWithFlags(ctx, root, "./cmd/corvint-tasks", "corvint-tasks", opts.Target, opts.Scratch, []string{"-ldflags=-X main.build=" + build})
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(binary.Path)
	if err != nil {
		return nil, err
	}
	files, component, err := assembleComponent("corvint-tasks", "corvint-tasks", subset, binary, data)
	if err != nil {
		return nil, err
	}
	files = append(files, resources...)

	extract := filepath.Join(opts.Scratch, "tasks-smoke")
	if err := writeTree(extract, files); err != nil {
		return nil, err
	}
	version, qualification, err := tasksArchiveSmoke(ctx, native, extract, closedGitEnv(toolDir))
	if err != nil {
		return nil, err
	}
	report := TasksArchiveReport{Profile: "corvint-tasks-archive/0", Target: opts.Target, Build: build, Component: component, Toolchain: tc, Version: version, Qualification: qualification}
	entries, err := tasksArchiveEntries(files, report)
	if err != nil {
		return nil, err
	}
	archive, err := buildTarGzTwice(func() ([]ArchiveEntry, error) { return tasksArchiveEntries(files, report) })
	if err != nil {
		return nil, err
	}
	if err := verifyTarGz(archive, entries); err != nil {
		return nil, err
	}
	name := "corvint-tasks_" + strings.ReplaceAll(opts.Target, "/", "_") + ".tar.gz"
	report.ArchiveSHA256 = sha256Hex(archive)
	declaration, err := tasksReportAsset(report)
	if err != nil {
		return nil, err
	}
	retained, err := retainBundle(opts.Scratch, opts.OutputParent, opts.BundleName, []ArchiveEntry{
		{Path: name, Mode: 0o644, Data: archive},
		{Path: "SHA256SUMS", Mode: 0o644, Data: []byte(report.ArchiveSHA256 + "  " + name + "\n")},
		declaration,
	})
	if err != nil {
		return nil, err
	}
	report.ArchivePath = filepath.Join(retained, name)
	return &report, nil
}

// tasksReportAsset is the per-platform release qualification asset
// build-verification_<goos>_<goarch>.json (UPD-V0-017): the report bound to the
// archive digest, without the host-local archive path. A release carrying
// archives for several platforms publishes one per platform.
func tasksReportAsset(report TasksArchiveReport) (ArchiveEntry, error) {
	report.ArchivePath = ""
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return ArchiveEntry{}, err
	}
	return ArchiveEntry{Path: "build-verification_" + strings.ReplaceAll(report.Target, "/", "_") + ".json", Mode: 0o644, Data: append(b, '\n')}, nil
}

// admitTasksTarget reports whether target is the build host (native: smoke runs) or the
// cross-built target, and refuses everything else (CAL-V0-320).
func admitTasksTarget(target, host string) (native bool, err error) {
	if err := validateTasksTarget(target); err != nil {
		return false, err
	}
	if target == host {
		return true, nil
	}
	if target != tasksCrossTarget {
		return false, fmt.Errorf("Tasks archive smoke requires its native target host")
	}
	return false, nil
}

// tasksArchiveSmoke runs the extracted binary's version and help only for a native target. A
// cross-built target is never executed and its qualification says so.
func tasksArchiveSmoke(ctx context.Context, native bool, extract string, env []string) ([]byte, string, error) {
	if !native {
		return nil, tasksCrossQualification, nil
	}
	bin := filepath.Join(extract, "bin/corvint-tasks")
	version, _, err := runCaptured(ctx, extract, env, subprocessTimeout, bin, "version")
	if err != nil {
		return nil, "", err
	}
	if !json.Valid(version) {
		return nil, "", fmt.Errorf("Tasks version response is not JSON")
	}
	help, _, err := runCaptured(ctx, extract, env, subprocessTimeout, bin, "help")
	if err != nil {
		return nil, "", err
	}
	if err := verifyTasksArchiveHelp(help); err != nil {
		return nil, "", err
	}
	return version, "reproducible-build-and-native-version-help-smoke-only", nil
}

// tasksCrossQualification is recorded in the manifest of a cross-built archive: the Linux
// runtime is NOT_OBSERVED until a Linux host runs the commands in docs/INSTALL.md.
const tasksCrossQualification = "reproducible-cross-build-only-target-runtime-not-observed"

func tasksArchiveEntries(files []ArchiveEntry, report TasksArchiveReport) ([]ArchiveEntry, error) {
	manifest, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	entries := append([]ArchiveEntry{}, files...)
	entries = append(entries, ArchiveEntry{Path: "MANIFEST.json", Mode: 0o644, Data: append(manifest, '\n')}, ArchiveEntry{Path: "README.md", Mode: 0o644, Data: []byte("# Corvint Tasks\n\nRun bin/corvint-tasks help. Templates are under templates/; the setup guide is docs/TASKS-EXTERNAL-AGENTS.md. Source and license notices accompany this binary. Verify SHA256SUMS before use. The version retains its explicit unverified label: this archive proves reproducible builds and, for the build host's own platform, native version/help execution, not queue qualification, authentication, or a full workflow-bundle release. A cross-built archive (MANIFEST.json qualification " + tasksCrossQualification + "), such as linux/amd64, was never executed by its builder: its runtime is NOT_OBSERVED until a host of that platform runs `bin/corvint-tasks version` and a read-only `roadmap`. Other platforms are NOT_RUN.\n")})
	entries = append(entries, ArchiveEntry{Path: "SHA256SUMS", Mode: 0o644, Data: []byte(renderSHA256SUMS(entries))})
	return entries, nil
}

func verifyTasksArchiveHelp(raw []byte) error {
	var help struct {
		Outcome string `json:"outcome"`
		Items   []struct {
			Implemented []string `json:"implemented"`
			Usage       []string `json:"usage"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &help); err != nil || help.Outcome != "OK" || len(help.Items) != 1 {
		return fmt.Errorf("Tasks help smoke failed")
	}
	got := map[string]bool{}
	for _, name := range help.Items[0].Implemented {
		got[name] = true
	}
	for _, name := range []string{"claim", "plan preview", "submit", "gate run", "complete"} {
		if !got[name] {
			return fmt.Errorf("Tasks archive omits %s", name)
		}
	}
	return nil
}

func tasksArchiveResources(source Export) ([]ArchiveEntry, error) {
	paths := []string{"docs/TASKS-EXTERNAL-AGENTS.md", "internal/tasks/cli/testdata/external-agents/policy.json", "internal/tasks/cli/testdata/external-agents/queue.json", "internal/tasks/cli/testdata/external-agents/ticket-create.json"}
	var entries []ArchiveEntry
	for _, path := range paths {
		file, ok := findExportFile(source, path)
		if !ok {
			return nil, fmt.Errorf("Tasks archive requires %s", path)
		}
		name := strings.Replace(path, "internal/tasks/cli/testdata/external-agents/", "templates/", 1)
		entries = append(entries, ArchiveEntry{Path: name, Mode: 0o644, Data: file.Data})
	}
	return entries, nil
}
