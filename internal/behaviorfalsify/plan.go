package behaviorfalsify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

const (
	maxDocumentBytes  = 32 << 20
	maxArtifactBytes  = 64 << 20
	maxWorkspaceBytes = 1 << 30
	maxWorkspaceFiles = 100_000
	// maxControlListLength bounds UnrelatedCriteria and RequiredSetup (BBF-V0-010): at this cap,
	// acceptedReceiptBound() in execute.go stays below maxDocumentBytes even when both lists are
	// full, so the per-receipt floor can never exceed the 32 MiB document bound.
	maxControlListLength = 1000
)

var (
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	gitOIDPattern = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	idPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,255}$`)
	envPattern    = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,127}$`)
)

var controlKinds = map[ControlKind]bool{
	WrongLocator: true, WrongExpectedValue: true, OmittedAssertion: true,
	OmittedEvent: true, ReorderedEvent: true, WrongProject: true,
	SuppressedPersistence: true, OppositeBranch: true, ChangedFixtureValue: true,
}

var reservedEnvironmentKeys = map[string]bool{
	"CORVINT_BEHAVIOR_COMMAND_ROLE": true,
	"HOME":                          true,
	"PATH":                          true,
}

func BuildPlan(request Request) (Plan, error) {
	request = canonicalRequest(request)
	if err := validateRequest(request); err != nil {
		return Plan{}, err
	}
	workspace, err := workspaceDigest(request.DisposableRoot)
	if err != nil {
		return Plan{}, fmt.Errorf("disposable workspace: %w", err)
	}
	planRequest := request
	planRequest.Controls = nil
	plan := Plan{Schema: PlanSchema, Request: planRequest, WorkspaceSHA256: workspace}
	for index, control := range request.Controls {
		perturbation := digestJSON(struct {
			Kind       ControlKind
			Definition map[string]string
			Hook       *Command
		}{control.Kind, control.Definition, control.Hook})
		plan.Controls = append(plan.Controls, PlannedControl{ControlSpec: control, Ordinal: index + 1, PerturbationSHA256: perturbation})
	}
	plan.Digest = digestJSON(plan)
	if err := ensureDocumentBound(plan); err != nil {
		return Plan{}, fmt.Errorf("plan: %w", err)
	}
	return plan, nil
}

func canonicalRequest(request Request) Request {
	if resolved, err := filepath.EvalSymlinks(request.DisposableRoot); err == nil {
		request.DisposableRoot = resolved
	}
	request.Controls = append([]ControlSpec(nil), request.Controls...)
	for index := range request.Controls {
		request.Controls[index].Definition = cloneDefinition(request.Controls[index].Definition)
		request.Controls[index].UnrelatedCriteria = append([]string(nil), request.Controls[index].UnrelatedCriteria...)
		request.Controls[index].RequiredSetup = append([]string(nil), request.Controls[index].RequiredSetup...)
		slices.Sort(request.Controls[index].UnrelatedCriteria)
		slices.Sort(request.Controls[index].RequiredSetup)
	}
	sort.Slice(request.Controls, func(i, j int) bool { return request.Controls[i].ID < request.Controls[j].ID })
	request.Repositories.Application = resolvePath(request.Repositories.Application)
	request.Repositories.Test = resolvePath(request.Repositories.Test)
	request.Repositories.Documentation = resolvePath(request.Repositories.Documentation)
	request.DeclaredEnvKeys = append([]string(nil), request.DeclaredEnvKeys...)
	slices.Sort(request.DeclaredEnvKeys)
	return request
}

func validateRequest(request Request) error {
	if err := validateTarget(request.Target); err != nil {
		return err
	}
	if err := validateRunner(request.Runner, request.DeclaredEnvKeys); err != nil {
		return err
	}
	if len(request.Controls) == 0 || len(request.Controls) > 64 || request.Attempts < 1 || request.Attempts > 16 || request.TimeoutSeconds < 1 || request.TimeoutSeconds > 3600 || request.WallClockSeconds < request.TimeoutSeconds || request.WallClockSeconds > 24*60*60 {
		return errors.New("invalid falsification bounds")
	}
	if time.Duration(request.WallClockSeconds)*time.Second <= cleanupReserve {
		return errors.New("wall clock budget does not exceed cleanup reserve")
	}
	if request.ExternalState != "none" {
		return errors.New("persistent external state is outside the admitted boundary")
	}
	if err := validateWorkspace(request); err != nil {
		return err
	}
	if err := validateRepositoryBindings(request); err != nil {
		return err
	}
	if err := validateCommand(request.Cleanup); err != nil {
		return fmt.Errorf("cleanup command: %w", err)
	}
	seen := map[string]bool{}
	for _, control := range request.Controls {
		if err := validateControl(request.Target, control); err != nil {
			return fmt.Errorf("control %q: %w", control.ID, err)
		}
		if seen[control.ID] {
			return errors.New("duplicate control identity")
		}
		seen[control.ID] = true
	}
	return nil
}

func validateTarget(target TargetIdentity) error {
	for _, value := range []string{target.ContractID, target.CriterionID, target.AssertionID, target.ContractFile, target.TestID, target.TestFile, target.TestTitle, target.Project} {
		if strings.TrimSpace(value) == "" || len(value) > 4096 {
			return errors.New("incomplete target identity")
		}
	}
	if !digestPattern.MatchString(target.ContractSHA256) || !gitOIDPattern.MatchString(target.ApplicationRevision) || !gitOIDPattern.MatchString(target.TestRevision) || !gitOIDPattern.MatchString(target.DocumentationRevision) || target.TestLine < 1 {
		return errors.New("invalid target revision or digest")
	}
	if !repositoryRelative(target.TestFile) || !repositoryRelative(target.ContractFile) {
		return errors.New("contract and test paths must be repository-relative")
	}
	return nil
}

func validateRunner(runner RunnerIdentity, keys []string) error {
	for _, value := range []string{runner.Runner, runner.RunnerVersion, runner.Browser, runner.BrowserVersion, runner.ConfigFile} {
		if strings.TrimSpace(value) == "" || len(value) > 4096 {
			return errors.New("incomplete runner identity")
		}
	}
	if runner.Runner != "playwright" || !digestPattern.MatchString(runner.ConfigSHA256) || !digestPattern.MatchString(runner.EnvironmentSHA256) {
		return errors.New("invalid runner identity")
	}
	if !repositoryRelative(runner.ConfigFile) {
		return errors.New("config path must be repository-relative")
	}
	if !uniqueStrings(keys, envPattern.MatchString) {
		return errors.New("invalid declared environment keys")
	}
	for _, key := range keys {
		if reservedEnvironmentKeys[key] {
			return errors.New("declared environment overrides runner boundary")
		}
	}
	if digestJSON(declaredEnvironment(keys)) != runner.EnvironmentSHA256 {
		return errors.New("declared environment identity mismatch")
	}
	return nil
}

func validateControl(target TargetIdentity, control ControlSpec) error {
	if !idPattern.MatchString(control.ID) || !controlKinds[control.Kind] || len(control.Definition) == 0 || len(control.Definition) > 64 {
		return errors.New("invalid control identity or definition")
	}
	for key, value := range control.Definition {
		if !idPattern.MatchString(key) || strings.TrimSpace(value) == "" || len(value) > 4096 {
			return errors.New("invalid perturbation definition")
		}
	}
	if !uniqueStrings(control.UnrelatedCriteria, func(value string) bool { return idPattern.MatchString(value) && value != target.CriterionID }) || !uniqueStrings(control.RequiredSetup, idPattern.MatchString) {
		return errors.New("invalid unrelated criterion or setup identity")
	}
	if len(control.UnrelatedCriteria) > maxControlListLength || len(control.RequiredSetup) > maxControlListLength {
		return errors.New("unrelated criteria or required setup list too long")
	}
	switch control.Disposition {
	case "run":
		if control.Hook == nil {
			return errors.New("supported control has no hook")
		}
		if err := validateCommand(*control.Hook); err != nil {
			return err
		}
	case "not_supported", "not_run":
		if control.Hook != nil {
			return errors.New("unexecuted control carries a hook")
		}
	default:
		return errors.New("invalid control disposition")
	}
	return nil
}

func validateWorkspace(request Request) error {
	root, err := cleanAbsoluteDir(request.DisposableRoot)
	if err != nil || root != request.DisposableRoot {
		return errors.New("disposable root must be an absolute real directory")
	}
	if request.Repositories.Application == "" || request.Repositories.Test == "" || request.Repositories.Documentation == "" {
		return errors.New("repository roots missing")
	}
	for _, protected := range repositoryRootList(request.Repositories) {
		clean, err := cleanAbsoluteDir(protected)
		if err != nil || clean != protected || pathsOverlap(root, clean) {
			return errors.New("disposable root overlaps a protected repository root")
		}
	}
	marker := filepath.Join(root, MarkerName)
	info, err := os.Lstat(marker)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || !digestPattern.MatchString(request.MarkerSHA256) {
		return errors.New("disposable marker missing or invalid")
	}
	actual, err := digestFile(marker, maxArtifactBytes)
	if err != nil || actual != request.MarkerSHA256 {
		return errors.New("disposable marker digest mismatch")
	}
	return nil
}

func validateRepositoryBindings(request Request) error {
	bindings := []struct {
		root     string
		revision string
	}{
		{request.Repositories.Application, request.Target.ApplicationRevision},
		{request.Repositories.Test, request.Target.TestRevision},
		{request.Repositories.Documentation, request.Target.DocumentationRevision},
	}
	for _, binding := range bindings {
		head, err := gitBytes(binding.root, "rev-parse", "HEAD")
		if err != nil || strings.TrimSpace(string(head)) != binding.revision {
			return errors.New("repository revision binding mismatch")
		}
	}
	contract, err := gitBlob(request.Repositories.Documentation, request.Target.DocumentationRevision, request.Target.ContractFile)
	if err != nil || digestBytes(contract) != request.Target.ContractSHA256 {
		return errors.New("contract revision or digest mismatch")
	}
	if _, err := gitBlob(request.Repositories.Test, request.Target.TestRevision, request.Target.TestFile); err != nil {
		return errors.New("test path missing at bound revision")
	}
	config, err := gitBlob(request.Repositories.Test, request.Target.TestRevision, request.Runner.ConfigFile)
	if err != nil || digestBytes(config) != request.Runner.ConfigSHA256 {
		return errors.New("runner config revision or digest mismatch")
	}
	return nil
}

func validateCommand(command Command) error {
	if !filepath.IsAbs(command.Path) || !digestPattern.MatchString(command.ExecutableSHA256) {
		return errors.New("command path or digest invalid")
	}
	info, err := os.Lstat(command.Path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0111 == 0 || info.Mode()&os.ModeSymlink != 0 || info.Size() > maxArtifactBytes {
		return errors.New("command executable unavailable")
	}
	digest, err := digestFile(command.Path, maxArtifactBytes)
	if err != nil || digest != command.ExecutableSHA256 {
		return errors.New("command executable drift")
	}
	return nil
}

func Decode(data []byte, value any) error {
	if len(data) > maxDocumentBytes {
		return errors.New("document exceeds bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}

func Encode(value any) ([]byte, error) {
	data, err := marshalJSON(value)
	if err != nil {
		return nil, err
	}
	if len(data)+1 > maxDocumentBytes {
		return nil, errors.New("document exceeds bound")
	}
	return append(data, '\n'), nil
}

func marshalJSON(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	data := buffer.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}
	return append([]byte(nil), data...), nil
}

func ensureDocumentBound(value any) error {
	_, err := Encode(value)
	return err
}

func workspaceDigest(root string) (string, error) {
	hash := sha256.New()
	files, bytesRead := 0, int64(0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.Mode().IsDir() && !info.Mode().IsRegular()) {
			return errors.New("workspace contains a symlink or special file")
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%s\x00%s\x00", filepath.ToSlash(relative), info.Mode().String())
		if info.Mode().IsDir() {
			return nil
		}
		files++
		bytesRead += info.Size()
		if files > maxWorkspaceFiles || bytesRead > maxWorkspaceBytes {
			return errors.New("workspace exceeds bound")
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(hash, "%d\x00", info.Size())
		copied, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if copied != info.Size() {
			return errors.New("workspace changed during digest")
		}
		return closeErr
	})
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func cleanAbsoluteDir(path string) (string, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", errors.New("path is not clean and absolute")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("path is not a real directory")
	}
	return path, nil
}

func resolvePath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

func repositoryRootList(roots RepositoryRoots) []string {
	values := []string{roots.Application, roots.Test, roots.Documentation}
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func repositoryRelative(path string) bool {
	return path != "" && !filepath.IsAbs(path) && filepath.Clean(path) == path && path != ".." && !strings.HasPrefix(path, ".."+string(filepath.Separator)) && !strings.Contains(path, ":")
}

func gitBlob(root, revision, path string) ([]byte, error) {
	return gitBytes(root, "cat-file", "blob", revision+":"+filepath.ToSlash(path))
}

func gitBytes(root string, args ...string) ([]byte, error) {
	return gitrun.Run(context.Background(), gitrun.NewBudget(1, 10*time.Second), gitrun.Options{
		Dir: root, Env: isolatedGitEnvironment(root), StdoutLimit: maxDocumentBytes,
	}, args...)
}

func isolatedGitEnvironment(root string) []string {
	return []string{
		"HOME=" + root, "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LANG=C", "LC_ALL=C", "TZ=UTC",
		"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull,
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1",
		"GIT_GRAFT_FILE=" + os.DevNull, "GCM_INTERACTIVE=never", "GIT_ASKPASS=", "GIT_CEILING_DIRECTORIES=" + filepath.Dir(root),
	}
}

func pathsOverlap(a, b string) bool { return within(a, b) || within(b, a) }

func within(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func uniqueStrings(values []string, valid func(string) bool) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if !valid(value) || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}

func declaredEnvironment(keys []string) map[string]string {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = os.Getenv(key)
	}
	return values
}

func digestFile(path string, limit int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > limit {
		return "", errors.New("file exceeds bound")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func digestJSON(value any) string {
	copyValue := value
	if plan, ok := value.(Plan); ok {
		plan.Digest = ""
		copyValue = plan
	}
	if report, ok := value.(Report); ok {
		report.Digest = ""
		copyValue = report
	}
	data, _ := marshalJSON(copyValue)
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func samePlan(a, b Plan) bool { return reflect.DeepEqual(a, b) }
