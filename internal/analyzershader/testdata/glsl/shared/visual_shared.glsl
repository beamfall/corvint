// =============================================================================
// Beamfall portable visual shared library — GLSL ES 3.00 port of
// beamfall-apple-ui/Sources/BeamfallAppleLab/Shaders/VisualShared.h
//
// KEEP FUNCTION NAMES AND SEMANTICS IDENTICAL TO VisualShared.h. A visual body
// written against this library ports to Metal by type renames only
// (vec2->float2, mat2->float2x2) — see tools/glsl2msl.mjs.
//
// Portable-subset rules for visual bodies (PORTABLE-CONTRACT.md):
//   - texture reads ONLY via IMG(uv) / PREV(uv) macros (defined by wrappers)
//   - two-arg arctangent is atan2_(y, x)  (never atan(y,x))
//   - no raw texture()/sample() calls, no derivatives beyond fwidth()
//   - visuals output LINEAR HDR; never call tonemap()
// =============================================================================

const int NBANDS = 64;

struct VisualUniforms {
    vec2  resolution;
    float time;
    float dt;

    float amplitude;
    float bass;
    float mid;
    float treble;
    float beat;
    float beatPhase;
    float onset;

    float sensitivity;
    float param0;
    float param1;
    float param2;

    float level;
    float bassImpact;
    float spectralCentroid;
    float flux;
    float beatCount;
    float trailDecay;

    vec4 accent;
    vec4 motion;
    vec4 palette;
    vec4 presentation0;
    vec4 presentation1;
    vec4 safeRect;
    vec4 spectrum[16];
};

float atan2_(float y, float x) { return atan(y, x); }
float saturate(float x) { return clamp(x, 0.0, 1.0); }
vec3 saturate3(vec3 x) { return clamp(x, vec3(0.0), vec3(1.0)); }

// ---- spectrum reads ----
float band(VisualUniforms u, int i) {
    i = clamp(i, 0, NBANDS - 1);
    return u.spectrum[i >> 2][i & 3];
}

float bandRange(VisualUniforms u, int lo, int hi) {
    lo = clamp(lo, 0, NBANDS); hi = clamp(hi, 0, NBANDS);
    float s = 0.0; int n = 0;
    for (int i = lo; i < hi; ++i) { s += band(u, i); n++; }
    return n > 0 ? s / float(n) : 0.0;
}

float bandAt(VisualUniforms u, float f) {
    float x = clamp(f, 0.0, 1.0) * float(NBANDS - 1);
    int i = int(x); float fr = x - float(i);
    return mix(band(u, i), band(u, i + 1), fr);
}

float bandAtWrapped(VisualUniforms u, float f) {
    float x = fract(f) * float(NBANDS);
    int i0 = int(floor(x)) % NBANDS;
    int i1 = (i0 + 1) % NBANDS;
    return mix(band(u, i0), band(u, i1), fract(x));
}

// ---- color: IQ cosine palettes ----
vec3 cosinePalette(float t, vec3 a, vec3 b, vec3 c, vec3 d) {
    return a + b * cos(6.2831853 * (c * t + d));
}

vec3 palNeon(float t) {
    return cosinePalette(t, vec3(0.50, 0.50, 0.50), vec3(0.50, 0.48, 0.44),
                            vec3(1.00, 1.00, 1.00), vec3(0.00, 0.18, 0.38));
}
vec3 palHeatIce(float t) {
    return cosinePalette(t, vec3(0.46, 0.36, 0.46), vec3(0.54, 0.48, 0.54),
                            vec3(1.00, 0.92, 1.18), vec3(0.02, 0.30, 0.58));
}
vec3 palNight(float t) {
    return cosinePalette(t, vec3(0.28, 0.34, 0.40), vec3(0.32, 0.40, 0.46),
                            vec3(1.10, 0.95, 1.25), vec3(0.55, 0.28, 0.05));
}

vec3 palCity(float t) {
    t = fract(t);
    vec3 hot = vec3(1.00, 0.18, 0.02);
    vec3 amber = vec3(1.00, 0.62, 0.05);
    vec3 mag = vec3(1.00, 0.04, 0.74);
    vec3 violet = vec3(0.42, 0.14, 1.00);
    vec3 cyan = vec3(0.00, 0.78, 1.00);
    vec3 blue = vec3(0.04, 0.22, 1.00);
    vec3 a = mix(hot, amber, smoothstep(0.00, 0.16, t));
    vec3 b = mix(a, mag, smoothstep(0.14, 0.42, t));
    vec3 c = mix(b, violet, smoothstep(0.38, 0.62, t));
    vec3 d = mix(c, cyan, smoothstep(0.58, 0.84, t));
    return mix(d, blue, smoothstep(0.82, 1.00, t));
}

vec3 palEmberIce(float t) {
    t = fract(t);
    vec3 ember = vec3(1.00, 0.16, 0.01);
    vec3 gold = vec3(1.00, 0.74, 0.08);
    vec3 rose = vec3(1.00, 0.06, 0.42);
    vec3 ice = vec3(0.06, 0.86, 1.00);
    vec3 midnight = vec3(0.06, 0.10, 0.38);
    vec3 a = mix(ember, gold, smoothstep(0.00, 0.24, t));
    vec3 b = mix(a, rose, smoothstep(0.20, 0.50, t));
    vec3 c = mix(b, ice, smoothstep(0.46, 0.78, t));
    return mix(c, midnight, smoothstep(0.76, 1.00, t));
}

vec3 palUltraviolet(float t) {
    t = fract(t);
    vec3 violet = vec3(0.36, 0.08, 1.00);
    vec3 mag = vec3(1.00, 0.02, 0.86);
    vec3 blue = vec3(0.02, 0.20, 1.00);
    vec3 cyan = vec3(0.00, 0.78, 1.00);
    vec3 red = vec3(1.00, 0.08, 0.12);
    vec3 a = mix(violet, mag, smoothstep(0.00, 0.28, t));
    vec3 b = mix(a, blue, smoothstep(0.24, 0.56, t));
    vec3 c = mix(b, cyan, smoothstep(0.52, 0.80, t));
    return mix(c, red, smoothstep(0.78, 1.00, t));
}

vec3 palDeepSea(float t) {
    t = fract(t);
    vec3 blue = vec3(0.02, 0.16, 1.00);
    vec3 cyan = vec3(0.00, 0.78, 1.00);
    vec3 magenta = vec3(0.94, 0.04, 0.92);
    vec3 violet = vec3(0.42, 0.08, 1.00);
    vec3 amber = vec3(1.00, 0.48, 0.04);
    vec3 a = mix(blue, cyan, smoothstep(0.00, 0.28, t));
    vec3 b = mix(a, magenta, smoothstep(0.24, 0.54, t));
    vec3 c = mix(b, violet, smoothstep(0.50, 0.76, t));
    return mix(c, amber, smoothstep(0.74, 1.00, t));
}

vec3 palSolarFlare(float t) {
    t = fract(t);
    vec3 red = vec3(1.00, 0.07, 0.02);
    vec3 orange = vec3(1.00, 0.38, 0.02);
    vec3 gold = vec3(1.00, 0.86, 0.08);
    vec3 pink = vec3(1.00, 0.04, 0.62);
    vec3 cyan = vec3(0.00, 0.70, 1.00);
    vec3 a = mix(red, orange, smoothstep(0.00, 0.24, t));
    vec3 b = mix(a, gold, smoothstep(0.20, 0.44, t));
    vec3 c = mix(b, pink, smoothstep(0.40, 0.72, t));
    return mix(c, cyan, smoothstep(0.70, 1.00, t));
}

// Miami sunset: deep purple -> hot pink -> coral -> gold -> teal.
vec3 palMiami(float t) {
    t = fract(t);
    vec3 c = vec3(0.28, 0.04, 0.55);
    c = mix(c, vec3(1.00, 0.05, 0.55), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(1.00, 0.35, 0.28), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(1.00, 0.78, 0.10), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.02, 0.72, 0.80), smoothstep(0.76, 1.00, t));
    return c;
}

// Abyss: monochrome blue depth — near-black -> navy -> cobalt -> azure -> pale cyan.
vec3 palAbyss(float t) {
    t = fract(t);
    vec3 c = vec3(0.01, 0.02, 0.08);
    c = mix(c, vec3(0.02, 0.08, 0.35), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(0.05, 0.25, 0.85), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(0.15, 0.55, 1.00), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.70, 0.90, 1.05), smoothstep(0.76, 1.00, t));
    return c;
}

// Sakura: muted rose — deep plum -> rose -> blush -> petal white -> soft coral.
vec3 palSakura(float t) {
    t = fract(t);
    vec3 c = vec3(0.30, 0.06, 0.28);
    c = mix(c, vec3(0.85, 0.20, 0.45), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(1.00, 0.45, 0.60), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(1.00, 0.78, 0.82), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(1.00, 0.40, 0.32), smoothstep(0.76, 1.00, t));
    return c;
}

// Horizon: film teal/orange duotone — deep teal -> cyan -> cream -> amber -> burnt orange.
vec3 palHorizon(float t) {
    t = fract(t);
    vec3 c = vec3(0.02, 0.35, 0.42);
    c = mix(c, vec3(0.10, 0.75, 0.75), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(1.00, 0.90, 0.70), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(1.00, 0.55, 0.12), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.75, 0.20, 0.04), smoothstep(0.76, 1.00, t));
    return c;
}

// Molten gold: warm-only — maroon -> crimson -> orange -> gold -> pale gold.
vec3 palMoltenGold(float t) {
    t = fract(t);
    vec3 c = vec3(0.30, 0.02, 0.04);
    c = mix(c, vec3(0.85, 0.10, 0.05), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(1.00, 0.42, 0.03), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(1.00, 0.80, 0.10), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(1.00, 0.95, 0.55), smoothstep(0.76, 1.00, t));
    return c;
}

// Glacier: cold pale — midnight navy -> steel blue -> ice cyan -> white-blue -> pale violet.
vec3 palGlacier(float t) {
    t = fract(t);
    vec3 c = vec3(0.03, 0.07, 0.25);
    c = mix(c, vec3(0.15, 0.40, 0.75), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(0.25, 0.80, 1.00), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(0.80, 0.95, 1.05), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.55, 0.55, 0.95), smoothstep(0.76, 1.00, t));
    return c;
}

// Velvet: dark wine, dwells long in shadow before dusty rose -> mauve.
vec3 palVelvet(float t) {
    t = fract(t);
    vec3 c = vec3(0.06, 0.02, 0.06);
    c = mix(c, vec3(0.35, 0.03, 0.12), smoothstep(0.00, 0.45, t));
    c = mix(c, vec3(0.65, 0.08, 0.22), smoothstep(0.42, 0.68, t));
    c = mix(c, vec3(0.85, 0.35, 0.45), smoothstep(0.64, 0.86, t));
    c = mix(c, vec3(0.55, 0.30, 0.50), smoothstep(0.84, 1.00, t));
    return c;
}

// Patina: earthy — copper -> rust -> verdigris teal -> seafoam -> bone.
vec3 palPatina(float t) {
    t = fract(t);
    vec3 c = vec3(0.45, 0.22, 0.10);
    c = mix(c, vec3(0.80, 0.40, 0.15), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(0.15, 0.60, 0.52), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(0.55, 0.85, 0.75), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.95, 0.92, 0.80), smoothstep(0.76, 1.00, t));
    return c;
}

// Jade: deep pine -> jade -> seafoam -> pale mint; dwells dark.
vec3 palJade(float t) {
    t = fract(t);
    vec3 c = vec3(0.01, 0.10, 0.07);
    c = mix(c, vec3(0.02, 0.38, 0.28), smoothstep(0.00, 0.40, t));
    c = mix(c, vec3(0.05, 0.62, 0.45), smoothstep(0.36, 0.64, t));
    c = mix(c, vec3(0.35, 0.85, 0.65), smoothstep(0.60, 0.84, t));
    c = mix(c, vec3(0.80, 1.00, 0.88), smoothstep(0.82, 1.00, t));
    return c;
}

// Amber: committed honey monochrome — umber -> amber -> honey gold -> pale straw.
vec3 palAmber(float t) {
    t = fract(t);
    vec3 c = vec3(0.15, 0.06, 0.01);
    c = mix(c, vec3(0.55, 0.25, 0.02), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(0.95, 0.55, 0.05), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(1.00, 0.80, 0.20), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(1.00, 0.95, 0.65), smoothstep(0.76, 1.00, t));
    return c;
}

// Smoke: near-grayscale — charcoal -> slate -> silver -> white smoke -> blue-grey.
vec3 palSmoke(float t) {
    t = fract(t);
    vec3 c = vec3(0.04, 0.04, 0.05);
    c = mix(c, vec3(0.22, 0.24, 0.28), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(0.55, 0.58, 0.64), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(0.82, 0.86, 0.92), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.60, 0.66, 0.78), smoothstep(0.76, 1.00, t));
    return c;
}

// Candy: bright pastel — bubblegum pink -> lilac -> sky cyan -> mint -> lemon.
vec3 palCandy(float t) {
    t = fract(t);
    vec3 c = vec3(1.00, 0.35, 0.70);
    c = mix(c, vec3(0.75, 0.50, 0.95), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(0.35, 0.75, 1.00), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(0.45, 0.95, 0.80), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(1.00, 0.90, 0.45), smoothstep(0.76, 1.00, t));
    return c;
}

// Infrared: heat ramp, white-hot held to the very top.
vec3 palInfrared(float t) {
    t = fract(t);
    vec3 c = vec3(0.12, 0.01, 0.03);
    c = mix(c, vec3(0.60, 0.02, 0.06), smoothstep(0.00, 0.30, t));
    c = mix(c, vec3(1.00, 0.10, 0.05), smoothstep(0.26, 0.55, t));
    c = mix(c, vec3(1.00, 0.50, 0.05), smoothstep(0.50, 0.80, t));
    c = mix(c, vec3(1.60, 1.30, 1.00), smoothstep(0.84, 1.00, t));
    return c;
}

// Lagoon: deep sea green -> turquoise -> aqua -> seafoam -> warm sand.
vec3 palLagoon(float t) {
    t = fract(t);
    vec3 c = vec3(0.02, 0.20, 0.28);
    c = mix(c, vec3(0.02, 0.55, 0.60), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(0.10, 0.85, 0.85), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(0.55, 0.95, 0.85), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.95, 0.80, 0.50), smoothstep(0.76, 1.00, t));
    return c;
}

// Desert: muted earth — sand -> terracotta -> dusty rose -> sage -> cream.
vec3 palDesert(float t) {
    t = fract(t);
    vec3 c = vec3(0.55, 0.35, 0.18);
    c = mix(c, vec3(0.85, 0.45, 0.25), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(0.95, 0.65, 0.45), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(0.75, 0.70, 0.45), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(1.00, 0.90, 0.70), smoothstep(0.76, 1.00, t));
    return c;
}

// Absinthe: violet -> electric blue -> cyan -> golden lime -> violet.
vec3 palAbsinthe(float t) {
    t = fract(t);
    vec3 c = vec3(0.25, 0.05, 0.60);
    c = mix(c, vec3(0.08, 0.30, 0.95), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(0.00, 0.80, 0.80), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(0.82, 0.95, 0.12), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.45, 0.08, 0.75), smoothstep(0.76, 1.00, t));
    return c;
}

// Rose gold: muted metallic — deep mauve -> rose -> salmon -> champagne -> antique gold.
vec3 palRoseGold(float t) {
    t = fract(t);
    vec3 c = vec3(0.28, 0.08, 0.20);
    c = mix(c, vec3(0.85, 0.22, 0.38), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(1.00, 0.45, 0.38), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(1.00, 0.78, 0.58), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.92, 0.62, 0.28), smoothstep(0.76, 1.00, t));
    return c;
}

// Deep space: dark-dominant nebula; narrow starlight-white burst near the top.
vec3 palDeepSpace(float t) {
    t = fract(t);
    vec3 c = vec3(0.05, 0.02, 0.15);
    c = mix(c, vec3(0.15, 0.08, 0.50), smoothstep(0.00, 0.35, t));
    c = mix(c, vec3(0.85, 0.25, 0.75), smoothstep(0.32, 0.62, t));
    c = mix(c, vec3(0.30, 0.55, 1.00), smoothstep(0.58, 0.80, t));
    c = mix(c, vec3(1.20, 1.15, 1.30), smoothstep(0.82, 0.90, t));
    return c;
}

// Tuscany: olive & wine — olive gold -> chartreuse-olive -> deep wine -> plum -> brass.
vec3 palTuscany(float t) {
    t = fract(t);
    vec3 c = vec3(0.50, 0.50, 0.12);
    c = mix(c, vec3(0.75, 0.70, 0.15), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(0.45, 0.08, 0.20), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(0.70, 0.20, 0.40), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.80, 0.65, 0.25), smoothstep(0.76, 1.00, t));
    return c;
}

// Neon noir: black with a single narrow hot-magenta spike.
vec3 palNeonNoir(float t) {
    t = fract(t);
    vec3 c = vec3(0.03, 0.02, 0.06);
    c = mix(c, vec3(0.12, 0.05, 0.25), smoothstep(0.28, 0.46, t));
    c = mix(c, vec3(1.00, 0.05, 0.60), smoothstep(0.44, 0.52, t));
    c = mix(c, vec3(0.10, 0.08, 0.30), smoothstep(0.50, 0.60, t));
    c = mix(c, vec3(0.05, 0.04, 0.10), smoothstep(0.58, 0.95, t));
    return c;
}

// Porcelain: white-dominant — pale blue shadow -> white -> blush hint -> white -> grey-blue.
vec3 palPorcelain(float t) {
    t = fract(t);
    vec3 c = vec3(0.75, 0.82, 0.92);
    c = mix(c, vec3(0.95, 0.97, 1.00), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(1.00, 0.92, 0.90), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(0.98, 1.00, 0.98), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.70, 0.75, 0.88), smoothstep(0.76, 1.00, t));
    return c;
}

// Rust: iron oxide — near-black -> oxide brown -> rust orange -> ochre -> dark umber.
vec3 palRust(float t) {
    t = fract(t);
    vec3 c = vec3(0.08, 0.04, 0.03);
    c = mix(c, vec3(0.38, 0.12, 0.06), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(0.72, 0.28, 0.08), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(0.90, 0.55, 0.15), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.30, 0.15, 0.10), smoothstep(0.76, 1.00, t));
    return c;
}

// Cherry cola: dark cherry -> crimson -> red-orange -> cream -> caramel.
vec3 palCherryCola(float t) {
    t = fract(t);
    vec3 c = vec3(0.20, 0.04, 0.05);
    c = mix(c, vec3(0.65, 0.05, 0.15), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(1.00, 0.15, 0.10), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(1.00, 0.85, 0.65), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.80, 0.45, 0.15), smoothstep(0.76, 1.00, t));
    return c;
}

// Prismatic: full rainbow sweep, high saturation.
vec3 palPrismatic(float t) {
    t = fract(t);
    vec3 c = vec3(1.00, 0.06, 0.10);
    c = mix(c, vec3(1.00, 0.75, 0.05), smoothstep(0.00, 0.26, t));
    c = mix(c, vec3(0.00, 0.85, 0.75), smoothstep(0.24, 0.52, t));
    c = mix(c, vec3(0.10, 0.30, 1.00), smoothstep(0.48, 0.78, t));
    c = mix(c, vec3(0.75, 0.10, 0.95), smoothstep(0.76, 1.00, t));
    return c;
}

vec3 palByIndex(float t, int idx) {
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

vec3 sanitizeColor(vec3 col) {
    // NaN/Inf guard (isfinite/select port): NaN != NaN under IEEE
    bvec3 nan = notEqual(col, col);
    col = mix(col, vec3(0.0), vec3(nan));
    return clamp(col, vec3(0.0), vec3(16.0));
}

vec3 suppressGreenCast(vec3 col) {
    col = sanitizeColor(col);
    float rb = max(col.r, col.b);
    float lum = (col.r + col.g + col.b) * 0.3333333;
    float greenCap = max(rb * 1.08, lum * 0.78);
    float excess = max(0.0, col.g - greenCap);
    col.g -= excess * 0.78;
    col.b += excess * 0.86;
    col.r += excess * 0.34;
    return col;
}

// Green-cast suppression is applied ONCE to the finished frame by the platform
// wrapper, not per palette read. Per-sample suppression was both expensive
// (~15 extra ops on every one of up to 120 palette reads per pixel, since most
// visuals call palVisual inside a loop) and wrong under accumulation:
// suppress(a) + suppress(b) != suppress(a + b) for a non-linear correction.
vec3 palVisual(float t, VisualUniforms u) {
    if (u.palette.x >= 0.0) {
        return palByIndex(t, int(floor(u.palette.x + 0.5)));
    }
    float phase = u.palette.z;
    // mod() not % : GLSL leaves integer % undefined when either operand is
    // negative, and a host may hand us a negative free-mode phase.
    int a = int(mod(floor(phase), 32.0));
    int b = int(mod(float(a + 1), 32.0));
    float blend = smoothstep(0.18, 0.82, fract(phase));
    float nudge = u.beatPhase * 0.015 + u.spectralCentroid * 0.08;
    return mix(palByIndex(t + nudge, a), palByIndex(t + nudge, b), blend);
}

vec3 peakWhite(vec3 col, float peak) {
    return mix(col, vec3(2.6, 2.35, 2.10), saturate(peak));
}

vec3 palRoleTrace(float t, VisualUniforms u) {
    return palVisual(t + 0.03 * u.beatPhase, u);
}

vec3 palRoleBass(float t, VisualUniforms u) {
    vec3 hot = palVisual(0.03 + t * 0.18 + 0.04 * u.beatPhase, u);
    vec3 body = palVisual(t + 0.10, u);
    return mix(body, hot, 0.62 + 0.25 * u.bassImpact);
}

vec3 palRoleTreble(float t, VisualUniforms u) {
    vec3 spark = palVisual(0.62 + t * 0.28 + 0.10 * u.spectralCentroid, u);
    return peakWhite(spark, u.treble * u.onset * 0.14);
}

vec3 palRolePeak(float t, VisualUniforms u) {
    return peakWhite(palVisual(t + 0.08 * u.spectralCentroid, u), saturate(u.onset * 0.18 + u.bassImpact * 0.10));
}

vec3 palRoleFog(float t, VisualUniforms u) {
    vec3 c = palVisual(t + 0.04 * u.spectralCentroid, u);
    float grey = dot(c, vec3(0.299, 0.587, 0.114));
    return mix(vec3(grey * 0.62), c, 0.42);
}

vec3 palRoleRim(float t, VisualUniforms u) {
    return mix(palVisual(t, u), palRolePeak(t + 0.18, u), 0.35 + 0.35 * u.flux);
}

vec3 accentize(vec3 col, vec4 accent, float amt) {
    vec3 a = normalize(accent.rgb + 1e-3);
    return mix(col, col * (0.5 + a), clamp(amt, 0.0, 1.0));
}

float luma(vec3 c) { return dot(c, vec3(0.299, 0.587, 0.114)); }

float beatRadians(VisualUniforms u, float offset) {
    return (u.beatPhase + offset) * 6.2831853;
}

float beatWave(VisualUniforms u, float offset) {
    return 0.5 + 0.5 * sin(beatRadians(u, offset));
}

float phasePulse(VisualUniforms u, float offset, float width) {
    float d = abs(fract(u.beatPhase - offset + 0.5) - 0.5);
    return exp(-d * d / max(width * width, 1e-4));
}

// ---- presentation hooks ----
float presentationMode(VisualUniforms u) { return u.presentation0.x; }
float presentationChrome(VisualUniforms u) { return u.presentation0.y; }
float presentationMetadata(VisualUniforms u) { return u.presentation0.z; }
float presentationIntensity(VisualUniforms u) { return clamp(u.presentation1.x, 0.0, 1.0); }

float safeRectMask(vec2 uv, vec4 rect, float feather) {
    vec2 lo = smoothstep(rect.xy - feather, rect.xy + feather, uv);
    vec2 hi = smoothstep(1.0 - rect.zw - feather, 1.0 - rect.zw + feather, 1.0 - uv);
    return lo.x * lo.y * hi.x * hi.y;
}

float overlaySafeMask(vec2 uv, VisualUniforms u) {
    return safeRectMask(uv, u.safeRect, 0.025);
}

// ---- geometry / line helpers ----
mat2 rot2(float a) {
    float c = cos(a), s = sin(a);
    return mat2(c, -s, s, c);
}

vec2 centered(vec2 uv, vec2 res) {
    vec2 p = uv * 2.0 - 1.0;
    p.x *= res.x / max(res.y, 1.0);
    return p;
}

vec2 motionCentered(vec2 uv, VisualUniforms u) {
    vec2 p = uv * 2.0 - 1.0;
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

float aaFill(float d) {
    float w = max(fwidth(d), 1e-5);
    return clamp(0.5 - d / w, 0.0, 1.0);
}
float aaLine(float d, float halfWidth) {
    float w = max(fwidth(d), 1e-5);
    return 1.0 - smoothstep(halfWidth, halfWidth + w, abs(d));
}
// Repeating line at fract(phase) == 0.5. Takes the CONTINUOUS phase so fwidth
// never sees the fract() 1->0 jump: aaLine(fract(p)-0.5, hw) reads fwidth ~= 1.0
// on the quad straddling the wrap and returns ~0.5 there, painting a ghost line
// at every cell boundary. Always use this for cyclic/repeating rulings.
float aaLineCyclic(float phase, float halfWidth) {
    float w = max(fwidth(phase), 1e-5);
    return 1.0 - smoothstep(halfWidth, halfWidth + w, abs(fract(phase) - 0.5));
}

// ---- noise toolbox ----
float hash11(float p) {
    p = fract(p * 0.1031);
    p *= p + 33.33;
    p *= p + p;
    return fract(p);
}
float hash21(vec2 p) {
    vec3 p3 = fract(vec3(p.xyx) * 0.1031);
    p3 += dot(p3, p3.yzx + 33.33);
    return fract((p3.x + p3.y) * p3.z);
}
vec2 hash22(vec2 p) {
    vec3 p3 = fract(vec3(p.xyx) * vec3(0.1031, 0.1030, 0.0973));
    p3 += dot(p3, p3.yzx + 33.33);
    return fract((p3.xx + p3.yz) * p3.zy);
}
float vnoise(vec2 p) {
    vec2 i = floor(p), f = fract(p);
    vec2 w = f * f * (3.0 - 2.0 * f);
    float a = hash21(i + vec2(0, 0));
    float b = hash21(i + vec2(1, 0));
    float c = hash21(i + vec2(0, 1));
    float d = hash21(i + vec2(1, 1));
    return mix(mix(a, b, w.x), mix(c, d, w.x), w.y);
}
float fbm(vec2 p) {
    float v = 0.0, a = 0.5;
    for (int i = 0; i < 5; ++i) { v += a * vnoise(p); p *= 2.0; a *= 0.5; }
    return v;
}
float fbmRidged(vec2 p) {
    float v = 0.0, a = 0.5;
    for (int i = 0; i < 5; ++i) {
        float n = 1.0 - abs(2.0 * vnoise(p) - 1.0);
        v += a * n * n; p *= 2.0; a *= 0.5;
    }
    return v;
}

vec3 hsv2rgb(vec3 c) {
    vec3 p = abs(fract(c.xxx + vec3(1.0, 2.0/3.0, 1.0/3.0)) * 6.0 - 3.0);
    return c.z * mix(vec3(1.0), clamp(p - 1.0, 0.0, 1.0), c.y);
}

// POST CHAIN ONLY — visuals must not call this.
vec3 tonemap(vec3 x) {
    x = (x * (2.51 * x + 0.03)) / (x * (2.43 * x + 0.59) + 0.14);
    return pow(clamp(x, vec3(0.0), vec3(1.0)), vec3(1.0 / 2.2));
}
