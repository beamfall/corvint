package dogfoodflow

import "strings"

// cemMigrationCommit is the V1-1277 commit that rewrote every committed sealed
// CEM to cem/1.0 in place (decision 0525 §3 step 6 and §5, DCW-V0-050). From it
// on it is the seal baseline: a sealed CEM it rewrote must keep the bytes it
// wrote. Tests point it at their own fixture commit.
var cemMigrationCommit = "0b6f1bc245b44a8620c5e64f91e87f6d08da08d0"

// migratedSeals lists the sealed CEM paths that BASE..HEAD modifies only
// through the migration commit: that commit is in the range, BASE holds
// exactly the bytes it rewrote, and HEAD holds exactly the bytes it wrote
// (DCW-V0-050). It reads three listings, not one Git call per path.
func (f *flow) migratedSeals(base, target string) map[string]bool {
	if !f.migrationInRange(base, target) {
		return nil
	}
	raw, status := f.git(true, "diff-tree", "-r", "--raw", "--no-renames", "--no-abbrev", "--no-commit-id", cemMigrationCommit+"^", cemMigrationCommit, "--", ".corvint/changes")
	if status != 0 {
		return nil
	}
	baseBlobs, targetBlobs := f.sealedBlobs(base), f.sealedBlobs(target)
	admitted := map[string]bool{}
	for _, line := range nonEmpty(strings.Split(raw, "\n")) {
		meta, path, _ := strings.Cut(line, "\t")
		fields := strings.Fields(meta)
		if len(fields) != 5 || fields[4] != "M" || sealedCEMPath.FindStringSubmatch(path) == nil {
			continue
		}
		if before, after := fields[2], fields[3]; before != after && baseBlobs[path] == before && targetBlobs[path] == after {
			admitted[path] = true
		}
	}
	return admitted
}

// sealedBlobs maps each regular file under .corvint/changes at rev to its blob.
func (f *flow) sealedBlobs(rev string) map[string]string {
	blobs := map[string]string{}
	listing, status := f.git(true, "ls-tree", "-r", "--full-tree", rev, "--", ".corvint/changes")
	if status != 0 {
		return blobs
	}
	for _, line := range nonEmpty(strings.Split(listing, "\n")) {
		meta, path, _ := strings.Cut(line, "\t")
		if fields := strings.Fields(meta); len(fields) == 3 && fields[1] == "blob" {
			blobs[path] = fields[2]
		}
	}
	return blobs
}

// migratedConstituent reports a sealed CEM that target holds as the migration
// commit's rewrite of its bind commit's sidecar. Its bind commit's excluded
// sidecar still holds the source bytes, so the stable verifier cannot verify
// the rewrite there (CEM-V1-104); the batch must be split at the migration
// commit instead (DCW-V0-050).
func (f *flow) migratedConstituent(target, path, bindBlob, sealedBlob string) bool {
	if cemMigrationCommit == "" || bindBlob == "" || !f.gitSucceeds("merge-base", "--is-ancestor", cemMigrationCommit, target) {
		return false
	}
	before, after := f.migration(path)
	return before == bindBlob && after == sealedBlob && before != after
}

func (f *flow) migrationInRange(base, target string) bool {
	return cemMigrationCommit != "" && f.gitSucceeds("merge-base", "--is-ancestor", cemMigrationCommit, target) &&
		!f.gitSucceeds("merge-base", "--is-ancestor", cemMigrationCommit, base)
}

// migration returns the blobs of path before and after the migration commit.
func (f *flow) migration(path string) (string, string) {
	return f.gitRevision(cemMigrationCommit + "^:" + path), f.gitRevision(cemMigrationCommit + ":" + path)
}

// isMigrationCommit reports the migration commit itself when it only modifies
// sealed CEM paths, which batch-check then counts as bound (DCW-V0-050).
func (f *flow) isMigrationCommit(parent, commit string) bool {
	if cemMigrationCommit == "" || commit != cemMigrationCommit {
		return false
	}
	changed, status := f.git(true, "diff-tree", "-r", "--no-renames", "--no-commit-id", "--name-status", parent, commit)
	lines := nonEmpty(strings.Split(changed, "\n"))
	if status != 0 || len(lines) == 0 {
		return false
	}
	for _, line := range lines {
		state, path, _ := strings.Cut(line, "\t")
		if state != "M" || sealedCEMPath.FindStringSubmatch(path) == nil {
			return false
		}
	}
	return true
}
