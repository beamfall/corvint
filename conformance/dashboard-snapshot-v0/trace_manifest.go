package main

import (
	"bytes"
	"math/big"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	traceManifestSchema  = "corvint-dashboard-trace-conformance-manifest/0"
	maxTraceManifestSize = 1 << 20
)

type traceConfiguredSource struct {
	adapterID         string
	configuredOrdinal string
	relativePath      string
	hasPath           bool
}

type traceManifest struct {
	configuredSources []traceConfiguredSource
	generatedAt       time.Time
}

type traceCLIArguments struct {
	root     string
	manifest string
	snapshot string
}

func parseTraceCLIArguments(arguments []string) (traceCLIArguments, error) {
	if len(arguments) != 7 || arguments[0] != "verify" || arguments[1] != "--root" || arguments[3] != "--manifest" || arguments[5] != "--snapshot" {
		return traceCLIArguments{}, reject(rejectPrivacyText)
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		return traceCLIArguments{}, reject(rejectPrivacyText)
	}
	for _, value := range []string{arguments[2], arguments[4], arguments[6]} {
		if !safeAbsoluteArgument(value) {
			return traceCLIArguments{}, reject(rejectPrivacyText)
		}
	}
	return traceCLIArguments{root: arguments[2], manifest: arguments[4], snapshot: arguments[6]}, nil
}

func safeAbsoluteArgument(value string) bool {
	if value == "" || len(value) > 4096 || !utf8.ValidString(value) || !filepath.IsAbs(value) || filepath.Clean(value) != value {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func parseTraceManifest(raw []byte) (traceManifest, error) {
	if len(raw) == 0 || len(raw) > maxTraceManifestSize {
		return traceManifest{}, reject(rejectSize)
	}
	value, err := decodeUTF8(raw)
	if err != nil {
		return traceManifest{}, err
	}
	canonicalBytes, err := canonical(value, true)
	if err != nil || !bytes.Equal(raw, canonicalBytes) {
		return traceManifest{}, reject(rejectCanonical)
	}
	root, err := object(value)
	if err != nil {
		return traceManifest{}, reject(rejectRoot)
	}
	if err := exactFields(root, "configuredSources", "generatedAt", "schema"); err != nil {
		return traceManifest{}, err
	}
	if root["schema"] != traceManifestSchema {
		return traceManifest{}, reject(rejectRoot)
	}
	generatedAt, _, err := timestamp(root["generatedAt"], false)
	if err != nil {
		return traceManifest{}, err
	}
	rows, err := array(root["configuredSources"])
	if err != nil {
		return traceManifest{}, err
	}
	if len(rows) == 0 || len(rows) > 10_000 {
		return traceManifest{}, reject(rejectSize)
	}
	result := traceManifest{configuredSources: make([]traceConfiguredSource, 0, len(rows)), generatedAt: generatedAt}
	previousKey := ""
	previousOrdinal := make(map[string]string)
	for _, rawRow := range rows {
		row, err := object(rawRow)
		if err != nil {
			return traceManifest{}, err
		}
		if err := exactFields(row, "adapterId", "configuredOrdinal", "relativePath"); err != nil {
			return traceManifest{}, err
		}
		adapterID, err := enum(row["adapterId"], adapterIDs)
		if err != nil {
			return traceManifest{}, err
		}
		ordinal, _, err := decimal(row["configuredOrdinal"], false)
		if err != nil {
			return traceManifest{}, err
		}
		relativePath, hasPath, err := nullableString(row["relativePath"])
		if err != nil {
			return traceManifest{}, err
		}
		if adapterID == "local-trace-v1" {
			if !hasPath || !safeTraceRelativePath(relativePath) {
				return traceManifest{}, reject(rejectPrivacyText)
			}
		} else if hasPath {
			return traceManifest{}, reject(rejectEnum)
		}
		key := adapterID + "\x00" + leftPadDecimal(ordinal)
		if previousKey != "" && previousKey >= key {
			return traceManifest{}, reject(rejectOrdering)
		}
		previousKey = key
		if previous, exists := previousOrdinal[adapterID]; !exists {
			if ordinal != "0" {
				return traceManifest{}, reject(rejectOrdering)
			}
		} else if !nextDecimal(previous, ordinal) {
			return traceManifest{}, reject(rejectOrdering)
		}
		previousOrdinal[adapterID] = ordinal
		result.configuredSources = append(result.configuredSources, traceConfiguredSource{adapterID: adapterID, configuredOrdinal: ordinal, relativePath: relativePath, hasPath: hasPath})
	}
	return result, nil
}

func safeTraceRelativePath(value string) bool {
	if value == "" || len(value) > 4096 || !utf8.ValidString(value) || strings.Contains(value, "\\") || path.IsAbs(value) || strings.HasSuffix(value, "/") || hasVolumePrefix(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	components := strings.Split(value, "/")
	for _, component := range components {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	return path.Clean(value) == value
}

func hasVolumePrefix(value string) bool {
	return len(value) >= 2 && ((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) && value[1] == ':' || strings.HasPrefix(value, "//")
}

func leftPadDecimal(value string) string {
	if len(value) >= 32 {
		return value
	}
	return strings.Repeat("0", 32-len(value)) + value
}

func nextDecimal(previous, current string) bool {
	previousValue, ok := new(big.Int).SetString(previous, 10)
	if !ok {
		return false
	}
	previousValue.Add(previousValue, big.NewInt(1))
	return previousValue.String() == current
}
