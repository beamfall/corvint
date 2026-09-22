package genesis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	Profile         = "genesis-inventory/0.1-experimental"
	SummaryProfile  = "genesis-inventory-summary/0.1-experimental"
	maxSummaryBytes = 16 * 1024
)

var authorityCharacters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._:/-"

// CompileRepositoryInventory compiles the frozen read-only Genesis receipt.
func CompileRepositoryInventory(ctx context.Context, root, activation string, authorityID *string, revision string, excludedPrefixes []string) map[string]any {
	limits := defaultLimits()
	if activation != "init" && activation != "adopt" {
		return invalidReceipt(activation, authorityID, "invalid-activation", limits)
	}
	if authorityID != nil && !validAuthority(*authorityID) {
		return invalidReceipt(activation, authorityID, "invalid-authority-id", limits)
	}
	exclusions, err := normalizeExclusions(excludedPrefixes, limits.MaxPathBytes)
	if err != nil {
		return invalidReceipt(activation, authorityID, "invalid-exclusions", limits)
	}
	repo, err := openRepository(ctx, root, limits, revision)
	if err != nil {
		return invalidReceipt(activation, authorityID, errorCode(err, "invalid-repository"), limits)
	}
	commit, tree, objectFormat, err := repo.resolve(ctx, revision)
	if err != nil {
		return invalidReceipt(activation, authorityID, errorCode(err, "invalid-repository"), limits)
	}
	entries, tracked, err := repo.treeEntries(ctx, commit, objectFormat)
	if err != nil {
		document := invalidDocument(activation, authorityID, errorCode(err, "git-read-failed"))
		if errorCode(err, "") != "malformed-tree-entry" {
			document["operationalState"] = "PARTIAL"
		}
		document["repository"] = repositoryMap(authorityID, objectFormat, commit, tree)
		return sealReceipt(document, limits)
	}

	gaps := make([]map[string]any, 0)
	omitted := tracked - len(entries)
	if omitted != 0 {
		gaps = append(gaps, gap("entry-budget-exhausted", omitted))
	}
	blobs, exhausted, blobErr := repo.readBlobs(ctx, entries, exclusions)
	blobState := "EXHAUSTED"
	if blobErr != nil {
		blobs = map[string][]byte{}
		exhausted = map[string]struct{}{}
		blobState = "UNAVAILABLE"
		unavailable := 0
		for _, entry := range entries {
			if readableBlob(entry, exclusions, limits) {
				unavailable++
			}
		}
		gaps = append(gaps, gap(errorCode(blobErr, "git-read-failed"), unavailable))
	} else if len(exhausted) != 0 {
		blobState = "BUDGET_LIMITED"
	}

	records := make([]Record, 0, len(entries))
	for _, entry := range entries {
		records = append(records, ClassifyEntry(entry, blobs, exhausted, exclusions, limits))
	}
	dirtyState := "CLEAN"
	dirty, dirtyErr := repo.dirty(ctx)
	if dirtyErr != nil {
		dirtyState = "UNKNOWN"
		gaps = append(gaps, gap(errorCode(dirtyErr, "git-read-failed"), 1))
	} else if dirty {
		dirtyState = "DIRTY"
		gaps = append(gaps, gap("dirty-worktree", 1))
	}

	classificationCounts := zeroCounts(Classifications())
	sourceCounts := zeroCounts(SourceClassOrder())
	boundaryCounts := zeroCounts(BoundaryOrder())
	inspectedEntries := 0
	inspectedOIDs := make(map[string]struct{})
	reasonCounts := make(map[string]int)
	pathless := 0
	for _, record := range records {
		classificationCounts[record.Classification]++
		if record.Path == nil {
			pathless++
		}
		for _, sourceClass := range record.Classes {
			sourceCounts[sourceClass]++
		}
		for _, boundary := range record.Boundaries {
			boundaryCounts[boundary]++
		}
		if _, ok := blobs[record.OID]; ok {
			inspectedEntries++
			inspectedOIDs[record.OID] = struct{}{}
		}
		if record.Classification == "UNSUPPORTED" || record.Classification == "UNKNOWN" {
			reasonCounts[record.Reason]++
		}
	}
	for code, count := range reasonCounts {
		gaps = append(gaps, gap(code, count))
	}
	if activation == "adopt" {
		gaps = append(gaps, gap("HISTORY_NOT_SCANNED", 1))
	}
	sort.Slice(gaps, func(left, right int) bool {
		leftCode, rightCode := gaps[left]["code"].(string), gaps[right]["code"].(string)
		if leftCode != rightCode {
			return leftCode < rightCode
		}
		return gaps[left]["count"].(int) < gaps[right]["count"].(int)
	})

	frontier := semanticFrontier(sourceCounts, omitted+pathless)
	unresolved := classificationCounts["UNKNOWN"] + classificationCounts["UNSUPPORTED"] + omitted
	operationalState := "COMPLETE"
	if unresolved != 0 || dirtyState != "CLEAN" || activation == "adopt" {
		operationalState = "PARTIAL"
	}
	blobCandidates, blobRejected, blobUnresolved := blobResolverCounts(records)
	inspectedBytes := 0
	for oid := range inspectedOIDs {
		inspectedBytes += len(blobs[oid])
	}
	entryValues := make([]any, 0, len(records))
	for _, record := range records {
		entryValues = append(entryValues, recordMap(record))
	}
	document := map[string]any{
		"activation": activation,
		"denominator": map[string]any{
			"boundaryCounts": boundaryCounts, "classificationCounts": classificationCounts,
			"classifiedEntries": len(records), "inspectedBlobBytes": inspectedBytes,
			"inspectedEntries": inspectedEntries, "sourceClassCounts": sourceCounts,
			"trackedEntries": tracked,
		},
		"dirtyState": dirtyState, "entries": entryValues, "gaps": mapsToAny(gaps),
		"mechanicalResolvers": []any{
			resolver("git-object-identity", 2, 2, 0, "EXHAUSTED", 0),
			resolver("git-tree-inventory", len(records), tracked, 0, chooseState(omitted == 0, "EXHAUSTED", "BUDGET_LIMITED"), omitted),
			resolver("path-and-format-classification", classificationCounts["INCLUDED"]+classificationCounts["EXCLUDED"], len(records), classificationCounts["UNSUPPORTED"], "EXHAUSTED", classificationCounts["UNKNOWN"]),
			resolver("blob-format-inspection", blobCandidates-blobRejected-blobUnresolved, blobCandidates, blobRejected, blobState, blobUnresolved),
		},
		"model":            map[string]any{"callCount": 0, "eligible": false, "inputTokens": 0},
		"operationalState": operationalState, "profile": Profile,
		"repository": repositoryMap(authorityID, objectFormat, commit, tree),
		"scope": map[string]any{
			"excludedPrefixes": exclusions,
			"limits": map[string]any{
				"maxBlobBytes": limits.MaxBlobBytes, "maxEntries": limits.MaxEntries,
				"maxPathBytes": limits.MaxPathBytes, "maxTotalBlobBytes": limits.MaxTotalBlobBytes,
				"maxTreeBytes": limits.MaxTreeBytes,
			},
			"policy": "mechanical-inventory-v0",
		},
		"semanticFrontier": frontier,
	}
	return sealReceipt(document, limits)
}

func validAuthority(value string) bool {
	if len(value) == 0 || len(value) > 128 || !strings.ContainsRune(authorityCharacters, rune(value[0])) {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune(authorityCharacters, character) {
			return false
		}
	}
	return true
}

func normalizeExclusions(values []string, limit int) ([]string, error) {
	if len(values) > 256 {
		return nil, genesisError("invalid-exclusions")
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !utf8.ValidString(value) {
			return nil, genesisError("invalid-exclusions")
		}
		safe := safePath([]byte(value), limit)
		if safe == nil || *safe != value {
			return nil, genesisError("invalid-exclusions")
		}
		value = strings.TrimRight(value, "/")
		if _, duplicate := seen[value]; duplicate {
			return nil, genesisError("invalid-exclusions")
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func invalidReceipt(activation string, authorityID *string, code string, limits Limits) map[string]any {
	return sealReceipt(invalidDocument(activation, authorityID, code), limits)
}

func invalidDocument(activation string, authorityID *string, code string) map[string]any {
	var activationValue any
	if activation == "init" || activation == "adopt" {
		activationValue = activation
	}
	var authorityValue any
	if authorityID != nil && validAuthority(*authorityID) {
		authorityValue = *authorityID
	}
	composition := "LOCAL_ONLY"
	if authorityValue != nil {
		composition = "COMPOSABLE"
	}
	return map[string]any{
		"activation": activationValue,
		"denominator": map[string]any{
			"boundaryCounts": zeroCounts(BoundaryOrder()), "classificationCounts": zeroCounts(Classifications()),
			"classifiedEntries": 0, "inspectedBlobBytes": 0, "inspectedEntries": 0,
			"sourceClassCounts": zeroCounts(SourceClassOrder()), "trackedEntries": nil,
		},
		"dirtyState": "UNKNOWN", "entries": []any{}, "gaps": []any{gap(code, 1)},
		"mechanicalResolvers": []any{},
		"model":               map[string]any{"callCount": 0, "eligible": false, "inputTokens": 0},
		"operationalState":    "INVALID", "profile": Profile,
		"repository": map[string]any{
			"authorityId": authorityValue, "composition": composition,
			"objectFormat": nil, "revision": nil, "tree": nil,
		},
		"scope":            map[string]any{"excludedPrefixes": []string{}, "policy": "mechanical-inventory-v0"},
		"semanticFrontier": []any{},
	}
}

func sealReceipt(document map[string]any, limits Limits) map[string]any {
	preimage, err := canonicalUTF8(document)
	if err != nil {
		return nil
	}
	digest := sha256.Sum256(append(append([]byte("atlas-genesis-inventory/0.1-experimental"), 0), preimage...))
	result := cloneMap(document)
	result["receiptId"] = "genesis-inventory:sha256:" + hex.EncodeToString(digest[:])
	encoded, err := canonicalUTF8(result)
	if err != nil || len(encoded) > limits.MaxReceiptBytes {
		if document["operationalState"] == "INVALID" {
			return result
		}
		return invalidReceipt(fmt.Sprint(document["activation"]), authorityPointer(document), "receipt-budget-exceeded", limits)
	}
	return result
}

func VerifyInventoryReceipt(receipt map[string]any) bool {
	identifier, ok := receipt["receiptId"].(string)
	if !ok {
		return false
	}
	document := cloneMap(receipt)
	delete(document, "receiptId")
	preimage, err := canonicalUTF8(document)
	if err != nil {
		return false
	}
	digest := sha256.Sum256(append(append([]byte("atlas-genesis-inventory/0.1-experimental"), 0), preimage...))
	return identifier == "genesis-inventory:sha256:"+hex.EncodeToString(digest[:])
}

// SummarizeInventoryReceipt returns the sealed bounded default CLI view.
func SummarizeInventoryReceipt(receipt map[string]any, maxEntriesPerClass int) (map[string]any, error) {
	if !VerifyInventoryReceipt(receipt) || maxEntriesPerClass < 0 || maxEntriesPerClass > 20 {
		return nil, genesisError("invalid-inventory-receipt")
	}
	for _, key := range []string{"activation", "denominator", "dirtyState", "gaps", "mechanicalResolvers", "model", "operationalState", "profile", "receiptId", "repository", "semanticFrontier"} {
		if _, ok := receipt[key]; !ok {
			return nil, genesisError("invalid-inventory-receipt")
		}
	}
	entries, ok := receipt["entries"].([]any)
	if !ok {
		return nil, genesisError("invalid-inventory-receipt")
	}
	sourceSamples := make(map[string]any)
	summary := map[string]any{
		"activation": receipt["activation"], "denominator": receipt["denominator"],
		"dirtyState": receipt["dirtyState"], "gaps": receipt["gaps"],
		"mechanicalResolvers": receipt["mechanicalResolvers"], "model": receipt["model"],
		"operationalState": receipt["operationalState"], "profile": receipt["profile"],
		"receiptId": receipt["receiptId"], "repository": receipt["repository"],
		"sampleLimitPerClass": maxEntriesPerClass, "samplesTruncated": false,
		"sourceSamples": sourceSamples, "semanticFrontier": receipt["semanticFrontier"],
		"summaryReceiptId": "genesis-inventory-summary:sha256:" + strings.Repeat("0", 64),
		"summaryProfile":   SummaryProfile,
	}
	if encoded, _ := canonicalUTF8(summary); len(encoded) > maxSummaryBytes {
		return nil, genesisError("summary-budget-exceeded")
	}
	candidates := make(map[string][]map[string]any)
	for _, sourceClass := range append(SourceClassOrder(), "UNCLASSIFIED") {
		candidates[sourceClass] = nil
	}
	for _, rawEntry := range entries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			return nil, genesisError("invalid-inventory-receipt")
		}
		classes, ok := entry["classes"].([]string)
		if !ok {
			return nil, genesisError("invalid-inventory-receipt")
		}
		admitted := make([]string, 0, len(classes))
		for _, sourceClass := range classes {
			if _, known := candidates[sourceClass]; known {
				admitted = append(admitted, sourceClass)
			}
		}
		if len(admitted) == 0 {
			admitted = []string{"UNCLASSIFIED"}
		}
		for _, sourceClass := range admitted {
			candidates[sourceClass] = append(candidates[sourceClass], entry)
		}
	}
	rank := map[string]int{"UNKNOWN": 0, "UNSUPPORTED": 1, "EXCLUDED": 2, "INCLUDED": 3}
	truncated := false
	for _, sourceClass := range append(SourceClassOrder(), "UNCLASSIFIED") {
		values := candidates[sourceClass]
		sort.SliceStable(values, func(left, right int) bool {
			leftRank, leftOK := rank[fmt.Sprint(values[left]["classification"])]
			rightRank, rightOK := rank[fmt.Sprint(values[right]["classification"])]
			if !leftOK {
				leftRank = 4
			}
			if !rightOK {
				rightRank = 4
			}
			if leftRank != rightRank {
				return leftRank < rightRank
			}
			leftPath, rightPath := samplePath(values[left]["path"]), samplePath(values[right]["path"])
			if leftPath != rightPath {
				return leftPath < rightPath
			}
			return fmt.Sprint(values[left]["pathSha256"]) < fmt.Sprint(values[right]["pathSha256"])
		})
		if len(values) == 0 {
			continue
		}
		samples := make([]any, 0, min(maxEntriesPerClass, len(values)))
		sourceSamples[sourceClass] = samples
		for _, entry := range values[:min(maxEntriesPerClass, len(values))] {
			sample := map[string]any{
				"boundaries": entry["boundaries"], "classification": entry["classification"],
				"path": entry["path"], "pathSha256": entry["pathSha256"],
				"reason": entry["reason"], "roles": entry["roles"],
			}
			samples = append(samples, sample)
			sourceSamples[sourceClass] = samples
			if encoded, _ := canonicalUTF8(summary); len(encoded) > maxSummaryBytes {
				samples = samples[:len(samples)-1]
				sourceSamples[sourceClass] = samples
				truncated = true
			}
		}
		if len(samples) == 0 {
			delete(sourceSamples, sourceClass)
		}
		if len(values) > len(samples) {
			truncated = true
		}
	}
	summary["samplesTruncated"] = truncated
	sealed := sealSummary(summary)
	if encoded, _ := canonicalUTF8(sealed); len(encoded) > maxSummaryBytes {
		return nil, genesisError("summary-budget-exceeded")
	}
	return sealed, nil
}

func sealSummary(summary map[string]any) map[string]any {
	document := cloneMap(summary)
	delete(document, "summaryReceiptId")
	preimage, _ := canonicalUTF8(document)
	digest := sha256.Sum256(append(append([]byte("atlas-genesis-inventory-summary/0.1-experimental"), 0), preimage...))
	document["summaryReceiptId"] = "genesis-inventory-summary:sha256:" + hex.EncodeToString(digest[:])
	return document
}

func zeroCounts(vocabulary []string) map[string]int {
	counts := make(map[string]int, len(vocabulary))
	for _, item := range vocabulary {
		counts[item] = 0
	}
	return counts
}

// semanticFrontier claims no declared-source absence while a tracked entry went unclassified:
// omitted by the entry budget, or refused by safePath and so given no source classes.
func semanticFrontier(sourceCounts map[string]int, unclassified int) []any {
	values := make([]map[string]any, 0)
	for _, sourceClass := range SourceClassOrder() {
		if count := sourceCounts[sourceClass]; count != 0 && sourceClass != "ASSET" && sourceClass != "LOCKFILE" {
			values = append(values, map[string]any{"count": count, "reason": "mechanical-inventory-only", "sourceClass": sourceClass})
		}
	}
	for _, sourceClass := range []string{"INSTRUCTIONS", "SPECIFICATION", "TEST", "CI", "OWNERSHIP"} {
		if unclassified != 0 {
			break
		}
		if sourceCounts[sourceClass] == 0 {
			values = append(values, map[string]any{"count": 0, "reason": "declared-source-absent", "sourceClass": sourceClass})
		}
	}
	order := make(map[string]int)
	for index, sourceClass := range SourceClassOrder() {
		order[sourceClass] = index
	}
	sort.SliceStable(values, func(left, right int) bool {
		return order[values[left]["sourceClass"].(string)] < order[values[right]["sourceClass"].(string)]
	})
	return mapsToAny(values)
}

func blobResolverCounts(records []Record) (int, int, int) {
	denominator, rejected, unresolved := 0, 0, 0
	for _, record := range records {
		if (record.Mode != "100644" && record.Mode != "100755") || record.ObjectType != "blob" || record.Path == nil || record.Classification == "EXCLUDED" {
			continue
		}
		denominator++
		if record.Classification == "UNSUPPORTED" {
			rejected++
		}
		if record.Classification == "UNKNOWN" {
			unresolved++
		}
	}
	return denominator, rejected, unresolved
}

func repositoryMap(authorityID *string, objectFormat, revision, tree string) map[string]any {
	var authority any
	composition := "LOCAL_ONLY"
	if authorityID != nil {
		authority, composition = *authorityID, "COMPOSABLE"
	}
	return map[string]any{"authorityId": authority, "composition": composition, "objectFormat": objectFormat, "revision": revision, "tree": tree}
}

func recordMap(record Record) map[string]any {
	return map[string]any{
		"boundaries": record.Boundaries, "classification": record.Classification, "classes": record.Classes,
		"mode": record.Mode, "objectType": record.ObjectType, "oid": record.OID, "path": record.Path,
		"pathSha256": record.PathSHA256, "reason": record.Reason, "roles": record.Roles, "size": record.Size,
	}
}

func resolver(name string, admitted, denominator, rejected int, state string, unresolved int) map[string]any {
	return map[string]any{"admitted": admitted, "denominator": denominator, "name": name, "rejected": rejected, "state": state, "unresolved": unresolved}
}
func gap(code string, count int) map[string]any { return map[string]any{"code": code, "count": count} }
func chooseState(condition bool, yes, no string) string {
	if condition {
		return yes
	}
	return no
}
func samplePath(value any) string {
	if value == nil {
		return ""
	}
	if path, ok := value.(*string); ok {
		if path == nil {
			return ""
		}
		return *path
	}
	return fmt.Sprint(value)
}
func cloneMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
func mapsToAny[T any](values []T) []any {
	result := make([]any, len(values))
	for index := range values {
		result[index] = values[index]
	}
	return result
}

func authorityPointer(document map[string]any) *string {
	repository, _ := document["repository"].(map[string]any)
	authority, _ := repository["authorityId"].(string)
	if authority == "" {
		return nil
	}
	return &authority
}

func errorCode(err error, fallback string) string {
	if typed, ok := err.(genesisError); ok {
		return string(typed)
	}
	return fallback
}

func canonicalUTF8(value any) ([]byte, error) {
	var output bytes.Buffer
	if err := appendCanonicalUTF8(&output, reflect.ValueOf(value)); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func appendCanonicalUTF8(output *bytes.Buffer, value reflect.Value) error {
	if !value.IsValid() || (value.Kind() == reflect.Interface && value.IsNil()) {
		output.WriteString("null")
		return nil
	}
	if value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			output.WriteString("null")
			return nil
		}
		return appendCanonicalUTF8(output, value.Elem())
	}
	switch value.Kind() {
	case reflect.Bool:
		if value.Bool() {
			output.WriteString("true")
		} else {
			output.WriteString("false")
		}
	case reflect.String:
		appendUTF8String(output, value.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		output.WriteString(strconv.FormatInt(value.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		output.WriteString(strconv.FormatUint(value.Uint(), 10))
	case reflect.Slice, reflect.Array:
		output.WriteByte('[')
		for index := 0; index < value.Len(); index++ {
			if index != 0 {
				output.WriteByte(',')
			}
			if err := appendCanonicalUTF8(output, value.Index(index)); err != nil {
				return err
			}
		}
		output.WriteByte(']')
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("canonical JSON map key must be a string")
		}
		keys := value.MapKeys()
		sort.Slice(keys, func(left, right int) bool { return keys[left].String() < keys[right].String() })
		output.WriteByte('{')
		for index, key := range keys {
			if index != 0 {
				output.WriteByte(',')
			}
			appendUTF8String(output, key.String())
			output.WriteByte(':')
			if err := appendCanonicalUTF8(output, value.MapIndex(key)); err != nil {
				return err
			}
		}
		output.WriteByte('}')
	default:
		return fmt.Errorf("unsupported canonical JSON type %s", value.Type())
	}
	return nil
}

func appendUTF8String(output *bytes.Buffer, value string) {
	output.WriteByte('"')
	for _, character := range value {
		switch character {
		case '"':
			output.WriteString(`\"`)
		case '\\':
			output.WriteString(`\\`)
		case '\b':
			output.WriteString(`\b`)
		case '\f':
			output.WriteString(`\f`)
		case '\n':
			output.WriteString(`\n`)
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			if character < 0x20 {
				fmt.Fprintf(output, `\u%04x`, character)
			} else {
				output.WriteRune(character)
			}
		}
	}
	output.WriteByte('"')
}
