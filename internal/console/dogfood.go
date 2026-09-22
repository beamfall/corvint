package console

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// DogfoodProfile is the report profile `make dogfood-change` writes.
const DogfoodProfile = "corvint-dogfood-change/0"

// dogfoodReportPath is where that report lands, relative to the repository.
const dogfoodReportPath = ".corvint/dogfood-report.json"

// maxDogfoodBytes bounds one report.
const maxDogfoodBytes = 8 << 20

// DogfoodStep is one step of the loop and the reason it did or did not
// produce. A NOT_PRODUCED reason is the point of the pane: the loop's own
// account of what it could not do is what an operator needs to see, so it is
// never summarised away (docs/DOGFOOD.md).
type DogfoodStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// Produced reports whether this step produced its artifact.
func (s DogfoodStep) Produced() bool { return s.Status == "PRODUCED" }

// Dogfood is the rendered dogfood pane.
type Dogfood struct {
	Profile   string        `json:"profile"`
	Base      string        `json:"base"`
	Target    string        `json:"target"`
	Complete  bool          `json:"complete"`
	Steps     []DogfoodStep `json:"steps"`
	OCMStatus any           `json:"ocmStatus"`
	Outcome   *string       `json:"localOutcomeEvidenceSha256"`
	Anchor    struct {
		State     string  `json:"state"`
		MergeBase *string `json:"mergeBase"`
	} `json:"anchor"`
	Check any `json:"dogfoodCheck"`

	// Path, ModifiedAt and Source describe where the report was read from.
	// Absent is what distinguishes "no loop has run here" from "the loop ran
	// and produced nothing", which LAC-V0-008 forbids collapsing together.
	Path       string    `json:"-"`
	ModifiedAt time.Time `json:"-"`
	Absent     bool      `json:"-"`
	Source     Source    `json:"-"`
	Err        string    `json:"-"`
}

// NotProduced counts the steps that did not produce.
func (d *Dogfood) NotProduced() int {
	count := 0
	for _, step := range d.Steps {
		if !step.Produced() {
			count++
		}
	}
	return count
}

// ReadDogfood reads the last dogfood report of one repository. The report is
// untracked local derived state, not committed evidence: it carries no
// content digest and no owning verifier, so the console states none of the six
// axes for it rather than supplying one (LAC-V0-007).
func ReadDogfood(root string) *Dogfood {
	path := filepath.Join(root, dogfoodReportPath)
	report := &Dogfood{Path: dogfoodReportPath, Source: Source{
		Argv:     []string{"read", path},
		Worktree: root, ObservedAt: time.Now().UTC(), Axes: UnstatedAxes(),
	}}

	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		report.Absent = true
		return report
	}
	if err != nil {
		report.Err = err.Error()
		return report
	}
	report.ModifiedAt = info.ModTime().UTC()

	// readBounded refuses a symlink (e.g. a committed link to /dev/zero) and
	// caps the read itself, so the report can never exhaust the console's
	// memory the way a bare os.Stat size check plus os.ReadFile did.
	raw, err := readBounded(root, filepath.FromSlash(dogfoodReportPath), maxDogfoodBytes)
	if err != nil {
		report.Err = err.Error()
		return report
	}
	if err := json.Unmarshal(raw, report); err != nil {
		report.Err = "the dogfood report is not decodable JSON: " + err.Error()
		return report
	}
	if report.Profile != DogfoodProfile {
		report.Err = "the report carries profile " + report.Profile + ", want " + DogfoodProfile
		return report
	}
	return report
}
