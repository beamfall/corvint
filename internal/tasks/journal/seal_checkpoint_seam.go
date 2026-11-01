package journal

// ForgeSealedBaseAggregatesForTest re-encodes wc, whose base is at its
// FullSeq, on a base whose receipt bytes and archive payload are lowered by
// less, while every current total keeps wc's value: the forged base of the
// LGL-V0-891 review regression (V1-2621). Production code never calls it.
func ForgeSealedBaseAggregatesForTest(wc *WriterCheckpoint, less uint64) (base, delta []byte, err error) {
	if wc == nil || wc.base == nil {
		return nil, nil, errCheckpoint("writer checkpoint", "no base to forge")
	}
	b := wc.base
	m := &b.meta
	bc := &WriterCheckpoint{Checkpoint: m.header, FullSeq: b.seq, ReceiptBytes: m.receiptBytes, Cost: m.cost, Intent: m.intent, Invalidation: wc.Invalidation, base: b, total: b.n}
	bc.Entries = nil
	nb, err := bc.full()
	if err != nil {
		return nil, nil, err
	}
	nb.segments = m.segments
	if nb.ReceiptBytes < less || nb.Cost.PayloadBytes < less {
		return nil, nil, errCheckpoint("writer checkpoint", "base too small to forge")
	}
	nb.ReceiptBytes -= less
	nb.Cost.PayloadBytes -= less
	forged, err := rebase(wc, nb)
	if err != nil {
		return nil, nil, err
	}
	return forged.Encode()
}
