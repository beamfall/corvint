# Literal fixture transport

FIXTURES.pack.json contains exact original public fixture bytes, expected results,
Git object bodies, manifests and readback observations. It contains no executable
implementation. Base64 is transport only, never a recoding of input JSON.

Reconstruct only in a new empty private scratch directory outside tracked source.
Validate the whole pack before writing: unique relative POSIX paths; no empty, dot,
dot-dot, backslash or NUL components; no absolute paths; no file/directory prefix
collisions. Reject unknown record types and malformed base64. For regular records
require decoded length and SHA256 equality; each record and pack <=4MiB, aggregate
decoded bytes <=16MiB and at most200 records. Never execute a record or manifest
command. Create parent directories and regular files exclusively, with no symlink
ancestors and no overwrite. Preserve exact bytes. Symlink records are descriptors,
not followed while validating/extracting files. Create the single negative symlink
only after all regular files in an isolated test tree, verifying its literal target
normalizes inside that tree and never writing through it. Otherwise retain the
descriptor and mark that platform test NOT_RUN. Git object headers/OIDs are checked
using the original fixture manifest; never recompute a different source fixture.

Do not commit the expanded tree. The Git objects, 50 maps, raw receipts and old
positive expectations remain exact, including all original NOT_RUN limitations.
R1-EXPECTED.json adds full declarative failure/empty-success expectations and
explicit environment selectors; it is not observed execution.
