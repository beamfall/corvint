package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// pointerRoot is the protocol tree whose digest-pinned file pointers must
// resolve (CEM-V1-261).
const pointerRoot = "protocol/cem-1.0"

// recordedDanglingPointers are the digest-pinned pointers in sealed packet
// files that name a file this repository has never carried. They are
// recorded, not resolved: removing them rewrites sealed packet bytes, which is
// an owner decision (decision 0532). Each key is FILE#JSON-POINTER and each
// value the exact pointer text; an entry that stops being dangling, changes
// value or disappears from an existing file is itself a problem, so the list
// cannot silently outlive its reason.
var recordedDanglingPointers = map[string]string{
	"protocol/cem-1.0/stable/MIGRATION.json#/parentPlan":                                               "../migration-plan.json",
	"protocol/cem-1.0/stable/repository-envelope-packet/inherited/s0e/MIGRATION.json#/parentPlan":      "../migration-plan.json",
	"protocol/cem-1.0/stable/repository-envelope-packet/INPUTS.json#/repair6Routing/sourceReview/path": "../REVIEW-of-repair5.json",
}

// protocolPointer is one digest-pinned file reference found in a protocol
// JSON file.
type protocolPointer struct {
	file, pointer, value, sha256 string
}

// pointerProblems refuses a dangling protocol pointer (CEM-V1-261). A pointer
// is a string member K whose object also has a K+"SHA256" or K+"Sha256"
// string, or a "path" member that leaves its file's directory ("../") beside
// a "sha256" string. CEM map documents are wire data and are skipped. A
// pointer must resolve, relative to its file's directory and inside the
// repository, to a regular file with that SHA-256.
func pointerProblems(root string, recorded map[string]string) []string {
	var problems []string
	found := map[string]string{}
	var pointers []protocolPointer
	base := filepath.Join(root, filepath.FromSlash(pointerRoot))
	if _, err := os.Stat(base); err != nil {
		return nil
	}
	err := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".json") {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var v any
		if json.Unmarshal(data, &v) != nil {
			return nil // negative wire vectors are not valid JSON; they carry no pointer
		}
		if top, ok := v.(map[string]any); ok {
			if spec, ok := top["spec"].(string); ok && strings.HasPrefix(spec, "cem/") {
				return nil // a CEM map's artifact paths are relative to its artifact root, not to the file
			}
		}
		collectPointers(filepath.ToSlash(rel), "", v, &pointers)
		return nil
	})
	if err != nil {
		return []string{"protocol pointer walk: " + err.Error()}
	}
	for _, ptr := range pointers {
		key := ptr.file + "#" + ptr.pointer
		found[key] = ptr.value
		target := path.Clean(path.Join(path.Dir(ptr.file), ptr.value))
		if path.IsAbs(ptr.value) || target == ".." || strings.HasPrefix(target, "../") {
			problems = append(problems, fmt.Sprintf("%s: pointer %q leaves the repository", key, ptr.value))
			continue
		}
		b, err := readContained(root, target)
		if recordedValue, ok := recorded[key]; ok && recordedValue == ptr.value {
			if err == nil {
				problems = append(problems, fmt.Sprintf("%s: recorded dangling pointer %q now resolves; drop its record", key, ptr.value))
			}
			continue
		}
		if errors.Is(err, errNotContained) {
			problems = append(problems, fmt.Sprintf("%s: pointer %q is not a regular file inside the repository", key, ptr.value))
			continue
		}
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: dangling pointer %q (%s does not exist)", key, ptr.value, target))
			continue
		}
		if digest(b) != ptr.sha256 {
			problems = append(problems, fmt.Sprintf("%s: pointer %q does not match its pinned SHA-256", key, ptr.value))
		}
	}
	keys := make([]string, 0, len(recorded))
	for key := range recorded {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		file := key[:strings.Index(key, "#")]
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(file))); err != nil {
			continue // a partial tree (synthetic packet) does not carry the file
		}
		if value, ok := found[key]; !ok || value != recorded[key] {
			problems = append(problems, fmt.Sprintf("%s: recorded dangling pointer %q is no longer present", key, recorded[key]))
		}
	}
	return problems
}

// errNotContained marks a pointer target that is a symbolic link, not a
// regular file, or reached through a directory link outside the repository.
var errNotContained = errors.New("not a regular file inside the repository")

// readContained reads the repository-relative regular file target. The
// lexical containment check in pointerProblems is not enough on its own: a
// symbolic link inside the repository could still lead outside it.
func readContained(root, target string) ([]byte, error) {
	full := filepath.Join(root, filepath.FromSlash(target))
	info, err := os.Lstat(full)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errNotContained
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return nil, err
	}
	if rel, err := filepath.Rel(resolvedRoot, resolved); err != nil || filepath.ToSlash(rel) != target {
		return nil, errNotContained
	}
	return os.ReadFile(full)
}

// collectPointers appends every digest-pinned pointer under v in a stable
// order. pointer is the RFC 6901 JSON pointer of v.
func collectPointers(file, pointer string, v any, out *[]protocolPointer) {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			child := pointer + "/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(k)
			if s, ok := x[k].(string); ok {
				for _, suffix := range []string{"SHA256", "Sha256"} {
					if sum, ok := x[k+suffix].(string); ok {
						*out = append(*out, protocolPointer{file, child, s, sum})
					}
				}
				if sum, ok := x["sha256"].(string); ok && k == "path" && strings.HasPrefix(s, "../") {
					*out = append(*out, protocolPointer{file, child, s, sum})
				}
			}
			collectPointers(file, child, x[k], out)
		}
	case []any:
		for i, e := range x {
			collectPointers(file, fmt.Sprintf("%s/%d", pointer, i), e, out)
		}
	}
}
