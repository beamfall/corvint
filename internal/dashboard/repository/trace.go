package repository

import (
	"bytes"
	"context"
	"sort"

	dashboardauthority "github.com/Beamfall/corvint/internal/dashboard/authority"
)

const (
	maxTreePathsPerChild = 64
	maxTreePathBytes     = 8_192
)

func (authority *Authority) QualifyTrace(ctx context.Context, revision string, requestedPaths []string) dashboardauthority.TraceQualification {
	if authority == nil {
		return authorityUnavailableResult()
	}
	authority.mu.Lock()
	defer authority.mu.Unlock()
	if authority.closed || authority.finished || authority.objectFailed || !validObjectID(revision, authority.snapshot.ObjectFormat) {
		if authority.closed || authority.finished || authority.objectFailed {
			return authorityUnavailableResult()
		}
		return dashboardauthority.TraceQualification{Code: dashboardauthority.TraceInvalidIdentity}
	}
	paths, valid := sortedUniquePaths(requestedPaths)
	if !valid {
		return dashboardauthority.TraceQualification{Code: dashboardauthority.TraceInvalidIdentity}
	}
	reservation := authority.budget.reserveObject(revision, "commit")
	if reservation == objectTypeConflict {
		return dashboardauthority.TraceQualification{Code: dashboardauthority.TraceInvalidIdentity}
	}
	if reservation == objectExhausted {
		authority.objectFailed = true
		return authority.failObjectQualification(ctx)
	}
	for _, pathValue := range paths {
		if !authority.budget.reservePath(revision + "\x00" + pathValue) {
			authority.objectFailed = true
			return authority.failObjectQualification(ctx)
		}
	}
	revisionCheck := authority.checkObjects(ctx, []objectExpectation{{id: revision, expectedType: "commit"}})[revision]
	if revisionCheck == objectCheckInvalid {
		return dashboardauthority.TraceQualification{Code: dashboardauthority.TraceInvalidIdentity}
	}
	if revisionCheck == objectCheckOversize {
		return dashboardauthority.TraceQualification{Code: dashboardauthority.TracePathUntracked}
	}
	if revisionCheck != objectCheckQualified {
		return authority.failObjectQualification(ctx)
	}

	commitResult, failure := authority.run(ctx, authority.layout.worktree.path, nil,
		[]string{"rev-parse", "--verify", revision + "^{commit}"})
	if failure != nil || commitResult.exit != 0 {
		return authority.failObjectQualification(ctx)
	}
	commitFields, parseErr := strictLFFields(commitResult.stdout, 1)
	if parseErr != nil || !validObjectID(commitFields[0], authority.snapshot.ObjectFormat) {
		return authority.failObjectQualification(ctx)
	}
	if commitFields[0] != revision {
		return dashboardauthority.TraceQualification{Code: dashboardauthority.TraceInvalidIdentity}
	}

	treeResult, failure := authority.run(ctx, authority.layout.worktree.path, nil,
		[]string{"rev-parse", "--verify", revision + "^{tree}"})
	if failure != nil || treeResult.exit != 0 {
		return authority.failObjectQualification(ctx)
	}
	treeRevision, parseErr := parseSingleObject(treeResult.stdout, authority.snapshot.ObjectFormat)
	if parseErr != nil {
		return authority.failObjectQualification(ctx)
	}

	ancestorResult, failure := authority.run(ctx, authority.layout.worktree.path, nil,
		[]string{"merge-base", "--is-ancestor", revision, authority.snapshot.HeadRevision})
	if failure != nil || len(ancestorResult.stdout) != 0 || (ancestorResult.exit != 0 && ancestorResult.exit != 1) {
		return authority.failObjectQualification(ctx)
	}
	if ancestorResult.exit == 1 {
		return dashboardauthority.TraceQualification{Code: dashboardauthority.TraceAncestryBound}
	}
	distanceResult, failure := authority.run(ctx, authority.layout.worktree.path, nil,
		[]string{"rev-list", "--ancestry-path", "--count", "--max-count=10001", revision + ".." + authority.snapshot.HeadRevision})
	if failure != nil || distanceResult.exit != 0 {
		return authority.failObjectQualification(ctx)
	}
	distance, parseErr := parseCount(distanceResult.stdout)
	if parseErr != nil {
		return authority.failObjectQualification(ctx)
	}
	if distance >= 10_001 {
		return dashboardauthority.TraceQualification{Code: dashboardauthority.TraceAncestryBound}
	}

	objects := make([]treeObject, 0, len(paths))
	for offset := 0; offset < len(paths); {
		end := treeChunkEnd(revision, paths, offset)
		if end == offset {
			return authority.failObjectQualification(ctx)
		}
		tail := []string{"ls-tree", "-z", "--full-tree", revision, "--"}
		tail = append(tail, paths[offset:end]...)
		result, failure := authority.run(ctx, authority.layout.worktree.path, nil, tail)
		if failure != nil || result.exit != 0 {
			return authority.failObjectQualification(ctx)
		}
		chunkObjects, regular, parseErr := parseTreeObjects(result.stdout, paths[offset:end], authority.snapshot.ObjectFormat)
		if parseErr != nil {
			return authority.failObjectQualification(ctx)
		}
		if !regular {
			return dashboardauthority.TraceQualification{Code: dashboardauthority.TracePathUntracked}
		}
		objects = append(objects, chunkObjects...)
		offset = end
	}

	witnesses := []dashboardauthority.Witness{
		snapshotHeadWitness(authority.snapshot),
		{Kind: "TRACE_REVISION", ObjectFormat: authority.snapshot.ObjectFormat, ObjectID: revision, ObjectType: "commit", Revision: &revision},
	}
	uniqueObjects := make(map[string]struct{}, len(objects))
	for _, object := range objects {
		uniqueObjects[object.objectID] = struct{}{}
	}
	objectIDs := make([]string, 0, len(uniqueObjects))
	for objectID := range uniqueObjects {
		objectIDs = append(objectIDs, objectID)
	}
	sort.Strings(objectIDs)
	expectations := make([]objectExpectation, 0, len(objectIDs))
	for _, objectID := range objectIDs {
		reservation := authority.budget.reserveObject(objectID, "blob")
		if reservation == objectTypeConflict {
			return dashboardauthority.TraceQualification{Code: dashboardauthority.TraceInvalidIdentity}
		}
		if reservation == objectExhausted {
			authority.objectFailed = true
			return authority.failObjectQualification(ctx)
		}
		expectations = append(expectations, objectExpectation{id: objectID, expectedType: "blob"})
	}
	checks := authority.checkObjects(ctx, expectations)
	for _, objectID := range objectIDs {
		switch checks[objectID] {
		case objectCheckQualified:
		case objectCheckInvalid:
			return dashboardauthority.TraceQualification{Code: dashboardauthority.TraceInvalidIdentity}
		case objectCheckOversize:
			return dashboardauthority.TraceQualification{Code: dashboardauthority.TracePathUntracked}
		default:
			return authority.failObjectQualification(ctx)
		}
	}
	for _, objectID := range objectIDs {
		revisionCopy := revision
		witnesses = append(witnesses, dashboardauthority.Witness{
			Kind: "TRACE_PATH_OBJECT", ObjectFormat: authority.snapshot.ObjectFormat,
			ObjectID: objectID, ObjectType: "blob", Revision: &revisionCopy,
		})
	}
	pathWitnesses := make([]dashboardauthority.PathWitness, 0, len(objects))
	for _, object := range objects {
		revisionCopy := revision
		pathWitnesses = append(pathWitnesses, dashboardauthority.PathWitness{Path: object.path, Witness: dashboardauthority.Witness{
			Kind: "TRACE_PATH_OBJECT", ObjectFormat: authority.snapshot.ObjectFormat,
			ObjectID: object.objectID, ObjectType: "blob", Revision: &revisionCopy,
		}})
	}
	return dashboardauthority.TraceQualification{
		Code: dashboardauthority.TraceQualified, ObjectFormat: authority.snapshot.ObjectFormat,
		TreeRevision: treeRevision, Witnesses: cloneWitnesses(witnesses), PathWitnesses: pathWitnesses,
	}
}

func (authority *Authority) failObjectQualification(ctx context.Context) dashboardauthority.TraceQualification {
	if ctx != nil && ctx.Err() != nil {
		return dashboardauthority.TraceQualification{Code: dashboardauthority.TraceInterrupted}
	}
	return authorityUnavailableResult()
}

func authorityUnavailableResult() dashboardauthority.TraceQualification {
	return dashboardauthority.TraceQualification{Code: dashboardauthority.TraceObjectUnavailable}
}

func snapshotHeadWitness(snapshot dashboardauthority.Snapshot) dashboardauthority.Witness {
	return dashboardauthority.Witness{Kind: "SNAPSHOT_HEAD", ObjectFormat: snapshot.ObjectFormat, ObjectID: snapshot.HeadRevision, ObjectType: "commit"}
}

func cloneWitnesses(values []dashboardauthority.Witness) []dashboardauthority.Witness {
	result := make([]dashboardauthority.Witness, len(values))
	for index, witness := range values {
		result[index] = witness
		if witness.Revision != nil {
			revision := *witness.Revision
			result[index].Revision = &revision
		}
	}
	return result
}

func treeChunkEnd(revision string, paths []string, offset int) int {
	_ = revision
	pathBytes := uint64(0)
	end := offset
	for end < len(paths) && end-offset < maxTreePathsPerChild {
		nextBytes := uint64(len(paths[end]))
		if pathBytes > maxTreePathBytes || nextBytes > maxTreePathBytes-pathBytes {
			break
		}
		pathBytes += nextBytes
		end++
	}
	return end
}

func argumentBytes(arguments []string) uint64 {
	var total uint64
	for _, argument := range arguments {
		total += uint64(len(argument)) + 1
	}
	return total
}

func sortTreeObjects(objects []treeObject) {
	sort.Slice(objects, func(left, right int) bool {
		if objects[left].objectID != objects[right].objectID {
			return objects[left].objectID < objects[right].objectID
		}
		return bytes.Compare([]byte(objects[left].path), []byte(objects[right].path)) < 0
	})
}
