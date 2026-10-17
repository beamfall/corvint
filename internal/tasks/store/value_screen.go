package store

import (
	"path/filepath"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// MaxDenyValuesFileBytes bounds the adopter values file read for one write
// (KHN-V0-029).
const MaxDenyValuesFileBytes = 1 << 20

// ValueScreenUnavailableDetail prefixes the refusal of a screened write
// whose declared values file cannot be read (KHN-V0-029): the screen fails
// closed rather than letting the write through unscreened.
const ValueScreenUnavailableDetail = "SCREEN_VALUES_UNAVAILABLE"

// valueScreenInput builds the KHN-V0-028 value screen for a fresh NOTE_SET,
// KNOWHOW_* or OBLIGATIONS_* write while policy declares knowHow.screen. The
// values file, resolved against the primary worktree, is read for this write
// only; its bytes are never stored, logged or echoed.
func valueScreenInput(repo *intent.Repository, env *mutation.Envelope, policy []byte) (*mutation.ValueScreen, error) {
	if !screenedOperation(env.Operation) {
		return nil, nil
	}
	p, err := intent.DecodePolicy(policy)
	if err != nil {
		return nil, err
	}
	return LoadValueScreen(repo, p)
}

// LoadValueScreen reads the declared screen, or returns nil when policy
// declares none.
func LoadValueScreen(repo *intent.Repository, p *intent.Policy) (*mutation.ValueScreen, error) {
	s := p.ValueScreen()
	if s == nil {
		return nil, nil
	}
	var raw []byte
	if s.DenyValuesFile != nil {
		b, err := intent.ReadFile(filepath.Join(repo.PrimaryWorktree, filepath.FromSlash(*s.DenyValuesFile)), MaxDenyValuesFileBytes)
		if err != nil {
			return nil, wire.Errorf(wire.CodeMissingEvidence, "/knowHow/screen/denyValuesFile", "%s: the declared values file %s cannot be read, so the screened write is refused", ValueScreenUnavailableDetail, *s.DenyValuesFile)
		}
		raw = b
	}
	return mutation.NewValueScreen(raw, s.Classes), nil
}

func screenedOperation(op string) bool {
	switch op {
	case mutation.OpNoteSet, mutation.OpKnowHowAdd, mutation.OpKnowHowRetract, mutation.OpKnowHowReconfirm:
		return true
	}
	return ticket.IsObligationOperation(op)
}
