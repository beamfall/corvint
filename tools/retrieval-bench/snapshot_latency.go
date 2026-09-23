package main

// TCP-V0-021: measurement state is an observation of the loader, not a
// directory-presence inference. Scratch copies are the only mutable input.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/contextindex"
)

func observedSnapshotState(stderr string) string {
	switch strings.TrimSpace(stderr) {
	case "corvint-bench-snapshot: hit=true":
		return "OBSERVED_HIT"
	case "corvint-bench-snapshot: hit=false":
		return "OBSERVED_MISS"
	default:
		return "UNKNOWN"
	}
}

func cleanWorkspace(ctx context.Context, root string) error {
	status, err := git(ctx, root, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	if status != "" {
		return fmt.Errorf("workspace copy left dirty: %s", status)
	}
	return nil
}

// Every sample receives a matched cold call; only the first sample for each
// materialized tree writes a snapshot. Renaming happens outside timed spans.
func prepareSnapshotSample(ctx context.Context, configuration options, item sample, root string, retrieve retriever) (answer arm, err error) {
	directory := contextindex.SnapshotDirectory(root)
	hidden := ""
	_, statErr := os.Stat(directory)
	primed := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return arm{}, statErr
	}
	if primed {
		hiddenParent, err := os.MkdirTemp("", "corvint-retrieval-hidden-")
		if err != nil {
			return arm{}, err
		}
		defer os.RemoveAll(hiddenParent)
		hidden = filepath.Join(hiddenParent, "index")
		if err := os.Rename(directory, hidden); err != nil {
			return arm{}, err
		}
		defer func() { err = errors.Join(err, os.Rename(hidden, directory)) }()
	}
	contextCtx, observation := beginContextCapture(ctx, item, "cold")
	started := time.Now()
	answer, err = retrieve(contextCtx, configuration.corvintGo, root, queryText(item), configuration.limit)
	answer.WallMillis = round(float64(time.Since(started).Nanoseconds()) / 1e6)
	if captureErr := flushContextCapture(ctx, observation); captureErr != nil {
		return answer, errors.Join(err, captureErr)
	}
	answer.Ranked = without(answer.Ranked, givenFiles(item), configuration.limit)
	if err != nil {
		return answer, err
	}
	if answer.CacheState != "OBSERVED_MISS" {
		return answer, fmt.Errorf("cold context cache observation is %s", answer.CacheState)
	}
	if primed {
		return answer, nil
	}
	if _, err := runCorvintGo(ctx, configuration.corvintGo, "--root", root, "index"); err != nil {
		return answer, fmt.Errorf("prime snapshot: %w", err)
	}
	return answer, cleanWorkspace(ctx, root)
}

func validateSnapshotPair(cold, warm arm) error {
	if warm.Error != "" {
		return fmt.Errorf("warm context: %s", warm.Error)
	}
	if warm.CacheState != "OBSERVED_HIT" {
		return fmt.Errorf("warm context cache observation is %s", warm.CacheState)
	}
	if !reflect.DeepEqual(cold.Ranked, warm.Ranked) || cold.Abstained != warm.Abstained || cold.State != warm.State || cold.TopScore != warm.TopScore {
		return errors.New("cold/hit context rankings differ")
	}
	return nil
}

func registerBeforeRun(ctx context.Context, configuration options, samples []sample, digest string) (map[string]any, map[string]any, error) {
	identity := map[string]any{"version": "NOT_RUN", "sha256": "NOT_RUN"}
	var err error
	if needsCorvint(configuration.arms) {
		identity, err = corvintIdentity(ctx, configuration.corvintGo)
		if err != nil {
			return nil, nil, err
		}
	}
	registered := registration(digest, identity, configuration.arms)
	registered["snapshot_latency"] = configuration.snapshotLatency
	registered["limit"] = configuration.limit
	registered["max_samples"] = configuration.maxSamples
	registered["environment"] = measurementEnvironment()
	ids := make([]string, 0, len(samples))
	for _, item := range samples {
		ids = append(ids, item.ID)
	}
	registered["sample_ids"] = ids
	executable, err := os.Executable()
	if err != nil {
		return nil, nil, err
	}
	benchDigest, err := digestFile(executable)
	if err != nil {
		return nil, nil, err
	}
	registered["bench_sha256"] = benchDigest
	if configuration.registrationPath != "" {
		registered["registered_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		if err := writeRegistration(configuration.registrationPath, registered); err != nil {
			return nil, nil, err
		}
	}
	return identity, registered, nil
}

func writeRegistration(path string, registered map[string]any) error {
	data, err := json.MarshalIndent(registered, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("pre-run registration: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return file.Sync()
}

func measurementEnvironment() []string {
	result := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "CORVINT_CONTEXT_") || key == "CORVINT_SNAPSHOT_FORMAT" || key == "CORVINT_INDEX_SHARDS" || key == "GOMAXPROCS" || key == "GOGC" {
			result = append(result, entry)
		}
	}
	sort.Strings(result)
	return result
}

func digestFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func verifyRunIdentity(ctx context.Context, configuration options, before, registered map[string]any) error {
	if !reflect.DeepEqual(registered["environment"], measurementEnvironment()) {
		return errors.New("measurement environment changed during registered run")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	digest, err := digestFile(executable)
	if err != nil {
		return err
	}
	if digest != registered["bench_sha256"] {
		return errors.New("bench binary changed during registered run")
	}
	if !needsCorvint(configuration.arms) {
		return nil
	}
	after, err := corvintIdentity(ctx, configuration.corvintGo)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(before, after) {
		return errors.New("corvint changed during registered run")
	}
	return nil
}

// Registration and report must remain distinct: the final report is written
// with truncation, whereas the preregistration must survive every run.
func validateRegistrationOutput(configuration options) error {
	if configuration.registrationPath == "" || configuration.output == "" {
		return nil
	}
	registration, err := prospectiveOutputPath(configuration.registrationPath, 40)
	if err != nil {
		return fmt.Errorf("registration path: %w", err)
	}
	output, err := prospectiveOutputPath(configuration.output, 40)
	if err != nil {
		return fmt.Errorf("report path: %w", err)
	}
	if registration == output {
		return errors.New("--registration and --output must name distinct files")
	}
	registrationInfo, registrationErr := os.Stat(registration)
	outputInfo, outputErr := os.Stat(output)
	if registrationErr == nil && outputErr == nil && os.SameFile(registrationInfo, outputInfo) {
		return errors.New("--registration and --output must name distinct files")
	}
	return nil
}

// Resolve parent aliases even when the destination does not exist yet, and
// follow a dangling final symlink to the file the eventual write would open.
func prospectiveOutputPath(path string, links int) (string, error) {
	if links == 0 {
		return "", errors.New("too many output path symlinks")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return "", err
	}
	target := filepath.Join(parent, filepath.Base(absolute))
	info, err := os.Lstat(target)
	if os.IsNotExist(err) {
		return target, nil
	}
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return target, nil
	}
	linked, err := os.Readlink(target)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(linked) {
		linked = filepath.Join(parent, linked)
	}
	return prospectiveOutputPath(linked, links-1)
}
