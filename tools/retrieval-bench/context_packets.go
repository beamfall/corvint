package main

// RBD-V0-001..006: private experimental diagnostics, separate from scorer wire.
import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxCaptureBytes       = 64 << 20
	maxCaptureHeader      = 1 << 20
	maxCaptureMetadata    = 64 << 10
	maxCaptureInvocations = 1024
	captureFooterReserve  = 1024
)

type captureDescriptor struct {
	SampleOrdinal     int    `json:"sample_ordinal"`
	SampleID          string `json:"sample_id"`
	Repo              string `json:"repo"`
	BaseCommit        string `json:"base_commit"`
	InvocationOrdinal int    `json:"invocation_ordinal"`
	Phase             string `json:"phase"`
	TaskBytes         int    `json:"task_bytes"`
	TaskSHA256        string `json:"task_sha256"`
}

type captureStream struct {
	Status string  `json:"status"`
	Base64 *string `json:"base64,omitempty"`
	Bytes  *int    `json:"bytes,omitempty"`
	SHA256 string  `json:"sha256,omitempty"`
}

type captureRecord struct {
	Type        string            `json:"type"`
	Invocation  captureDescriptor `json:"invocation"`
	Stdout      captureStream     `json:"stdout"`
	Stderr      captureStream     `json:"stderr"`
	ParseStatus string            `json:"parse_status"`
	Error       string            `json:"error,omitempty"`
}

type contextCapture struct {
	file     *os.File
	expected []captureDescriptor
	next     int
	calls    int
	bytes    int
	digest   hash.Hash
	partial  bool
	err      error
}

type captureScope struct {
	recorder      *contextCapture
	sampleOrdinal int
}
type captureScopeKey struct{}
type captureObservationKey struct{}

// No encoding, digesting, or I/O occurs during the timed subprocess call.
type captureObservation struct {
	task                   string
	descriptor             captureDescriptor
	stdout, stderr         []byte
	stderrTruncated        bool
	observed               bool
	transportErr, parseErr error
}

func captureHash(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func describeCapture(item sample, sampleOrdinal, invocationOrdinal int, phase string) captureDescriptor {
	task := queryText(item)
	return captureDescriptor{sampleOrdinal, item.ID, item.Repo, item.BaseCommit, invocationOrdinal, phase, len(task), captureHash([]byte(task))}
}

func planContextCapture(configuration options, samples []sample) ([]captureDescriptor, error) {
	if configuration.contextPackets == "" {
		return nil, nil
	}
	if !configuration.arms["context"] || configuration.registrationPath == "" || len(configuration.summaries) != 0 {
		return nil, errors.New("--context-packets requires context arm, registration, and a retrieval run")
	}
	phases := []string{"ordinary"}
	if configuration.snapshotLatency {
		phases = []string{"cold", "hit"}
	}
	if len(samples) > maxCaptureInvocations/len(phases) {
		return nil, errors.New("context capture exceeds 1024 invocations")
	}
	descriptors := make([]captureDescriptor, 0, len(samples)*len(phases))
	for ordinal, item := range samples {
		for _, phase := range phases {
			descriptors = append(descriptors, describeCapture(item, ordinal, len(descriptors), phase))
		}
	}
	if err := validateCapturePath(configuration); err != nil {
		return nil, err
	}
	return descriptors, nil
}

func validateCapturePath(configuration options) error {
	destination, err := filepath.Abs(configuration.contextPackets)
	if err != nil {
		return err
	}
	for path := destination; ; path = filepath.Dir(path) {
		info, statErr := os.Lstat(path)
		if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("context capture path has a symlink")
		}
		if path == destination && statErr == nil {
			return errors.New("context capture destination already exists")
		}
		if statErr != nil && !(path == destination && os.IsNotExist(statErr)) {
			return statErr
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	files := []string{configuration.samples, configuration.registrationPath, configuration.output}
	for _, path := range files {
		if path == "" {
			continue
		}
		resolved, err := prospectiveOutputPath(path, 40)
		if err != nil {
			return err
		}
		if resolved == destination {
			return errors.New("context capture overlaps samples, registration, or report")
		}
	}
	roots := []string{configuration.corpus}
	for _, path := range configuration.snapshots {
		roots = append(roots, path)
	}
	for _, path := range roots {
		if path == "" {
			continue
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return err
		}
		if err := refuseCaptureAncestor(destination, resolved); err != nil {
			return err
		}
		relative, err := filepath.Rel(resolved, destination)
		if err != nil {
			return err
		}
		if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
			return errors.New("context capture overlaps source snapshot or corpus")
		}
	}
	return nil
}

func openContextCapture(configuration options, expected []captureDescriptor, registered map[string]any) (*contextCapture, error) {
	if configuration.contextPackets == "" {
		return nil, nil
	}
	header := struct {
		Type         string              `json:"type"`
		Profile      string              `json:"profile"`
		Registration map[string]any      `json:"registration"`
		Invocations  []captureDescriptor `json:"invocations"`
	}{"header", "corvint-retrieval-context-capture-v0", registered, expected}
	line, err := captureLine(header)
	if err != nil {
		return nil, err
	}
	if len(line) > maxCaptureHeader {
		return nil, errors.New("context capture header exceeds 1 MiB")
	}
	// Acquire the parent before validation; the rooted helper binds the validated
	// pathname to this held directory before creating any file.
	parent, err := os.OpenRoot(filepath.Dir(configuration.contextPackets))
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	file, err := openCaptureFile(parent, configuration)
	if err != nil {
		return nil, fmt.Errorf("context capture: %w", err)
	}
	info, err := file.Stat()
	if err == nil {
		err = refuseCaptureFileAliases(info, configuration)
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	if configuration.captureInfo != nil {
		*configuration.captureInfo = info
	}
	recorder := &contextCapture{file: file, expected: expected, digest: sha256.New()}
	if err := recorder.write(line); err != nil {
		file.Close()
		return nil, err
	}
	return recorder, nil
}

func captureLine(value any) ([]byte, error) {
	data, err := json.Marshal(value)
	return append(data, '\n'), err
}

func (recorder *contextCapture) write(line []byte) error {
	if len(line) > maxCaptureBytes-captureFooterReserve-recorder.bytes {
		return errors.New("context capture exceeds 64 MiB encoded budget")
	}
	n, err := recorder.file.Write(line)
	if err != nil {
		return err
	}
	if n != len(line) {
		return io.ErrShortWrite
	}
	recorder.bytes += n
	_, _ = recorder.digest.Write(line)
	return nil
}

func beginContextCapture(ctx context.Context, item sample, phase string) (context.Context, *captureObservation) {
	scope, _ := ctx.Value(captureScopeKey{}).(*captureScope)
	if scope == nil {
		return ctx, nil
	}
	observation := &captureObservation{descriptor: describeCapture(item, scope.sampleOrdinal, scope.recorder.calls, phase)}
	scope.recorder.calls++
	return context.WithValue(ctx, captureObservationKey{}, observation), observation
}

func completeCaptureStream(data []byte, status string) captureStream {
	encoded, size := base64.StdEncoding.EncodeToString(data), len(data)
	return captureStream{status, &encoded, &size, captureHash(data)}
}

func flushContextCapture(ctx context.Context, observation *captureObservation) error {
	if observation == nil {
		return nil
	}
	scope := ctx.Value(captureScopeKey{}).(*captureScope)
	recorder := scope.recorder
	if recorder.err != nil {
		return recorder.err
	}
	recorder.err = recorder.append(observation)
	return recorder.err
}

func (recorder *contextCapture) append(observation *captureObservation) error {
	if recorder.next >= len(recorder.expected) || observation.descriptor != recorder.expected[recorder.next] {
		return errors.New("context capture invocation identity mismatch")
	}
	if observation.observed && (len(observation.task) != observation.descriptor.TaskBytes || captureHash([]byte(observation.task)) != observation.descriptor.TaskSHA256) {
		return errors.New("context capture task identity mismatch")
	}
	record := captureRecord{Type: "invocation", Invocation: observation.descriptor, Stdout: captureStream{Status: "NOT_PRODUCED"}, Stderr: captureStream{Status: "NOT_PRODUCED"}, ParseStatus: "NOT_RUN"}
	if observation.observed {
		stderrStatus := "COMPLETE"
		if observation.stderrTruncated {
			stderrStatus = "TRUNCATED"
		}
		record.Stderr = completeCaptureStream(observation.stderr, stderrStatus)
		if observation.transportErr == nil {
			record.Stdout = completeCaptureStream(observation.stdout, "COMPLETE")
			record.ParseStatus = "PARSED"
			if observation.parseErr != nil {
				record.ParseStatus = "MALFORMED"
			}
		}
	}
	failure := observation.transportErr
	if failure == nil {
		failure = observation.parseErr
	}
	if !observation.observed {
		failure = errors.New("context invocation was not observed")
	}
	if failure != nil {
		record.Error = string([]byte(failure.Error())[:min(len(failure.Error()), 4096)])
	}
	metadata := record
	metadata.Stdout.Base64, metadata.Stderr.Base64 = nil, nil
	encodedMetadata, err := captureLine(metadata)
	if err != nil {
		return err
	}
	if len(encodedMetadata) > maxCaptureMetadata {
		return errors.New("context capture metadata exceeds 64 KiB")
	}
	line, err := captureLine(record)
	if err != nil {
		return err
	}
	if err := recorder.write(line); err != nil {
		return err
	}
	recorder.next++
	recorder.partial = recorder.partial || record.Stdout.Status != "COMPLETE"
	return nil
}

func (recorder *contextCapture) finish() error {
	if recorder == nil {
		return nil
	}
	if recorder.err != nil {
		return recorder.err
	}
	footer := struct {
		Type        string `json:"type"`
		Status      string `json:"status"`
		Invocations int    `json:"invocations"`
		TotalBytes  int    `json:"total_bytes"`
		PriorSHA256 string `json:"prior_sha256"`
	}{"footer", "COMPLETE", recorder.next, 0, hex.EncodeToString(recorder.digest.Sum(nil))}
	if recorder.partial || recorder.next != len(recorder.expected) || recorder.calls != recorder.next {
		footer.Status = "PARTIAL"
	}
	line, err := captureLine(footer)
	if err != nil {
		return err
	}
	for footer.TotalBytes != recorder.bytes+len(line) {
		footer.TotalBytes = recorder.bytes + len(line)
		line, err = captureLine(footer)
		if err != nil {
			return err
		}
	}
	if len(line) > captureFooterReserve || footer.TotalBytes > maxCaptureBytes {
		return errors.New("context capture footer exceeds encoded budget")
	}
	if n, err := recorder.file.Write(line); err != nil {
		return err
	} else if n != len(line) {
		return io.ErrShortWrite
	}
	if err := recorder.file.Sync(); err != nil {
		return err
	}
	return recorder.file.Close()
}

// Filesystem identity catches case/normalization aliases and mount aliases that
// a lexical containment check cannot. This runs before creating the capture.
func refuseCaptureAncestor(destination, protected string) error {
	protectedInfo, err := os.Stat(protected)
	if err != nil {
		return err
	}
	for ancestor := filepath.Dir(destination); ; ancestor = filepath.Dir(ancestor) {
		info, err := os.Stat(ancestor)
		if err != nil {
			return err
		}
		if os.SameFile(info, protectedInfo) {
			return errors.New("context capture overlaps source snapshot or corpus identity")
		}
		if ancestor == filepath.Dir(ancestor) {
			return nil
		}
	}
}

// The capture now exists, so even a previously absent case/normalization alias
// of a prospective report can be compared by identity before any capture writes.
func refuseCaptureFileAliases(capture os.FileInfo, configuration options) error {
	for _, path := range []string{configuration.samples, configuration.registrationPath, configuration.output} {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if os.SameFile(capture, info) {
			return errors.New("context capture aliases samples, registration, or report identity")
		}
	}
	return nil
}

// Retain the original capture identity across its close. Open the report without
// truncation, then compare its actual identity before any destructive operation.
// This protects against an alias added after initial capture validation as well.
func writeCaptureReport(path string, data []byte, capture os.FileInfo) (err error) {
	if capture == nil {
		return errors.New("context capture identity unavailable before report publication")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if os.SameFile(capture, info) {
		return errors.New("report aliases context capture identity")
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	n, err := file.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

// Bind path validation to the directory handle used for creation. Once these
// identities agree, later pathname redirection cannot redirect the rooted open.
func openCaptureFile(parent *os.Root, configuration options) (*os.File, error) {
	held, err := parent.Stat(".")
	if err != nil {
		return nil, err
	}
	if err := validateCapturePath(configuration); err != nil {
		return nil, err
	}
	current, err := os.Stat(filepath.Dir(configuration.contextPackets))
	if err != nil {
		return nil, err
	}
	if !os.SameFile(held, current) {
		return nil, errors.New("context capture parent identity changed during validation")
	}
	return parent.OpenFile(filepath.Base(configuration.contextPackets), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
}
