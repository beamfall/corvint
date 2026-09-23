package wire

// validateDiscrimination checks one cem/0.3 hunk discrimination witness
// (TCQ-V0-056): closed keys, a full tree revision and selection digest,
// consistent counts, one described survivor per survived mutant inside the
// hunk's newRange, positive bounds, and a state that agrees with the counts.
func validateDiscrimination(value Value, newRange Range) (DiscriminationWitness, error) {
	if value.Kind != KindObject {
		return DiscriminationWitness{}, fieldError("hunk discriminates must be an object")
	}
	keys := []string{"treeRevision", "selectionSha256", "mutants", "killed", "survived", "survivors", "bounds", "state", "detail"}
	if err := requireClosedKeys(value.Obj, keys); err != nil {
		return DiscriminationWitness{}, err
	}
	revision, _ := value.Obj.Get("treeRevision")
	if revision.Kind != KindString || !IsGitOid(revision.Str) {
		return DiscriminationWitness{}, fieldError("discriminates treeRevision must be a full Git object ID")
	}
	selection, _ := value.Obj.Get("selectionSha256")
	if selection.Kind != KindString || !IsSha256(selection.Str) {
		return DiscriminationWitness{}, fieldError("discriminates selectionSha256 must be a lowercase SHA-256 digest")
	}
	counts, err := discriminationCounts(value.Obj)
	if err != nil {
		return DiscriminationWitness{}, err
	}
	survivorsValue, _ := value.Obj.Get("survivors")
	if survivorsValue.Kind != KindArray {
		return DiscriminationWitness{}, fieldError("discriminates survivors must be an array")
	}
	survivors, err := validateSurvivors(survivorsValue.Arr, newRange)
	if err != nil {
		return DiscriminationWitness{}, err
	}
	if int64(len(survivors)) != counts[2] {
		return DiscriminationWitness{}, fieldError("discriminates survived must equal the number of survivors")
	}
	boundsValue, _ := value.Obj.Get("bounds")
	bounds, err := validateDiscriminationBounds(boundsValue)
	if err != nil {
		return DiscriminationWitness{}, err
	}
	state, _ := value.Obj.Get("state")
	if state.Kind != KindString || !discriminationStateAgrees(state.Str, counts) {
		return DiscriminationWitness{}, fieldError("discriminates state must be %s, %s, or %s and agree with its counts",
			DiscriminationDiscriminates, DiscriminationSurvived, DiscriminationNotRun)
	}
	detail, _ := value.Obj.Get("detail")
	if detail.Kind != KindString || !boundedPrintable(detail.Str, MaxDiscriminationTextBytes, true) {
		return DiscriminationWitness{}, fieldError("discriminates detail must be printable text of at most %d bytes", MaxDiscriminationTextBytes)
	}
	return DiscriminationWitness{
		TreeRevision: revision.Str, SelectionSha256: selection.Str,
		Mutants: counts[0], Killed: counts[1], Survived: counts[2],
		Survivors: survivors, Bounds: bounds, State: state.Str, Detail: detail.Str,
	}, nil
}

// discriminationCounts reads mutants, killed, and survived: non-negative wire
// integers with killed+survived never exceeding mutants (mutants that did not
// compile are neither).
func discriminationCounts(object *Object) ([3]int64, error) {
	var counts [3]int64
	for index, name := range []string{"mutants", "killed", "survived"} {
		count, _ := object.Get(name)
		if count.Kind != KindInt || count.Int < 0 {
			return counts, fieldError("discriminates %s must be a non-negative wire integer", name)
		}
		counts[index] = count.Int
	}
	if counts[1]+counts[2] > counts[0] {
		return counts, fieldError("discriminates killed and survived must not exceed mutants")
	}
	return counts, nil
}

func validateSurvivors(entries []Value, newRange Range) ([]SurvivingMutant, error) {
	survivors := []SurvivingMutant{}
	for _, entry := range entries {
		if entry.Kind != KindObject {
			return nil, fieldError("discriminates survivors entries must be objects")
		}
		if err := requireClosedKeys(entry.Obj, []string{"operator", "line", "description"}); err != nil {
			return nil, err
		}
		operator, _ := entry.Obj.Get("operator")
		if operator.Kind != KindString || !boundedPrintable(operator.Str, 64, false) {
			return nil, fieldError("discriminates survivor operator must be printable text of 1..64 bytes")
		}
		line, _ := entry.Obj.Get("line")
		if line.Kind != KindInt || line.Int < newRange.Start || line.Int >= newRange.Start+newRange.Count {
			return nil, fieldError("discriminates survivor line must be inside newRange")
		}
		description, _ := entry.Obj.Get("description")
		if description.Kind != KindString || !boundedPrintable(description.Str, MaxDiscriminationTextBytes, false) {
			return nil, fieldError("discriminates survivor description must be printable text of 1..%d bytes", MaxDiscriminationTextBytes)
		}
		survivors = append(survivors, SurvivingMutant{Operator: operator.Str, Line: line.Int, Description: description.Str})
	}
	return survivors, nil
}

func validateDiscriminationBounds(value Value) (DiscriminationBounds, error) {
	if value.Kind != KindObject {
		return DiscriminationBounds{}, fieldError("discriminates bounds must be an object")
	}
	names := []string{"maxHunks", "maxMutants", "wallTimeSeconds"}
	if err := requireClosedKeys(value.Obj, names); err != nil {
		return DiscriminationBounds{}, err
	}
	var bounds [3]int64
	for index, name := range names {
		bound, _ := value.Obj.Get(name)
		if bound.Kind != KindInt || bound.Int < 1 {
			return DiscriminationBounds{}, fieldError("discriminates bounds %s must be a positive wire integer", name)
		}
		bounds[index] = bound.Int
	}
	return DiscriminationBounds{MaxHunks: bounds[0], MaxMutants: bounds[1], WallTimeSeconds: bounds[2]}, nil
}

// discriminationStateAgrees pins each state to its counts: discriminates is
// at least one kill and no survivor, survived is at least one survivor, and
// not-run is no mutant at all.
func discriminationStateAgrees(state string, counts [3]int64) bool {
	switch state {
	case DiscriminationDiscriminates:
		return counts[1] >= 1 && counts[2] == 0
	case DiscriminationSurvived:
		return counts[2] >= 1
	case DiscriminationNotRun:
		return counts[0] == 0
	}
	return false
}

// boundedPrintable reports whether text is at most limit bytes, free of
// control characters, and (unless allowEmpty) non-empty.
func boundedPrintable(text string, limit int, allowEmpty bool) bool {
	if len(text) > limit || (text == "" && !allowEmpty) {
		return false
	}
	for _, character := range text {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}
