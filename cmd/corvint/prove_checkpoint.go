package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

const checkpointByteBound = 256 << 10
const checkpointEntryBound = 256

type checkpointHandle struct {
	identity  string
	Path      string `json:"path"`
	BlobHash  string `json:"blob_hash"`
	Line      *int   `json:"line,omitempty"`
	Kind      string `json:"kind,omitempty"`
	ID        string `json:"id,omitempty"`
	Authority string `json:"authority,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type checkpointDocument struct {
	Version     string   `json:"version"`
	Task        string   `json:"task"`
	Obligations []string `json:"obligations"`
	Repository  struct {
		ObjectFormat     string `json:"object_format"`
		BaseCommit       string `json:"base_commit"`
		BaseTree         string `json:"base_tree"`
		DirtyPathsSHA256 string `json:"dirty_paths_sha256"`
	} `json:"repository"`
	Handles          []checkpointHandle `json:"handles"`
	Critical         []checkpointHandle `json:"critical"`
	Unknowns         string             `json:"unknowns"`
	FailedApproaches string             `json:"failed_approaches"`
	Verification     []map[string]any   `json:"verification"`
	Provenance       map[string]any     `json:"provenance"`
}

func checkpointError(code, message string) error {
	return &gokernel.Error{Code: code, Message: message}
}

func parseProveCheckpointArguments(roots, rest []string) (options, error) {
	result := options{command: "prove", proveMode: "checkpoint"}
	root := "."
	for index := 0; index < len(roots); index++ {
		if roots[index] == "--root" {
			root, index = roots[index+1], index+1
			continue
		}
		root = strings.TrimPrefix(roots[index], "--root=")
	}
	for index := 0; index < len(rest); index++ {
		name, value, inline := strings.Cut(rest[index], "=")
		if name != "--checkpoint" {
			return result, argumentError("unrecognized arguments: " + rest[index])
		}
		if result.prove.checkpointPath != "" {
			return result, argumentError("argument --checkpoint: may not be repeated")
		}
		if !inline {
			index++
			if index == len(rest) || argparseOptionLike(rest[index]) {
				return result, argumentError("argument --checkpoint: expected one argument")
			}
			value = rest[index]
		}
		if value == "" {
			return result, argumentError("argument --checkpoint: expected one argument")
		}
		result.prove.checkpointPath = value
	}
	// Repository validity belongs to the ordered Git read boundary, after argv.
	resolved, err := normalizeQueryRoot(root)
	if err != nil {
		return result, err
	}
	result.root = resolved
	return result, nil
}

func readCheckpointDocument(filename string) (checkpointDocument, error) {
	var document checkpointDocument
	raw, err := readBoundedFile(filename, checkpointByteBound)
	if err != nil {
		return document, checkpointError("unreadable-checkpoint-document", "cannot read checkpoint document within 256 KiB")
	}
	return decodeCheckpointDocument(raw)
}

func decodeCheckpointDocument(raw []byte) (checkpointDocument, error) {
	var document checkpointDocument
	var object map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if !utf8.Valid(raw) || decoder.Decode(&object) != nil {
		return document, checkpointError("invalid-checkpoint-document", "checkpoint must be a canonical JSON object")
	}
	canonical, err := gokernel.CanonicalJSON(object)
	if err != nil || !bytes.Equal(raw, canonical) || !checkpointSchema(object) {
		return document, checkpointError("invalid-checkpoint-document", "checkpoint schema or canonical JSON is invalid")
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		return document, checkpointError("invalid-checkpoint-document", "checkpoint schema is invalid")
	}
	if len(document.Handles) > checkpointEntryBound || len(document.Critical) > checkpointEntryBound {
		return document, checkpointError("checkpoint-bound-exceeded", "checkpoint handles and critical each permit at most 256 entries")
	}
	for i, value := range object["handles"].([]any) {
		identity, _ := gokernel.CanonicalJSON(value)
		document.Handles[i].identity = string(identity)
	}
	document.Handles = uniqueCheckpointHandles(document.Handles)
	return document, nil
}

func checkpointSchema(object map[string]any) bool {
	if !checkpointKeys(object, "version task obligations repository handles critical unknowns failed_approaches verification provenance", "") {
		return false
	}
	if stringAt(object, "version") != "corvint-checkpoint/0" {
		return false
	}
	for _, key := range []string{"task", "unknowns", "failed_approaches"} {
		if _, ok := object[key].(string); !ok {
			return false
		}
	}
	obligations, ok := object["obligations"].([]any)
	if !ok {
		return false
	}
	for _, value := range obligations {
		if _, ok := value.(string); !ok {
			return false
		}
	}
	repository, ok := object["repository"].(map[string]any)
	if !ok || !checkpointKeys(repository, "object_format base_commit base_tree dirty_paths_sha256", "") {
		return false
	}
	format := stringAt(repository, "object_format")
	if format != "sha1" && format != "sha256" {
		return false
	}
	for _, key := range []string{"base_commit", "base_tree"} {
		if !checkpointObjectID(stringAt(repository, key), format) {
			return false
		}
	}
	if !checkpointObjectID(stringAt(repository, "dirty_paths_sha256"), "sha256") {
		return false
	}
	if !checkpointHandleArray(object["handles"], format, true) {
		return false
	}
	if !checkpointHandleArray(object["critical"], format, false) {
		return false
	}
	verification, ok := object["verification"].([]any)
	if !ok {
		return false
	}
	for _, value := range verification {
		row, ok := value.(map[string]any)
		if !ok || !checkpointKeys(row, "command observed_status provenance", "") {
			return false
		}
		for _, key := range []string{"command", "observed_status", "provenance"} {
			if _, ok := row[key].(string); !ok {
				return false
			}
		}
	}
	provenance, ok := object["provenance"].(map[string]any)
	if !ok || !checkpointKeys(provenance, "", "receiptId packet_sha256") {
		return false
	}
	for _, value := range provenance {
		if _, ok := value.(string); !ok {
			return false
		}
	}
	if digest, ok := provenance["packet_sha256"]; ok && !checkpointObjectID(digest.(string), "sha256") {
		return false
	}
	return true
}

func checkpointKeys(object map[string]any, required, optional string) bool {
	allowed := stringSet(strings.Fields(required + " " + optional))
	for _, key := range strings.Fields(required) {
		if _, ok := object[key]; !ok {
			return false
		}
	}
	for key := range object {
		if _, ok := allowed[key]; !ok {
			return false
		}
	}
	return true
}

func checkpointHandleArray(value any, format string, total bool) bool {
	entries, ok := value.([]any)
	if !ok {
		return false
	}
	blobs := map[string]string{}
	for _, entry := range entries {
		handle, ok := entry.(map[string]any)
		if !ok || !checkpointKeys(handle, "path blob_hash", "line kind id authority reason") {
			return false
		}
		path, ok := handle["path"].(string)
		if !ok {
			return false
		}
		blob := stringAt(handle, "blob_hash")
		if !checkpointObjectID(blob, format) {
			return false
		}
		if total && blobs[path] != "" && blobs[path] != blob {
			return false
		}
		blobs[path] = blob
		for _, key := range []string{"kind", "id", "authority", "reason"} {
			if value, exists := handle[key]; exists {
				if _, ok := value.(string); !ok {
					return false
				}
			}
		}
		_, kind := handle["kind"]
		_, id := handle["id"]
		if kind != id {
			return false
		}
		if kind && (stringAt(handle, "kind") == "" || stringAt(handle, "id") == "") {
			return false
		}
		if value, exists := handle["line"]; exists {
			number, ok := value.(json.Number)
			line, err := number.Int64()
			if !ok || err != nil || line < 1 {
				return false
			}
		}
	}
	return true
}

func checkpointObjectID(value, format string) bool {
	size := 40
	if format == "sha256" {
		size = 64
	}
	if len(value) != size {
		return false
	}
	for _, char := range value {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}

func uniqueCheckpointHandles(handles []checkpointHandle) []checkpointHandle {
	result := make([]checkpointHandle, 0, len(handles))
	seen := map[string]bool{}
	for _, handle := range handles {
		encoded, _ := gokernel.CanonicalJSON(handle)
		key := handle.identity
		if key == "" {
			key = string(encoded)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, handle)
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}

func checkpointPathFramable(value string) bool {
	if value == "" || value == "." || filepath.IsAbs(value) || strings.HasPrefix(value, "../") {
		return false
	}
	if path.Clean(value) != value || utf8.RuneCountInString(value) > 1024 {
		return false
	}
	return !strings.ContainsAny(value, "\x00\r\n")
}

func readCheckpointTree(ctx context.Context, gitExecutable, root, tree string) (map[string]string, error) {
	deadline, cancel := context.WithTimeout(ctx, proveGitDeadline)
	defer cancel()
	command := hermeticGitCommand(deadline, gitExecutable, root, "ls-tree", "-r", "-t", "-z", "--full-tree", tree)
	raw, err := boundedOutput(command, proveBoundsFrom(ctx).checkpointTreeBytes)
	if err != nil {
		return nil, checkpointError("unsupported-prove-tree", "cannot list the current tree within 64 MiB")
	}
	entries := map[string]string{}
	if len(raw) > 0 && raw[len(raw)-1] != 0 {
		return nil, checkpointError("unsupported-prove-tree", "Git tree output is malformed")
	}
	for _, item := range bytes.Split(raw, []byte{0}) {
		if len(item) == 0 {
			continue
		}
		metadata, file, ok := bytes.Cut(item, []byte{'\t'})
		fields := strings.Fields(string(metadata))
		if !ok || len(fields) != 3 || len(file) == 0 || !validGitObjectID(fields[2]) {
			return nil, checkpointError("unsupported-prove-tree", "Git tree output is malformed")
		}
		entries[string(file)] = fields[0]
	}
	return entries, nil
}

func checkpointDirtyDigest(paths []string) string {
	// DirtyPaths already returns a sorted, unique, nonnil list.
	raw, _ := gokernel.CanonicalJSON(paths)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func checkpointVerdict(handle checkpointHandle, tree map[string]string, index *contextindex.Index, cited map[string]citedBlob, dirty map[string]struct{}) string {
	if !checkpointPathFramable(handle.Path) {
		return "unframable"
	}
	mode, present := tree[handle.Path]
	if present && mode != "100644" && mode != "100755" && mode != "120000" {
		return "unsupported"
	}
	if _, admitted := index.Sources[handle.Path]; present && !admitted {
		return "unsupported"
	}
	if _, changed := dirty[handle.Path]; changed {
		return "dirty"
	}
	blob, found := cited[handle.Path]
	if !present && !found {
		return "path-deleted"
	}
	if blob.oid != handle.BlobHash {
		return "blob-changed"
	}
	return "unchanged"
}

func checkpointInstructionOrSpec(index *contextindex.Index, file string) bool {
	for _, document := range index.Documents {
		if document.Path == file {
			return document.Kind != "decision"
		}
	}
	return false
}

func judgeCheckpoint(document checkpointDocument, index *contextindex.Index, revision string, tree map[string]string, cited map[string]citedBlob, dirtyPaths []string) map[string]any {
	handles := make([]map[string]any, 0, len(document.Handles))
	missing := make([]map[string]any, 0)
	byPath := map[string][]map[string]any{}
	critical := map[string]bool{}
	for _, selector := range document.Critical {
		critical[selector.Path] = true
	}
	dirty := stringSet(dirtyPaths)
	dirtySetMoved := checkpointDirtyDigest(dirtyPaths) != document.Repository.DirtyPathsSHA256
	eligible := []string{}
	for _, handle := range document.Handles {
		verdict := checkpointVerdict(handle, tree, index, cited, dirty)
		row := map[string]any{"path": handle.Path, "verdict": verdict}
		if revision != document.Repository.BaseTree {
			row["tree_moved"] = true
		}
		if index.CommitRevision != document.Repository.BaseCommit {
			row["commit_moved"] = true
		}
		if dirtySetMoved {
			row["dirty_set_moved"] = true
		}
		if handle.Authority != "" {
			row["claimed_authority"] = handle.Authority
		}
		blob, hasBlob := cited[handle.Path]
		if hasBlob && blob.objectType == "blob" && blob.oid != handle.BlobHash && critical[handle.Path] && checkpointInstructionOrSpec(index, handle.Path) {
			row["authority_changed"] = true
		}
		if critical[handle.Path] && checkpointEligible(verdict) {
			eligible = append(eligible, handle.Path)
		}
		handles = append(handles, row)
		byPath[handle.Path] = append(byPath[handle.Path], row)
	}
	results := contextindex.CheckpointResults(index, eligible)
	for _, selector := range document.Critical {
		rows := byPath[selector.Path]
		reason := "handle-undeclared"
		if len(rows) > 0 {
			reason = stringAt(rows[0], "verdict")
		}
		if checkpointEligible(reason) {
			matches := checkpointMatchingRows(results, selector)
			reason = "selector-unresolved"
			if len(matches) > 0 {
				for _, row := range rows {
					existing, _ := row["rows"].([]map[string]any)
					row["rows"] = sortedCheckpointRows(append(existing, matches...))
				}
				continue
			}
		}
		absent := map[string]any{"path": selector.Path, "reason": reason, "recovery": "Re-run query or impact at the current revision for this path and update the caller checkpoint."}
		if selector.Kind != "" {
			absent["kind"], absent["id"] = selector.Kind, selector.ID
		}
		missing = append(missing, absent)
	}
	sort.SliceStable(missing, func(i, j int) bool {
		for _, key := range []string{"path", "kind", "id"} {
			a, b := stringAt(missing[i], key), stringAt(missing[j], key)
			if a != b {
				return a < b
			}
		}
		return false
	})
	return map[string]any{"tool": "prove", "ok": true, "revision": revision, "handles": handles, "critical_missing": missing,
		"obligations": document.Obligations, "obligations_authority": "caller-reported-unverified",
		"task": document.Task, "unknowns": document.Unknowns, "failed_approaches": document.FailedApproaches, "verification": document.Verification}
}

func checkpointEligible(verdict string) bool {
	return verdict == "unchanged" || verdict == "blob-changed"
}

func checkpointMatchingRows(results []map[string]any, selector checkpointHandle) []map[string]any {
	matches := []map[string]any{}
	for _, result := range results {
		if selector.Kind != "" && (stringAt(result, "kind") != selector.Kind || stringAt(result, "id") != selector.ID) {
			continue
		}
		for _, row := range mapsFromAny(result["evidence"]) {
			if stringAt(row, "path") == selector.Path {
				matches = append(matches, row)
			}
		}
	}
	return sortedCheckpointRows(matches)
}

func sortedCheckpointRows(rows []map[string]any) []map[string]any {
	unique := make([]map[string]any, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		raw, _ := gokernel.CanonicalJSON(row)
		if seen[string(raw)] {
			continue
		}
		seen[string(raw)] = true
		unique = append(unique, row)
	}
	sort.Slice(unique, func(i, j int) bool {
		if integerAt(unique[i], "line") != integerAt(unique[j], "line") {
			return integerAt(unique[i], "line") < integerAt(unique[j], "line")
		}
		for _, key := range []string{"reason", "blob_hash", "confidence", "authority"} {
			a, b := stringAt(unique[i], key), stringAt(unique[j], key)
			if a != b {
				return a < b
			}
		}
		return false
	})
	return unique
}

// checkpointRevision resolves the tree from the captured immutable commit, so a
// HEAD move between these reads cannot pair one commit with another commit's tree.
func checkpointRevision(ctx context.Context, gitExecutable, root string) (string, string, error) {
	deadline, cancel := context.WithTimeout(ctx, proveGitDeadline)
	defer cancel()
	identities := make([]string, 0, 2)
	ref := "HEAD^{commit}"
	for range 2 {
		command := hermeticGitCommand(deadline, gitExecutable, root, "rev-parse", "--verify", "--quiet", ref)
		output, err := command.Output()
		identity := string(bytes.TrimSpace(output))
		if err != nil || !validGitObjectID(identity) {
			return "", "", checkpointHeadRefusal()
		}
		identities = append(identities, identity)
		ref = identity + "^{tree}"
	}
	return identities[0], identities[1], nil
}

func checkpointClosingRead(ctx context.Context, gitExecutable, root, commit, revision string, dirtyBefore []string) error {
	dirtyAfter, err := affected.DirtyPaths(ctx, gitExecutable, root)
	if err != nil {
		return checkpointError("unsupported-prove-history", "cannot read the worktree status")
	}
	head, tree, err := checkpointRevision(ctx, gitExecutable, root)
	if err != nil {
		return err
	}
	if head != commit || tree != revision || !equalStringSlices(dirtyBefore, dirtyAfter) {
		return checkpointError("unsupported-prove-drift", "the worktree or HEAD changed while the checkpoint was being proved")
	}
	return nil
}
