package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/dashboard/model"
)

const (
	errorProfile         = "corvint-dashboard-error/0"
	errorInvalidArgument = "DASHBOARD_INVALID_ARGUMENT"
	errorRepository      = "DASHBOARD_REPOSITORY_UNAVAILABLE"
	errorResource        = "DASHBOARD_RESOURCE_EXHAUSTED"
	errorInterrupted     = "DASHBOARD_INTERRUPTED"
	errorInternal        = "DASHBOARD_INTERNAL_ERROR"
	errorOutputWrite     = "OUTPUT_WRITE_FAILED"
	maxArgumentBytes     = 4096
	maxAdapterIDBytes    = 128
	maxSnapshotBytes     = model.MaxSnapshotBytes
)

var (
	adapterPattern   = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	objectIDPattern  = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	generatedPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{9}Z$`)
)

type sourceOption struct {
	AdapterID    string
	RelativePath string
}

type bundleOption struct {
	CEM          string
	OCM          string
	Profile      string
	ExpectedBase string
	Target       string
}

type options struct {
	Root        string
	Sources     []sourceOption
	Bundle      *bundleOption
	GeneratedAt string
}

type compileRequest struct {
	Root        string
	Sources     []sourceOption
	Bundle      *bundleOption
	GeneratedAt string
}

type compiler func(context.Context, compileRequest) ([]byte, error)

type dashboardError struct{ code string }

func (e *dashboardError) Error() string { return e.code }

func parseArguments(arguments []string) (options, error) {
	var parsed options
	if len(arguments) == 0 || arguments[0] != "snapshot" {
		return parsed, &dashboardError{code: errorInvalidArgument}
	}

	seen := make(map[string]bool)
	sourcePairs := make(map[string]bool)
	var cem, ocm, profile, expectedBase, target string
	conformance := false
	for index := 1; index < len(arguments); {
		flag := arguments[index]
		if flag == "--conformance" {
			if seen[flag] {
				return options{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			conformance = true
			index++
			continue
		}
		if index+1 >= len(arguments) || !strings.HasPrefix(flag, "--") {
			return options{}, &dashboardError{code: errorInvalidArgument}
		}
		value := arguments[index+1]
		index += 2
		switch flag {
		case "--root":
			if seen[flag] || !validRoot(value) {
				return options{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			parsed.Root = value
		case "--source":
			source, ok := parseSource(value)
			if !ok || sourcePairs[value] {
				return options{}, &dashboardError{code: errorInvalidArgument}
			}
			sourcePairs[value] = true
			parsed.Sources = append(parsed.Sources, source)
		case "--cem":
			if seen[flag] || !validRelativePath(value) {
				return options{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			cem = value
		case "--ocm":
			if seen[flag] || !validRelativePath(value) {
				return options{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			ocm = value
		case "--cem-ocm-profile":
			if seen[flag] || !validCEMOCMProfile(value) {
				return options{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			profile = value
		case "--expected-base":
			if seen[flag] || !objectIDPattern.MatchString(value) {
				return options{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			expectedBase = value
		case "--target":
			if seen[flag] || !objectIDPattern.MatchString(value) {
				return options{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			target = value
		case "--generated-at":
			if seen[flag] || !validGeneratedAt(value) {
				return options{}, &dashboardError{code: errorInvalidArgument}
			}
			seen[flag] = true
			parsed.GeneratedAt = value
		default:
			return options{}, &dashboardError{code: errorInvalidArgument}
		}
	}

	if parsed.Root == "" {
		return options{}, &dashboardError{code: errorInvalidArgument}
	}
	bundleCount := countPresent(cem, ocm, profile, expectedBase, target)
	if bundleCount != 0 && bundleCount != 5 {
		return options{}, &dashboardError{code: errorInvalidArgument}
	}
	if bundleCount == 5 {
		parsed.Bundle = &bundleOption{CEM: cem, OCM: ocm, Profile: profile, ExpectedBase: expectedBase, Target: target}
	}
	if conformance != (parsed.GeneratedAt != "") {
		return options{}, &dashboardError{code: errorInvalidArgument}
	}
	return parsed, nil
}

func validCEMOCMProfile(value string) bool {
	return value == "cem/0.1+ocm/0.1" || value == "cem/0.2+ocm/0.1"
}

func parseSource(value string) (sourceOption, bool) {
	if len(value) == 0 || !utf8.ValidString(value) {
		return sourceOption{}, false
	}
	adapterID, relativePath, found := strings.Cut(value, "=")
	if !found {
		return sourceOption{}, false
	}
	if len(adapterID) > maxAdapterIDBytes || !adapterPattern.MatchString(adapterID) || !validRelativePath(relativePath) {
		return sourceOption{}, false
	}
	return sourceOption{AdapterID: adapterID, RelativePath: relativePath}, true
}

func validRoot(value string) bool {
	if !validArgumentString(value) || !filepath.IsAbs(value) {
		return false
	}
	return filepath.Clean(value) == value
}

func validRelativePath(value string) bool {
	if !validArgumentString(value) || path.IsAbs(value) || filepath.IsAbs(value) {
		return false
	}
	if strings.Contains(value, "\\") || filepath.VolumeName(value) != "" {
		return false
	}
	if path.Clean(value) != value || value == "." {
		return false
	}
	for _, component := range strings.Split(value, "/") {
		if component == "" || component == "." || component == ".." {
			return false
		}
	}
	return true
}

func validArgumentString(value string) bool {
	if value == "" || len(value) > maxArgumentBytes || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character == 0 || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validGeneratedAt(value string) bool {
	if !generatedPattern.MatchString(value) {
		return false
	}
	_, err := time.Parse("2006-01-02T15:04:05.000000000Z", value)
	return err == nil
}

func countPresent(values ...string) int {
	count := 0
	for _, value := range values {
		if value != "" {
			count++
		}
	}
	return count
}

func emitError(output io.Writer, code string) {
	_, _ = io.WriteString(output, `{"code":"`+code+`","profile":"`+errorProfile+`"}`+"\n")
}

func normalizedErrorCode(err error) string {
	if errors.Is(err, context.Canceled) {
		return errorInterrupted
	}
	var dashboardFailure *dashboardError
	if errors.As(err, &dashboardFailure) {
		switch dashboardFailure.code {
		case errorInvalidArgument, errorRepository, errorResource, errorInterrupted, errorInternal:
			return dashboardFailure.code
		}
	}
	return errorInternal
}

func runContext(ctx context.Context, arguments []string, stdout, stderr io.Writer, compile compiler) (exit int) {
	defer func() {
		if recover() != nil {
			emitError(stderr, errorInternal)
			exit = 2
		}
	}()
	if !supportedPlatform(runtime.GOOS) {
		emitError(stderr, errorInternal)
		return 2
	}
	parsed, err := parseArguments(arguments)
	if err != nil {
		emitError(stderr, errorInvalidArgument)
		return 2
	}
	if err := ctx.Err(); err != nil {
		emitError(stderr, errorInterrupted)
		return 2
	}
	request := compileRequest{
		Root: parsed.Root, Sources: parsed.Sources, Bundle: parsed.Bundle, GeneratedAt: parsed.GeneratedAt,
	}
	encoded, err := compile(ctx, request)
	if err != nil {
		emitError(stderr, normalizedErrorCode(err))
		return 2
	}
	if err := ctx.Err(); err != nil {
		emitError(stderr, errorInterrupted)
		return 2
	}
	if len(encoded) > maxSnapshotBytes {
		emitError(stderr, errorResource)
		return 2
	}
	if _, err := model.VerifyCanonical(encoded); err != nil {
		emitError(stderr, errorInternal)
		return 2
	}
	if !writeSnapshot(stdout, encoded) {
		emitError(stderr, errorOutputWrite)
		return 2
	}
	return 0
}

func supportedPlatform(goos string) bool {
	return goos == "darwin" || goos == "linux" || goos == "windows"
}

func writeSnapshot(output io.Writer, encoded []byte) (complete bool) {
	defer func() {
		if recover() != nil {
			complete = false
		}
	}()
	offset := 0
	for offset < len(encoded) {
		written, err := output.Write(encoded[offset:])
		if written < 0 || written > len(encoded)-offset {
			return false
		}
		offset += written
		if err != nil || written == 0 {
			return false
		}
	}
	return true
}

// isRoadmapCommand reports whether arguments select the `roadmap`
// subcommand rather than the default `snapshot` one.
func isRoadmapCommand(arguments []string) bool {
	return len(arguments) > 0 && arguments[0] == "roadmap"
}

func run(arguments []string, stdout, stderr io.Writer) int {
	if isRoadmapCommand(arguments) {
		return runRoadmapContext(context.Background(), arguments, stdout, stderr)
	}
	return runContext(context.Background(), arguments, stdout, stderr, compileSnapshot)
}

func runProcess(arguments []string, stdout, stderr io.Writer, compile compiler) int {
	containOutputPipeSignal()
	ctx, cancel := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer cancel()
	if isRoadmapCommand(arguments) {
		return runRoadmapContext(ctx, arguments, stdout, stderr)
	}
	return runContext(ctx, arguments, stdout, stderr, compile)
}

func main() {
	os.Exit(runProcess(os.Args[1:], os.Stdout, os.Stderr, compileSnapshot))
}
