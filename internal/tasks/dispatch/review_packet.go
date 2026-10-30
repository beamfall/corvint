package dispatch

import (
	"context"
	"fmt"
	"strconv"
)

// Review packet bounds (ATR-V0-141, ERG-V0-019): the packet a review role's
// prompt carries is capped between these sizes.
const (
	MinReviewPacketBytes     = 4096
	MaxReviewPacketBytes     = 262144
	DefaultReviewPacketBytes = 32768
)

// ReviewPacket is a role's opt-in ATR-V0-141 review packet. Enabled
// defaults to true; an absent MaxBytes takes the default and an explicit
// value out of range is refused. SourceRepository names a configuration
// sourceRepositories entry whose checkout the packet's commit is resolved in
// (ERG-V0-022); empty reads the task repository.
type ReviewPacket struct {
	Enabled          *bool  `json:"enabled,omitempty"`
	MaxBytes         *int   `json:"maxBytes,omitempty"`
	SourceRepository string `json:"sourceRepository,omitempty"`
}

// MaxSourceRepositories bounds a configuration's sourceRepositories.
const MaxSourceRepositories = 16

// PacketSource is the ERG-V0-022 source repository a review packet resolves
// its commit in: a configured name and its clean absolute checkout path. The
// zero value is the task repository.
type PacketSource struct{ Name, Path string }

func (r *ReviewPacket) validate() error {
	if m := r.MaxBytes; m != nil && (*m < MinReviewPacketBytes || *m > MaxReviewPacketBytes) {
		return fmt.Errorf("reviewPacket needs maxBytes %d..%d", MinReviewPacketBytes, MaxReviewPacketBytes)
	}
	return nil
}

// on reports a present reviewPacket that is not disabled.
func (r *ReviewPacket) on() bool { return r != nil && (r.Enabled == nil || *r.Enabled) }

func (r *ReviewPacket) maxBytes() int {
	if r.MaxBytes == nil {
		return DefaultReviewPacketBytes
	}
	return *r.MaxBytes
}

// ReviewPacketSource is the optional Queue capability that builds the
// ERG-V0-017 packet for a ticket's newest candidate, framed as untrusted
// data, with the hand-off note at notePath (empty for none) as its partial
// checkpoint, resolving the commit in source (ERG-V0-022). A Queue without it
// launches review roles without a packet.
type ReviewPacketSource interface {
	ReviewPacket(ctx context.Context, ticket string, maxBytes int, notePath string, source PacketSource) (string, error)
}

// reviewPacketFor is the ATR-V0-141 packet for a launch of a role with r on,
// or "" with the reason it was not offered.
func (d *Dispatcher) reviewPacketFor(ctx context.Context, obs *Observation, r *ReviewPacket, ticket string) (string, error) {
	src, ok := d.Queue.(ReviewPacketSource)
	if !ok {
		return "", fmt.Errorf("the queue does not build review packets")
	}
	var source PacketSource
	if name := r.SourceRepository; name != "" {
		source = PacketSource{Name: name, Path: d.Config.SourceRepositories[name]}
	}
	return src.ReviewPacket(ctx, ticket, r.maxBytes(), d.handoffNoteFor(obs, ticket), source)
}

// reviewPacketDetail records the offered packet's size on the launched
// event (ATR-V0-141).
func reviewPacketDetail(detail map[string]string, packet string) {
	if packet != "" {
		detail["reviewPacketBytes"] = strconv.Itoa(len(packet))
	}
}
