package console

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// EnvelopeProfile is the only result profile this console consumes.
const EnvelopeProfile = "taskman-command-result/0"

// maxEnvelopeBytes bounds one `atm` reply. A reply larger than this is
// reported as an unreadable source rather than buffered without limit.
const maxEnvelopeBytes = 32 << 20

// Envelope is one `taskman-command-result/0` reply, kept as the tool wrote
// it. Items stay untyped: the console renders the fields it understands and
// must not silently drop the ones it does not.
type Envelope struct {
	Profile   string           `json:"profile"`
	Command   []string         `json:"command"`
	Outcome   string           `json:"outcome"`
	Codes     []string         `json:"codes"`
	Warnings  []string         `json:"warnings"`
	Items     []map[string]any `json:"items"`
	Untrusted []string         `json:"untrusted"`
	Page      map[string]any   `json:"page"`
}

// Refused reports whether the tool refused. A refusal is rendered as itself
// (LAC-V0-009); it is never an empty board.
func (e *Envelope) Refused() bool { return e != nil && e.Outcome != "OK" }

// Source is the attribution of one rendered value: the exact invocation, the
// tool that answered, and the repository revision it answered about
// (LAC-V0-006). It travels with the value so the attribution is reachable
// without a second query.
type Source struct {
	Argv        []string
	Tool        string
	Revision    string
	Worktree    string
	ObservedAt  time.Time
	Axes        Axes
	Err         string
	Outcome     string
	Codes       []string
	Warnings    []string
	UntrustedIn []string
}

// Line renders the invocation as a copyable command.
func (s Source) Line() string { return strings.Join(s.Argv, " ") }

// Taskman invokes `atm` in one repository. It is the only way this console
// reaches the ticket store: nothing here imports corvint-taskman, reads its
// state directory, or writes anything it owns (decision 0081).
type Taskman struct {
	Binary   string
	Repo     string
	Version  string
	Revision string
	Timeout  time.Duration
}

// Run executes one `atm` verb and decodes its envelope. A non-zero exit is
// not an error by itself: `atm` reports a refusal in the envelope with its
// own codes, and that refusal is what the operator must see.
func (t *Taskman) Run(ctx context.Context, args ...string) (*Envelope, Source) {
	argv := append([]string{t.Binary}, args...)
	source := Source{
		Argv: argv, Tool: t.Version, Revision: t.Revision,
		Worktree: t.Repo, ObservedAt: time.Now().UTC(), Axes: UnstatedAxes(),
	}
	raw, stderr, runErr := runTool(ctx, t.Repo, nil, t.Timeout, maxEnvelopeBytes, argv...)
	if boundaryFailure(runErr) {
		source.Err = runErr.Error()
		return nil, source
	}
	// A refusal may arrive on stderr only with a non-zero exit; a zero exit
	// with empty stdout produced no reply (LAC-V0-022).
	if len(raw) == 0 && runErr != nil {
		raw = stderr
	}
	if len(raw) > maxEnvelopeBytes {
		source.Err = fmt.Sprintf("%s replied with more than %d bytes", t.Binary, maxEnvelopeBytes)
		return nil, source
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		source.Err = describeExec(t.Binary, runErr, string(stderr))
		return nil, source
	}
	var envelope Envelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		source.Err = fmt.Sprintf("%s did not reply with a %s envelope: %v", t.Binary, EnvelopeProfile, err)
		return nil, source
	}
	if envelope.Profile != EnvelopeProfile {
		source.Err = fmt.Sprintf("%s replied with profile %q, want %q", t.Binary, envelope.Profile, EnvelopeProfile)
		return nil, source
	}
	if runErr != nil && !envelope.Refused() {
		source.Err = describeExec(t.Binary, runErr, "")
		return nil, source
	}
	source.Outcome = envelope.Outcome
	source.Codes = envelope.Codes
	source.Warnings = envelope.Warnings
	source.UntrustedIn = envelope.Untrusted
	return &envelope, source
}

func describeExec(binary string, err error, _ string) string {
	var exit *ExitError
	if errors.As(err, &exit) {
		return fmt.Sprintf("%s %v; a non-zero exit cannot carry a successful reply", binary, err)
	}
	if err != nil {
		return fmt.Sprintf("could not run %s: %v", binary, err)
	}
	return fmt.Sprintf("%s produced no output", binary)
}

// Capabilities is what the tool says it can do, read from `atm help`: the
// verbs it implements, the verbs it does not, and the enumerations a board
// must use instead of carrying its own (LAC-V0-013, LAC-V0-016).
type Capabilities struct {
	Implemented []string
	Omitted     []string
	Statuses    []string
	Eligibility []string
	Note        string
	Source      Source
	Err         string
}

// Implements reports whether the tool named this verb as implemented.
func (c *Capabilities) Implements(verb string) bool {
	for _, v := range c.Implemented {
		if v == verb {
			return true
		}
	}
	return false
}

// Reason is the tool's own explanation for a verb it did not implement. An
// unimplemented verb renders as a disabled control carrying this string; the
// console never invents one and never hides the control (LAC-V0-013).
func (c *Capabilities) Reason(verb string) string {
	for _, v := range c.Omitted {
		if v == verb {
			return "`" + verb + "` is listed by `" + "corvint-tasks help" + "` as not implemented in this build"
		}
	}
	if c.Err != "" {
		return c.Err
	}
	return "`corvint-tasks help` named this verb neither implemented nor omitted"
}

// ReadCapabilities asks the tool what it can do. Every board column and every
// control is decided from this, at request time.
func (t *Taskman) ReadCapabilities(ctx context.Context) *Capabilities {
	envelope, source := t.Run(ctx, "help")
	caps := &Capabilities{Source: source}
	if envelope == nil {
		caps.Err = source.Err
		return caps
	}
	if envelope.Refused() {
		caps.Err = "`corvint-tasks help` refused: " + strings.Join(envelope.Codes, ", ")
		return caps
	}
	if len(envelope.Items) == 0 {
		caps.Err = "`corvint-tasks help` returned no item, so the console cannot tell which verbs exist"
		return caps
	}
	item := envelope.Items[0]
	caps.Implemented = stringsOf(item["implemented"])
	caps.Omitted = stringsOf(item["omitted"])
	caps.Statuses = stringsOf(item["statuses"])
	caps.Eligibility = stringsOf(item["eligibility"])
	if note, ok := item["note"].(string); ok {
		caps.Note = note
	}
	if len(caps.Statuses) == 0 {
		caps.Err = "`corvint-tasks help` published no status enumeration, so this build cannot supply the board's columns"
	}
	return caps
}

func stringsOf(value any) []string {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, entry := range list {
		if s, ok := entry.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
