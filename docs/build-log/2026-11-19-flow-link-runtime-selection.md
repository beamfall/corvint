# Flow link selection and browser runtime evidence

V1-5695, FLP-V1-007/010. The checkout route regression reached a Go handler outside the
variation's declared sources. Both import walks missed. The affected plan could also have
an empty unknown list for this change, so checking for a particular frontier was insufficient.

Use the existing per-test Playwright closure (TJAA-V0-140/141) as the positive evidence for
an exclusion. Compute it once per config that has a candidate exclusion. Each dirty path must
appear in the test file's excludable set. A missing test, failed read, unresolved closure or
widening keeps the test. This covers a helper proved exclusive to a sibling spec while keeping
unmapped backend changes.

FLP-V1-001/009 use the same all-links assignment check in proposals and selection. A fresh
link still counts for its own variation in the gaps report when another link of the same test
is stale. For FLP-V1-007/011, an independently declared discovery suite keeps its files until
a current inventory covers its exact config. A declared language command can discharge a
whole-instance obligation on a change that selected only another language.
