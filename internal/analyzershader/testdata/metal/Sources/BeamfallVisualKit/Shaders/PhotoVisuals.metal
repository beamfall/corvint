#include "VisualShared.h"

// =============================================================================
// Photo Visuals (ADR-0047 Decision 6) — image-capable Visuals that render the
// CURRENT IMAGE (the Ambient Queue photo, or now-playing artwork) as audio-
// reactive material rather than a passive crossfade. Ambient-Display-only;
// inherits the Ambient Queue item's Reveal contract (no new privacy surface).
//
// uImage (texture 0) is the current image, bound by VisualRenderer. When no
// photo is present the renderer binds a 1x1 black placeholder, so these shaders
// always produce a coherent (if dim) result — they never read an unbound slot.
//
// ENHANCED vs FLAT layer (the beat-grid degrade pattern). The renderer threads
// the optional depth/saliency/palette Enrichment (Core's image-enrich.json,
// produced by the Enrichment Plugin) into the float4 uniform slots:
//   u.palette = (dominantColor.rgb, presentFlag)   presentFlag: 0 absent,
//                                                   0.5 flat-analysed, 1 usable
//   u.motion  = (salientMinX, salientMinY, salientMaxX, salientMaxY)  [0,1] box
//   u.presentation1.w = depthBias (parallax strength, [0,1])
// When presentFlag < 1 the Visual takes the FLAT shader-only path (Ken-Burns
// pan, center framing, theme palette); when >= 1 it lights up the enhanced
// layer (2.5D parallax, subject framing, photo-driven lighting). Either way it
// is "flat but flashy" — never a dead frame.
//
// SAFETY: photo Visuals stay inside the shared 3Hz / luminance ceilings. They
// modulate the photo gently (slow pans, soft displacement, beat-eased facets);
// no strobing, no full-frame luminance flips. The per-Visual safety manifest in
// VisualRegistry declares maxFlashHz <= 3 and a low luminance delta.
// =============================================================================

static inline float3 photoSample(texture2d<float> img, sampler s, float2 uv) {
    return max(img.sample(s, clamp(uv, float2(0.0), float2(1.0))).rgb, float3(0.0));
}

// Enrichment accessors — read the threaded float4 slots with safe fallbacks so a
// shader written against the enhanced layer still works on the flat path.
static inline float enrichPresent(constant VisualUniforms& u) { return u.palette.w; }
static inline float depthBias(constant VisualUniforms& u) { return clamp(u.presentation1.w, 0.0, 1.0); }
static inline float4 salientBox(constant VisualUniforms& u) {
    float4 b = u.motion; // (minX, minY, maxX, maxY)
    // Degenerate/absent box → full frame.
    if (b.z <= b.x || b.w <= b.y) return float4(0.0, 0.0, 1.0, 1.0);
    return clamp(b, float4(0.0), float4(1.0));
}
static inline float2 salientCenter(constant VisualUniforms& u) {
    float4 b = salientBox(u);
    return float2((b.x + b.z) * 0.5, (b.y + b.w) * 0.5);
}
// Dominant photo color for photo-driven lighting; falls back to the theme accent
// when no enrichment is present (flat path).
static inline float3 photoLight(constant VisualUniforms& u) {
    if (enrichPresent(u) >= 1.0) return max(u.palette.rgb, float3(0.02));
    return normalize(u.accent.rgb + 1e-3);
}

// A slow Ken-Burns pan/zoom used by the FLAT fallback (and as the base motion of
// the enhanced path). Bounded, beat-eased — never a jump.
static inline float2 kenBurns(float2 uv, constant VisualUniforms& u, float2 anchor) {
    float t = u.time * 0.02;
    float zoom = 1.0 - (0.06 + 0.02 * sin(t)) * (0.6 + 0.4 * u.level);
    float2 pan = anchor + float2(sin(t * 0.7), cos(t * 0.55)) * 0.03;
    return (uv - pan) * zoom + pan;
}

// ---------------------------------------------------------------------------
// PARALLAX (Flagship hero) — photo as a 2.5D stage. The bass pushes the camera
// through subject-aware depth layers when enrichment is present; falls back to a
// flat Ken-Burns pan when it is absent. (ADR-0047 photo catalog: Parallax.)
// ---------------------------------------------------------------------------
fragment float4 frag_photo_parallax(VOut in [[stage_in]],
                                    constant VisualUniforms& u [[buffer(0)]],
                                    texture2d<float> uImage [[texture(0)]],
                                    texture2d<float> uPrev [[texture(1)]],
                                    sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 anchor = salientCenter(u);
    float push = depthBias(u) * (0.5 + 0.5 * u.bassImpact); // 0 on flat path
    float enhanced = step(1.0, enrichPresent(u));

    // Base Ken-Burns; the enhanced layer adds a subject-centered parallax shift
    // proportional to how near the pixel reads (depth proxy = distance from the
    // subject center, inverted). On the flat path push==0 so this is pure pan.
    float2 base = kenBurns(uv, u, anchor);
    float2 toC = uv - anchor;
    float depthHint = 1.0 - clamp(length(toC) * 1.3, 0.0, 1.0); // nearer at center
    float2 parallax = -toC * push * (0.04 + 0.10 * depthHint) * enhanced;
    float3 photo = photoSample(uImage, uSamp, base + parallax);

    // Photo-driven rim light from the dominant color, gathered toward the subject.
    float3 light = photoLight(u);
    float rim = smoothstep(0.9, 0.2, length(toC)) * (0.25 + 0.6 * u.level);
    float3 col = photo * (0.65 + 0.5 * u.level);
    col += light * rim * (0.4 + 0.6 * depthHint) * (0.6 + u.bassImpact);

    // Soft vignette + gentle feedback for a settled, gallery feel.
    col *= 1.0 - smoothstep(0.7, 1.25, length(centered(uv, u.resolution)));
    col = feedbackTrail(suppressGreenCast(col), uPrev, uSamp, uv, min(u.trailDecay, 0.55), 0.002, 0.0);
    return float4(suppressGreenCast(col), 1.0);
}

// ---------------------------------------------------------------------------
// MOSAIC (Standard) — photo rebuilt from tiles/folded facets that pulse to the
// spectrum. Absorbs the photo-Kaleidoscope concept. Overlay-capable. Uses the
// photo as texture; enrichment (palette) tints the grout, but it is NOT required
// — the flat path simply uses the theme accent. (ADR-0047 photo catalog: Mosaic.)
// ---------------------------------------------------------------------------
fragment float4 frag_photo_mosaic(VOut in [[stage_in]],
                                  constant VisualUniforms& u [[buffer(0)]],
                                  texture2d<float> uImage [[texture(0)]],
                                  texture2d<float> uPrev [[texture(1)]],
                                  sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    // Tile grid scales gently with the spectrum brightness; bounded so tiles stay
    // legible (no shimmer-strobe).
    float tiles = mix(10.0, 22.0, clamp(u.spectralCentroid, 0.0, 1.0));
    float2 g = uv * tiles;
    float2 cell = floor(g);
    float2 f = fract(g);

    // Per-tile spectrum band → a small beat-eased lift/scale of that facet.
    float band = bandAtWrapped(u, (cell.x + cell.y * 0.37) / tiles);
    float lift = band * (0.5 + 0.5 * u.beatPhase);
    float2 sampUV = (cell + 0.5) / tiles;                 // tile center sample
    sampUV += (f - 0.5) * (0.85 - 0.25 * lift) / tiles;   // facet inset (the grout)
    float3 photo = photoSample(uImage, uSamp, sampUV);

    // Grout between facets, tinted by the photo palette (or theme accent on flat).
    float2 e = abs(f - 0.5);
    float grout = 1.0 - smoothstep(0.40, 0.50, max(e.x, e.y));
    float3 tint = photoLight(u);
    float3 col = photo * (0.6 + 0.9 * lift) * grout;
    col += tint * (1.0 - grout) * (0.10 + 0.5 * band) * (0.5 + u.bassImpact);
    col *= overlaySafeMask(uv, u);
    col = feedbackTrail(suppressGreenCast(col), uPrev, uSamp, uv, min(u.trailDecay, 0.6), 0.002, 0.001);
    return float4(suppressGreenCast(col), 1.0);
}

// ---------------------------------------------------------------------------
// LIQUID (Flagship) — photo on a fluid surface rippling to the bass. Overlay-
// capable. Pure shader-only material (no enrichment required); the palette, when
// present, warms the caustic highlights. (ADR-0047 photo catalog: Liquid.)
// ---------------------------------------------------------------------------
fragment float4 frag_photo_liquid(VOut in [[stage_in]],
                                  constant VisualUniforms& u [[buffer(0)]],
                                  texture2d<float> uImage [[texture(0)]],
                                  texture2d<float> uPrev [[texture(1)]],
                                  sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = centered(uv, u.resolution);

    // Fluid displacement: layered ridged noise, bass-driven amplitude. Bounded so
    // the photo never tears; this is a ripple, not a strobe.
    float t = u.time * 0.10;
    float2 disp = float2(
        fbm(p * 2.3 + float2(t, 0.0)),
        fbm(p * 2.1 - float2(0.0, t * 0.9))
    ) - 0.5;
    disp *= 0.05 + 0.06 * u.bass + 0.04 * u.bassImpact;
    float3 photo = photoSample(uImage, uSamp, uv + disp);

    // Caustic highlights ride the ridged field; warmed by the photo palette.
    float caustic = fbmRidged(p * 3.0 + t * 1.5);
    caustic = smoothstep(0.6, 0.95, caustic) * (0.4 + 0.9 * u.treble);
    float3 light = photoLight(u);
    float3 col = photo * (0.7 + 0.4 * u.level);
    col += light * caustic * (0.8 + u.onset * 1.5);
    col *= overlaySafeMask(uv, u);
    col = feedbackTrail(suppressGreenCast(col), uPrev, uSamp, uv, min(u.trailDecay, 0.62), 0.004, 0.002);
    return float4(suppressGreenCast(col), 1.0);
}
