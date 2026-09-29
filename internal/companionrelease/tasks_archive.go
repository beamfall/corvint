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
	Version       json.RawMessage   `json:"version"`
	ArchivePath   string            `json:"archivePath,omitempty"`
	ArchiveSHA256 string            `json:"archiveSha256,omitempty"`
	Qualification string            `json:"qualification"`
}

// RunTasksArchive reuses the companion pipeline for a standalone native Tasks
// download. The closed target retains the existing macOS arm64 qualification boundary.
func RunTasksArchive(ctx context.Context, opts Options) (*TasksArchiveReport, error) {
	if err := validateTarget(opts.Target); err != nil {
		return nil, err
	}
	if opts.Target != runtime.GOOS+"/"+runtime.GOARCH {
		return nil, fmt.Errorf("Tasks archive smoke requires its native target host")
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
	version, _, err := runCaptured(ctx, extract, closedGitEnv(toolDir), subprocessTimeout, filepath.Join(extract, "bin/corvint-tasks"), "version")
	if err != nil {
		return nil, err
	}
	if !json.Valid(version) {
		return nil, fmt.Errorf("Tasks version response is not JSON")
	}
	help, _, err := runCaptured(ctx, extract, closedGitEnv(toolDir), subprocessTimeout, filepath.Join(extract, "bin/corvint-tasks"), "help")
	if err != nil {
		return nil, err
	}
	if err := verifyTasksArchiveHelp(help); err != nil {
		return nil, err
	}
	report := TasksArchiveReport{Profile: "corvint-tasks-archive/0", Target: opts.Target, Build: build, Component: component, Toolchain: tc, Version: version, Qualification: "reproducible-build-and-native-version-help-smoke-only"}
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
	retained, err := retainBundle(opts.Scratch, opts.OutputParent, opts.BundleName, []ArchiveEntry{
		{Path: name, Mode: 0o644, Data: archive},
		{Path: "SHA256SUMS", Mode: 0o644, Data: []byte(report.ArchiveSHA256 + "  " + name + "\n")},
	})
	if err != nil {
		return nil, err
	}
	report.ArchivePath = filepath.Join(retained, name)
	return &report, nil
}

func tasksArchiveEntries(files []ArchiveEntry, report TasksArchiveReport) ([]ArchiveEntry, error) {
	manifest, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	entries := append([]ArchiveEntry{}, files...)
	entries = append(entries, ArchiveEntry{Path: "MANIFEST.json", Mode: 0o644, Data: append(manifest, '\n')}, ArchiveEntry{Path: "README.md", Mode: 0o644, Data: []byte("# Corvint Tasks\n\nRun bin/corvint-tasks help. Templates are under templates/; the setup guide is docs/TASKS-EXTERNAL-AGENTS.md. Source and license notices accompany this binary. Verify SHA256SUMS before use. The version retains its explicit unverified label: this archive proves reproducible builds and native version/help execution, not queue qualification, authentication, or a full workflow-bundle release. Other platforms are NOT_RUN.\n")})
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
