#ifndef VISUAL_SHARED_H
#define VISUAL_SHARED_H

// =============================================================================
// BeamfallAppleLab — shared shader contract (ADR-0047 Visual Runtime input vocabulary)
//
// Every Visual is ONE fullscreen fragment shader named `frag_<id>`, sharing this
// header and the `fullscreen_vertex` in Common.metal. The renderer builds one
// pipeline per Visual and feeds the SAME `VisualUniforms` (buffer 0), the image
// texture (texture 0), and the PREVIOUS scene frame for feedback (texture 1).
//
// HDR + POST: Visuals render into an rgba16Float scene buffer and OUTPUT LINEAR
// HDR — values MAY (and should) exceed 1.0 on cores/edges/transients so the
// shared bloom pass catches them. DO NOT call tonemap() in a Visual; the post
// chain tonemaps last. Push bright highlights to ~3–8x.
//
// FEEDBACK: sample `uPrev` (previous scene frame) and add prev * u.trailDecay to
// your colour for motion trails. A small zoom/rotate on the sample UV makes the
// trails flow. u.trailDecay is set by the renderer from the per-Visual registry.
//
// VisualUniforms MUST stay byte-identical to `UniformsHead` + spectrum tail in
// Uniforms.swift. Head is copied first; spectrum[16] follows.
// =============================================================================

#include <metal_stdlib>
using namespace metal;

constant int NBANDS = 64; // spectrum resolution (low -> high)

struct VisualUniforms {
    float2 resolution;   // drawable size in pixels
    float  time;         // seconds since launch (monotonic)
    float  dt;           // seconds since last frame

    // ---- audio features (all roughly 0..1, already sensitivity-scaled) ----
    float  amplitude;    // smoothed overall level / envelope
    float  bass;         // low-band energy
    float  mid;          // mid-band energy
    float  treble;       // high-band energy
    float  beat;         // 1.0 on a detected beat, decays toward 0
    float  beatPhase;    // 0..1 sawtooth ramp between beats (tempo position)
    float  onset;        // transient/onset flash, decays fast

    // ---- user controls ----
    float  sensitivity;  // raw user gain
    float  param0;       // per-Visual knob 0..1 (label in VisualRegistry)
    float  param1;       // per-Visual knob 0..1
    float  param2;       // per-Visual knob 0..1

    // ---- extended reactivity (added for the redesign) ----
    float  level;            // slow AGC'd loudness, stable exposure 0..1
    float  bassImpact;       // gated kick peak: snaps to 1 on a hit, decays fast
    float  spectralCentroid; // 0..1 "brightness"/timbre of the spectrum
    float  flux;             // raw spectral flux / transient strength
    float  beatCount;        // running integer beat counter (as float)
    float  trailDecay;       // feedback decay for THIS Visual (0=none .. ~0.97)

    float4 accent;       // theme accent color, rgb (a = 1)
    float4 motion;       // x=angle, y=tiltX, z=tiltY, w=perspective/depth
    float4 palette;      // x=index (-1=auto), y=cycle speed, z=auto phase, w=blend
    float4 presentation0;// x=mode, y=chrome, z=metadata, w=anchor
    float4 presentation1;// x=intensity, y=safe zone strength, z=idle seconds, w=reserved
    float4 safeRect;     // normalized overlay/content rect: minX,minY,maxX,maxY
    float4 spectrum[16]; // 64 frequency bins, low -> high, packed as float4[16]
};

// ---- dynamic-index band read, i in [0, NBANDS) ----
static inline float band(constant VisualUniforms& u, int i) {
    i = clamp(i, 0, NBANDS - 1);
    return u.spectrum[i >> 2][i & 3];
}

// average band energy over [lo, hi) of the 64-bin spectrum
static inline float bandRange(constant VisualUniforms& u, int lo, int hi) {
    lo = clamp(lo, 0, NBANDS); hi = clamp(hi, 0, NBANDS);
    float s = 0.0; int n = 0;
    for (int i = lo; i < hi; ++i) { s += band(u, i); n++; }
    return n > 0 ? s / float(n) : 0.0;
}

// smooth band read at a fractional position f in [0,1] across the spectrum
static inline float bandAt(constant VisualUniforms& u, float f) {
    float x = clamp(f, 0.0, 1.0) * float(NBANDS - 1);
    int i = int(x); float fr = x - float(i);
    return mix(band(u, i), band(u, i + 1), fr);
}

// Periodic spectrum read for spatially-wrapped domains. Unlike bandAt(fract(x)),
// this interpolates across the bin-63 -> bin-0 seam instead of snapping.
static inline float bandAtWrapped(constant VisualUniforms& u, float f) {
    float x = fract(f) * float(NBANDS);
    int i0 = int(floor(x)) % NBANDS;
    int i1 = (i0 + 1) % NBANDS;
    return mix(band(u, i0), band(u, i1), fract(x));
}

// vertex -> fragment payload (uv in 0..1, origin top-left)
struct VOut {
    float4 position [[position]];
    float2 uv;
};

// ---------------------------------------------------------------------------
// Color — IQ cosine palettes. Visuals should colour from these, NOT a flat
// accent tint. accent is woven in lightly (see accentize).
// ---------------------------------------------------------------------------
static inline float3 cosinePalette(float t, float3 a, float3 b, float3 c, float3 d) {
    return a + b * cos(6.2831853 * (c * t + d));
}

static inline float3 oklchPaletteColor(float hue, float lightness, float chroma, float gain) {
    float a = chroma * cos(6.2831853 * hue);
    float b = chroma * sin(6.2831853 * hue);
    float l_ = lightness + 0.39633778 * a + 0.21580376 * b;
    float m_ = lightness - 0.10556135 * a - 0.06385417 * b;
    float s_ = lightness - 0.08948418 * a - 1.29148555 * b;
    float l = l_ * l_ * l_;
    float m = m_ * m_ * m_;
    float s = s_ * s_ * s_;
    float3 rgb = float3(
        4.07674166 * l - 3.30771159 * m + 0.23096993 * s,
       -1.26843800 * l + 2.60975740 * m - 0.34131940 * s,
       -0.00419609 * l - 0.70341861 * m + 1.70761470 * s
    );
    return max(rgb * gain, float3(0.0));
}

// Full-spectrum neon: magenta -> orange -> green -> cyan -> violet.
static inline float3 palNeon(float t) {
    return cosinePalette(t, float3(0.50, 0.50, 0.50), float3(0.50, 0.48, 0.44),
                            float3(1.00, 1.00, 1.00), float3(0.00, 0.18, 0.38));
}
// Heat vs ice: bass orange/red, mids violet, treble cyan/white.
static inline float3 palHeatIce(float t) {
    return cosinePalette(t, float3(0.46, 0.36, 0.46), float3(0.54, 0.48, 0.54),
                            float3(1.00, 0.92, 1.18), float3(0.02, 0.30, 0.58));
}
// Beamfall night: teal/cyan base with magenta/gold accents.
static inline float3 palNight(float t) {
    return cosinePalette(t, float3(0.28, 0.34, 0.40), float3(0.32, 0.40, 0.46),
                            float3(1.10, 0.95, 1.25), float3(0.55, 0.28, 0.05));
}

// Restricted Beamfall energy palette: hot orange -> magenta -> electric cyan.
// Use this for the reference look; reserve near-white for explicit peak masks.
static inline float3 palCity(float t) {
    t = fract(t);
    float3 hot = float3(1.00, 0.18, 0.02);
    float3 amber = float3(1.00, 0.62, 0.05);
    float3 mag = float3(1.00, 0.04, 0.74);
    float3 violet = float3(0.42, 0.14, 1.00);
    float3 cyan = float3(0.00, 0.78, 1.00);
    float3 blue = float3(0.04, 0.22, 1.00);
    float3 a = mix(hot, amber, smoothstep(0.00, 0.16, t));
    float3 b = mix(a, mag, smoothstep(0.14, 0.42, t));
    float3 c = mix(b, violet, smoothstep(0.38, 0.62, t));
    float3 d = mix(c, cyan, smoothstep(0.58, 0.84, t));
    return mix(d, blue, smoothstep(0.82, 1.00, t));
}

static inline float3 palEmberIce(float t) {
    t = fract(t);
    float3 ember = float3(1.00, 0.16, 0.01);
    float3 gold = float3(1.00, 0.74, 0.08);
    float3 rose = float3(1.00, 0.06, 0.42);
    float3 ice = float3(0.06, 0.86, 1.00);
    float3 midnight = float3(0.06, 0.10, 0.38);
    float3 a = mix(ember, gold, smoothstep(0.00, 0.24, t));
    float3 b = mix(a, rose, smoothstep(0.20, 0.50, t));
    float3 c = mix(b, ice, smoothstep(0.46, 0.78, t));
    return mix(c, midnight, smoothstep(0.76, 1.00, t));
}

static inline float3 palUltraviolet(float t) {
    t = fract(t);
    float3 violet = float3(0.36, 0.08, 1.00);
    float3 mag = float3(1.00, 0.02, 0.86);
    float3 blue = float3(0.02, 0.20, 1.00);
    float3 cyan = float3(0.00, 0.78, 1.00);
    float3 red = float3(1.00, 0.08, 0.12);
    float3 a = mix(violet, mag, smoothstep(0.00, 0.28, t));
    float3 b = mix(a, blue, smoothstep(0.24, 0.56, t));
    float3 c = mix(b, cyan, smoothstep(0.52, 0.80, t));
    return mix(c, red, smoothstep(0.78, 1.00, t));
}

static inline float3 palDeepSea(float t) {
    t = fract(t);
    float3 blue = float3(0.02, 0.16, 1.00);
    float3 cyan = float3(0.00, 0.78, 1.00);
    float3 magenta = float3(0.94, 0.04, 0.92);
    float3 violet = float3(0.42, 0.08, 1.00);
    float3 amber = float3(1.00, 0.48, 0.04);
    float3 a = mix(blue, cyan, smoothstep(0.00, 0.28, t));
    float3 b = mix(a, magenta, smoothstep(0.24, 0.54, t));
    float3 c = mix(b, violet, smoothstep(0.50, 0.76, t));
    return mix(c, amber, smoothstep(0.74, 1.00, t));
}

static inline float3 palSolarFlare(float t) {
    t = fract(t);
    float3 red = float3(1.00, 0.07, 0.02);
    float3 orange = float3(1.00, 0.38, 0.02);
    float3 gold = float3(1.00, 0.86, 0.08);
    float3 pink = float3(1.00, 0.04, 0.62);
    float3 cyan = float3(0.00, 0.70, 1.00);
    float3 a = mix(red, orange, smoothstep(0.00, 0.24, t));
    float3 b = mix(a, gold, smoothstep(0.20, 0.44, t));
    float3 c = mix(b, pink, smoothstep(0.40, 0.72, t));
    return mix(c, cyan, smoothstep(0.70, 1.00, t));
}

// Heritage Apple-only palettes. They occupied indices 5/6/7 before the table was
// realigned to the documented contract (5 neon, 6 heatIce, 7 night — see
// beamfall-visual-shaders/PORTABLE-CONTRACT.md). Kept defined, deliberately
// unwired: they are not in palByIndex and no catalog entry can select them.
static inline float3 palElectricGarden(float t) {
    t = fract(t);
    float3 fern = oklchPaletteColor(0.39, 0.70, 0.17, 1.25);
    float3 lime = oklchPaletteColor(0.31, 0.82, 0.16, 1.18);
    float3 coral = oklchPaletteColor(0.05, 0.68, 0.21, 1.18);
    float3 lagoon = oklchPaletteColor(0.52, 0.72, 0.15, 1.20);
    float3 ultramarine = oklchPaletteColor(0.68, 0.62, 0.18, 1.35);
    float3 a = mix(fern, lime, smoothstep(0.00, 0.22, t));
    float3 b = mix(a, coral, smoothstep(0.18, 0.46, t));
    float3 c = mix(b, lagoon, smoothstep(0.42, 0.72, t));
    return mix(c, ultramarine, smoothstep(0.68, 1.00, t));
}

static inline float3 palCoralVerdant(float t) {
    t = fract(t);
    float3 rose = oklchPaletteColor(0.97, 0.70, 0.18, 1.22);
    float3 apricot = oklchPaletteColor(0.11, 0.78, 0.16, 1.14);
    float3 leaf = oklchPaletteColor(0.36, 0.66, 0.16, 1.24);
    float3 mint = oklchPaletteColor(0.47, 0.78, 0.13, 1.15);
    float3 wine = oklchPaletteColor(0.91, 0.56, 0.18, 1.32);
    float3 a = mix(rose, apricot, smoothstep(0.00, 0.25, t));
    float3 b = mix(a, leaf, smoothstep(0.20, 0.52, t));
    float3 c = mix(b, mint, smoothstep(0.48, 0.78, t));
    return mix(c, wine, smoothstep(0.74, 1.00, t));
}

static inline float3 palOpalPrism(float t) {
    t = fract(t);
    float3 ruby = oklchPaletteColor(0.00, 0.66, 0.20, 1.22);
    float3 topaz = oklchPaletteColor(0.16, 0.82, 0.15, 1.12);
    float3 jade = oklchPaletteColor(0.42, 0.74, 0.14, 1.18);
    float3 sky = oklchPaletteColor(0.58, 0.74, 0.14, 1.20);
    float3 orchid = oklchPaletteColor(0.84, 0.68, 0.17, 1.24);
    float3 a = mix(ruby, topaz, smoothstep(0.00, 0.24, t));
    float3 b = mix(a, jade, smoothstep(0.20, 0.50, t));
    float3 c = mix(b, sky, smoothstep(0.46, 0.76, t));
    return mix(c, orchid, smoothstep(0.72, 1.00, t));
}


// ---- palettes 8..31, ported verbatim from beamfall-visual-shaders
// shared/visual_shared.glsl (vec3 -> float3 only). Keep in lockstep.

// Miami sunset: deep purple -> hot pink -> coral -> gold -> teal.
static inline float3 palMiami(float t) {
    t = fract(t);
    float3 c = float3(0.28, 0.04, 0.55);
    c = mix(c, float3(1.00, 0.05, 0.55), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(1.00, 0.35, 0.28), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(1.00, 0.78, 0.10), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.02, 0.72, 0.80), smoothstep(0.76, 1.00, t));
    return c;
}

// Abyss: monochrome blue depth — near-black -> navy -> cobalt -> azure -> pale cyan.
static inline float3 palAbyss(float t) {
    t = fract(t);
    float3 c = float3(0.01, 0.02, 0.08);
    c = mix(c, float3(0.02, 0.08, 0.35), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(0.05, 0.25, 0.85), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(0.15, 0.55, 1.00), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.70, 0.90, 1.05), smoothstep(0.76, 1.00, t));
    return c;
}

// Sakura: muted rose — deep plum -> rose -> blush -> petal white -> soft coral.
static inline float3 palSakura(float t) {
    t = fract(t);
    float3 c = float3(0.30, 0.06, 0.28);
    c = mix(c, float3(0.85, 0.20, 0.45), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(1.00, 0.45, 0.60), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(1.00, 0.78, 0.82), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(1.00, 0.40, 0.32), smoothstep(0.76, 1.00, t));
    return c;
}

// Horizon: film teal/orange duotone — deep teal -> cyan -> cream -> amber -> burnt orange.
static inline float3 palHorizon(float t) {
    t = fract(t);
    float3 c = float3(0.02, 0.35, 0.42);
    c = mix(c, float3(0.10, 0.75, 0.75), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(1.00, 0.90, 0.70), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(1.00, 0.55, 0.12), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.75, 0.20, 0.04), smoothstep(0.76, 1.00, t));
    return c;
}

// Molten gold: warm-only — maroon -> crimson -> orange -> gold -> pale gold.
static inline float3 palMoltenGold(float t) {
    t = fract(t);
    float3 c = float3(0.30, 0.02, 0.04);
    c = mix(c, float3(0.85, 0.10, 0.05), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(1.00, 0.42, 0.03), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(1.00, 0.80, 0.10), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(1.00, 0.95, 0.55), smoothstep(0.76, 1.00, t));
    return c;
}

// Glacier: cold pale — midnight navy -> steel blue -> ice cyan -> white-blue -> pale violet.
static inline float3 palGlacier(float t) {
    t = fract(t);
    float3 c = float3(0.03, 0.07, 0.25);
    c = mix(c, float3(0.15, 0.40, 0.75), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(0.25, 0.80, 1.00), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(0.80, 0.95, 1.05), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.55, 0.55, 0.95), smoothstep(0.76, 1.00, t));
    return c;
}

// Velvet: dark wine, dwells long in shadow before dusty rose -> mauve.
static inline float3 palVelvet(float t) {
    t = fract(t);
    float3 c = float3(0.06, 0.02, 0.06);
    c = mix(c, float3(0.35, 0.03, 0.12), smoothstep(0.00, 0.45, t));
    c = mix(c, float3(0.65, 0.08, 0.22), smoothstep(0.42, 0.68, t));
    c = mix(c, float3(0.85, 0.35, 0.45), smoothstep(0.64, 0.86, t));
    c = mix(c, float3(0.55, 0.30, 0.50), smoothstep(0.84, 1.00, t));
    return c;
}

// Patina: earthy — copper -> rust -> verdigris teal -> seafoam -> bone.
static inline float3 palPatina(float t) {
    t = fract(t);
    float3 c = float3(0.45, 0.22, 0.10);
    c = mix(c, float3(0.80, 0.40, 0.15), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(0.15, 0.60, 0.52), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(0.55, 0.85, 0.75), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.95, 0.92, 0.80), smoothstep(0.76, 1.00, t));
    return c;
}

// Jade: deep pine -> jade -> seafoam -> pale mint; dwells dark.
static inline float3 palJade(float t) {
    t = fract(t);
    float3 c = float3(0.01, 0.10, 0.07);
    c = mix(c, float3(0.02, 0.38, 0.28), smoothstep(0.00, 0.40, t));
    c = mix(c, float3(0.05, 0.62, 0.45), smoothstep(0.36, 0.64, t));
    c = mix(c, float3(0.35, 0.85, 0.65), smoothstep(0.60, 0.84, t));
    c = mix(c, float3(0.80, 1.00, 0.88), smoothstep(0.82, 1.00, t));
    return c;
}

// Amber: committed honey monochrome — umber -> amber -> honey gold -> pale straw.
static inline float3 palAmber(float t) {
    t = fract(t);
    float3 c = float3(0.15, 0.06, 0.01);
    c = mix(c, float3(0.55, 0.25, 0.02), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(0.95, 0.55, 0.05), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(1.00, 0.80, 0.20), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(1.00, 0.95, 0.65), smoothstep(0.76, 1.00, t));
    return c;
}

// Smoke: near-grayscale — charcoal -> slate -> silver -> white smoke -> blue-grey.
static inline float3 palSmoke(float t) {
    t = fract(t);
    float3 c = float3(0.04, 0.04, 0.05);
    c = mix(c, float3(0.22, 0.24, 0.28), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(0.55, 0.58, 0.64), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(0.82, 0.86, 0.92), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.60, 0.66, 0.78), smoothstep(0.76, 1.00, t));
    return c;
}

// Candy: bright pastel — bubblegum pink -> lilac -> sky cyan -> mint -> lemon.
static inline float3 palCandy(float t) {
    t = fract(t);
    float3 c = float3(1.00, 0.35, 0.70);
    c = mix(c, float3(0.75, 0.50, 0.95), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(0.35, 0.75, 1.00), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(0.45, 0.95, 0.80), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(1.00, 0.90, 0.45), smoothstep(0.76, 1.00, t));
    return c;
}

// Infrared: heat ramp, white-hot held to the very top.
static inline float3 palInfrared(float t) {
    t = fract(t);
    float3 c = float3(0.12, 0.01, 0.03);
    c = mix(c, float3(0.60, 0.02, 0.06), smoothstep(0.00, 0.30, t));
    c = mix(c, float3(1.00, 0.10, 0.05), smoothstep(0.26, 0.55, t));
    c = mix(c, float3(1.00, 0.50, 0.05), smoothstep(0.50, 0.80, t));
    c = mix(c, float3(1.60, 1.30, 1.00), smoothstep(0.84, 1.00, t));
    return c;
}

// Lagoon: deep sea green -> turquoise -> aqua -> seafoam -> warm sand.
static inline float3 palLagoon(float t) {
    t = fract(t);
    float3 c = float3(0.02, 0.20, 0.28);
    c = mix(c, float3(0.02, 0.55, 0.60), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(0.10, 0.85, 0.85), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(0.55, 0.95, 0.85), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.95, 0.80, 0.50), smoothstep(0.76, 1.00, t));
    return c;
}

// Desert: muted earth — sand -> terracotta -> dusty rose -> sage -> cream.
static inline float3 palDesert(float t) {
    t = fract(t);
    float3 c = float3(0.55, 0.35, 0.18);
    c = mix(c, float3(0.85, 0.45, 0.25), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(0.95, 0.65, 0.45), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(0.75, 0.70, 0.45), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(1.00, 0.90, 0.70), smoothstep(0.76, 1.00, t));
    return c;
}

// Absinthe: violet -> electric blue -> cyan -> golden lime -> violet.
static inline float3 palAbsinthe(float t) {
    t = fract(t);
    float3 c = float3(0.25, 0.05, 0.60);
    c = mix(c, float3(0.08, 0.30, 0.95), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(0.00, 0.80, 0.80), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(0.82, 0.95, 0.12), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.45, 0.08, 0.75), smoothstep(0.76, 1.00, t));
    return c;
}

// Rose gold: muted metallic — deep mauve -> rose -> salmon -> champagne -> antique gold.
static inline float3 palRoseGold(float t) {
    t = fract(t);
    float3 c = float3(0.28, 0.08, 0.20);
    c = mix(c, float3(0.85, 0.22, 0.38), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(1.00, 0.45, 0.38), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(1.00, 0.78, 0.58), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.92, 0.62, 0.28), smoothstep(0.76, 1.00, t));
    return c;
}

// Deep space: dark-dominant nebula; narrow starlight-white burst near the top.
static inline float3 palDeepSpace(float t) {
    t = fract(t);
    float3 c = float3(0.05, 0.02, 0.15);
    c = mix(c, float3(0.15, 0.08, 0.50), smoothstep(0.00, 0.35, t));
    c = mix(c, float3(0.85, 0.25, 0.75), smoothstep(0.32, 0.62, t));
    c = mix(c, float3(0.30, 0.55, 1.00), smoothstep(0.58, 0.80, t));
    c = mix(c, float3(1.20, 1.15, 1.30), smoothstep(0.82, 0.90, t));
    return c;
}

// Tuscany: olive & wine — olive gold -> chartreuse-olive -> deep wine -> plum -> brass.
static inline float3 palTuscany(float t) {
    t = fract(t);
    float3 c = float3(0.50, 0.50, 0.12);
    c = mix(c, float3(0.75, 0.70, 0.15), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(0.45, 0.08, 0.20), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(0.70, 0.20, 0.40), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.80, 0.65, 0.25), smoothstep(0.76, 1.00, t));
    return c;
}

// Neon noir: black with a single narrow hot-magenta spike.
static inline float3 palNeonNoir(float t) {
    t = fract(t);
    float3 c = float3(0.03, 0.02, 0.06);
    c = mix(c, float3(0.12, 0.05, 0.25), smoothstep(0.28, 0.46, t));
    c = mix(c, float3(1.00, 0.05, 0.60), smoothstep(0.44, 0.52, t));
    c = mix(c, float3(0.10, 0.08, 0.30), smoothstep(0.50, 0.60, t));
    c = mix(c, float3(0.05, 0.04, 0.10), smoothstep(0.58, 0.95, t));
    return c;
}

// Porcelain: white-dominant — pale blue shadow -> white -> blush hint -> white -> grey-blue.
static inline float3 palPorcelain(float t) {
    t = fract(t);
    float3 c = float3(0.75, 0.82, 0.92);
    c = mix(c, float3(0.95, 0.97, 1.00), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(1.00, 0.92, 0.90), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(0.98, 1.00, 0.98), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.70, 0.75, 0.88), smoothstep(0.76, 1.00, t));
    return c;
}

// Rust: iron oxide — near-black -> oxide brown -> rust orange -> ochre -> dark umber.
static inline float3 palRust(float t) {
    t = fract(t);
    float3 c = float3(0.08, 0.04, 0.03);
    c = mix(c, float3(0.38, 0.12, 0.06), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(0.72, 0.28, 0.08), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(0.90, 0.55, 0.15), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.30, 0.15, 0.10), smoothstep(0.76, 1.00, t));
    return c;
}

// Cherry cola: dark cherry -> crimson -> red-orange -> cream -> caramel.
static inline float3 palCherryCola(float t) {
    t = fract(t);
    float3 c = float3(0.20, 0.04, 0.05);
    c = mix(c, float3(0.65, 0.05, 0.15), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(1.00, 0.15, 0.10), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(1.00, 0.85, 0.65), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.80, 0.45, 0.15), smoothstep(0.76, 1.00, t));
    return c;
}

// Prismatic: full rainbow sweep, high saturation.
static inline float3 palPrismatic(float t) {
    t = fract(t);
    float3 c = float3(1.00, 0.06, 0.10);
    c = mix(c, float3(1.00, 0.75, 0.05), smoothstep(0.00, 0.26, t));
    c = mix(c, float3(0.00, 0.85, 0.75), smoothstep(0.24, 0.52, t));
    c = mix(c, float3(0.10, 0.30, 1.00), smoothstep(0.48, 0.78, t));
    c = mix(c, float3(0.75, 0.10, 0.95), smoothstep(0.76, 1.00, t));
    return c;
}

static inline float3 palByIndex(float t, int idx) {
    idx = clamp(idx, 0, 31);
    if (idx == 1) return palEmberIce(t);
    if (idx == 2) return palUltraviolet(t);
    if (idx == 3) return palDeepSea(t);
    if (idx == 4) return palSolarFlare(t);
    if (idx == 5) return palNeon(t);
    if (idx == 6) return palHeatIce(t);
    if (idx == 7) return palNight(t);
    if (idx == 8) return palMiami(t);
    if (idx == 9) return palAbyss(t);
    if (idx == 10) return palSakura(t);
    if (idx == 11) return palHorizon(t);
    if (idx == 12) return palMoltenGold(t);
    if (idx == 13) return palGlacier(t);
    if (idx == 14) return palVelvet(t);
    if (idx == 15) return palPatina(t);
    if (idx == 16) return palJade(t);
    if (idx == 17) return palAmber(t);
    if (idx == 18) return palSmoke(t);
    if (idx == 19) return palCandy(t);
    if (idx == 20) return palInfrared(t);
    if (idx == 21) return palLagoon(t);
    if (idx == 22) return palDesert(t);
    if (idx == 23) return palAbsinthe(t);
    if (idx == 24) return palRoseGold(t);
    if (idx == 25) return palDeepSpace(t);
    if (idx == 26) return palTuscany(t);
    if (idx == 27) return palNeonNoir(t);
    if (idx == 28) return palPorcelain(t);
    if (idx == 29) return palRust(t);
    if (idx == 30) return palCherryCola(t);
    if (idx == 31) return palPrismatic(t);
    return palCity(t);
}

static inline float3 sanitizeColor(float3 col) {
    col = select(float3(0.0), col, isfinite(col));
    return clamp(col, float3(0.0), float3(16.0));
}

static inline float3 suppressGreenCast(float3 col) {
    col = sanitizeColor(col);
    float yellowMud = smoothstep(0.06, 0.34, min(col.r, col.g) - col.b)
                    * (1.0 - smoothstep(0.02, 0.24, abs(col.r - col.g)));
    col.b += yellowMud * 0.10 * max(col.r, col.g);
    float rb = max(col.r, col.b);
    float greenExcess = max(0.0, col.g - rb - 0.10);
    col.g -= greenExcess * 0.72;
    col.b += greenExcess * 0.40;
    col.r += greenExcess * 0.28;
    return col;
}

static inline float3 palVisual(float t, constant VisualUniforms& u) {
    if (u.palette.x >= 0.0) {
        return suppressGreenCast(palByIndex(t, int(floor(u.palette.x + 0.5))));
    }

    float phase = u.palette.z;
    // floor-mod, not fmod: fmod is truncated, so a negative phase would stay
    // negative and index outside the table. MSL has no GLSL-style mod().
    float fa = floor(phase);
    fa = fa - 32.0 * floor(fa / 32.0);   // always in [0, 32)
    int a = int(fa);
    int b = (a + 1) % 32;                // a >= 0, so % is well-defined here
    float blend = smoothstep(0.18, 0.82, fract(phase));
    float nudge = u.beatPhase * 0.02 + u.spectralCentroid * 0.13 + u.flux * 0.04 + u.bassImpact * 0.025;
    return suppressGreenCast(mix(palByIndex(t + nudge, a), palByIndex(t + nudge, b), blend));
}

static inline float3 peakWhite(float3 col, float peak) {
    return mix(col, float3(2.6, 2.35, 2.10), saturate(peak));
}

static inline float3 palRoleTrace(float t, constant VisualUniforms& u) {
    return palVisual(t + 0.03 * u.beatPhase, u);
}

static inline float3 palRoleBass(float t, constant VisualUniforms& u) {
    float3 hot = palVisual(0.03 + t * 0.18 + 0.04 * u.beatPhase, u);
    float3 body = palVisual(t + 0.10, u);
    return mix(body, hot, 0.62 + 0.25 * u.bassImpact);
}

static inline float3 palRoleTreble(float t, constant VisualUniforms& u) {
    float3 spark = palVisual(0.62 + t * 0.28 + 0.10 * u.spectralCentroid, u);
    return peakWhite(spark, u.treble * u.onset * 0.14);
}

static inline float3 palRolePeak(float t, constant VisualUniforms& u) {
    return peakWhite(palVisual(t + 0.08 * u.spectralCentroid, u), saturate(u.onset * 0.18 + u.bassImpact * 0.10));
}

static inline float3 palRoleFog(float t, constant VisualUniforms& u) {
    float3 c = palVisual(t + 0.04 * u.spectralCentroid, u);
    float grey = dot(c, float3(0.299, 0.587, 0.114));
    return mix(float3(grey * 0.62), c, 0.42);
}

static inline float3 palRoleRim(float t, constant VisualUniforms& u) {
    return mix(palVisual(t, u), palRolePeak(t + 0.18, u), 0.35 + 0.35 * u.flux);
}

// weave the theme accent into a palette colour without flattening it
static inline float3 accentize(float3 col, float4 accent, float amt) {
    float3 a = normalize(accent.rgb + 1e-3);
    return mix(col, col * (0.5 + a), clamp(amt, 0.0, 1.0));
}

static inline float luma(float3 c) { return dot(c, float3(0.299, 0.587, 0.114)); }

static inline float beatRadians(constant VisualUniforms& u, float offset) {
    return (u.beatPhase + offset) * 6.2831853;
}

static inline float beatWave(constant VisualUniforms& u, float offset) {
    return 0.5 + 0.5 * sin(beatRadians(u, offset));
}

static inline float phasePulse(constant VisualUniforms& u, float offset, float width) {
    float d = abs(fract(u.beatPhase - offset + 0.5) - 0.5);
    return exp(-d * d / max(width * width, 1e-4));
}

// ---------------------------------------------------------------------------
// Presentation hooks for Apple audio-over-visualization clients.
// mode: 0 lab, 1 blackout, 2 projector dust, 3 prism drift, 4 quiet bloom,
//       5 gallery drift, 6 spectrum stage, 7 ambient shift.
// chrome: 0 hidden, 1 visible, 2 idle dimmed.
// metadata: 0 hidden, 1 track start, 2 on demand, 3 track end, 4 idle.
// ---------------------------------------------------------------------------
static inline float presentationMode(constant VisualUniforms& u) { return u.presentation0.x; }
static inline float presentationChrome(constant VisualUniforms& u) { return u.presentation0.y; }
static inline float presentationMetadata(constant VisualUniforms& u) { return u.presentation0.z; }
static inline float presentationIntensity(constant VisualUniforms& u) { return clamp(u.presentation1.x, 0.0, 1.0); }

static inline float safeRectMask(float2 uv, float4 rect, float feather) {
    float2 lo = smoothstep(rect.xy - feather, rect.xy + feather, uv);
    float2 hi = smoothstep(1.0 - rect.zw - feather, 1.0 - rect.zw + feather, 1.0 - uv);
    return lo.x * lo.y * hi.x * hi.y;
}

static inline float overlaySafeMask(float2 uv, constant VisualUniforms& u) {
    return safeRectMask(uv, u.safeRect, 0.025);
}

// ---------------------------------------------------------------------------
// Geometry / line helpers
// ---------------------------------------------------------------------------
static inline float2x2 rot2(float a) {
    float c = cos(a), s = sin(a);
    return float2x2(c, -s, s, c);
}

// aspect-correct centered coords (~[-1,1], x scaled by aspect)
static inline float2 centered(float2 uv, float2 res) {
    float2 p = uv * 2.0 - 1.0;
    p.x *= res.x / max(res.y, 1.0);
    return p;
}

// Audio-reactive visual-space transform. Unlike post-rotation, this changes the
// coordinate system the visual is built from, so rings, grids, particles, and
// raymarched cameras rotate/tilt before their geometry is generated.
static inline float2 motionCentered(float2 uv, constant VisualUniforms& u) {
    float2 p = uv * 2.0 - 1.0;
    float aspect = u.resolution.x / max(u.resolution.y, 1.0);
    p.x *= aspect;

    float angle = u.motion.x;
    if (abs(angle) > 0.0001) {
        p = rot2(angle) * p;
    }

    float tx = u.motion.y;
    float ty = u.motion.z;
    float depth = u.motion.w;
    if (abs(tx) + abs(ty) + abs(depth) > 0.0001) {
        float y = p.y;
        float x = p.x / max(aspect, 1e-3);
        float z = 1.0 + depth + x * tx + y * ty;
        z = max(0.35, z);
        p /= z;
        p.y += ty * 0.18;
        p.x += tx * aspect * 0.10;
    }

    return p;
}

// antialiased fill of an SDF (inside d<0). uses screen-space derivative.
static inline float aaFill(float d) {
    float w = max(fwidth(d), 1e-5);
    return clamp(0.5 - d / w, 0.0, 1.0);
}
// antialiased line: bright where |d| < halfWidth, soft edge ~1px
static inline float aaLine(float d, float halfWidth) {
    float w = max(fwidth(d), 1e-5);
    return 1.0 - smoothstep(halfWidth, halfWidth + w, abs(d));
}
// repeating line at fract(phase) == 0.5. takes the CONTINUOUS phase so fwidth
// never sees the fract() 1->0 jump: aaLine(fract(p)-0.5, hw) reads fwidth ~= 1.0
// on the quad straddling the wrap and returns ~0.5 there, painting a ghost line
// at every cell boundary. always use this for cyclic/repeating rulings.
static inline float aaLineCyclic(float phase, float halfWidth) {
    float w = max(fwidth(phase), 1e-5);
    return 1.0 - smoothstep(halfWidth, halfWidth + w, abs(fract(phase) - 0.5));
}

// ---------------------------------------------------------------------------
// Feedback trails — STABLE max-based blend (never runs away like additive does).
// Call once near the end with your freshly-computed `col`:
//   col = feedbackTrail(col, uPrev, uSamp, in.uv, u.trailDecay, zoom, rotAmt);
// `zoom` (~0.0..0.03) pulls trails outward; `rotAmt` (~0.0..0.02) swirls them.
// Background stays black; bright features leave a decaying streak.
// ---------------------------------------------------------------------------
static inline float3 feedbackTrail(float3 col, texture2d<float> prev, sampler s,
                                   float2 uv, float decay, float zoom, float rotAmt) {
    float2 q = uv - 0.5;
    q = rot2(rotAmt) * q;
    q *= (1.0 - zoom);
    float3 p = suppressGreenCast(prev.sample(s, q + 0.5).rgb);
    // cap effective decay: high decay + zoom feedback floods the frame otherwise
    return suppressGreenCast(max(suppressGreenCast(col), p * clamp(decay, 0.0, 0.90)));
}

// ---------------------------------------------------------------------------
// Noise toolbox
// ---------------------------------------------------------------------------
static inline float hash11(float p) {
    p = fract(p * 0.1031);
    p *= p + 33.33;
    p *= p + p;
    return fract(p);
}
static inline float hash21(float2 p) {
    float3 p3 = fract(float3(p.xyx) * 0.1031);
    p3 += dot(p3, p3.yzx + 33.33);
    return fract((p3.x + p3.y) * p3.z);
}
static inline float2 hash22(float2 p) {
    float3 p3 = fract(float3(p.xyx) * float3(0.1031, 0.1030, 0.0973));
    p3 += dot(p3, p3.yzx + 33.33);
    return fract((p3.xx + p3.yz) * p3.zy);
}
static inline float vnoise(float2 p) {
    float2 i = floor(p), f = fract(p);
    float2 u = f * f * (3.0 - 2.0 * f);
    float a = hash21(i + float2(0, 0));
    float b = hash21(i + float2(1, 0));
    float c = hash21(i + float2(0, 1));
    float d = hash21(i + float2(1, 1));
    return mix(mix(a, b, u.x), mix(c, d, u.x), u.y);
}
static inline float fbm(float2 p) {
    float v = 0.0, a = 0.5;
    for (int i = 0; i < 5; ++i) { v += a * vnoise(p); p *= 2.0; a *= 0.5; }
    return v;
}
// ridged fbm — filament/wisp structure
static inline float fbmRidged(float2 p) {
    float v = 0.0, a = 0.5;
    for (int i = 0; i < 5; ++i) {
        float n = 1.0 - abs(2.0 * vnoise(p) - 1.0);
        v += a * n * n; p *= 2.0; a *= 0.5;
    }
    return v;
}

static inline float3 hsv2rgb(float3 c) {
    float3 p = abs(fract(c.xxx + float3(1.0, 2.0/3.0, 1.0/3.0)) * 6.0 - 3.0);
    return c.z * mix(float3(1.0), clamp(p - 1.0, 0.0, 1.0), c.y);
}

// filmic ACES-ish tonemap + gamma. Used by the POST chain only — Visuals must
// NOT call this (they output linear HDR).
static inline float3 tonemap(float3 x) {
    x = (x * (2.51 * x + 0.03)) / (x * (2.43 * x + 0.59) + 0.14);
    return pow(clamp(x, 0.0, 1.0), float3(1.0 / 2.2));
}

#endif // VISUAL_SHARED_H
