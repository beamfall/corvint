# Exact wave-1 evidence bundle and inner archives

The final tree consolidates 434 added evidence files from immutable source commit
`ceb6eb0eda5ff4e3c7c60a75f718c8dc44092c79` into
`NEXTGEN-WAVE1-BUNDLE.tar.gz.b64`. `NEXTGEN-WAVE1-BUNDLE.manifest.json` pins the source
commit/tree, every full relative path, Git mode, byte count, SHA-256 and Git blob identity,
plus the tar, gzip and wrapped text hashes. The decoded files total 5,437,889 bytes.
All 434 payloads were restored and compared with their source Git blobs before removal;
`NEXTGEN-WAVE1-BUNDLE.validation.json` records that proof and restore safety witnesses.

This second packaging layer is necessary because the frozen impact input limit is 100
changed paths: the previous evidence layout made the wave's delivered diff 498 paths.
Only newly added evidence paths are consolidated; no artifact present at the original
wave base is deleted. Runtime limits and measured evidence remain unchanged.

From the checkout root, verify without writing, or restore into a fresh scratch directory:

```sh
python3 script/restore-nextgen-wave1-evidence.py --verify-only
python3 script/restore-nextgen-wave1-evidence.py --output /private/tmp/corvint-wave1-evidence
```

The restore helper uses Python's standard library on POSIX systems. It validates bounded
manifest/container/member sizes, digests, regular-file modes, safe relative paths, unique
members and exact counts before creating output. It refuses an existing output directory
or symlink and never uses `extractall`. Output parents must already exist. A filesystem
write failure can leave a partial newly created directory; verification failures create
no output. Manifest authenticity comes from the reviewed Git tree containing it.

Historical paths in registrations and reports now identify files beneath that restored
root, or at the pinned source commit. This includes registrations, outcomes, summaries,
all 284 existing inner archives and their unchanged original archive manifest. The outer
bundle does not change a byte inside any of them.

## Earlier per-report packaging

The first complete staging commit, `f105bc587d5a55123cf7f6ec71b42a435396ada0`,
contains the original raw files. CEM refused that 20,484,370-byte patch with
`git-output-exceeded`: the frozen textual patch ceiling is 8 MiB. This is a
preserved NOT_PRODUCED result, not permission to enlarge the wire limit.

Inside the restored tree, large raw reports and packet strings remain deterministic gzip
inside line-wrapped base64 text (`.json.gz.b64`). Every file round-tripped to its
exact original bytes before replacement. `nextgen-wave1-evidence-archive.json`
maps original paths to archives and pins both SHA-256 digests and byte counts.
Registrations, outcomes and summaries are readable after outer restoration. Existing original
path references in frozen records identify the decoded artifact; their text and
hashes have not been rewritten to disguise the packaging change.

After outer restoration, inspect an inner archived report without changing this checkout.
This example verifies all inner archives and restores the original relative paths beneath
a second fresh scratch directory, `/private/tmp/corvint-wave1-restored`:

```python
import base64, gzip, hashlib, io, json
from pathlib import Path

repo = Path('/private/tmp/corvint-wave1-evidence')  # verified outer restoration
out = Path('/private/tmp/corvint-wave1-restored')
out.mkdir(mode=0o700)  # refuses preexisting output
manifest = json.loads((repo / 'benchmarks/results/nextgen-wave1-evidence-archive.json').read_text())
for entry in manifest['files']:
    relative = Path(entry['original_path'])
    assert not relative.is_absolute() and '..' not in relative.parts
    encoded = (repo / entry['archived_path']).read_bytes()
    assert hashlib.sha256(encoded).hexdigest() == entry['archive_sha256']
    with gzip.GzipFile(fileobj=io.BytesIO(base64.b64decode(encoded))) as stream:
        raw = stream.read(entry['original_bytes'] + 1)
    assert len(raw) == entry['original_bytes']
    assert hashlib.sha256(raw).hexdigest() == entry['original_sha256']
    target = out / relative
    target.parent.mkdir(parents=True, exist_ok=True)
    with target.open('xb') as stream:
        stream.write(raw)
```

No measurement was rerun, edited, filtered or recomputed during packaging.

A second preparation refused `malformed-patch: expected --- file header` because
eight exact empty stdout logs produce header-only new-file Git diffs. Empty logs
are now archived by the same lossless method, explicitly pinned at zero bytes.
No synthetic log text was inserted and the failed preparation is preserved.
