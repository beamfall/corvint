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
	"sync"
	"time"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/dogfoodflow"
	"github.com/Beamfall/corvint/internal/dogfoodoperation"
	"github.com/Beamfall/corvint/internal/secretscreen"
	"github.com/Beamfall/corvint/internal/tracerecordrepo"
)

type repository struct {
	aggregateFault     func(string) error
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
				if kind == reflect.TypeFor[state]() && name == "aggregateOutcome" {
					continue
				}
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
	case reflect.Int, reflect.Int64:
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
		for index, arg := range check.Argv {
			if arg == "dogfood-check" || strings.HasSuffix(arg, "/dogfood-check.sh") || runsDogfoodCheck(check.Argv[index:]) {
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
	if err = repo.validateAggregateState(&saved); err != nil {
		return nil, err
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
	if rawAggregate, present := object["aggregateOutcome"]; present && string(rawAggregate) == "null" {
		return errors.New("aggregate-schema-invalid")
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
	return dogfoodoperation.LockDirectory(repo.directory)
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

// runsDogfoodCheck reports `dogfood check` or `dogfood seal`, which run the
// final check itself (LCP-V0-015).
func runsDogfoodCheck(args []string) bool {
	return len(args) > 1 && args[0] == "dogfood" && (args[1] == "check" || args[1] == "seal")
}

// Preserve the existing local-completion diagnostic and filesystem cause.
func operationLockFailure(err error) error { return dogfoodoperation.Failure(err) }

const aggregateHistoryLimit int64 = 64 << 20
const aggregateSnapshotProfile = "corvint-dogfood-aggregate-snapshot/0"

var aggregateSnapshotRoles = []string{"check-capture", "check-stderr", "check-stdout", "legacy-failure", "legacy-stderr", "legacy-stdout", "prior-outcome", "prior-report", "state-before"}

func prefixedDigest(raw []byte) string { return "sha256:" + digest(raw) }
func validAggregateDigest(value string) bool {
	return strings.HasPrefix(value, "sha256:") && keyPattern.MatchString(strings.TrimPrefix(value, "sha256:"))
}
func aggregateCanonicalJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	parsed, err := wire.Parse(raw)
	if err != nil {
		return nil, err
	}
	return wire.CanonicalValue(parsed), nil
}
func snapshotDigest(value aggregateSnapshotManifest) (string, error) {
	raw, err := aggregateCanonicalJSON(value)
	if err != nil {
		return "", err
	}
	return prefixedDigest(append([]byte("corvint-dogfood-aggregate-snapshot/v0\x00"), raw...)), nil
}
func aggregateOldValid(value *aggregateOld, role string) bool {
	if value == nil {
		return true
	}
	if !validAggregateDigest(value.SHA256) || value.Bytes < 0 || value.Bytes > maxArtifactBytes {
		return false
	}
	if role == "outcome" {
		return value.Kind == "legacy-outcome" || value.Kind == "aggregate-outcome" || value.Kind == "failed-recorder-output"
	}
	return value.Kind == "legacy-report" || value.Kind == "aggregate-report"
}

// validateAggregateState performs only bounded reads. A pending marker loads
// before any evaluation and never adopts reservations, repairs files or takes a
// writer lock on status/event paths.
func (repo *repository) validateAggregateState(saved *state) error {
	a := saved.AggregateOutcome
	if a == nil {
		return nil
	}
	bad := errors.New("aggregate-schema-invalid")
	if a.Profile != tracerecordrepo.AggregateOutcomeProfile || !validAggregateDigest(a.BindingSHA256) || !validAggregateDigest(a.ReceiptSHA256) || !validAggregateDigest(a.LegacyFailureSHA256) || !validAggregateDigest(a.ActiveSnapshotSHA256) || a.AttemptCount < 1 || a.AttemptCount > 64 || len(a.Snapshots) < 1 || len(a.Snapshots) > 64 || a.Issued == nil || len(a.Issued) > 3 {
		return bad
	}
	if a.Phase != "PREPARED" && a.Phase != "OUTCOME_PUBLISHED" && a.Phase != "REPORT_PUBLISHED" && a.Phase != "COMMITTED" {
		return bad
	}
	if a.PreservationState != "RESERVED" && a.PreservationState != "SEALED" {
		return bad
	}
	if !aggregateOldValid(a.ExpectedOld.Outcome, "outcome") || !aggregateOldValid(a.ExpectedOld.Report, "report") {
		return bad
	}
	roles := []string{"outcome", "report", "check-report"}
	for i, item := range a.Issued {
		if item.Role != roles[i] || !validAggregateDigest(item.SHA256) || item.Bytes < 1 || item.Bytes > maxArtifactBytes {
			return bad
		}
	}
	if len(a.Issued) > 0 && a.Issued[0].SHA256 != a.ReceiptSHA256 {
		return bad
	}
	minimum := map[string]int{"PREPARED": 0, "OUTCOME_PUBLISHED": 1, "REPORT_PUBLISHED": 2, "COMMITTED": 3}[a.Phase]
	if len(a.Issued) < minimum {
		return bad
	}
	active := false
	for i, reference := range a.Snapshots {
		if reference.Ordinal != i+1 || !validAggregateDigest(reference.SnapshotSHA256) || reference.ReservedBytes < 1 || reference.ReservedBytes > aggregateHistoryLimit {
			return bad
		}
		if reference.SnapshotSHA256 == a.ActiveSnapshotSHA256 {
			if active || reference.Ordinal != a.AttemptCount {
				return bad
			}
			active = true
		}
	}
	if !active || a.AttemptCount != len(a.Snapshots) {
		return bad
	}
	if a.Phase != "COMMITTED" && (saved.Lifecycle == "satisfied" || saved.Terminal != nil) {
		return bad
	}
	if err := repo.validateAggregateLedger(a); err != nil {
		return err
	}
	_, err := repo.aggregatePublishedPrefix(saved)
	return err
}

func (repo *repository) aggregateDirectory() string { return repo.local("aggregate-outcome") }
func (repo *repository) aggregateSnapshotDirectory(binding, snapshot string) string {
	return filepath.Join(repo.aggregateDirectory(), "history", strings.TrimPrefix(binding, "sha256:"), strings.TrimPrefix(snapshot, "sha256:"))
}
func (repo *repository) aggregateReservationPath(ordinal int) string {
	return filepath.Join(repo.aggregateDirectory(), "reservations", fmt.Sprintf("%03d.json", ordinal))
}
func (repo *repository) aggregateSourcePaths() map[string]string {
	return map[string]string{
		"check-capture":  repo.local("aggregate-check.json"),
		"state-before":   repo.local("state.json"),
		"prior-outcome":  filepath.Join(repo.auth.GitDir, "corvint", "local-outcome.json"),
		"prior-report":   filepath.Join(repo.auth.Root, ".corvint", "dogfood-report.json"),
		"legacy-failure": repo.local("aggregate-outcome/legacy-failure.json"),
		"legacy-stdout":  repo.local("aggregate-outcome/legacy-failure.stdout"),
		"legacy-stderr":  repo.local("aggregate-outcome/legacy-failure.stderr"),
		"check-stdout":   repo.local("final-check.stdout"),
		"check-stderr":   repo.local("final-check.stderr"),
	}
}

func aggregateEntryMatches(entry aggregateSnapshotEntry, raw []byte, err error) bool {
	if !entry.Present {
		return os.IsNotExist(err)
	}
	return err == nil && entry.SHA256 != nil && int64(len(raw)) == entry.Bytes && prefixedDigest(raw) == *entry.SHA256
}
func aggregateEntriesValid(entries []aggregateSnapshotEntry, sourceState string) bool {
	if len(entries) != len(aggregateSnapshotRoles) {
		return false
	}
	for i, entry := range entries {
		if entry.Role != aggregateSnapshotRoles[i] || entry.Bytes < 0 || entry.Bytes > maxArtifactBytes {
			return false
		}
		if entry.Present {
			if entry.SHA256 == nil || !validAggregateDigest(*entry.SHA256) {
				return false
			}
		} else if entry.SHA256 != nil || entry.Bytes != 0 {
			return false
		}
		if entry.Role == "state-before" && (!entry.Present || entry.SHA256 == nil || *entry.SHA256 != sourceState) {
			return false
		}
	}
	return true
}

func (repo *repository) readAggregateReservation(ordinal int) (aggregateReservation, error) {
	var value aggregateReservation
	raw, err := readFile(repo.aggregateReservationPath(ordinal), maxStateBytes)
	if err != nil {
		return value, err
	}
	if err = strictJSON(raw, &value, maxStateBytes); err != nil {
		return value, err
	}
	canonical, err := aggregateCanonicalJSON(value)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || value.Ordinal != ordinal || ordinal < 1 || ordinal > 64 || !validAggregateDigest(value.BindingSHA256) || !validAggregateDigest(value.SnapshotSHA256) || !validAggregateDigest(value.SourceStateSHA256) || !aggregateEntriesValid(value.Entries, value.SourceStateSHA256) || !aggregateOldValid(value.ExpectedOld.Outcome, "outcome") || !aggregateOldValid(value.ExpectedOld.Report, "report") || value.MaximumBytes < 1 || value.MaximumBytes > aggregateHistoryLimit {
		return value, errors.New("aggregate-prior-evidence-drift")
	}
	manifest := aggregateSnapshotManifest{aggregateSnapshotProfile, value.BindingSHA256, value.SourceStateSHA256, value.Entries}
	identity, err := snapshotDigest(manifest)
	if err != nil || identity != value.SnapshotSHA256 {
		return value, errors.New("aggregate-prior-evidence-drift")
	}
	return value, nil
}

// aggregatePreservation reads both reservation and payload closure. A final
// manifest can prove SEALED even if the writer was interrupted before its ack.
func (repo *repository) aggregatePreservation(a *aggregateState) (bool, error) {
	reference := a.Snapshots[len(a.Snapshots)-1]
	reservation, err := repo.readAggregateReservation(reference.Ordinal)
	if err != nil {
		return false, err
	}
	if reservation.BindingSHA256 != a.BindingSHA256 || reservation.SnapshotSHA256 != a.ActiveSnapshotSHA256 || reservation.MaximumBytes != reference.ReservedBytes || !reflect.DeepEqual(reservation.ExpectedOld, a.ExpectedOld) {
		return false, errors.New("aggregate-prior-evidence-drift")
	}
	directory := repo.aggregateSnapshotDirectory(reservation.BindingSHA256, reservation.SnapshotSHA256)
	manifestRaw, manifestErr := readFile(filepath.Join(directory, "manifest.json"), maxStateBytes)
	sealed := manifestErr == nil
	if manifestErr != nil && !os.IsNotExist(manifestErr) {
		return false, manifestErr
	}
	expected := aggregateSnapshotManifest{aggregateSnapshotProfile, reservation.BindingSHA256, reservation.SourceStateSHA256, reservation.Entries}
	encoded, err := aggregateCanonicalJSON(expected)
	if err != nil {
		return false, err
	}
	if sealed && !bytes.Equal(manifestRaw, append(encoded, '\n')) {
		return false, errors.New("aggregate-prior-evidence-drift")
	}
	if !sealed && a.PreservationState == "SEALED" {
		return false, errors.New("aggregate-prior-evidence-drift")
	}
	live, sourceErr := repo.aggregateReservationSources(reservation)
	if sourceErr != nil {
		return false, sourceErr
	}
	for _, entry := range reservation.Entries {
		payload := filepath.Join(directory, entry.Role+".bin")
		raw, readErr := readFile(payload, maxArtifactBytes)
		if sealed || entry.Role == "state-before" {
			if !aggregateEntryMatches(entry, raw, readErr) {
				return false, errors.New("aggregate-prior-evidence-drift")
			}
		} else if !os.IsNotExist(readErr) {
			if !aggregateEntryMatches(entry, raw, readErr) {
				return false, errors.New("aggregate-prior-evidence-drift")
			}
		} else {
			original, originalErr := readFile(live[entry.Role], maxArtifactBytes)
			if !aggregateEntryMatches(entry, original, originalErr) {
				return false, errors.New("aggregate-prior-evidence-drift")
			}
		}
	}
	return sealed, nil
}

// writeAggregateExclusive closes a unique stage before its exclusive install.
// Error stages remain visible and charged; successful cleanup is observed.
// Legacy capture uses its caller's existing lifecycle; selected aggregate
// publication carries the original operation deadline through every write.
func writeAggregateExclusive(name string, raw []byte) error {
	return writeAggregateExclusiveContext(context.Background(), name, raw)
}

func aggregateWriteAllowed(ctx context.Context) error {
	if err := dogfoodoperation.Check(ctx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if budget := gitrun.OperationBudgetFrom(ctx); budget != nil && !time.Now().Before(budget.Deadline()) {
		return errors.New("aggregate-operation-deadline")
	}
	return nil
}

func writeAggregateExclusiveContext(ctx context.Context, name string, raw []byte) error {
	if err := aggregateWriteAllowed(ctx); err != nil {
		return err
	}
	if len(raw) > maxArtifactBytes {
		return errors.New("aggregate-history-bound-exceeded")
	}
	if err := checkParents(name); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(name), ".aggregate-stage-")
	if err != nil {
		return err
	}
	temporary := file.Name()
	if _, err = file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	closed, err := readFile(temporary, maxArtifactBytes)
	if err != nil || !bytes.Equal(closed, raw) {
		return errors.New("aggregate-publication-failed")
	}
	if err = aggregateWriteAllowed(ctx); err != nil {
		return err
	}
	if err = os.Link(temporary, name); err != nil {
		return err
	}
	if err = os.Remove(temporary); err != nil {
		return err
	}
	return nil
}

func (repo *repository) aggregateReservations() ([]aggregateReservation, error) {
	directory := filepath.Join(repo.aggregateDirectory(), "reservations")
	if err := checkParents(directory); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	values := []aggregateReservation{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".aggregate-stage-") {
			continue
		}
		ordinal := len(values) + 1
		if entry.IsDir() || entry.Name() != fmt.Sprintf("%03d.json", ordinal) || ordinal > 64 {
			return nil, errors.New("aggregate-prior-evidence-drift")
		}
		reservation, err := repo.readAggregateReservation(ordinal)
		if err != nil {
			return nil, err
		}
		values = append(values, reservation)
	}
	return values, nil
}

// Count each owned path once, including fault stages. Each reservation keeps
// its conservative unused allowance; no age-based refund or eviction occurs.
func (repo *repository) aggregateHistoryUsage() (int64, error) {
	reservations, err := repo.aggregateReservations()
	if err != nil {
		return 0, err
	}
	directory := repo.aggregateDirectory()
	if err := checkParents(directory); err != nil {
		return 0, err
	}
	var total int64
	counts := make([]int64, len(reservations))
	err = filepath.WalkDir(directory, func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("aggregate-prior-evidence-drift")
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return errors.New("aggregate-prior-evidence-drift")
		}
		total += info.Size()
		if total > aggregateHistoryLimit {
			return errors.New("aggregate-history-bound-exceeded")
		}
		for i, reservation := range reservations {
			prefix := repo.aggregateSnapshotDirectory(reservation.BindingSHA256, reservation.SnapshotSHA256) + string(os.PathSeparator)
			if name == repo.aggregateReservationPath(reservation.Ordinal) || strings.HasPrefix(name, prefix) {
				counts[i] += info.Size()
				break
			}
		}
		return nil
	})
	if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		return 0, err
	}
	// Same-directory publication stages live beside the shared artifact. Only
	// this enrollment's explicit prefix is charged; unrelated files are left alone.
	ownedPrefix := ".aggregate-" + repo.session + "-" + repo.generation + "-"
	for _, parent := range []string{filepath.Join(repo.auth.GitDir, "corvint"), filepath.Join(repo.auth.Root, ".corvint")} {
		if err := checkParents(parent); err != nil {
			return 0, err
		}
		dir, openErr := os.Open(parent)
		if os.IsNotExist(openErr) {
			continue
		}
		if openErr != nil {
			return 0, openErr
		}
		entries, readErr := dir.ReadDir(4097)
		closeErr := dir.Close()
		if readErr != nil && readErr != io.EOF {
			return 0, readErr
		}
		if closeErr != nil {
			return 0, closeErr
		}
		if len(entries) > 4096 {
			return 0, errors.New("aggregate-history-bound-exceeded")
		}
		for _, entry := range entries {
			if !strings.HasPrefix(entry.Name(), ownedPrefix) {
				continue
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return 0, errors.New("aggregate-prior-evidence-drift")
			}
			info, err := entry.Info()
			if err != nil {
				return 0, err
			}
			if !info.Mode().IsRegular() {
				return 0, errors.New("aggregate-prior-evidence-drift")
			}
			matched := false
			for i, reservation := range reservations {
				if strings.HasPrefix(entry.Name(), repo.aggregateStagePrefix(reservation.Ordinal)) {
					counts[i] += info.Size()
					matched = true
					break
				}
			}
			if !matched {
				return 0, errors.New("aggregate-prior-evidence-drift")
			}
			total += info.Size()
		}
	}
	for i, reservation := range reservations {
		if counts[i] > reservation.MaximumBytes {
			return 0, errors.New("aggregate-history-bound-exceeded")
		}
		total += reservation.MaximumBytes - counts[i]
	}
	if total > aggregateHistoryLimit {
		return 0, errors.New("aggregate-history-bound-exceeded")
	}
	return total, nil
}

func (repo *repository) aggregateCaptureSnapshot(saved *state, binding string) (aggregateSnapshotManifest, map[string][]byte, error) {
	result := aggregateSnapshotManifest{Profile: aggregateSnapshotProfile, BindingSHA256: binding, Entries: []aggregateSnapshotEntry{}}
	data := map[string][]byte{}
	paths := repo.aggregateSnapshotPaths(saved)
	for _, role := range aggregateSnapshotRoles {
		raw, err := readFile(paths[role], maxArtifactBytes)
		if err != nil && !os.IsNotExist(err) {
			return result, nil, err
		}
		entry := aggregateSnapshotEntry{Role: role, Present: err == nil}
		if err == nil {
			identity := prefixedDigest(raw)
			entry.SHA256 = &identity
			entry.Bytes = int64(len(raw))
			data[role] = raw
			if role == "state-before" {
				result.SourceStateSHA256 = identity
			}
		}
		result.Entries = append(result.Entries, entry)
	}
	if !aggregateEntriesValid(result.Entries, result.SourceStateSHA256) {
		return result, nil, errors.New("aggregate-prior-evidence-drift")
	}
	return result, data, nil
}

func (repo *repository) aggregateFaultPoint(name string) error {
	if repo.aggregateFault != nil {
		return repo.aggregateFault(name)
	}
	return nil
}

// preserveAggregate reserves before writing payloads, saves state-before before
// its first state update, and resumes at most one consecutive orphan ordinal.
func (repo *repository) preserveAggregate(ctx context.Context, saved *state, proposed *aggregateState, receipt []byte) error {
	if err := aggregateWriteAllowed(ctx); err != nil {
		return err
	}
	if saved.AggregateOutcome != nil {
		proposed = saved.AggregateOutcome
		sealed, err := repo.aggregatePreservation(proposed)
		if err != nil {
			return err
		}
		if sealed {
			// A process may die after installing the closed manifest but
			// before acknowledging it in state. Only this explicit resume
			// repairs the acknowledgement; read-only consumers do not write.
			if proposed.PreservationState != "SEALED" {
				if err := aggregateWriteAllowed(ctx); err != nil {
					return err
				}
				proposed.PreservationState = "SEALED"
				return repo.save(saved)
			}
			return nil
		}
		reservation, err := repo.readAggregateReservation(proposed.AttemptCount)
		if err != nil {
			return err
		}
		return repo.finishAggregatePreservation(ctx, saved, reservation)
	}
	return repo.reserveAggregateSnapshot(ctx, saved, proposed, receipt)
}

func (repo *repository) reserveAggregateSnapshot(ctx context.Context, saved *state, proposed *aggregateState, receipt []byte) error {
	if err := aggregateWriteAllowed(ctx); err != nil {
		return err
	}
	previous := 0
	references := []aggregateSnapshotReference{}
	if saved.AggregateOutcome != nil {
		previous = saved.AggregateOutcome.AttemptCount
		references = append(references, saved.AggregateOutcome.Snapshots...)
		if saved.AggregateOutcome.BindingSHA256 != proposed.BindingSHA256 {
			return errors.New("aggregate-binding-drift")
		}
	}
	ordinal := previous + 1
	if ordinal > 64 {
		return errors.New("aggregate-history-bound-exceeded")
	}

	manifest, payloads, err := repo.aggregateCaptureSnapshot(saved, proposed.BindingSHA256)
	if err != nil {
		return err
	}
	identity, err := snapshotDigest(manifest)
	if err != nil {
		return err
	}
	reservations, err := repo.aggregateReservations()
	if err != nil {
		return err
	}
	// At most one consecutive original reservation may await its state ack.
	if len(reservations) < previous || len(reservations) > ordinal {
		return errors.New("aggregate-prior-evidence-drift")
	}
	maximum := int64(len(receipt)) + 3*maxArtifactBytes + 4*maxLogBytes + 6*maxStateBytes
	for _, entry := range manifest.Entries {
		maximum += entry.Bytes
	}
	reservation := aggregateReservation{Ordinal: ordinal, BindingSHA256: proposed.BindingSHA256, SnapshotSHA256: identity, SourceStateSHA256: manifest.SourceStateSHA256, ExpectedOld: proposed.ExpectedOld, Entries: manifest.Entries, MaximumBytes: maximum}
	if len(reservations) == ordinal {
		if !reflect.DeepEqual(reservations[ordinal-1], reservation) {
			return errors.New("aggregate-prior-evidence-drift")
		}
	} else {
		used, err := repo.aggregateHistoryUsage()
		if err != nil {
			return err
		}
		if used+maximum > aggregateHistoryLimit {
			return errors.New("aggregate-history-bound-exceeded")
		}
		raw, err := aggregateCanonicalJSON(reservation)
		if err != nil {
			return err
		}
		if err = writeAggregateExclusiveContext(ctx, repo.aggregateReservationPath(ordinal), append(raw, '\n')); err != nil {
			return err
		}
		if err = repo.aggregateFaultPoint("reservation-installed"); err != nil {
			return err
		}
	}
	directory := repo.aggregateSnapshotDirectory(proposed.BindingSHA256, identity)
	stateBefore := filepath.Join(directory, "state-before.bin")
	if old, err := readFile(stateBefore, maxStateBytes); os.IsNotExist(err) {
		if err = writeAggregateExclusiveContext(ctx, stateBefore, payloads["state-before"]); err != nil {
			return err
		}
	} else if err != nil || !bytes.Equal(old, payloads["state-before"]) {
		return errors.New("aggregate-prior-evidence-drift")
	}
	if err = repo.aggregateFaultPoint("state-before-closed"); err != nil {
		return err
	}
	prepared := filepath.Join(directory, "outcome.prepared")
	if old, err := readFile(prepared, maxArtifactBytes); os.IsNotExist(err) {
		if err = writeAggregateExclusiveContext(ctx, prepared, receipt); err != nil {
			return err
		}
	} else if err != nil || !bytes.Equal(old, receipt) {
		return errors.New("aggregate-prior-evidence-drift")
	}
	proposed.ActiveSnapshotSHA256 = identity
	proposed.AttemptCount = ordinal
	proposed.PreservationState = "RESERVED"
	proposed.Snapshots = append(references, aggregateSnapshotReference{ordinal, identity, maximum})
	saved.AggregateOutcome = proposed
	saved.Lifecycle = "active"
	saved.Terminal = nil
	saved.Coordination = nil
	if err := aggregateWriteAllowed(ctx); err != nil {
		return err
	}
	if err = repo.save(saved); err != nil {
		return err
	}
	if err = repo.aggregateFaultPoint("reservation-state-saved"); err != nil {
		return err
	}
	return repo.finishAggregatePreservation(ctx, saved, reservation)
}

func (repo *repository) finishAggregatePreservation(ctx context.Context, saved *state, reservation aggregateReservation) error {
	if err := aggregateWriteAllowed(ctx); err != nil {
		return err
	}
	a := saved.AggregateOutcome
	directory := repo.aggregateSnapshotDirectory(a.BindingSHA256, a.ActiveSnapshotSHA256)
	live, sourceErr := repo.aggregateReservationSources(reservation)
	if sourceErr != nil {
		return sourceErr
	}
	for _, entry := range reservation.Entries {
		if !entry.Present {
			continue
		}
		name := filepath.Join(directory, entry.Role+".bin")
		raw, err := readFile(name, maxArtifactBytes)
		if os.IsNotExist(err) {
			raw, err = readFile(live[entry.Role], maxArtifactBytes)
			if !aggregateEntryMatches(entry, raw, err) {
				return errors.New("aggregate-prior-evidence-drift")
			}
			if err = writeAggregateExclusiveContext(ctx, name, raw); err != nil {
				return err
			}
		} else if !aggregateEntryMatches(entry, raw, err) {
			return errors.New("aggregate-prior-evidence-drift")
		}
		if err = repo.aggregateFaultPoint("payload-closed:" + entry.Role); err != nil {
			return err
		}
	}
	manifest := aggregateSnapshotManifest{aggregateSnapshotProfile, reservation.BindingSHA256, reservation.SourceStateSHA256, reservation.Entries}
	raw, err := aggregateCanonicalJSON(manifest)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	name := filepath.Join(directory, "manifest.json")
	if old, err := readFile(name, maxStateBytes); os.IsNotExist(err) {
		if err = writeAggregateExclusiveContext(ctx, name, raw); err != nil {
			return err
		}
	} else if err != nil || !bytes.Equal(old, raw) {
		return errors.New("aggregate-prior-evidence-drift")
	}
	if err = repo.aggregateFaultPoint("snapshot-manifest-installed"); err != nil {
		return err
	}
	if sealed, err := repo.aggregatePreservation(a); err != nil || !sealed {
		return errors.New("aggregate-prior-evidence-drift")
	}
	a.PreservationState = "SEALED"
	if err := aggregateWriteAllowed(ctx); err != nil {
		return err
	}
	if err = repo.save(saved); err != nil {
		return err
	}
	return repo.aggregateFaultPoint("snapshot-sealed-acknowledged")
}

type aggregateLegacyCapture struct {
	ordinal        int
	stdout, stderr []byte
}

func strictLegacyAdmission(stdout, stderr []byte, status int) bool {
	if status != 2 || len(stdout) != 0 || len(stderr) > 64<<10 {
		return false
	}
	var refusal struct {
		Code  string `json:"code"`
		Error string `json:"error"`
		OK    bool   `json:"ok"`
	}
	return strictJSON(stderr, &refusal, 64<<10) == nil && refusal.Code == "admitted-path-limit" && !refusal.OK && refusal.Error == "changed_paths exceeds 200 paths"
}

func (repo *repository) captureLegacyAdmission(attempt dogfoodflow.LegacyRecordAttempt) (*aggregateLegacyCapture, error) {
	if !strictLegacyAdmission(attempt.Stdout, attempt.Stderr, attempt.Exit) {
		return nil, nil
	}
	used, err := repo.aggregateHistoryUsage()
	if err != nil {
		return nil, err
	}
	// No later work can race this allowance under the operation lock. A crash
	// leaves inert captured bytes, never an executable future reservation.
	if used+int64(len(attempt.Stdout)+len(attempt.Stderr))+maxArtifactBytes+2*maxStateBytes > aggregateHistoryLimit {
		return nil, errors.New("aggregate-history-bound-exceeded")
	}
	parent := repo.local("aggregate-outcome/legacy")
	if err = checkParents(parent); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil, err
	}
	if len(entries) >= 64 {
		return nil, errors.New("aggregate-history-bound-exceeded")
	}
	for i, entry := range entries {
		if !entry.IsDir() || entry.Name() != fmt.Sprintf("%03d", i+1) {
			return nil, errors.New("aggregate-prior-evidence-drift")
		}
	}
	ordinal := len(entries) + 1
	directory := filepath.Join(parent, fmt.Sprintf("%03d", ordinal))
	if err = os.Mkdir(directory, 0700); err != nil {
		return nil, err
	}
	if err = writeAggregateExclusive(filepath.Join(directory, "stdout"), attempt.Stdout); err != nil {
		return nil, err
	}
	if err = writeAggregateExclusive(filepath.Join(directory, "stderr"), attempt.Stderr); err != nil {
		return nil, err
	}
	return &aggregateLegacyCapture{ordinal, bytes.Clone(attempt.Stdout), bytes.Clone(attempt.Stderr)}, nil
}

func (repo *repository) completeLegacyFailure(saved *state, snap snapshot, capture *aggregateLegacyCapture) error {
	if capture == nil {
		return nil
	}
	report, err := readFile(filepath.Join(repo.auth.Root, ".corvint", "dogfood-report.json"), maxArtifactBytes)
	if err != nil {
		return err
	}
	var header struct{ Profile, Base, Target string }
	if _, err = wire.Parse(report); err != nil {
		return err
	}
	if json.Unmarshal(report, &header) != nil || header.Profile != "corvint-dogfood-change/0" || header.Base != saved.Plan.Base || header.Target != snap.target || saved.ReportSet == nil {
		return errors.New("aggregate-legacy-failure-unverified")
	}
	cem, err := readFile(filepath.Join(repo.auth.Root, ".corvint", "change.cem.json"), maxArtifactBytes)
	if err != nil {
		return err
	}
	value := aggregateLegacyFailure{Base: saved.Plan.Base, Target: snap.target, Tree: snap.tree, Session: saved.Session, Generation: saved.Generation, PlanDigest: saved.PlanDigest, ReportSetDigest: saved.ReportSet.Digest, CEMSHA256: prefixedDigest(cem), ReportSHA256: prefixedDigest(report), ExitStatus: 2, StdoutSHA256: prefixedDigest(capture.stdout), StderrSHA256: prefixedDigest(capture.stderr), CaptureOrdinal: capture.ordinal, Task: "Local completion " + saved.PlanDigest, Verification: []string{displayChecks(saved.Plan)}, Outcome: "passed"}
	raw, err := aggregateCanonicalJSON(value)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	used, err := repo.aggregateHistoryUsage()
	if err != nil {
		return err
	}
	extra := int64(len(report) + 2*len(raw) + len(capture.stdout) + len(capture.stderr))
	if used+extra > aggregateHistoryLimit {
		return errors.New("aggregate-history-bound-exceeded")
	}
	directory := repo.local(fmt.Sprintf("aggregate-outcome/legacy/%03d", capture.ordinal))
	if err = writeAggregateExclusive(filepath.Join(directory, "report.json"), report); err != nil {
		return err
	}
	if err = writeAggregateExclusive(filepath.Join(directory, "failure.json"), raw); err != nil {
		return err
	}
	// Aliases name the latest complete provenance, while all earlier raw captures
	// remain immutable. A crash during alias updates cannot validate the tuple.
	for _, item := range []struct {
		name string
		raw  []byte
	}{{"legacy-failure.stdout", capture.stdout}, {"legacy-failure.stderr", capture.stderr}, {"legacy-failure.json", raw}} {
		if err = writeFile(repo.local("aggregate-outcome/"+item.name), item.raw); err != nil {
			return err
		}
	}
	return nil
}

func (repo *repository) validateLegacyFailure(saved *state, snap snapshot) (aggregateLegacyFailure, []byte, error) {
	var failure aggregateLegacyFailure
	raw, err := readFile(repo.local("aggregate-outcome/legacy-failure.json"), maxStateBytes)
	if err != nil {
		return failure, nil, errors.New("aggregate-legacy-failure-unverified")
	}
	if err = strictJSON(raw, &failure, maxStateBytes); err != nil {
		return failure, nil, errors.New("aggregate-legacy-failure-unverified")
	}
	if failure.Session != saved.Session || failure.Generation != saved.Generation || failure.PlanDigest != saved.PlanDigest || failure.Base != saved.Plan.Base || failure.Target != snap.target || failure.Tree != snap.tree || saved.ReportSet == nil || failure.ReportSetDigest != saved.ReportSet.Digest || failure.CaptureOrdinal < 1 || failure.CaptureOrdinal > 64 || failure.Task != "Local completion "+saved.PlanDigest || !reflect.DeepEqual(failure.Verification, []string{displayChecks(saved.Plan)}) || failure.Outcome != "passed" {
		return failure, nil, errors.New("aggregate-legacy-failure-unverified")
	}
	directory := repo.local(fmt.Sprintf("aggregate-outcome/legacy/%03d", failure.CaptureOrdinal))
	original, err := readFile(filepath.Join(directory, "failure.json"), maxStateBytes)
	if err != nil || !bytes.Equal(original, raw) {
		return failure, nil, errors.New("aggregate-legacy-failure-unverified")
	}
	stdout, outErr := readFile(filepath.Join(directory, "stdout"), maxArtifactBytes)
	stderr, errErr := readFile(filepath.Join(directory, "stderr"), 64<<10)
	report, reportErr := readFile(filepath.Join(directory, "report.json"), maxArtifactBytes)
	cem, cemErr := readFile(filepath.Join(repo.auth.Root, ".corvint", "change.cem.json"), maxArtifactBytes)
	if outErr != nil || errErr != nil || reportErr != nil || cemErr != nil || !strictLegacyAdmission(stdout, stderr, failure.ExitStatus) || prefixedDigest(stdout) != failure.StdoutSHA256 || prefixedDigest(stderr) != failure.StderrSHA256 || prefixedDigest(report) != failure.ReportSHA256 || prefixedDigest(cem) != failure.CEMSHA256 {
		return failure, nil, errors.New("aggregate-legacy-failure-unverified")
	}
	if saved.AggregateOutcome != nil && saved.AggregateOutcome.LegacyFailureSHA256 != prefixedDigest(raw) {
		return failure, nil, errors.New("aggregate-legacy-failure-unverified")
	}
	return failure, report, nil
}

type aggregateCheckStart struct {
	Session    string `json:"session"`
	Generation string `json:"generation"`
	Snapshot   string `json:"snapshotSha256"`
	State      string `json:"stateSha256"`
	Report     string `json:"reportSha256"`
	Base       string `json:"base"`
	Target     string `json:"target"`
	Tree       string `json:"tree"`
	Qualified  bool   `json:"qualified"`
}

// The start descriptor is itself an observation, including when neither stream
// could open. Validate it against the still-pending admission before reserving
// another snapshot; old diagnostics remain in their original bounded directory.
func (repo *repository) aggregatePendingCheckStarted(saved *state, receipt []byte) (bool, error) {
	a := saved.AggregateOutcome
	directory := repo.aggregateSnapshotDirectory(a.BindingSHA256, a.ActiveSnapshotSHA256)
	raw, err := readFile(filepath.Join(directory, "check.started.json"), maxStateBytes)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("aggregate-prior-evidence-drift")
	}
	var start aggregateCheckStart
	if strictJSON(raw, &start, maxStateBytes) != nil {
		return false, errors.New("aggregate-prior-evidence-drift")
	}
	canonical, err := aggregateCanonicalJSON(start)
	value, parseErr := tracerecordrepo.ParseAggregateOutcome(receipt)
	if err != nil || parseErr != nil || !bytes.Equal(raw, canonical) || len(a.Issued) < 2 || start.Session != saved.Session || start.Generation != saved.Generation || start.Snapshot != a.ActiveSnapshotSHA256 || start.Base != saved.Plan.Base || start.Target != value.Target || start.Tree != value.Tree || start.Report != a.Issued[1].SHA256 || start.Qualified || !validAggregateDigest(start.State) || prefixedDigest(receipt) != a.ReceiptSHA256 {
		return false, errors.New("aggregate-prior-evidence-drift")
	}
	// Successful check publication advances state. Its capture has independent
	// validation; only an unpublished check must match the exact admission state.
	if a.Phase == "REPORT_PUBLISHED" && len(a.Issued) == 2 {
		stateRaw, err := readFile(repo.local("state.json"), maxStateBytes)
		if err != nil || prefixedDigest(stateRaw) != start.State {
			return false, errors.New("aggregate-prior-evidence-drift")
		}
	}
	return true, nil
}

// A later failed check or explicit supported report rewrite preserves its new
// raw observations in another immutable snapshot under the same logical binding.
func (repo *repository) preserveAggregateReplacement(ctx context.Context, saved *state, receipt []byte) error {
	if err := aggregateWriteAllowed(ctx); err != nil {
		return err
	}
	a := saved.AggregateOutcome
	if a == nil {
		return errors.New("aggregate-enrollment-required")
	}
	sealed, err := repo.aggregatePreservation(a)
	if err != nil || !sealed {
		return errors.New("aggregate-prior-evidence-drift")
	}
	reservation, err := repo.readAggregateReservation(a.AttemptCount)
	if err != nil {
		return err
	}
	started, err := repo.aggregatePendingCheckStarted(saved, receipt)
	if err != nil {
		return err
	}
	changed := a.Phase == "COMMITTED" || started
	live := repo.aggregateSnapshotPaths(saved)
	for _, entry := range reservation.Entries {
		if entry.Role != "check-stdout" && entry.Role != "check-stderr" && entry.Role != "check-capture" {
			continue
		}
		raw, err := readFile(live[entry.Role], maxArtifactBytes)
		if !aggregateEntryMatches(entry, raw, err) {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	proposed := *a
	proposed.Snapshots = append([]aggregateSnapshotReference(nil), a.Snapshots...)
	if len(a.Issued) < 2 {
		return errors.New("aggregate-prior-evidence-drift")
	}
	outcome, err := readFile(live["prior-outcome"], maxArtifactBytes)
	if err != nil {
		return err
	}
	report, err := readFile(live["prior-report"], maxArtifactBytes)
	if err != nil {
		return err
	}
	proposed.ExpectedOld = aggregateExpectedOld{&aggregateOld{prefixedDigest(outcome), int64(len(outcome)), "aggregate-outcome"}, &aggregateOld{prefixedDigest(report), int64(len(report)), "aggregate-report"}}
	proposed.Issued = []aggregateIssued{{"outcome", prefixedDigest(outcome), int64(len(outcome))}, {"report", prefixedDigest(report), int64(len(report))}}
	proposed.Phase = "REPORT_PUBLISHED"
	return repo.reserveAggregateSnapshot(ctx, saved, &proposed, receipt)
}

func (repo *repository) aggregateStagePrefix(ordinal int) string {
	return fmt.Sprintf(".aggregate-%s-%s-%03d-", repo.session, repo.generation, ordinal)
}

func (repo *repository) validateAggregateLedger(a *aggregateState) error {
	reservations, err := repo.aggregateReservations()
	if err != nil {
		return err
	}
	if len(reservations) < a.AttemptCount || len(reservations) > a.AttemptCount+1 {
		return errors.New("aggregate-prior-evidence-drift")
	}
	seen := map[string]bool{}
	for i, reference := range a.Snapshots {
		reservation := reservations[i]
		if reference.Ordinal != reservation.Ordinal || reference.SnapshotSHA256 != reservation.SnapshotSHA256 || reference.ReservedBytes != reservation.MaximumBytes || reservation.BindingSHA256 != a.BindingSHA256 || seen[reference.SnapshotSHA256] {
			return errors.New("aggregate-prior-evidence-drift")
		}
		seen[reference.SnapshotSHA256] = true
		partial := *a
		partial.AttemptCount = i + 1
		partial.Snapshots = a.Snapshots[:i+1]
		partial.ActiveSnapshotSHA256 = reference.SnapshotSHA256
		if i < len(a.Snapshots)-1 {
			partial.ExpectedOld = reservation.ExpectedOld
			partial.PreservationState = "SEALED"
		}
		if _, err := repo.aggregatePreservation(&partial); err != nil {
			return err
		}
	}
	if len(reservations) > a.AttemptCount {
		orphan := reservations[len(reservations)-1]
		raw, err := readFile(repo.local("state.json"), maxStateBytes)
		if err != nil || prefixedDigest(raw) != orphan.SourceStateSHA256 {
			return errors.New("aggregate-prior-evidence-drift")
		}
		if orphan.BindingSHA256 != a.BindingSHA256 {
			return errors.New("aggregate-prior-evidence-drift")
		}
		var original state
		if strictJSON(raw, &original, maxStateBytes) != nil {
			return errors.New("aggregate-prior-evidence-drift")
		}
		live := repo.aggregateSnapshotPaths(&original)
		for _, entry := range orphan.Entries {
			raw, err := readFile(live[entry.Role], maxArtifactBytes)
			if !aggregateEntryMatches(entry, raw, err) {
				return errors.New("aggregate-prior-evidence-drift")
			}
		}
	}
	_, err = repo.aggregateHistoryUsage()
	return err
}

func aggregateOldMatches(value *aggregateOld, raw []byte, err error) bool {
	if value == nil {
		return os.IsNotExist(err)
	}
	return err == nil && int64(len(raw)) == value.Bytes && prefixedDigest(raw) == value.SHA256
}
func aggregateIssuedMatches(value aggregateIssued, raw []byte, err error) bool {
	return err == nil && int64(len(raw)) == value.Bytes && prefixedDigest(raw) == value.SHA256
}

// aggregatePublishedPrefix accepts only old captured identities or an issued
// publication prefix. A phase acknowledgment is a lower bound, never a license
// to accept an unrelated file or a missing formerly-present file.
func (repo *repository) aggregatePublishedPrefix(saved *state) (int, error) {
	a := saved.AggregateOutcome
	if a == nil {
		return 0, errors.New("aggregate-enrollment-required")
	}
	live := repo.aggregateSourcePaths()
	outcome, outErr := readFile(live["prior-outcome"], maxArtifactBytes)
	report, reportErr := readFile(live["prior-report"], maxArtifactBytes)
	outcomeNew := len(a.Issued) > 0 && aggregateIssuedMatches(a.Issued[0], outcome, outErr)
	if !outcomeNew && !aggregateOldMatches(a.ExpectedOld.Outcome, outcome, outErr) {
		return 0, errors.New("aggregate-prior-evidence-drift")
	}
	rank := 0
	var receiptValue tracerecordrepo.AggregateOutcome
	if outcomeNew {
		value, err := tracerecordrepo.ParseAggregateOutcome(outcome)
		if err != nil {
			return 0, err
		}
		binding, err := tracerecordrepo.AggregateBinding(value)
		if err != nil || binding != a.BindingSHA256 {
			return 0, errors.New("aggregate-binding-drift")
		}
		receiptValue = value
		rank = 1
	}
	reportRank := 0
	if len(a.Issued) > 1 && aggregateIssuedMatches(a.Issued[1], report, reportErr) {
		reportRank = 2
	}
	if len(a.Issued) > 2 && aggregateIssuedMatches(a.Issued[2], report, reportErr) {
		reportRank = 3
	}
	if reportRank == 0 && !aggregateOldMatches(a.ExpectedOld.Report, report, reportErr) {
		return 0, errors.New("aggregate-prior-evidence-drift")
	}
	if reportRank > 0 {
		if !outcomeNew {
			return 0, errors.New("aggregate-prior-evidence-drift")
		}
		value, err := dogfoodflow.ParseAggregateReport(report)
		if err != nil {
			return 0, err
		}
		if value.Enrollment != (dogfoodflow.AggregateEnrollment{Session: saved.Session, Generation: saved.Generation, PlanDigest: saved.PlanDigest, BindingSHA256: a.BindingSHA256}) || value.LocalOutcomeEvidenceSHA256 != a.ReceiptSHA256 || value.Base != saved.Plan.Base || saved.ReportSet == nil || value.Target != saved.ReportSet.Target {
			return 0, errors.New("aggregate-binding-drift")
		}
		rank = reportRank
	}
	minimum := map[string]int{"PREPARED": 0, "OUTCOME_PUBLISHED": 1, "REPORT_PUBLISHED": 2, "COMMITTED": 3}[a.Phase]
	if rank < minimum || rank < len(a.Issued)-1 {
		return 0, errors.New("aggregate-prior-evidence-drift")
	}
	if len(a.Issued) == 3 {
		if err := repo.validateAggregateCheckCapture(saved, snapshot{target: receiptValue.Target, tree: receiptValue.Tree}); err != nil {
			return 0, err
		}
	}
	return rank, nil
}

func (repo *repository) publishAggregateRole(ctx context.Context, saved *state, role string, raw []byte) error {
	if err := aggregateWriteAllowed(ctx); err != nil {
		return err
	}
	a := saved.AggregateOutcome
	sealed, err := repo.aggregatePreservation(a)
	if err != nil || !sealed {
		return errors.New("aggregate-prior-evidence-drift")
	}
	rank, err := repo.aggregatePublishedPrefix(saved)
	if err != nil {
		return err
	}
	index := -1
	for i, name := range []string{"outcome", "report", "check-report"} {
		if name == role {
			index = i
		}
	}
	if index < 0 || rank < index || len(raw) < 1 || len(raw) > maxArtifactBytes {
		return errors.New("aggregate-publication-failed")
	}
	identity := aggregateIssued{role, prefixedDigest(raw), int64(len(raw))}
	if index == 0 && identity.SHA256 != a.ReceiptSHA256 {
		return errors.New("aggregate-binding-drift")
	}
	if len(a.Issued) < index {
		return errors.New("aggregate-prior-evidence-drift")
	}
	if len(a.Issued) > index && a.Issued[index] != identity {
		return errors.New("aggregate-prior-evidence-drift")
	}
	directory := repo.aggregateSnapshotDirectory(a.BindingSHA256, a.ActiveSnapshotSHA256)
	prepared := filepath.Join(directory, role+".prepared")
	if prior, err := readFile(prepared, maxArtifactBytes); os.IsNotExist(err) {
		if err = writeAggregateExclusiveContext(ctx, prepared, raw); err != nil {
			return err
		}
	} else if err != nil || !bytes.Equal(prior, raw) {
		return errors.New("aggregate-prior-evidence-drift")
	}
	if err = repo.aggregateFaultPoint("prepared-closed:" + role); err != nil {
		return err
	}
	if rank < index+1 {
		if _, err = repo.aggregateHistoryUsage(); err != nil {
			return err
		}
		destination := repo.aggregateSourcePaths()["prior-report"]
		if role == "outcome" {
			destination = repo.aggregateSourcePaths()["prior-outcome"]
		}
		if err = checkParents(destination); err != nil {
			return err
		}
		if err := aggregateWriteAllowed(ctx); err != nil {
			return err
		}
		if err = os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return err
		}
		if err := aggregateWriteAllowed(ctx); err != nil {
			return err
		}
		stage, err := os.CreateTemp(filepath.Dir(destination), repo.aggregateStagePrefix(a.AttemptCount)+role+"-")
		if err != nil {
			return err
		}
		// No child is live during publication. On an ordinary error, retire this
		// unreferenced temporary copy; the immutable prepared bytes and any issued
		// identity remain for retry. A process kill intentionally skips cleanup.
		defer os.Remove(stage.Name())
		if _, err = stage.Write(raw); err != nil {
			_ = stage.Close()
			return err
		}
		if err = stage.Close(); err != nil {
			return err
		}
		closed, err := readFile(stage.Name(), maxArtifactBytes)
		if err != nil || !bytes.Equal(closed, raw) {
			return errors.New("aggregate-publication-failed")
		}
		if err = repo.aggregateFaultPoint("stage-closed:" + role); err != nil {
			return err
		}
		if len(a.Issued) == index {
			a.Issued = append(a.Issued, identity)
			if err := aggregateWriteAllowed(ctx); err != nil {
				return err
			}
			if err = repo.save(saved); err != nil {
				return err
			}
		}
		if err = repo.aggregateFaultPoint("issued-saved:" + role); err != nil {
			return err
		}
		if _, err = repo.aggregatePublishedPrefix(saved); err != nil {
			return err
		}
		if err := aggregateWriteAllowed(ctx); err != nil {
			return err
		}
		if err = os.Rename(stage.Name(), destination); err != nil {
			return err
		}
		if err = repo.aggregateFaultPoint("artifact-renamed:" + role); err != nil {
			return err
		}
	}
	phases := []string{"OUTCOME_PUBLISHED", "REPORT_PUBLISHED", "REPORT_PUBLISHED"}
	if role != "outcome" || a.Phase == "PREPARED" {
		a.Phase = phases[index]
	}
	if err := aggregateWriteAllowed(ctx); err != nil {
		return err
	}
	if err = repo.save(saved); err != nil {
		return err
	}
	return repo.aggregateFaultPoint("phase-acknowledged:" + role)
}

func (repo *repository) aggregateSnapshotPaths(saved *state) map[string]string {
	paths := repo.aggregateSourcePaths()
	if saved.AggregateOutcome == nil {
		return paths
	}
	directory := repo.aggregateSnapshotDirectory(saved.AggregateOutcome.BindingSHA256, saved.AggregateOutcome.ActiveSnapshotSHA256)
	candidates := map[string]string{"check-stdout": filepath.Join(directory, "check.stdout"), "check-stderr": filepath.Join(directory, "check.stderr"), "check-capture": filepath.Join(directory, "check.capture.json")}
	observed := false
	for _, name := range candidates {
		if _, err := os.Lstat(name); !os.IsNotExist(err) {
			observed = true
		}
	}
	if observed {
		for role, name := range candidates {
			paths[role] = name
		}
	}
	return paths
}

func (repo *repository) aggregateReservationSources(reservation aggregateReservation) (map[string]string, error) {
	directory := repo.aggregateSnapshotDirectory(reservation.BindingSHA256, reservation.SnapshotSHA256)
	raw, err := readFile(filepath.Join(directory, "state-before.bin"), maxStateBytes)
	if err != nil || prefixedDigest(raw) != reservation.SourceStateSHA256 {
		return nil, errors.New("aggregate-prior-evidence-drift")
	}
	var original state
	if strictJSON(raw, &original, maxStateBytes) != nil || original.Session != repo.session || original.Generation != repo.generation {
		return nil, errors.New("aggregate-prior-evidence-drift")
	}
	if original.AggregateOutcome != nil {
		a := original.AggregateOutcome
		if a.AttemptCount != reservation.Ordinal-1 || a.BindingSHA256 != reservation.BindingSHA256 || !validAggregateDigest(a.ActiveSnapshotSHA256) {
			return nil, errors.New("aggregate-prior-evidence-drift")
		}
	} else if reservation.Ordinal != 1 {
		return nil, errors.New("aggregate-prior-evidence-drift")
	}
	return repo.aggregateSnapshotPaths(&original), nil
}

// aggregateCaptureWriter writes the observation to its admitted private file
// from the first byte. Neither cancellation nor a later failed revalidation can
// erase its prefix. The fixed file is never reopened for overwrite.
type aggregateCaptureWriter struct {
	mu     sync.Mutex
	file   *os.File
	bytes  int
	err    error
	closed bool
}

func (w *aggregateCaptureWriter) Write(raw []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, os.ErrClosed
	}
	if w.err != nil {
		return 0, w.err
	}
	n := min(len(raw), maxLogBytes-w.bytes)
	written, err := w.file.Write(raw[:n])
	w.bytes += written
	if err == nil && written != len(raw) {
		err = io.ErrShortWrite
	}
	w.err = err
	return written, err
}
func (w *aggregateCaptureWriter) close() ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		w.err = errors.Join(w.err, w.file.Close())
		w.closed = true
	}
	raw, err := readFile(w.file.Name(), maxLogBytes)
	if err == nil && len(raw) != w.bytes {
		err = errors.New("aggregate-prior-evidence-drift")
	}
	return raw, errors.Join(w.err, err)
}

// writeAggregateObservation only closes an already admitted diagnostic. It does
// not revalidate semantic success or refresh a cancelled operation's work budget.
func writeAggregateObservation(name string, raw []byte) error {
	if len(raw) > maxStateBytes {
		return errors.New("aggregate-history-bound-exceeded")
	}
	if err := checkParents(name); err != nil {
		return err
	}
	file, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	n, writeErr := file.Write(raw)
	if writeErr == nil && n != len(raw) {
		writeErr = io.ErrShortWrite
	}
	return errors.Join(writeErr, file.Close())
}

func (repo *repository) beginAggregateCheckCapture(ctx context.Context, saved *state, snap snapshot, report []byte) (*dogfoodflow.AggregateCheckStage, error) {
	if err := aggregateWriteAllowed(ctx); err != nil {
		return nil, err
	}
	a := saved.AggregateOutcome
	if a == nil || saved.ReportSet == nil {
		return nil, errors.New("aggregate-enrollment-required")
	}
	if sealed, err := repo.aggregatePreservation(a); err != nil || !sealed {
		return nil, errors.New("aggregate-prior-evidence-drift")
	}
	if _, err := repo.aggregateHistoryUsage(); err != nil {
		return nil, err
	}
	directory := repo.aggregateSnapshotDirectory(a.BindingSHA256, a.ActiveSnapshotSHA256)
	stateRaw, err := readFile(repo.local("state.json"), maxStateBytes)
	if err != nil {
		return nil, err
	}
	identity := aggregateCheckStart{saved.Session, saved.Generation, a.ActiveSnapshotSHA256, prefixedDigest(stateRaw), prefixedDigest(report), saved.Plan.Base, snap.target, snap.tree, false}
	start, err := aggregateCanonicalJSON(identity)
	if err != nil {
		return nil, err
	}
	// The existing reservation includes four stream maxima and six metadata
	// maxima. Two live streams plus their checked capture never exceed that.
	if err = writeAggregateExclusiveContext(ctx, filepath.Join(directory, "check.started.json"), start); err != nil {
		return nil, err
	}
	if err = repo.aggregateFaultPoint("check-started-closed"); err != nil {
		return nil, err
	}
	openStream := func(name string) (*aggregateCaptureWriter, error) {
		if err := aggregateWriteAllowed(ctx); err != nil {
			return nil, err
		}
		name = filepath.Join(directory, name)
		if err := checkParents(name); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		return &aggregateCaptureWriter{file: file}, nil
	}
	out, err := openStream("check.stdout")
	if err != nil {
		return nil, err
	}
	if err = repo.aggregateFaultPoint("check-stdout-opened"); err != nil {
		_, _ = out.close()
		return nil, err
	}
	errOut, err := openStream("check.stderr")
	if err != nil {
		_, _ = out.close()
		return nil, err
	}
	var once sync.Once
	var capture dogfoodflow.AggregateCheckCapture
	var finishErr error
	return &dogfoodflow.AggregateCheckStage{Stdout: out, Stderr: errOut, RetainFailure: func(failure error) error {
		reason := failure.Error()
		if len(reason) > 256 {
			reason = reason[:256]
		}
		raw, err := aggregateCanonicalJSON(struct {
			Start     string `json:"startSha256"`
			Failure   string `json:"failure"`
			Qualified bool   `json:"qualified"`
		}{prefixedDigest(start), reason, false})
		if err != nil {
			return err
		}
		return writeAggregateObservation(filepath.Join(directory, "check.publication-failure.json"), raw)
	}, Finish: func(code int, runErr error) (dogfoodflow.AggregateCheckCapture, error) {
		once.Do(func() {
			stdout, outErr := out.close()
			stderr, stderrErr := errOut.close()
			held := dogfoodoperation.Check(ctx) != nil
			failed := runErr != nil || ctx.Err() != nil || held || outErr != nil || stderrErr != nil || code != 0
			if runErr != nil || ctx.Err() != nil {
				code = -1
			}
			capture = dogfoodflow.AggregateCheckCapture{OriginalReport: bytes.Clone(report), Exit: code, Stdout: stdout, Stderr: stderr, Failed: failed}
			observed := struct {
				Start       string `json:"startSha256"`
				Exit        string `json:"exit"`
				Failed      bool   `json:"failed"`
				Held        bool   `json:"hold"`
				Closed      bool   `json:"streamsClosed"`
				Qualified   bool   `json:"qualified"`
				Stdout      string `json:"stdoutSha256"`
				Stderr      string `json:"stderrSha256"`
				StdoutBytes int    `json:"stdoutBytes"`
				StderrBytes int    `json:"stderrBytes"`
			}{prefixedDigest(start), strconv.Itoa(code), failed, held, outErr == nil && stderrErr == nil, false, prefixedDigest(stdout), prefixedDigest(stderr), len(stdout), len(stderr)}
			raw, encodeErr := aggregateCanonicalJSON(observed)
			if encodeErr == nil {
				encodeErr = writeAggregateObservation(filepath.Join(directory, "check.observed.json"), raw)
			}
			finishErr = errors.Join(outErr, stderrErr, encodeErr)
		})
		return capture, finishErr
	}}, nil
}

func (repo *repository) saveAggregateCheckCapture(ctx context.Context, saved *state, snap snapshot, capture dogfoodflow.AggregateCheckCapture) error {
	if err := aggregateWriteAllowed(ctx); err != nil {
		return err
	}
	a := saved.AggregateOutcome
	if a == nil || saved.ReportSet == nil {
		return errors.New("aggregate-enrollment-required")
	}
	if len(capture.Stdout) > maxLogBytes || len(capture.Stderr) > maxLogBytes || capture.Exit < -1 || capture.Exit > 255 {
		return errors.New("aggregate-publication-failed")
	}
	if sealed, err := repo.aggregatePreservation(a); err != nil || !sealed {
		return errors.New("aggregate-prior-evidence-drift")
	}
	if _, err := repo.aggregateHistoryUsage(); err != nil {
		return err
	}
	value := aggregateCheckCapture{Session: saved.Session, Generation: saved.Generation, PlanDigest: saved.PlanDigest, BindingSHA256: a.BindingSHA256, SnapshotSHA256: a.ActiveSnapshotSHA256, ReportSetDigest: saved.ReportSet.Digest, Base: saved.Plan.Base, Target: snap.target, Tree: snap.tree, ReportBeforeSHA256: prefixedDigest(capture.OriginalReport), ExitStatus: capture.Exit, Failed: capture.Failed, StdoutSHA256: prefixedDigest(capture.Stdout), StderrSHA256: prefixedDigest(capture.Stderr), StdoutBytes: int64(len(capture.Stdout)), StderrBytes: int64(len(capture.Stderr))}
	if len(capture.ProposedReport) > 0 {
		identity := prefixedDigest(capture.ProposedReport)
		value.ReportAfterSHA256 = &identity
	}
	raw, err := aggregateCanonicalJSON(value)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	directory := repo.aggregateSnapshotDirectory(a.BindingSHA256, a.ActiveSnapshotSHA256)
	items := []struct {
		name string
		raw  []byte
	}{{"check.stdout", capture.Stdout}, {"check.stderr", capture.Stderr}}
	if len(capture.ProposedReport) > 0 {
		items = append(items, struct {
			name string
			raw  []byte
		}{"check-report.prepared", capture.ProposedReport})
	}
	items = append(items, struct {
		name string
		raw  []byte
	}{"check.capture.json", raw})
	for _, item := range items {
		name := filepath.Join(directory, item.name)
		prior, readErr := readFile(name, maxArtifactBytes)
		if os.IsNotExist(readErr) {
			if err = writeAggregateExclusiveContext(ctx, name, item.raw); err != nil {
				return err
			}
		} else if readErr != nil || !bytes.Equal(prior, item.raw) {
			return errors.New("aggregate-prior-evidence-drift")
		}
		if err = repo.aggregateFaultPoint("check-capture-closed:" + item.name); err != nil {
			return err
		}
	}
	return nil
}

func (repo *repository) validateAggregateCheckCapture(saved *state, snap snapshot) error {
	a := saved.AggregateOutcome
	if a == nil || len(a.Issued) != 3 || saved.ReportSet == nil {
		return errors.New("final-check-required")
	}
	directory := repo.aggregateSnapshotDirectory(a.BindingSHA256, a.ActiveSnapshotSHA256)
	raw, err := readFile(filepath.Join(directory, "check.capture.json"), maxStateBytes)
	if err != nil {
		return errors.New("final-check-required")
	}
	var value aggregateCheckCapture
	if strictJSON(raw, &value, maxStateBytes) != nil {
		return errors.New("aggregate-prior-evidence-drift")
	}
	if value.Session != saved.Session || value.Generation != saved.Generation || value.PlanDigest != saved.PlanDigest || value.BindingSHA256 != a.BindingSHA256 || value.SnapshotSHA256 != a.ActiveSnapshotSHA256 || value.ReportSetDigest != saved.ReportSet.Digest || value.Base != saved.Plan.Base || value.Target != snap.target || value.Tree != snap.tree || value.ReportBeforeSHA256 != a.Issued[1].SHA256 || value.ReportAfterSHA256 == nil || *value.ReportAfterSHA256 != a.Issued[2].SHA256 || value.ExitStatus != 0 || value.Failed {
		return errors.New("final-check-required")
	}
	stdout, outErr := readFile(filepath.Join(directory, "check.stdout"), maxLogBytes)
	stderr, errErr := readFile(filepath.Join(directory, "check.stderr"), maxLogBytes)
	if outErr != nil || errErr != nil || int64(len(stdout)) != value.StdoutBytes || int64(len(stderr)) != value.StderrBytes || prefixedDigest(stdout) != value.StdoutSHA256 || prefixedDigest(stderr) != value.StderrSHA256 {
		return errors.New("aggregate-prior-evidence-drift")
	}
	if !bytes.HasSuffix(stdout, []byte("dogfood-check: PASS\n")) {
		return errors.New("final-check-required")
	}
	report, err := readFile(filepath.Join(directory, "check-report.prepared"), maxArtifactBytes)
	if err != nil || prefixedDigest(report) != a.Issued[2].SHA256 {
		return errors.New("aggregate-prior-evidence-drift")
	}
	parsed, err := dogfoodflow.ParseAggregateReport(report)
	if err != nil || parsed.CompletionState != "complete" || parsed.DogfoodCheck == nil || !parsed.DogfoodCheck.OutputsAgree {
		return errors.New("final-check-required")
	}
	return nil
}
