// Command cem-citation-baseline reproduces the per-map citation baseline
// (docs/build-log/2026-10-12-cem-citation-baseline-593-maps.md) over a
// directory of committed change maps and splits it by citation class
// (docs/specs/daily-change-evidence-workflow-v0.md, DCW-V0-049).
//
// For every MAPS/<bind>.cem.json, in name order, it runs the in-process
// equivalent of
//
//	corvint cem status --map MAPS/<bind>.cem.json --expected-base <baseRevision> --target <bind> --citation
//
// and writes one JSON row per map to stdout, in the baseline row shape plus a
// "class" member. A cem/1.0 map takes its class from the status envelope's
// citationClass (CEM-V1-106); a 0.x map, which carries no role, is classed by
// the paths of base..bind (dogfoodflow.ChoreRole, DCW-V0-048). A map whose
// status carries no citation block is an error row and an ERR line on stderr.
// --summary writes the aggregate for all maps and for each class.
//
// Usage:
//
//	cem-citation-baseline [--root DIR] [--maps DIR] [--summary FILE]
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/dogfoodflow"
)

// Citation classes, as `cem status --citation` names them for cem/1.0 maps.
const (
	classChange = "change"
	classChore  = "chore"
)

// tiers are the citation tiers, in the baseline's key order.
var tiers = []string{"anchored", "cited", "held", "mechanical-proved", "unknown"}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("cem-citation-baseline", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "repository root")
	maps := flags.String("maps", ".corvint/changes", "map directory, relative to --root")
	summaryPath := flags.String("summary", "", "write the aggregate summary JSON to this file")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: cem-citation-baseline [--root DIR] [--maps DIR] [--summary FILE]")
		return 2
	}
	absRoot, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	names, err := filepath.Glob(filepath.Join(absRoot, *maps, "*.cem.json"))
	if err != nil || len(names) == 0 {
		fmt.Fprintf(stderr, "no *.cem.json maps under %s\n", filepath.Join(absRoot, *maps))
		return 2
	}
	sort.Strings(names)
	var rows []row
	for _, name := range names {
		r := measure(ctx, absRoot, name)
		rows = append(rows, r)
		fmt.Fprintln(stdout, pyJSON(r.fields(), -1, 0))
		if r.citation == nil {
			fmt.Fprintf(stderr, "ERR %s %s\n", r.stem, pyJSON(r.issues, -1, 0))
		}
	}
	if *summaryPath != "" {
		data := pyJSON(summarize(rows), 1, 0) + "\n"
		if err := os.WriteFile(*summaryPath, []byte(data), 0o644); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	return 0
}

// row is one map's measurement.
type row struct {
	stem     string
	spec     string
	class    string
	valid    bool
	issues   []any
	citation map[string]any
}

// readMap reads one regular map file of at most the native map bound, the
// bound the status verifier applies, without following a symbolic link.
func readMap(name string) ([]byte, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("map is not a regular file")
	}
	if info.Size() > wire.MaxMapBytes {
		return nil, errors.New("map exceeds the " + strconv.Itoa(wire.MaxMapBytes) + " byte bound")
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, wire.MaxMapBytes+1))
	if err == nil && len(raw) > wire.MaxMapBytes {
		err = errors.New("map exceeds the " + strconv.Itoa(wire.MaxMapBytes) + " byte bound")
	}
	return raw, err
}

func measure(ctx context.Context, root, name string) row {
	stem := strings.TrimSuffix(filepath.Base(name), ".cem.json")
	r := row{stem: stem, issues: []any{}}
	raw, err := readMap(name)
	var head struct {
		Spec         string `json:"spec"`
		BaseRevision string `json:"baseRevision"`
	}
	if err == nil {
		err = json.Unmarshal(raw, &head)
	}
	if err != nil {
		r.issues = []any{issue("map-unreadable", err.Error())}
		return r
	}
	r.spec = head.Spec
	rel, _ := filepath.Rel(root, name)
	var out, errb bytes.Buffer
	cli.Run(ctx, root, []string{"status", "--map", filepath.ToSlash(rel), "--expected-base", head.BaseRevision, "--target", stem, "--citation"}, &out, &errb)
	var envelope map[string]any
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		if err := json.Unmarshal(errb.Bytes(), &envelope); err != nil {
			r.issues = []any{issue("status-unreadable", strings.TrimSpace(errb.String()))}
			return r
		}
	}
	if stable, ok := envelope["stableVerification"].(map[string]any); ok {
		r.valid = stable["outcome"] == "ACCEPT"
		r.citation, _ = stable["citation"].(map[string]any)
		if class, ok := envelope["citationClass"].(map[string]any); ok {
			r.class, _ = class["class"].(string)
		}
		if codes, ok := stable["issueCodes"].([]any); ok {
			for _, code := range codes {
				r.issues = append(r.issues, issue(fmt.Sprint(code), ""))
			}
		}
		if code, ok := stable["code"].(string); ok && len(r.issues) == 0 {
			r.issues = []any{issue(code, "")}
		}
	} else if verification, ok := envelope["verification"].(map[string]any); ok {
		r.valid = verification["valid"] == true
		r.citation, _ = verification["citation"].(map[string]any)
		if list, ok := verification["issues"].([]any); ok {
			r.issues = list
		}
		r.class, err = legacyClass(ctx, root, head.BaseRevision, stem)
		if err != nil {
			r.issues = append(r.issues, issue("class-unresolved", err.Error()))
		}
	} else {
		code, _ := envelope["code"].(string)
		message, _ := envelope["error"].(string)
		r.issues = []any{issue(code, message)}
	}
	return r
}

// legacyClass classes a 0.x map, which carries no role, by the paths its
// change touches: a change touching only CEM map paths is a chore.
func legacyClass(ctx context.Context, root, base, target string) (string, error) {
	command := exec.CommandContext(ctx, "git", "-C", root, "diff", "--no-renames", "--name-only", base, target)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		return "", errors.New(strings.TrimSpace(stderr.String()))
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if line != "" {
			paths = append(paths, line)
		}
	}
	if dogfoodflow.ChoreRole(paths) != "" {
		return classChore, nil
	}
	return classChange, nil
}

func issue(code, message string) map[string]any {
	return map[string]any{"code": code, "message": message}
}

// fields is the baseline row: the citation counts for a map with a citation
// block, otherwise the error shape, each with the map's class.
func (r row) fields() map[string]any {
	if r.citation == nil {
		return map[string]any{"class": r.class, "hunksTotal": 0, "issues": r.issues, "map": r.stem, "spec": r.spec, "valid": false}
	}
	hunks, _ := r.citation["hunks"].([]any)
	return map[string]any{
		"annotationHunks":   r.citation["annotationHunks"],
		"class":             r.class,
		"concentratedHunks": r.citation["concentratedHunks"],
		"distinctEvidence":  r.citation["distinctEvidence"],
		"hunksByClass":      r.citation["hunksByClass"],
		"hunksTotal":        len(hunks),
		"invariant":         r.citation["invariant"],
		"map":               r.stem,
		"maxFanOut":         r.citation["maxFanOut"],
		"spec":              r.spec,
		"specificity":       r.citation["specificity"],
		"valid":             r.valid,
	}
}

// summarize aggregates every map, then each class, in the baseline summary
// shape.
func summarize(rows []row) ordered {
	byClass := map[string][]row{}
	for _, r := range rows {
		byClass[r.class] = append(byClass[r.class], r)
	}
	out := ordered{{"all", aggregate(rows)}, {classChange, aggregate(byClass[classChange])}, {classChore, aggregate(byClass[classChore])}}
	if unresolved := byClass[""]; len(unresolved) > 0 {
		out = append(out, member{"unclassified", aggregate(unresolved)})
	}
	return out
}

func aggregate(rows []row) ordered {
	tierCount := map[string]int{}
	var withCitation, invalid, hunks, annotations, invariant, positive, zero, negative int
	specificity, fanOut := map[int]int{}, map[int]int{}
	for _, r := range rows {
		if !r.valid || r.citation == nil {
			invalid++
		}
		if r.citation == nil {
			continue
		}
		withCitation++
		list, _ := r.citation["hunks"].([]any)
		hunks += len(list)
		classes, _ := r.citation["hunksByClass"].(map[string]any)
		for _, tier := range tiers {
			tierCount[tier] += number(classes[tier])
		}
		annotations += number(r.citation["annotationHunks"])
		if r.citation["invariant"] == true {
			invariant++
		}
		value := number(r.citation["specificity"])
		switch {
		case value > 0:
			positive++
		case value == 0:
			zero++
		default:
			negative++
		}
		specificity[value]++
		fanOut[number(r.citation["maxFanOut"])]++
	}
	tierMembers := ordered{}
	for _, tier := range tiers {
		tierMembers = append(tierMembers, member{tier, tierCount[tier]})
	}
	return ordered{
		{"maps", len(rows)},
		{"withCitation", withCitation},
		{"invalidOrError", invalid},
		{"hunks", hunks},
		{"tiers", tierMembers},
		{"annotationHunks", annotations},
		{"invariantMaps", invariant},
		{"specificity", ordered{{"positive", positive}, {"zero", zero}, {"negative", negative}}},
		{"specificityHistogram", histogram(specificity)},
		{"maxFanOutHistogram", histogram(fanOut)},
	}
}

func histogram(counts map[int]int) ordered {
	keys := make([]int, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	out := ordered{}
	for _, key := range keys {
		out = append(out, member{strconv.Itoa(key), counts[key]})
	}
	return out
}

func number(value any) int {
	if f, ok := value.(float64); ok {
		return int(f)
	}
	return 0
}

// member and ordered keep the summary's member order.
type member struct {
	key   string
	value any
}

type ordered []member

// pyJSON encodes like the baseline's Python json.dumps: ", " and ": "
// separators on one line (indent < 0, map keys sorted), or one member per
// line at the given indent; ordered keeps its own order.
func pyJSON(value any, indent, depth int) string {
	var members []member
	switch v := value.(type) {
	case ordered:
		members = v
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			members = append(members, member{key, v[key]})
		}
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = pyJSON(item, indent, depth+1)
		}
		return join("[", "]", parts, indent, depth)
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
	parts := make([]string, len(members))
	for i, m := range members {
		key, _ := json.Marshal(m.key)
		parts[i] = string(key) + ": " + pyJSON(m.value, indent, depth+1)
	}
	return join("{", "}", parts, indent, depth)
}

func join(open, close string, parts []string, indent, depth int) string {
	if len(parts) == 0 {
		return open + close
	}
	if indent < 0 {
		return open + strings.Join(parts, ", ") + close
	}
	inner := "\n" + strings.Repeat(" ", indent*(depth+1))
	return open + inner + strings.Join(parts, ","+inner) + "\n" + strings.Repeat(" ", indent*depth) + close
}
