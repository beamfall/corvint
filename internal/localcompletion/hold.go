package localcompletion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// holdProfile names a coordinator preservation hold and its release record
// (LCP-V0-880).
const holdProfile = "corvint-local-completion-hold/0"

// maxHoldText bounds the hold reason and the release authorization.
const maxHoldText = 512

// hold is the coordinator's instruction to preserve one ACTIVE enrollment:
// while it exists, dogfood cancel refuses (LCP-V0-880). Release keeps it,
// with the authorization, as a release record (LCP-V0-881).
type hold struct {
	Profile       string `json:"profile"`
	Session       string `json:"session"`
	PlanDigest    string `json:"planDigest"`
	Base          string `json:"base"`
	Reason        string `json:"reason"`
	Authorization string `json:"authorization,omitempty"`
}

// holdPath and the release records live beside, not inside, the session
// directory, whose bounded entries are its generations.
func (repo *repository) holdPath() string {
	return filepath.Join(repo.directory, "holds", repo.session+".json")
}

func (repo *repository) releasesPath() string {
	return filepath.Join(repo.directory, "holds", repo.session+".releases")
}

// HoldError is the enrollment-preservation-hold refusal of a cancel; Detail
// names the hold's reason and both supported routes (LCP-V0-880).
type HoldError struct {
	Reason, Session string
}

func (e *HoldError) Error() string { return "enrollment-preservation-hold" }

// Detail names the coordinator's reason, the distinct-checkout route and the
// release that only the coordinator authorizes.
func (e *HoldError) Detail() string {
	return fmt.Sprintf("the coordinator's preservation hold keeps this ACTIVE enrollment (reason: %q); to work on a new base, leave it ACTIVE and begin a new enrollment in a distinct checkout (git worktree add or a fresh clone); only when the coordinator authorizes the cancel, release the hold with corvint dogfood hold --session-key %s --release-authorization TEXT, then cancel", e.Reason, e.Session)
}

// readHold returns the active hold, nil when there is none.
func (repo *repository) readHold() (*hold, error) {
	raw, err := readFile(repo.holdPath(), 4096)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var value hold
	if json.Unmarshal(raw, &value) != nil || value.Profile != holdProfile || value.Session != repo.session {
		return nil, errors.New("invalid-preservation-hold")
	}
	return &value, nil
}

// holdText admits a bounded, single-line, secret-screened reason.
func holdText(value string) error {
	if value == "" || len(value) > maxHoldText {
		return errors.New("invalid-local-completion-option-value")
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return errors.New("invalid-local-completion-option-value")
		}
	}
	if secretscreen.MatchString(value) {
		return errors.New("hold-text-secret-screened")
	}
	return nil
}

// Hold records the coordinator's preservation hold on the owner's ACTIVE
// enrollment (LCP-V0-880), or, with authorization, releases it and keeps the
// released hold and its authorization as a record (LCP-V0-881). Neither
// changes the enrollment state.
func Hold(ctx context.Context, root, key, reason, authorization string) (Evaluation, error) {
	if (reason == "") == (authorization == "") {
		return Evaluation{}, errors.New("invalid-local-completion-option")
	}
	repo, err := open(root, key)
	if err != nil {
		return Evaluation{}, err
	}
	unlock, err := repo.lock()
	if err != nil {
		return Evaluation{}, err
	}
	defer unlock()
	if err = repo.requireOwner(); err != nil {
		return Evaluation{}, err
	}
	saved, err := repo.load()
	if err != nil {
		return Evaluation{}, err
	}
	current, err := repo.readHold()
	if err != nil {
		return Evaluation{}, err
	}
	if reason != "" {
		if err = holdText(reason); err != nil {
			return Evaluation{}, err
		}
		if saved.Lifecycle != "active" {
			return Evaluation{}, errors.New("preservation-hold-requires-active")
		}
		if current != nil {
			return Evaluation{}, errors.New("preservation-hold-exists")
		}
		raw, _ := json.Marshal(hold{Profile: holdProfile, Session: key, PlanDigest: saved.PlanDigest, Base: saved.Plan.Base, Reason: reason})
		if err = writeFile(repo.holdPath(), raw); err != nil {
			return Evaluation{}, err
		}
		return repo.evaluate(ctx, saved)
	}
	if err = holdText(authorization); err != nil {
		return Evaluation{}, err
	}
	if current == nil {
		return Evaluation{}, errors.New("preservation-hold-absent")
	}
	current.Authorization = authorization
	raw, _ := json.Marshal(current)
	if err = os.MkdirAll(repo.releasesPath(), 0o700); err != nil {
		return Evaluation{}, err
	}
	entries, err := os.ReadDir(repo.releasesPath())
	if err != nil {
		return Evaluation{}, err
	}
	if err = writeFile(filepath.Join(repo.releasesPath(), fmt.Sprintf("release-%03d.json", len(entries)+1)), raw); err != nil {
		return Evaluation{}, err
	}
	if err = os.Remove(repo.holdPath()); err != nil {
		return Evaluation{}, err
	}
	return repo.evaluate(ctx, saved)
}
