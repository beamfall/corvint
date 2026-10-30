package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	cemcli "github.com/Beamfall/corvint/internal/cem/cli"
	"github.com/Beamfall/corvint/internal/cem/mdreport"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/cem/witnesspaths"
	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/lrfrepo"
	"github.com/Beamfall/corvint/internal/witnesspaths/datareaders"
)

type ocmCLIOptions struct {
	action        string
	obligation    string
	testPath      string
	hunks         []string
	claims        []string
	reason        string
	intent        string
	intentForm    string
	replace       bool
	mapPath       string
	cemPath       string
	expectedBase  string
	expectedGiven bool
	target        string
	targetGiven   bool
	maxUnknown    *int
	output        string
}

// emitOCMInputOrError renders the oracle's code-less envelope for a CLI-level
// input read failure, which the oracle raises before its OCM module runs, and
// the ordinary coded envelope for everything else.
func emitOCMInputOrError(stderr io.Writer, err error) {
	var inputError *lrfrepo.Error
	if lrfrepo.CodeOf(err) == "ocm-cli-error" && errors.As(err, &inputError) {
		_, _ = fmt.Fprintf(stderr, "{\"error\": %s, \"ok\": false}\n", pythonJSONString(inputError.Message))
		return
	}
	emitOCMError(stderr, err)
}

func emitOCMEnvelope(stdout, stderr io.Writer, envelope map[string]any) int {
	encoded, err := json.Marshal(envelope)
	if err != nil {
		emitOCMError(stderr, &lrfrepo.Error{Code: "output-failed", Message: "cannot encode OCM output"})
		return 2
	}
	encoded = escapeASCIIJSON(encoded)
	if _, err := fmt.Fprintf(stdout, "%s\n", encoded); err != nil {
		emitOCMError(stderr, &lrfrepo.Error{Code: "output-failed", Message: "cannot write OCM output"})
		return 2
	}
	if envelope["ok"] != true {
		return 1
	}
	return 0
}

func parseOCMInvocation(arguments []string) (string, []string, bool, error) {
	index := 0
	root := ""
	for index < len(arguments) && (arguments[index] == "--root" || strings.HasPrefix(arguments[index], "--root=")) {
		if arguments[index] == "--root" {
			if !rootPreambleValue(arguments, index+1) {
				return "", nil, false, nil
			}
			root = arguments[index+1]
			index += 2
		} else {
			root = strings.TrimPrefix(arguments[index], "--root=")
			index++
		}
	}
	if index >= len(arguments) || arguments[index] != "ocm" {
		return "", nil, false, nil
	}
	if root == "" {
		workingDirectory, err := os.Getwd()
		if err != nil {
			return "", nil, true, argumentError("cannot resolve current directory")
		}
		root = workingDirectory
	} else {
		resolved, err := normalizeRoot(root)
		if err != nil {
			return "", nil, true, err
		}
		root = resolved
	}
	return root, arguments[index+1:], true, nil
}

func runOCM(ctx context.Context, root string, arguments []string, stdout, stderr io.Writer) int {
	options, err := parseOCMFlags(arguments)
	if err != nil {
		if lrfrepo.CodeOf(err) != "" {
			emitOCMError(stderr, err)
		} else {
			emitError(stderr, err)
		}
		return 2
	}
	switch options.action {
	case "mark":
		return runOCMMark(ctx, root, options, stdout, stderr)
	case "prepare":
		return runOCMPrepare(ctx, root, options, stdout, stderr)
	case "link":
		return runOCMLink(ctx, root, options, stdout, stderr)
	}
	checked, err := lrfrepo.ReadOCM(ctx, root, lrfrepo.OCMReadOptions{
		OCMPath: options.mapPath, CEMPath: options.cemPath,
		ExpectedBase: options.expectedBase, Target: options.target,
		ExpectedBaseGiven: options.expectedGiven, TargetGiven: options.targetGiven,
		MaxUnknown: options.maxUnknown,
	})
	if err != nil {
		emitOCMInputOrError(stderr, err)
		return 2
	}
	valid := checked.Verification["valid"] == true
	ok := valid && len(checked.PolicyIssues) == 0
	var envelope map[string]any
	switch options.action {
	case "verify":
		envelope = map[string]any{
			"ok": ok, "mutates": false, "tool": "ocm-verify",
			"verification": checked.Verification, "counts": checked.Counts,
			"policyIssues": checked.PolicyIssues, "testExecution": checked.TestExecution,
		}
	case "status":
		envelope = map[string]any{
			"ok": checked.State == "ready-for-review", "mutates": false, "tool": "ocm-status",
			"map": checked.OCMAbsolute, "cem": checked.CEMAbsolute, "state": checked.State,
			"counts": checked.Counts, "worklist": checked.Worklist,
			"policyIssues": checked.PolicyIssues, "verification": checked.Verification,
			"testExecution": checked.TestExecution,
		}
	case "report":
		report := renderOCMReport(checked)
		path, writeErr := lrfrepo.PublishOCMReport(ctx, root, options.output, report, ok)
		if writeErr != nil {
			if lrfrepo.CodeOf(writeErr) == "ocm-cli-error" {
				var outputError *lrfrepo.Error
				if errors.As(writeErr, &outputError) {
					_, _ = fmt.Fprintf(stderr, "{\"error\": %s, \"ok\": false}\n", pythonJSONString(outputError.Message))
				} else {
					emitOCMError(stderr, writeErr)
				}
			} else {
				emitOCMError(stderr, writeErr)
			}
			return 2
		}
		envelope = map[string]any{
			"ok": ok, "mutates": true, "tool": "ocm-report", "report": path,
			"counts": checked.Counts, "policyIssues": checked.PolicyIssues,
			"verification": checked.Verification, "testExecution": checked.TestExecution,
		}
	}
	return emitOCMEnvelope(stdout, stderr, envelope)
}

// parseOCMMarkFlags mirrors the oracle's mark subparser: --map, --obligation,
// and --reason are required, --reason is a bounded choice, and --output is
// optional. The argparse stages run in the oracle's order.
func parseOCMMarkFlags(result ocmCLIOptions, arguments []string) (ocmCLIOptions, error) {
	present := map[string]bool{}
	var unrecognized []string
	for index := 0; index < len(arguments); {
		name, value, inline := strings.Cut(arguments[index], "=")
		target := map[string]*string{
			"--map": &result.mapPath, "--obligation": &result.obligation,
			"--reason": &result.reason, "--output": &result.output,
		}[name]
		if target == nil {
			unrecognized = append(unrecognized, arguments[index])
			index++
			continue
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return result, argumentError("argument " + name + ": expected one argument")
			}
			value = arguments[index+1]
			index += 2
		} else {
			index++
		}
		if name == "--reason" && !containsChoice(lrfrepo.UnknownReasons(), value) {
			return result, invalidChoice("--reason", value, lrfrepo.UnknownReasons())
		}
		present[name] = true
		*target = value
	}
	var missing []string
	for _, name := range []string{"--map", "--obligation", "--reason"} {
		if !present[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		return result, argumentError("the following arguments are required: " + strings.Join(missing, ", "))
	}
	if len(unrecognized) != 0 {
		return result, argumentError("unrecognized arguments: " + strings.Join(unrecognized, " "))
	}
	if !present["--output"] {
		result.output = ""
	}
	return result, nil
}

func containsChoice(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func runOCMMark(ctx context.Context, root string, options ocmCLIOptions, stdout, stderr io.Writer) int {
	result, err := lrfrepo.MarkOCM(ctx, root, lrfrepo.MarkOptions{
		MapPath: options.mapPath, Output: options.output,
		Obligation: options.obligation, Reason: options.reason,
	})
	if err != nil {
		emitOCMInputOrError(stderr, err)
		return 2
	}
	return emitOCMEnvelope(stdout, stderr, map[string]any{
		"ok": true, "mutates": true, "tool": "ocm-mark", "map": result.MapPath,
		"obligationId": result.ObligationID, "disposition": "unknown", "reason": options.reason,
	})
}

func parseOCMFlags(arguments []string) (ocmCLIOptions, error) {
	result := ocmCLIOptions{cemPath: ".corvint/change.cem.json", output: lrfrepo.DefaultOCMReportPath}
	if len(arguments) == 0 {
		return result, argumentError("the following arguments are required: ocm_command")
	}
	result.action = arguments[0]
	switch result.action {
	case "link":
		return parseOCMLinkFlags(result, arguments[1:])
	case "prepare":
		return parseOCMPrepareFlags(result, arguments[1:])
	case "mark":
		return parseOCMMarkFlags(result, arguments[1:])
	case "status", "verify", "report":
	default:
		return result, argumentError("argument ocm_command: invalid choice: " + pythonRepr(result.action))
	}
	for index := 1; index < len(arguments); {
		name, value, inline := strings.Cut(arguments[index], "=")
		allowed := name == "--map" || name == "--cem" || name == "--max-unknown" ||
			name == "--expected-base" || name == "--target" || name == "--output" && result.action == "report"
		if !allowed {
			return result, argumentError("unrecognized arguments: " + strings.Join(arguments[index:], " "))
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return result, argumentError("argument " + name + ": expected one argument")
			}
			value = arguments[index+1]
			index += 2
		} else {
			index++
		}
		switch name {
		case "--map":
			result.mapPath = value
		case "--cem":
			result.cemPath = value
		case "--expected-base":
			result.expectedBase = value
			result.expectedGiven = true
		case "--target":
			result.target = value
			result.targetGiven = true
		case "--output":
			result.output = value
		case "--max-unknown":
			// OCM maps admit at most 256 obligations. Saturating larger Python
			// integers at 257 preserves every possible threshold verdict.
			parsed, ok := pythonBoundedInteger(value, 256)
			if !ok {
				return result, argumentError("argument --max-unknown: value must be an integer")
			}
			if parsed < 0 {
				return result, argumentError("argument --max-unknown: value must be non-negative")
			}
			result.maxUnknown = &parsed
		}
	}
	if result.mapPath == "" {
		return result, argumentError("the following arguments are required: --map")
	}
	return result, nil
}

func renderOCMReport(checked *lrfrepo.OCMReadResult) []byte {
	target := "unavailable"
	if value, ok := checked.Verification["targetRevision"].(string); ok && value != "" {
		target = value
	}
	var output strings.Builder
	output.WriteString("# Obligation Closure Map review report\n\n")
	fmt.Fprintf(&output, "- Structural state: `%s`\n", checked.State)
	fmt.Fprintf(&output, "- Target revision: `%s`\n", target)
	fmt.Fprintf(&output, "- Obligations: `%v` total, `%v` linked, `%v` unknown\n",
		checked.Counts["total"], checked.Counts["linked"], checked.Counts["unknown"])
	fmt.Fprintf(&output, "- Test execution: `%v`\n\n", checked.TestExecution["state"])
	output.WriteString("Structural linkage does not assert implementation correctness, test adequacy, or a passing project gate.\n\n")
	output.WriteString("## Obligation worklist\n\n")
	output.WriteString("| # | Obligation | Disposition | Reason | Hunks | Claims |\n")
	output.WriteString("|---:|---|---|---|---:|---:|\n")
	for _, value := range checked.Worklist {
		item := value.(map[string]any)
		hunks, _ := item["hunkIds"].([]any)
		claims, _ := item["claimIds"].([]any)
		fmt.Fprintf(&output, "| %v | `%v` | `%v` | `%v` | %d | %d |\n",
			item["selector"], item["id"], item["disposition"], item["reason"], len(hunks), len(claims))
	}
	output.WriteString("\n## Verification issues\n\n")
	issues, _ := checked.Verification["issues"].([]any)
	policy, _ := checked.Verification["policyIssues"].([]any)
	combined := append(append([]any{}, issues...), policy...)
	if len(combined) == 0 {
		output.WriteString("- None.\n")
	} else {
		for _, value := range combined {
			item := value.(map[string]any)
			if subject, ok := item["subject"].(string); ok && subject != "" {
				fmt.Fprintf(&output, "- `%v` (%s)\n", item["code"], mdreport.CodeSpan(subject))
			} else {
				fmt.Fprintf(&output, "- `%v`\n", item["code"])
			}
		}
	}
	return []byte(output.String())
}

func emitOCMError(stderr io.Writer, err error) {
	code := lrfrepo.CodeOf(err)
	if code == "" {
		code = "internal-error"
	}
	message := err.Error()
	var adapter *lrfrepo.Error
	if errors.As(err, &adapter) {
		message = adapter.Message
	}
	var registered *cemcode.Error
	if errors.As(err, &registered) {
		message = registered.Message
	}
	_, _ = fmt.Fprintf(stderr, "{\"code\": %s, \"error\": %s, \"ok\": false}\n",
		wire.CanonicalString(code), wire.CanonicalString(message))
}

// parseOCMPrepareFlags mirrors the oracle's prepare subparser. --map and --cem
// carry defaults, so only --target and --intent are required.
func parseOCMPrepareFlags(result ocmCLIOptions, arguments []string) (ocmCLIOptions, error) {
	result.mapPath = lrfrepo.DefaultOCMMapPath
	present := map[string]bool{}
	var unrecognized []string
	for index := 0; index < len(arguments); {
		name, value, inline := strings.Cut(arguments[index], "=")
		if name == "--replace" {
			result.replace = true
			index++
			continue
		}
		target := map[string]*string{
			"--target": &result.target, "--expected-base": &result.expectedBase,
			"--intent": &result.intent, "--cem": &result.cemPath, "--map": &result.mapPath,
			"--max-unknown": new(string), "--intent-form": &result.intentForm,
		}[name]
		if target == nil {
			unrecognized = append(unrecognized, arguments[index])
			index++
			continue
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return result, argumentError("argument " + name + ": expected one argument")
			}
			value = arguments[index+1]
			index += 2
		} else {
			index++
		}
		if name == "--max-unknown" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				return result, argumentError("argument --max-unknown: value must be non-negative")
			}
			result.maxUnknown = &parsed
		}
		present[name] = true
		*target = value
	}
	var missing []string
	for _, name := range []string{"--target", "--intent"} {
		if !present[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		return result, argumentError("the following arguments are required: " + strings.Join(missing, ", "))
	}
	if len(unrecognized) != 0 {
		return result, argumentError("unrecognized arguments: " + strings.Join(unrecognized, " "))
	}
	form, known := ocmIntentForms[result.intentForm]
	if !known {
		return result, argumentError("argument --intent-form: invalid choice: " + result.intentForm)
	}
	result.intentForm = form
	result.expectedGiven = present["--expected-base"]
	return result, nil
}

// ocmIntentForms maps each experimental OIF-V0 --intent-form choice to the
// library form; requirements and an absent flag are the default form.
var ocmIntentForms = map[string]string{
	"": "", "requirements": "", lrfrepo.IntentFormADR: lrfrepo.IntentFormADR,
	lrfrepo.IntentFormRoadmap: lrfrepo.IntentFormRoadmap, lrfrepo.IntentFormOpenSpecSpec: lrfrepo.IntentFormOpenSpecSpec,
	lrfrepo.IntentFormOpenSpecDelta: lrfrepo.IntentFormOpenSpecDelta, lrfrepo.IntentFormSpecKit: lrfrepo.IntentFormSpecKit,
}

func runOCMPrepare(ctx context.Context, root string, options ocmCLIOptions, stdout, stderr io.Writer) int {
	prepared, err := lrfrepo.PrepareOCM(ctx, root, lrfrepo.PrepareOptions{
		MapPath: options.mapPath, CEMPath: options.cemPath, IntentPath: options.intent,
		Target: options.target, ExpectedBase: options.expectedBase,
		Replace: options.replace, MaxUnknown: options.maxUnknown, IntentForm: options.intentForm,
	})
	if err != nil {
		emitOCMInputOrError(stderr, err)
		return 2
	}
	counts, worklist := lrfrepo.OCMCountsAndWorklist(prepared.Document)
	var next any
	for _, item := range worklist {
		if entry, ok := item.(map[string]any); ok && entry["next"] != "done" {
			next = entry
			break
		}
	}
	envelope := map[string]any{
		"ok": true, "mutates": true, "tool": "ocm-prepare",
		"targetRevision": prepared.Target, "intentScope": prepared.IntentScope,
		"map": prepared.MapAbsolute, "cem": prepared.CEMAbsolute, "resumed": prepared.Resumed,
		"counts": counts, "worklist": worklist, "nextObligation": next,
		"nextActions": []any{prepared.StatusAction},
		// prepare reports observed alongside state; the read actions report only
		// state. The oracle differs the same way.
		"testExecution": map[string]any{"observed": false, "state": "NOT_RUN"},
	}
	addIntentFormMembers(envelope, options.intentForm, prepared)
	return emitOCMEnvelope(stdout, stderr, envelope)
}

// addIntentFormMembers adds the prepare envelope members only a declared form
// emits (OIF-V0-008, STA-V0-003 to STA-V0-005); the default form adds none.
func addIntentFormMembers(envelope map[string]any, form string, prepared *lrfrepo.PrepareResult) {
	switch form {
	case lrfrepo.IntentFormRoadmap:
		envelope["excludedTickets"] = append([]string{}, prepared.ExcludedTickets...)
	case lrfrepo.IntentFormOpenSpecSpec, lrfrepo.IntentFormOpenSpecDelta:
		envelope["nativeNames"] = prepared.NativeNames
	}
	if form == lrfrepo.IntentFormOpenSpecDelta || form == lrfrepo.IntentFormSpecKit {
		envelope["excludedRequirements"] = append([]map[string]string{}, prepared.ExcludedRequirements...)
	}
	if form == lrfrepo.IntentFormSpecKit && prepared.NativeStatus != nil {
		envelope["nativeStatus"] = *prepared.NativeStatus
	}
}

// parseOCMLinkFlags mirrors the oracle's link subparser. --hunk and --claim are
// append actions, so they may repeat and each occurrence is kept.
func parseOCMLinkFlags(result ocmCLIOptions, arguments []string) (ocmCLIOptions, error) {
	present := map[string]bool{}
	var unrecognized []string
	for index := 0; index < len(arguments); {
		name, value, inline := strings.Cut(arguments[index], "=")
		single := map[string]*string{
			"--map": &result.mapPath, "--cem": &result.cemPath, "--obligation": &result.obligation,
			"--test-path": &result.testPath, "--expected-base": &result.expectedBase,
			"--target": &result.target, "--output": &result.output,
		}[name]
		if single == nil && name != "--hunk" && name != "--claim" {
			unrecognized = append(unrecognized, arguments[index])
			index++
			continue
		}
		if !inline {
			if index+1 >= len(arguments) || argparseOptionLike(arguments[index+1]) {
				return result, argumentError("argument " + name + ": expected one argument")
			}
			value = arguments[index+1]
			index += 2
		} else {
			index++
		}
		present[name] = true
		switch name {
		case "--hunk":
			result.hunks = append(result.hunks, value)
		case "--claim":
			result.claims = append(result.claims, value)
		default:
			*single = value
		}
	}
	var missing []string
	for _, name := range []string{"--map", "--obligation", "--hunk", "--test-path", "--claim"} {
		if !present[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) != 0 {
		return result, argumentError("the following arguments are required: " + strings.Join(missing, ", "))
	}
	if len(unrecognized) != 0 {
		return result, argumentError("unrecognized arguments: " + strings.Join(unrecognized, " "))
	}
	result.expectedGiven = present["--expected-base"]
	result.targetGiven = present["--target"]
	if !present["--output"] {
		// The shared initializer defaults output to the REPORT path; link writes a
		// map, so an absent --output means the map path, not that default.
		result.output = ""
	}
	return result, nil
}

func runOCMLink(ctx context.Context, root string, options ocmCLIOptions, stdout, stderr io.Writer) int {
	linked, err := lrfrepo.LinkOCM(ctx, root, lrfrepo.LinkOptions{
		MapPath: options.mapPath, CEMPath: options.cemPath, Output: options.output,
		Obligation: options.obligation, Hunks: options.hunks, TestPath: options.testPath,
		Claims: options.claims, ExpectedBase: options.expectedBase, Target: options.target,
	})
	if err != nil {
		emitOCMInputOrError(stderr, err)
		return 2
	}
	return emitOCMEnvelope(stdout, stderr, map[string]any{
		"ok": true, "mutates": true, "tool": "ocm-link", "map": linked.MapPath,
		"obligationId": linked.ObligationID,
		"hunkIds":      toAnySlice(linked.HunkIDs), "claimIds": toAnySlice(linked.ClaimIDs),
	})
}

func toAnySlice(values []string) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func init() {
	cemcli.StableOCM = readStableOCMReview
	cemcli.IntentObligations = countIntentObligations
	cemcli.DataReaders = findPathPolicyDataReaders
}

// findPathPolicyDataReaders builds the target revision's index once and finds
// a declared data path's readers in it exactly as corvint witness does
// (AGW-V0-044, CAX-V0-064).
func findPathPolicyDataReaders(ctx context.Context, root, target string) (func(string, []string) []witnesspaths.Reader, error) {
	index, err := contextindex.BuildRevisionContext(ctx, root, target)
	if err != nil {
		return nil, err
	}
	unparsed := index.UnparsedPaths()
	return func(dataPath string, declared []string) []witnesspaths.Reader {
		return datareaders.Find(dataPath, declared, index, unparsed)
	}, nil
}

// countIntentObligations counts one intent's requirement IDs by the OCM-V0-001
// rule for a recorded intent-scope deferral (CEM-V1-644, CEM-V1-645).
func countIntentObligations(data []byte) (int, error) {
	requirements, _, _, _, err := lrfrepo.IntentRequirements(data)
	if err != nil {
		return 0, err
	}
	return len(requirements), nil
}

// readStableOCMReview verifies supplied OCM bytes against an exact cem/1.0
// map through lrfrepo.VerifyStableUniverse and returns the review OCM
// projection with the 0.x reader's field shape. A coded refusal is an invalid
// projection naming the code; the stable route never produces policyIssues and
// keeps testExecution NOT_RUN. The report binds mapSha256 and cemMapSha256 to
// the bytes it read (V1-1305).
func readStableOCMReview(ctx context.Context, repository string, cemRaw, ocmRaw []byte, expectedBase, target, artifactRoot string) (map[string]any, error) {
	projection := map[string]any{
		"testExecution": map[string]any{"state": "NOT_RUN"}, "obligations": []any{}, "claims": []any{},
	}
	universe, err := lrfrepo.VerifyStableUniverse(ctx, cemRaw, ocmRaw, lrfrepo.StableUniverseOptions{
		Root: repository, ExpectedBase: expectedBase, Target: target, ArtifactRoot: artifactRoot,
	})
	if err != nil {
		code := lrfrepo.CodeOf(err)
		if code == "" {
			return nil, err
		}
		projection["valid"], projection["state"] = false, "invalid"
		projection["verification"] = stableOCMFailure(code, strings.TrimPrefix(err.Error(), code+": "))
		if code == "invalid-path" || code == cemcode.RepositoryObjectUnavailable {
			projection["testExecution"] = map[string]any{"observed": false, "state": "NOT_RUN"}
		}
		return projection, nil
	}
	var document map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(ocmRaw)))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	rows := make([]any, 0, len(universe.Obligations))
	linked, unknown := 0, 0
	for index, obligation := range universe.Obligations {
		next := "link-or-review-unknown"
		switch obligation.Disposition {
		case "linked":
			linked++
			next = "done"
		case "unknown":
			unknown++
		}
		rows = append(rows, map[string]any{
			"selector": index + 1, "id": obligation.ID, "disposition": obligation.Disposition, "reason": obligation.Reason,
			"hunkIds": toAnySlice(obligation.HunkIDs), "claimIds": toAnySlice(obligation.ClaimIDs), "next": next,
		})
	}
	projection["valid"], projection["state"] = true, "ready-for-review"
	projection["verification"] = map[string]any{
		"valid": true, "spec": document["spec"], "targetRevision": universe.TargetRevision,
		"structuralCoverage": map[string]any{"total": len(rows), "linked": linked, "unknown": unknown},
		"testExecution":      map[string]any{"state": "NOT_RUN"}, "issues": []any{}, "policyIssues": []any{},
	}
	projection["obligations"] = rows
	for _, field := range []string{"claims", "intentScope", "targetRevision"} {
		if member, found := document[field]; found {
			projection[field] = member
		}
	}
	return projection, nil
}

// stableOCMFailure is the invalid OCM verification with the 0.x reader's
// failure shape (internal/lrfrepo finishOCMFailure).
func stableOCMFailure(code, message string) map[string]any {
	issue := map[string]any{"code": code}
	if message != "" {
		issue["message"] = message
	}
	if code == "invalid-path" || code == cemcode.RepositoryObjectUnavailable {
		return map[string]any{"valid": false, "issues": []any{map[string]any{"code": code}}}
	}
	return map[string]any{
		"valid": false, "spec": "ocm/0.1-experimental", "targetRevision": "",
		"structuralCoverage": map[string]any{"total": 0, "linked": 0, "unknown": 0},
		"testExecution":      map[string]any{"state": "NOT_RUN"},
		"issues":             []any{issue},
		"policyIssues":       []any{},
	}
}
