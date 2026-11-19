# Flow-link inventory dependencies

The docs commands and the affected planner both need the committed inventory, its Git bindings
and its freshness verdict (FLP-V1-001, FLP-V1-006 to FLP-V1-011). Decision 0595 places their
flow reader in the shared layer. Move the existing codec, tree access and judgment into
`internal/testinventory/reader`. Keep refresh, runner execution and listers in the tests layer.
The existing inventory API delegates to the shared implementation. The docs qualification owns
the reader's Go inputs on each release target (RCF-V1-010, RCF-V1-013).

The affected plan defines its flow tier where readers can use it without importing the
line-evidence collectors (LEV-V1-009). The version refusal still derives the supported versions
from the schema registry. Version 7 selects the full E2E scope, and an unknown member of
version 1 leaves the scope unknown (FSV-V1-004).
