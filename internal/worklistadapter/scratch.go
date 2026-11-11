package worklistadapter

// Scratch is the observer-owned tuple a producer may nest its source acquisition in.
type Scratch struct{ Parent, Root, Commit string }
