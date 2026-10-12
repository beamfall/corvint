package wire

// RoutedMap is a map read by ParseRouted: exactly one of Legacy and Stable is
// set, and Spec, BaseRevision and PatchSha256 repeat the header of that one.
type RoutedMap struct {
	Spec         string
	BaseRevision string
	PatchSha256  string
	Legacy       *Map
	Stable       *StableMap
}

// ParseRouted sends bytes that declare exactly StableSpec to ParseStable and
// every other input to ParseMap, whose acceptance it leaves unchanged
// (CEM-V1-065). A stable map is accepted only when ParseStable returns no
// error, including no graph error.
func ParseRouted(data []byte) (RoutedMap, error) {
	if DeclaresStable(data) {
		document, err := ParseStable(data)
		if err != nil {
			return RoutedMap{}, err
		}
		return RoutedMap{
			Spec: document.Change.Spec, BaseRevision: document.Change.BaseRevision,
			PatchSha256: document.Change.PatchSha256, Stable: document,
		}, nil
	}
	document, err := ParseMap(data)
	if err != nil {
		return RoutedMap{}, err
	}
	return RoutedMap{
		Spec: document.Spec, BaseRevision: document.BaseRevision,
		PatchSha256: document.PatchSha256, Legacy: document,
	}, nil
}
