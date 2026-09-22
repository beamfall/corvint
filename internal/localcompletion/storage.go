package localcompletion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/secretscreen"
)

type repository struct {
	auth               *gitauth.Repository
	directory, session string
	generation         string
}

func open(root, key string) (*repository, error) {
	if !keyPattern.MatchString(key) {
		return nil, errors.New("invalid-session-key")
	}
	auth, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, errors.New("repository-unavailable")
	}
	return &repository{auth: auth, directory: filepath.Join(auth.GitDir, "corvint", "local-completion"), session: key}, nil
}

func strictJSON(raw []byte, output any, bound int) error {
	if len(raw) == 0 || len(raw) > bound {
		return errors.New("input-bound-exceeded")
	}
	// The protocol parser rejects duplicate fields at every nesting level.
	value, err := wire.Parse(raw)
	if err != nil {
		return errors.New("invalid-local-completion-json")
	}
	if err = validateJSONTypes(value, reflect.TypeOf(output).Elem()); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return errors.New("invalid-local-completion-schema")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return errors.New("invalid-local-completion-json")
	}
	return nil
}

func validateJSONTypes(value wire.Value, kind reflect.Type) error {
	bad := errors.New("invalid-local-completion-schema")
	if kind.Kind() == reflect.Pointer {
		if value.Kind == wire.KindNull {
			return nil
		}
		return validateJSONTypes(value, kind.Elem())
	}
	switch kind.Kind() {
	case reflect.Struct:
		if value.Kind != wire.KindObject {
			return bad
		}
		for index := 0; index < kind.NumField(); index++ {
			field := kind.Field(index)
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			member, present := value.Obj.Get(name)
			if !present {
				if kind == reflect.TypeFor[Check]() && name == "allowCemSidecarOnlyReuse" {
					continue
				}
				return bad
			}
			if err := validateJSONTypes(member, field.Type); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if value.Kind == wire.KindNull {
			return nil
		}
		if value.Kind != wire.KindArray {
			return bad
		}
		for _, member := range value.Arr {
			if err := validateJSONTypes(member, kind.Elem()); err != nil {
				return err
			}
		}
	case reflect.String:
		if value.Kind != wire.KindString {
			return bad
		}
	case reflect.Bool:
		if value.Kind != wire.KindBool {
			return bad
		}
	case reflect.Int:
		if value.Kind != wire.KindInt {
			return bad
		}
	default:
		return bad
	}
	return nil
}

func validPath(value string) bool {
	if value == "" || len(value) > 512 || path.Clean(value) != value || value == "." || value == ".." || strings.HasPrefix(value, "../") || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return false
	}
	for _, c := range value {
		if c < 32 || c == 127 {
			return false
		}
	}
	return true
}

func validatePlan(plan Plan) error {
	if !wire.IsGitOid(plan.Base) {
		return errors.New("immutable-base-required")
	}
	if len(plan.Intents) < 1 || len(plan.Intents) > 16 || len(plan.Checks) < 1 || len(plan.Checks) > 16 {
		return errors.New("plan-bound-exceeded")
	}
	previous := ""
	for _, intent := range plan.Intents {
		if !validPath(intent) || intent <= previous {
			return errors.New("invalid-intent-scope")
		}
		if secretscreen.MatchString(intent) {
			return errors.New("secret-shaped-plan")
		}
		previous = intent
	}
	seen := map[string]bool{}
	for _, check := range plan.Checks {
		if !idPattern.MatchString(check.ID) || seen[check.ID] {
			return errors.New("invalid-check-id")
		}
		seen[check.ID] = true
		if len(check.Argv) < 1 || len(check.Argv) > 64 || check.TimeoutSeconds < 1 || check.TimeoutSeconds > 3600 {
			return errors.New("invalid-check-bound")
		}
		for index, arg := range check.Argv {
			if index == 0 && arg == "" || len(arg) > 4096 || strings.ContainsRune(arg, 0) {
				return errors.New("invalid-check-argv")
			}
		}
		if secretscreen.MatchString(strings.Join(check.Argv, " ")) {
			return errors.New("secret-shaped-plan")
		}
		for index, arg := range check.Argv {
			if index+1 < len(check.Argv) && strings.HasPrefix(arg, "--") && secretscreen.MatchString(strings.TrimPrefix(arg, "--")+"="+check.Argv[index+1]) {
				return errors.New("secret-shaped-plan")
			}
		}
		for _, arg := range check.Argv {
			if arg == "dogfood-check" || strings.HasSuffix(arg, "/dogfood-check.sh") {
				return errors.New("final-check-not-prerequisite")
			}
		}
	}
	return nil
}

// checkParents rejects static symlinks; hostile same-UID replacement is outside
// this local derived-state trust boundary, as in the underlying CEM publisher.
func checkParents(name string) error {
	clean := filepath.Clean(name)
	for current := clean; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return errors.New("local-state-symlink")
		}
		if current == filepath.Dir(current) {
			return nil
		}
	}
}

func readFile(name string, bound int) ([]byte, error) {
	if err := checkParents(name); err != nil {
		return nil, err
	}
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("local-state-not-regular")
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("local-state-not-regular")
	}
	raw, err := io.ReadAll(io.LimitReader(file, int64(bound)+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > bound {
		return nil, errors.New("local-state-bound-exceeded")
	}
	return raw, nil
}

// ReadPlan reads one bounded regular caller-owned input. Errors contain no
// rejected plan body or filesystem path.
func ReadPlan(name string) ([]byte, error) {
	parent, err := filepath.EvalSymlinks(filepath.Dir(name))
	if err != nil {
		return nil, errors.New("plan-unavailable")
	}
	name = filepath.Join(parent, filepath.Base(name))
	raw, err := readFile(name, MaxPlanBytes)
	if err != nil {
		return nil, errors.New("plan-unavailable")
	}
	return raw, nil
}

func writeFile(name string, raw []byte) error {
	if len(raw) > maxArtifactBytes {
		return errors.New("local-state-bound-exceeded")
	}
	if err := checkParents(name); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		return err
	}
	if info, err := os.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return errors.New("local-state-not-regular")
	}
	file, err := os.CreateTemp(filepath.Dir(name), ".publish-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if _, err = file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(temporary, name)
}

func (repo *repository) local(name string) string {
	if name == "state.json" || repo.generation == "" {
		return filepath.Join(repo.directory, repo.session, name)
	}
	return filepath.Join(repo.directory, repo.session, repo.generation, name)
}
func (repo *repository) load() (*state, error) {
	raw, err := readFile(repo.local("state.json"), maxStateBytes)
	if err != nil {
		return nil, err
	}
	var saved state
	if err = strictJSON(raw, &saved, maxStateBytes); err != nil {
		return nil, err
	}
	if err = validateStateMembers(raw); err != nil {
		return nil, err
	}
	if saved.Session != repo.session || saved.PlanDigest != valueDigest(saved.Plan) {
		return nil, errors.New("enrollment-drift")
	}
	if len(saved.Generation) != 68 || !strings.HasPrefix(saved.Generation, saved.PlanDigest+"-") {
		return nil, errors.New("invalid-enrollment-generation")
	}
	for _, c := range saved.Generation[65:] {
		if c < '0' || c > '9' {
			return nil, errors.New("invalid-enrollment-generation")
		}
	}
	repo.generation = saved.Generation
	if err = validatePlan(saved.Plan); err != nil {
		return nil, err
	}
	if saved.Lifecycle != "active" && saved.Lifecycle != "satisfied" && saved.Lifecycle != "cancelled" {
		return nil, errors.New("invalid-lifecycle")
	}
	if len(saved.Observations) > maxAttempts || len(saved.IntentPointers) != len(saved.Plan.Intents) || len(saved.Executables) != len(saved.Plan.Checks) {
		return nil, errors.New("enrollment-bound-exceeded")
	}
	for _, executable := range saved.Executables {
		if !filepath.IsAbs(executable) || filepath.Clean(executable) != executable || len(executable) > 4096 {
			return nil, errors.New("invalid-execution-path")
		}
	}
	for index, pointer := range saved.IntentPointers {
		if pointer.Path != saved.Plan.Intents[index] || !wire.IsGitOid(pointer.Revision) || (pointer.BlobHash != "" && !wire.IsGitOid(pointer.BlobHash)) {
			return nil, errors.New("invalid-enrollment-pointer")
		}
	}
	for index, observed := range saved.Observations {
		exit, parseErr := strconv.Atoi(observed.Exit)
		if parseErr != nil || exit < -1 || exit > 255 || strconv.Itoa(exit) != observed.Exit {
			return nil, errors.New("invalid-verification-exit")
		}
		if observed.Stdout.Path != repo.local(fmt.Sprintf("check-%03d-0.log", index+1)) || observed.Stderr.Path != repo.local(fmt.Sprintf("check-%03d-1.log", index+1)) || !wire.IsGitOid(observed.Target) || !wire.IsGitOid(observed.Tree) || !keyPattern.MatchString(observed.CheckDigest) || !keyPattern.MatchString(observed.ContentDigest) {
			return nil, errors.New("invalid-verification-observation")
		}
	}
	if saved.Review != "" && !keyPattern.MatchString(saved.Review) {
		return nil, errors.New("invalid-review-digest")
	}
	return &saved, nil
}

func requireFields(raw []byte, names ...string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || object == nil {
		return nil, errors.New("invalid-local-completion-schema")
	}
	for _, name := range names {
		if _, exists := object[name]; !exists {
			return nil, errors.New("missing-local-completion-field")
		}
	}
	return object, nil
}

func validateStateMembers(raw []byte) error {
	object, err := requireFields(raw, "session", "lifecycle", "plan", "planDigest", "intentPointers", "observations", "reportSet", "review", "terminal", "coordination", "executables", "generation")
	if err != nil {
		return err
	}
	if string(object["terminal"]) != "null" {
		if _, err = requireFields(object["terminal"], "reportSet", "checkExit", "artifacts"); err != nil {
			return err
		}
	}
	if string(object["reportSet"]) != "null" {
		if _, err = requireFields(object["reportSet"], "planDigest", "target", "bindings", "reports", "observations", "digest"); err != nil {
			return err
		}
	}
	var observations []json.RawMessage
	if json.Unmarshal(object["observations"], &observations) != nil {
		return errors.New("invalid-verification-observation")
	}
	for _, item := range observations {
		if _, err = requireFields(item, "checkId", "checkDigest", "testedCommit", "tree", "contentExcludingCem", "clean", "exit", "passed", "timedOut", "cancelled", "overflow", "secretScreened", "stdout", "stderr"); err != nil {
			return err
		}
	}
	return nil
}

func (repo *repository) save(saved *state) error {
	raw, err := json.Marshal(saved)
	if err != nil {
		return err
	}
	if len(raw) > maxStateBytes {
		return errors.New("local-state-bound-exceeded")
	}
	return writeFile(repo.local("state.json"), append(raw, '\n'))
}

func (repo *repository) owner() (string, error) {
	raw, err := readFile(filepath.Join(repo.directory, "owner"), 65)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	key := strings.TrimSuffix(string(raw), "\n")
	if !keyPattern.MatchString(key) {
		return "", errors.New("invalid-worktree-owner")
	}
	return key, nil
}

func (repo *repository) lock() (func(), error) {
	if err := checkParents(repo.directory); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(repo.directory, 0700); err != nil {
		return nil, err
	}
	name := filepath.Join(repo.directory, "operation.lock")
	if err := os.Mkdir(name, 0700); err != nil {
		return nil, errors.New("operation-in-progress")
	}
	return func() { _ = os.Remove(name) }, nil
}

func (repo *repository) requireOwner() error {
	owner, err := repo.owner()
	if err != nil {
		return err
	}
	if owner != repo.session {
		return errors.New("worktree-owner-mismatch")
	}
	return nil
}

func (repo *repository) git(ctx context.Context, args ...string) ([]byte, error) {
	environment := []string{"PATH=" + os.Getenv("PATH"), "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull, "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_TERMINAL_PROMPT=0"}
	argv := []string{"--no-optional-locks", "--git-dir=" + repo.auth.GitDir, "--work-tree=" + repo.auth.Root, "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "core.attributesFile=" + os.DevNull}
	return gitrun.Run(ctx, gitrun.NewBudget(1, 10*time.Second), gitrun.Options{Env: environment, StdoutLimit: maxArtifactBytes}, append(argv, args...)...)
}

// requireBaseAncestor distinguishes Git's clean not-ancestor exit from a probe
// failure, which stays an error rather than a claim about history.
func (repo *repository) requireBaseAncestor(ctx context.Context, base, target string) error {
	_, err := repo.git(ctx, "merge-base", "--is-ancestor", base, target)
	if err == nil {
		return nil
	}
	details, exited := cemcode.GitExitFailureDetails(err)
	if !exited || details.ExitCode != 1 || len(details.Stderr) != 0 {
		return err
	}
	return errors.New("base-not-ancestor-of-target")
}

type snapshot struct {
	target, tree, content string
	clean                 bool
}

func (repo *repository) snapshot(ctx context.Context) (snapshot, error) {
	var snap snapshot
	if err := repo.refreshAuthority(); err != nil {
		return snap, err
	}
	target, err := repo.auth.Resolve(ctx, "HEAD")
	if err != nil {
		return snap, err
	}
	snap.target = target
	tree, err := repo.git(ctx, "rev-parse", target+"^{tree}")
	if err != nil {
		return snap, err
	}
	snap.tree = strings.TrimSpace(string(tree))
	status, err := repo.git(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return snap, err
	}
	snap.clean = len(status) == 0
	listing, err := repo.git(ctx, "ls-tree", "-rz", target)
	if err != nil {
		return snap, err
	}
	var included []byte
	for _, row := range bytes.Split(listing, []byte{0}) {
		if len(row) == 0 {
			continue
		}
		pieces := bytes.SplitN(row, []byte{'\t'}, 2)
		if len(pieces) != 2 {
			return snap, errors.New("invalid-tree-listing")
		}
		if string(pieces[1]) == ".corvint/change.cem.json" {
			continue
		}
		included = append(included, row...)
		included = append(included, 0)
	}
	snap.content = digest(included)
	return snap, nil
}

// Git read budgets cover read phases, never the potentially hour-long selected
// command between them. Reopening revalidates the same administrative identity.
func (repo *repository) refreshAuthority() error {
	current, err := gitauth.Open(repo.auth.Root, gitrun.NewDefaultBudget())
	if err != nil {
		return err
	}
	if current.Root != repo.auth.Root || current.GitDir != repo.auth.GitDir || current.CommonDir != repo.auth.CommonDir {
		return errors.New("repository-identity-changed")
	}
	repo.auth = current
	return nil
}

func resolveExecutable(name string) (string, error) {
	resolved, err := exec.LookPath(name)
	if err != nil {
		return "", errors.New("check-executable-unavailable")
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		return "", errors.New("check-executable-unavailable")
	}
	return absolute, nil
}

func artifactFor(name string) (artifact, error) {
	raw, err := readFile(name, maxArtifactBytes)
	if err != nil {
		return artifact{}, err
	}
	return artifact{Path: name, Digest: digest(raw)}, nil
}
func artifactsCurrent(items []artifact) bool {
	for _, item := range items {
		if item.Path == "" || !keyPattern.MatchString(item.Digest) {
			return false
		}
		now, err := artifactFor(item.Path)
		if err != nil || now.Digest != item.Digest {
			return false
		}
	}
	return true
}
