package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

// `cw-trial import-heldout` maps the frozen held-out set
// (tools/cw-trial/testdata/heldout-v1: one JSON row per line plus a manifest,
// authored without sight of Corvint output) onto the run manifest, one field to
// one field, and refuses any row it cannot map. The frozen files are read,
// never written. The row's `source` (release, sample id, chunk file and its
// sha256) is carried onto the task, and the manifest's `tasks_sha256` is
// checked against the bytes read and recorded in the produced `source`.

const maxAnchorDiffBytes = 8 << 10

// heldoutRow is one frozen line; fields the run does not use are ignored.
type heldoutRow struct {
	ID          string         `json:"id"`
	Mode        string         `json:"mode"`
	Repository  string         `json:"repository"`
	ChangedFile string         `json:"changed_file"`
	Gold        []string       `json:"gold"`
	GoldKind    string         `json:"gold_kind"`
	Task        string         `json:"task"`
	Query       map[string]any `json:"query"`
	Source      map[string]any `json:"source"`
}

type heldoutManifest struct {
	Name        string `json:"name"`
	Partition   string `json:"partition"`
	TaskCount   int    `json:"task_count"`
	TasksSHA256 string `json:"tasks_sha256"`
}

func importHeldout(arguments []string, stdout, stderr io.Writer) int {
	var tasksPath, manifestPath, output, chunkRoot string
	flags := flag.NewFlagSet("cw-trial import-heldout", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&tasksPath, "tasks", "", "frozen tasks.jsonl")
	flags.StringVar(&manifestPath, "manifest", "", "frozen manifest.json")
	flags.StringVar(&output, "output", "", "run manifest path (default stdout)")
	flags.StringVar(&chunkRoot, "chunk-root", "", "when set, every row's source.corpus_chunk_file is read under this directory and its sha256 checked")
	if err := flags.Parse(arguments); err != nil {
		fmt.Fprintln(stderr, "cw-trial:", err)
		return 2
	}
	if tasksPath == "" || manifestPath == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "cw-trial: import-heldout takes --tasks FILE --manifest FILE [--output FILE] [--chunk-root DIR]")
		return 2
	}
	document, err := heldoutToManifest(tasksPath, manifestPath, chunkRoot)
	if err != nil {
		fmt.Fprintln(stderr, "cw-trial:", err)
		return 2
	}
	encoded, err := gokernel.CanonicalJSON(document)
	if err != nil {
		fmt.Fprintln(stderr, "cw-trial: cannot encode the manifest")
		return 2
	}
	encoded = append(encoded, '\n')
	if output == "" {
		_, err = stdout.Write(encoded)
	} else {
		err = os.WriteFile(output, encoded, 0o644)
	}
	if err != nil {
		fmt.Fprintln(stderr, "cw-trial: cannot write the manifest")
		return 2
	}
	return 0
}

func heldoutToManifest(tasksPath, manifestPath, chunkRoot string) (manifest, error) {
	frozen, err := readHeldoutManifest(manifestPath)
	if err != nil {
		return manifest{}, err
	}
	data, err := readBounded(tasksPath, maxTasksFile)
	if err != nil {
		return manifest{}, err
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	if digest != frozen.TasksSHA256 {
		return manifest{}, fmt.Errorf("%s: sha256 %s is not the manifest's tasks_sha256 %s", tasksPath, digest, frozen.TasksSHA256)
	}
	rows, err := readHeldoutRows(data)
	if err != nil {
		return manifest{}, fmt.Errorf("%s: %w", tasksPath, err)
	}
	if len(rows) != frozen.TaskCount {
		return manifest{}, fmt.Errorf("%s: %d rows, manifest task_count %d", tasksPath, len(rows), frozen.TaskCount)
	}
	tasks := make([]task, 0, len(rows))
	for _, row := range rows {
		item, err := heldoutTask(row)
		if err != nil {
			return manifest{}, fmt.Errorf("%s: row %q: %w", tasksPath, row.ID, err)
		}
		if err := verifyChunk(chunkRoot, row); err != nil {
			return manifest{}, fmt.Errorf("%s: row %q: %w", tasksPath, row.ID, err)
		}
		tasks = append(tasks, item)
	}
	source := fmt.Sprintf("%s: %s (tasks_sha256 %s)", frozen.Name, filepath.Base(tasksPath), digest)
	return manifest{Partition: frozen.Partition, Source: source, Tasks: tasks}, nil
}

func readHeldoutManifest(path string) (heldoutManifest, error) {
	data, err := readBounded(path, maxTasksFile)
	if err != nil {
		return heldoutManifest{}, err
	}
	var frozen heldoutManifest
	if err := json.Unmarshal(data, &frozen); err != nil {
		return heldoutManifest{}, fmt.Errorf("%s: %w", path, err)
	}
	if frozen.Partition == "" || frozen.TasksSHA256 == "" || frozen.TaskCount == 0 {
		return heldoutManifest{}, fmt.Errorf("%s: partition, tasks_sha256, and task_count are required", path)
	}
	return frozen, nil
}

func readHeldoutRows(data []byte) ([]heldoutRow, error) {
	rows := make([]heldoutRow, 0, 64)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 1<<20), maxTasksFile)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var row heldoutRow
		if err := json.Unmarshal([]byte(text), &row); err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		rows = append(rows, row)
	}
	return rows, scanner.Err()
}

// heldoutTask maps one row; every field the run needs must be present and
// well-formed, else the row is refused rather than guessed.
func heldoutTask(row heldoutRow) (task, error) {
	if row.ID == "" {
		return task{}, errors.New("id is required")
	}
	if row.Mode != "retrieval" && row.Mode != "change" {
		return task{}, fmt.Errorf("mode must be retrieval or change, not %q", row.Mode)
	}
	repo, commit, ok := strings.Cut(row.Repository, "@")
	if !ok || repo == "" || commit == "" {
		return task{}, fmt.Errorf("repository must be OWNER/NAME@COMMIT, not %q", row.Repository)
	}
	if row.Mode == "change" && row.ChangedFile == "" {
		return task{}, errors.New("a change row needs changed_file")
	}
	if !claimKinds[row.GoldKind] || row.GoldKind == "answer" {
		return task{}, fmt.Errorf("gold_kind must be test-file or source-file, not %q", row.GoldKind)
	}
	if len(row.Gold) == 0 {
		return task{}, errors.New("gold is empty")
	}
	release, _ := row.Source["release"].(string)
	text, err := heldoutText(release, row)
	if err != nil {
		return task{}, err
	}
	return task{
		ID: row.ID, Kind: row.Mode, Repo: repo, BaseCommit: commit, ChangedFile: row.ChangedFile,
		Text: text, Gold: map[string][]string{row.GoldKind: append([]string{}, row.Gold...)}, Source: row.Source,
	}, nil
}

// heldoutText renders the task question for the row's release from the
// sample's verbatim query fields. The gold kind is named so a path claim is
// judged rather than left UNJUDGED under the wrong kind; every arm sees the
// same text.
func heldoutText(release string, row heldoutRow) (string, error) {
	field := func(name string) string { value, _ := row.Query[name].(string); return value }
	kindLine := "\n\nReport each path as a claim of kind " + row.GoldKind + "."
	switch release {
	case "v2_trace2code":
		if row.Task == "" {
			return "", errors.New("task (the failure excerpt) is empty")
		}
		text := "A test run failed with the output below. Which source file(s) in this repository hold the root cause?"
		if command := field("command"); command != "" {
			text += "\n\nCommand: " + command
		}
		return text + "\n\n" + row.Task + kindLine, nil
	case "v2_edit2ripple":
		if row.Task == "" || field("anchor_diff") == "" {
			return "", errors.New("task (the intent) and query.anchor_diff are required")
		}
		diff, _ := truncate(field("anchor_diff"), maxAnchorDiffBytes)
		return "The file below was changed with the intent described. Which other file(s) in this repository must change alongside it?\n\nChanged file: " + row.ChangedFile + "\n\nIntent:\n" + row.Task + "\n\nDiff of the changed file:\n" + diff + kindLine, nil
	case "v2_comment2context":
		if row.Task == "" || field("given_file") == "" {
			return "", errors.New("task (the review comment) and query.given_file are required")
		}
		line, _ := row.Query["line"].(float64)
		text := fmt.Sprintf("A reviewer left the comment below on %s at line %d", field("given_file"), int(line))
		if title := field("pr_title"); title != "" {
			text += " in a pull request titled " + strings.TrimSpace(title)
		}
		text += ". Which file(s) in this repository must be read to understand and act on the comment?"
		if hunk := field("diff_hunk_context"); hunk != "" {
			text += "\n\nDiff hunk:\n" + hunk
		}
		return text + "\n\nComment:\n" + row.Task + kindLine, nil
	}
	return "", fmt.Errorf("source.release %q has no mapping", release)
}

// verifyChunk checks the row's chunk file digest when a chunk root is given.
func verifyChunk(chunkRoot string, row heldoutRow) error {
	if chunkRoot == "" {
		return nil
	}
	relative, _ := row.Source["corpus_chunk_file"].(string)
	expected, _ := row.Source["corpus_sha256"].(string)
	if relative == "" || expected == "" || !safeRelative(relative) {
		return errors.New("source.corpus_chunk_file and source.corpus_sha256 are required")
	}
	file, err := os.Open(filepath.Join(chunkRoot, filepath.FromSlash(relative)))
	if err != nil {
		return err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	if digest := hex.EncodeToString(hasher.Sum(nil)); digest != expected {
		return fmt.Errorf("%s: sha256 %s is not the row's corpus_sha256 %s", relative, digest, expected)
	}
	return nil
}
