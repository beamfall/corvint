package model

func validateRepositoryWitnessSemantics(source SourceInput, repository Repository) error {
	if source.AdapterID != "local-trace-v1" {
		if source.RepositoryWitnesses != nil {
			return invalidArgument()
		}
		return nil
	}
	if source.Validity != ValidityValid {
		if source.RepositoryWitnesses != nil {
			return invalidArgument()
		}
		return nil
	}
	if source.RepositoryWitnesses == nil || repository.HeadRevision == nil || repository.TreeRevision == nil ||
		repository.ObjectFormat == nil || repository.DirtyPathsSHA256 == nil {
		return invalidArgument()
	}
	for _, cohort := range source.Cohorts {
		if cohort.RepositoryObjectFormat == nil || *cohort.RepositoryObjectFormat != *repository.ObjectFormat ||
			cohort.DirtyPathsSHA256 == nil || *cohort.DirtyPathsSHA256 != *repository.DirtyPathsSHA256 ||
			cohort.SourceRevision == nil || cohort.SourceTreeRevision == nil {
			return invalidArgument()
		}
	}
	witnesses, err := normalizeRepositoryWitnesses(source.RepositoryWitnesses)
	if err != nil {
		return err
	}
	headCount := 0
	revisions := make(map[string]int, len(source.MembersValue()))
	for _, member := range source.MembersValue() {
		revisions[member.Revision] = 0
	}
	for _, witness := range witnesses {
		if witness.ObjectFormat != *repository.ObjectFormat {
			return invalidArgument()
		}
		switch witness.Kind {
		case "SNAPSHOT_HEAD":
			if witness.ObjectID != *repository.HeadRevision {
				return invalidArgument()
			}
			headCount++
		case "TRACE_REVISION":
			if _, ok := revisions[witness.ObjectID]; !ok {
				return invalidArgument()
			}
			revisions[witness.ObjectID]++
		case "TRACE_PATH_OBJECT":
			if _, ok := revisions[pointerValue(witness.Revision)]; !ok {
				return invalidArgument()
			}
		}
	}
	if headCount != 1 {
		return invalidArgument()
	}
	for _, count := range revisions {
		if count != 1 {
			return invalidArgument()
		}
	}
	return nil
}

func (source SourceInput) MembersValue() []TraceMember {
	if source.Members == nil {
		return nil
	}
	return *source.Members
}
