package doccompiler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func build(ctx context.Context, environment Environment, plan PatchPlan, options BuildOptions) (BuildResult, error) {
	result := BuildResult{
		BuildStrictStatus:    "NOT_RUN",
		OfflineQualification: "NOT_OBSERVED",
		OfflineEnforcement:   "environment hardening only; no OS network-denial boundary",
		Uncertainty: []string{
			"BUILD_STRICT_PASS does not imply OFFLINE_QUALIFIED",
			"config observations are lexical; MkDocs is the build authority",
			"proxy scrubbing does not prevent plugin, theme, or extension sockets",
		},
	}
	cleanupSupported, cleanupDescription := descendantCleanupQualification()
	result.ProcessContainment = cleanupDescription
	if !cleanupSupported {
		return result, failure("process-containment-unsupported", "%s", cleanupDescription)
	}
	if environment.Qualification != "QUALIFIED" {
		return result, failure("profile-unqualified", "Material P0 profile is not qualified: %s", strings.Join(environment.Observations.Uncertainty, "; "))
	}
	if err := validateTrustAttestation(environment, options.TrustAttestation); err != nil {
		return result, err
	}
	if plan.Profile != ExperimentalPatchPlanProfile || plan.ConfigSHA256 != environment.Config.SHA256 {
		return result, failure("plan-environment-mismatch", "patch plan does not bind the discovered config")
	}
	planHash, err := planDigest(plan)
	if err != nil || planHash != plan.PlanSHA256 {
		return result, failure("plan-digest-mismatch", "patch plan bytes do not match their digest")
	}
	mkdocsAbsolute, err := verifyExecutable(environment.ProjectRoot, environment.Toolchain.MkDocs, false)
	if err != nil {
		return result, err
	}
	pythonAbsolute, err := verifyExecutable(environment.ProjectRoot, environment.Toolchain.Python, true)
	if err != nil {
		return result, err
	}
	if _, err := verifyPinnedSource(environment.ProjectRoot, SourcePin{Path: environment.Toolchain.ProjectLock.Path, SHA256: environment.Toolchain.ProjectLock.SHA256, Size: environment.Toolchain.ProjectLock.Size}, defaultSourceBytes); err != nil {
		return result, err
	}
	for _, source := range plan.Sources {
		if _, err := verifyPinnedSource(environment.ProjectRoot, source, sourceLimit(options)); err != nil {
			return result, err
		}
	}
	outputParent, err := isolatedOutputParent(environment.ProjectRoot, options.OutputParent)
	if err != nil {
		return result, err
	}
	stageRoot, err := os.MkdirTemp("", "corvint-doccompiler-stage-")
	if err != nil {
		return result, failure("isolation-failed", "cannot create staged corpus root")
	}
	defer os.RemoveAll(stageRoot)
	outputRoot, err := os.MkdirTemp(outputParent, "corvint-doccompiler-site-")
	if err != nil {
		return result, failure("isolation-failed", "cannot create isolated output root")
	}
	removeOutput := true
	defer func() {
		if removeOutput {
			_ = os.RemoveAll(outputRoot)
		}
	}()
	result.OutputRoot = outputRoot
	if err := stageCorpus(environment, plan, stageRoot, options); err != nil {
		return result, err
	}
	stateRoot := filepath.Join(stageRoot, ".corvint-command-state")
	if err := createStateDirectories(stateRoot); err != nil {
		return result, err
	}
	configPath := filepath.Join(stageRoot, filepath.FromSlash(environment.Config.Path))
	duration := options.Timeout
	if duration <= 0 {
		duration = defaultBuildDuration
	}
	buildContext, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	stdoutLimit := options.MaxStdoutBytes
	if stdoutLimit <= 0 {
		stdoutLimit = defaultCommandBytes
	}
	stderrLimit := options.MaxStderrBytes
	if stderrLimit <= 0 {
		stderrLimit = defaultCommandBytes
	}
	candidateConfig, err := loadAuthoritativeConfig(buildContext, pythonAbsolute, stageRoot, stateRoot, configPath, defaultCommandBytes)
	if err != nil {
		return result, err
	}
	if len(plan.Nav.Entries) > 0 && candidateConfig.NavSHA256 != plan.Nav.ProposedNavSHA256 {
		return result, failure("nav-candidate-mismatch", "MkDocs authority did not load the exact proposed nav")
	}
	result.NavCandidateBuilt = true
	arguments := []string{"build", "--strict", "--clean", "--config-file", configPath, "--site-dir", outputRoot}
	result.Argv = append([]string{mkdocsAbsolute}, arguments...)
	environmentVariables := isolatedEnvironment(mkdocsAbsolute, stateRoot)
	commandOutput, commandErr := runCommand(buildContext, mkdocsAbsolute, arguments, stageRoot, environmentVariables, stdoutLimit, stderrLimit)
	result.Stdout = commandOutput.stdout
	result.Stderr = commandOutput.stderr
	if commandErr != nil {
		result.BuildStrictStatus = "FAIL"
		return result, commandErr
	}
	for _, source := range plan.Sources {
		if _, err := verifyPinnedSource(environment.ProjectRoot, source, sourceLimit(options)); err != nil {
			result.BuildStrictStatus = "FAIL"
			return result, err
		}
	}
	if _, err := verifyExecutable(environment.ProjectRoot, environment.Toolchain.MkDocs, false); err != nil {
		result.BuildStrictStatus = "FAIL"
		return result, err
	}
	if _, err := verifyExecutable(environment.ProjectRoot, environment.Toolchain.Python, true); err != nil {
		result.BuildStrictStatus = "FAIL"
		return result, err
	}
	if _, err := verifyPinnedSource(environment.ProjectRoot, SourcePin{Path: environment.Toolchain.ProjectLock.Path, SHA256: environment.Toolchain.ProjectLock.SHA256, Size: environment.Toolchain.ProjectLock.Size}, defaultSourceBytes); err != nil {
		result.BuildStrictStatus = "FAIL"
		return result, err
	}
	outputDigest, outputBytes, outputFiles, err := validateSite(outputRoot, options)
	if err != nil {
		result.BuildStrictStatus = "FAIL"
		return result, err
	}
	result.BuildStrictStatus = "PASS"
	result.OutputSHA256 = outputDigest
	result.OutputBytes = outputBytes
	result.OutputFiles = outputFiles
	result.DocumentCandidatesBuilt = len(plan.Documents) > 0
	result.Uncertainty = uniqueSorted(result.Uncertainty)
	removeOutput = false
	return result, nil
}

func validateTrustAttestation(environment Environment, trust EnvironmentTrustAttestation) error {
	if trust.Profile != "corvint-doccompiler-environment-trust/0" {
		return failure("environment-untrusted", "a project-owned environment trust attestation is required")
	}
	if trust.EnvironmentSHA256 != environment.Toolchain.EnvironmentSHA256 {
		return failure("environment-untrusted", "trust attestation does not bind the discovered environment")
	}
	if strings.TrimSpace(trust.Authority) == "" || !revisionPattern.MatchString(trust.Revision) {
		return failure("environment-untrusted", "trust attestation must name an authority and immutable revision")
	}
	return nil
}

func sourceLimit(options BuildOptions) int64 {
	if options.MaxSourceBytes > 0 {
		return options.MaxSourceBytes
	}
	return defaultSourceBytes
}

func isolatedOutputParent(projectRoot, requested string) (string, error) {
	parent := requested
	if parent == "" {
		parent = os.TempDir()
	}
	absolute, err := filepath.Abs(parent)
	if err != nil {
		return "", failure("invalid-output-parent", "cannot resolve output parent")
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", failure("invalid-output-parent", "cannot resolve output parent")
	}
	if contained(projectRoot, resolved) {
		return "", failure("output-inside-project", "output parent must be outside the project")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", failure("invalid-output-parent", "output parent is not a directory")
	}
	return resolved, nil
}

func stageCorpus(environment Environment, plan PatchPlan, stageRoot string, options BuildOptions) error {
	corpusLimit := options.MaxCorpusBytes
	if corpusLimit <= 0 {
		corpusLimit = defaultCorpusBytes
	}
	configRaw, _, err := readPinned(environment.ProjectRoot, environment.Config.Path, defaultConfigBytes)
	if err != nil {
		return err
	}
	if len(plan.Nav.Entries) > 0 {
		configRaw, err = applyBytePatch(configRaw, plan.Nav.Patch)
		if err != nil {
			return err
		}
	}
	if err := writeStageFile(stageRoot, environment.Config.Path, configRaw); err != nil {
		return err
	}
	docsRoot, err := resolveConfigRelative(environment.Config.Path, environment.Observations.DocsDir)
	if err != nil {
		return err
	}
	used, err := copyTree(environment.ProjectRoot, docsRoot, stageRoot, corpusLimit)
	if err != nil {
		return err
	}
	if environment.Observations.ThemeCustomDir != "" {
		customRoot, err := resolveConfigRelative(environment.Config.Path, environment.Observations.ThemeCustomDir)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(customRoot+"/", docsRoot+"/") {
			additional, err := copyTree(environment.ProjectRoot, customRoot, stageRoot, corpusLimit-used)
			if err != nil {
				return err
			}
			used += additional
		}
	}
	for _, document := range plan.Documents {
		current := []byte(nil)
		stagedPath := filepath.Join(stageRoot, filepath.FromSlash(document.Path))
		if data, readErr := os.ReadFile(stagedPath); readErr == nil {
			current = data
		} else if !os.IsNotExist(readErr) {
			return failure("corpus-unavailable", "cannot read staged document target")
		}
		proposed, patchErr := applyBytePatch(current, document.Patch)
		if patchErr != nil {
			return patchErr
		}
		digest := sha256.Sum256(proposed)
		if hex.EncodeToString(digest[:]) != document.ProposedSHA256 {
			return failure("proposal-digest-mismatch", "document byte patch changed after planning: %s", document.Path)
		}
		if used+int64(len(proposed)) > corpusLimit {
			return failure("corpus-too-large", "staged corpus exceeds its byte limit")
		}
		if err := writeStageFile(stageRoot, document.Path, proposed); err != nil {
			return err
		}
		used += int64(len(proposed))
	}
	return nil
}

func copyTree(projectRoot, relative, stageRoot string, limit int64) (int64, error) {
	clean, err := cleanRelative(relative)
	if err != nil {
		return 0, err
	}
	if err := rejectSymlinkComponents(projectRoot, clean); err != nil {
		return 0, err
	}
	sourceRoot := filepath.Join(projectRoot, filepath.FromSlash(clean))
	var total int64
	err = filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return failure("corpus-unavailable", "cannot walk staged corpus")
		}
		relativePath, err := filepath.Rel(projectRoot, path)
		if err != nil {
			return failure("path-escape", "cannot resolve corpus path")
		}
		info, err := entry.Info()
		if err != nil {
			return failure("corpus-unavailable", "cannot inspect corpus entry")
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return failure("symlink-rejected", "staged corpus contains a symlink: %s", filepath.ToSlash(relativePath))
		}
		if info.IsDir() {
			return os.MkdirAll(filepath.Join(stageRoot, relativePath), 0700)
		}
		if !info.Mode().IsRegular() {
			return failure("invalid-corpus-entry", "staged corpus contains a non-file: %s", filepath.ToSlash(relativePath))
		}
		if info.Size() > sourceLimit(BuildOptions{}) || total+info.Size() > limit {
			return failure("corpus-too-large", "staged corpus exceeds its byte limit")
		}
		data, _, err := readPinned(projectRoot, filepath.ToSlash(relativePath), sourceLimit(BuildOptions{}))
		if err != nil {
			return err
		}
		total += int64(len(data))
		return writeStageFile(stageRoot, filepath.ToSlash(relativePath), data)
	})
	return total, err
}

func writeStageFile(stageRoot, relative string, data []byte) error {
	clean, err := cleanRelative(relative)
	if err != nil {
		return err
	}
	target := filepath.Join(stageRoot, filepath.FromSlash(clean))
	if !contained(stageRoot, target) {
		return failure("path-escape", "stage target escapes isolation root")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return failure("isolation-failed", "cannot create stage directory")
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return failure("isolation-failed", "cannot create staged file")
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return failure("isolation-failed", "cannot write staged file")
	}
	if err := file.Close(); err != nil {
		return failure("isolation-failed", "cannot close staged file")
	}
	return nil
}

func validateSite(root string, options BuildOptions) (string, int64, int, error) {
	byteLimit := options.MaxOutputBytes
	if byteLimit <= 0 {
		byteLimit = defaultOutputBytes
	}
	fileLimit := options.MaxOutputFiles
	if fileLimit <= 0 {
		fileLimit = defaultOutputFiles
	}
	type outputFile struct {
		path   string
		digest [32]byte
		size   int64
	}
	files := make([]outputFile, 0)
	var total int64
	hasIndex := false
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return failure("invalid-build-output", "cannot inspect build output")
		}
		if path == root || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return failure("invalid-build-output", "build output contains a non-regular file")
		}
		if len(files)+1 > fileLimit || info.Size() > sourceLimit(BuildOptions{}) || total+info.Size() > byteLimit {
			return failure("build-output-too-large", "build output exceeds its bounds")
		}
		file, err := os.Open(path)
		if err != nil {
			return failure("invalid-build-output", "cannot read build output")
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, io.LimitReader(file, sourceLimit(BuildOptions{})+1))
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			return failure("invalid-build-output", "cannot hash build output")
		}
		relative, _ := filepath.Rel(root, path)
		relative = filepath.ToSlash(relative)
		var digest [32]byte
		copy(digest[:], hash.Sum(nil))
		files = append(files, outputFile{path: relative, digest: digest, size: info.Size()})
		total += info.Size()
		if relative == "index.html" {
			hasIndex = true
		}
		return nil
	})
	if err != nil {
		return "", 0, 0, err
	}
	if !hasIndex {
		return "", 0, 0, failure("false-strict-pass", "mkdocs exited zero without producing index.html")
	}
	sort.Slice(files, func(left, right int) bool { return files[left].path < files[right].path })
	tree := sha256.New()
	for _, file := range files {
		_, _ = tree.Write([]byte(file.path))
		_, _ = tree.Write([]byte{0})
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(file.size))
		_, _ = tree.Write(size[:])
		_, _ = tree.Write(file.digest[:])
	}
	return hex.EncodeToString(tree.Sum(nil)), total, len(files), nil
}

func BuildDigest(result BuildResult) (string, error) {
	copy := result
	copy.OutputRoot = ""
	encoded, err := json.Marshal(copy)
	if err != nil {
		return "", failure("build-encoding-failed", "cannot encode build result")
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

// QualifyOffline cannot promote a completed build from caller-supplied claims.
// P0 has no concrete external denial-harness integration, so no code path sets PASS.
func QualifyOffline(result BuildResult, _ OfflineAttestation) (BuildResult, error) {
	return result, failure("offline-observer-required", "offline qualification requires an external observer around the complete build process")
}

func applyBytePatch(current []byte, patch BytePatch) ([]byte, error) {
	if patch.StartByte < 0 || patch.EndByte < patch.StartByte || patch.EndByte > len(current) {
		return nil, failure("invalid-byte-patch", "byte patch range is outside its target")
	}
	originalDigest := sha256.Sum256(current[patch.StartByte:patch.EndByte])
	replacementDigest := sha256.Sum256([]byte(patch.Replacement))
	if hex.EncodeToString(originalDigest[:]) != patch.OriginalSHA256 || hex.EncodeToString(replacementDigest[:]) != patch.ReplacementSHA256 {
		return nil, failure("byte-patch-digest-mismatch", "byte patch does not bind its original and replacement bytes")
	}
	result := make([]byte, 0, len(current)-(patch.EndByte-patch.StartByte)+len(patch.Replacement))
	result = append(result, current[:patch.StartByte]...)
	result = append(result, patch.Replacement...)
	result = append(result, current[patch.EndByte:]...)
	return result, nil
}

func scanGeneratedAssets(root string, options BuildOptions) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return failure("offline-asset-scan-failed", "cannot inspect generated output")
		}
		if path == root || entry.IsDir() || found != "" {
			return nil
		}
		extension := strings.ToLower(filepath.Ext(path))
		if extension != ".html" && extension != ".css" && extension != ".js" && extension != ".svg" && extension != ".xml" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || int64(len(data)) > sourceLimit(options) {
			return failure("offline-asset-scan-failed", "cannot read bounded generated asset")
		}
		if hasFetchBearingReference(data) {
			relative, _ := filepath.Rel(root, path)
			found = filepath.ToSlash(relative)
		}
		return nil
	})
	return found, err
}

func hasFetchBearingReference(data []byte) bool {
	lower := bytes.ToLower(data)
	markers := [][]byte{
		[]byte("src=\"http://"), []byte("src=\"https://"), []byte("src=\"//"),
		[]byte("src='http://"), []byte("src='https://"), []byte("src='//"),
		[]byte("url(http://"), []byte("url(https://"), []byte("url(//"),
		[]byte("@import \"http://"), []byte("@import \"https://"),
		[]byte("fetch(\"http://"), []byte("fetch(\"https://"),
		[]byte("new websocket(\"ws://"), []byte("new websocket(\"wss://"),
	}
	for _, marker := range markers {
		if bytes.Contains(lower, marker) {
			return true
		}
	}
	return false
}
