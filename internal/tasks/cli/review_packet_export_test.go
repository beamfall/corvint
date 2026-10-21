package cli

// ShrinkReviewPacketHistory sets the packet's per-gate history bound to pages
// of size events until the test ends, so a test reaches the ERG-V0-027 bound
// with a few events.
func ShrinkReviewPacketHistory(t interface{ Cleanup(func()) }, pages, size int) {
	oldPages, oldSize := reviewPacketHistoryPages, reviewPacketHistoryPage
	reviewPacketHistoryPages, reviewPacketHistoryPage = pages, size
	t.Cleanup(func() { reviewPacketHistoryPages, reviewPacketHistoryPage = oldPages, oldSize })
}
