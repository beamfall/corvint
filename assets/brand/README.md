# Corvint artwork

The corvid direction was selected by the owner on 2026-09-19. Its watchful head and sweeping wing
form a compact silhouette. The lowercase wordmark uses custom vector paths, with no font dependency.
The initial visual exploration used the built-in imagegen tool; the selected design was redrawn as
editable SVG. [CRB-V0-014](../../docs/specs/corvint-rebrand-v0.md) governs the asset family.

| Use | Asset |
|---|---|
| Light background | `corvint-lockup-light.svg` / `corvint-mark-light.svg` |
| Dark background | `corvint-lockup-dark.svg` / `corvint-mark-dark.svg` |
| Avatar or unknown background | `corvint-mark-universal.png` |
| Fixed dark logo tile | `corvint-lockup-universal.svg` |
| Editor activity bar | `../../extensions/vscode/media/corvint.svg` (`currentColor`, 24 px) |

Each brand SVG has a matching PNG. Marks are 1024 × 1024; lockups are 2460 × 600. Light/dark files
have transparent backgrounds. Universal files include a dark rounded tile with transparent corners.
Use SVG for the README and scalable applications, PNG where vector images are unsupported. The root
README selects white artwork for dark mode and dark artwork for light mode.

Keep the supplied padding, preserve the aspect ratio, and use the standalone mark at small sizes.
The minimum recommended mark canvas is 24 px. Use dark ink `#111216` or white `#ffffff`; do not add
outlines, gradients, shadows, or independent colors to the head and wing.

SVG files are the editable masters. PNGs can be reproduced with librsvg:

```sh
for asset in assets/brand/*mark*.svg; do
  rsvg-convert --width 1024 "$asset" --output "${asset%.svg}.png"
done
for asset in assets/brand/*lockup*.svg; do
  rsvg-convert --width 2460 "$asset" --output "${asset%.svg}.png"
done
```

Rollback restores the brand asset family, editor icon, root README and CRB-V0-014 together from the
preceding revision. Historical rebrand qualification is not evidence for these replacement bytes.
