#include "VisualShared.h"

static inline float3 sampleImage(texture2d<float> img, sampler s, float2 uv) {
    return max(img.sample(s, fract(uv)).rgb, float3(0.0));
}

static inline float3 paletteArtwork(float3 photo, float key, constant VisualUniforms& u) {
    float lum = smoothstep(0.02, 0.95, luma(photo));
    float chroma = max(photo.r, max(photo.g, photo.b)) - min(photo.r, min(photo.g, photo.b));
    float3 pal = palVisual(key + lum * 0.18 + u.spectralCentroid * 0.08, u);
    return mix(float3(lum), pal, 0.82) * (0.40 + lum * 0.95 + chroma * 0.14);
}

static inline float ring(float r, float at, float w) {
    return aaLine(r - at, w);
}

static inline float boxFill(float2 p, float2 b) {
    float2 d = abs(p) - b;
    return aaFill(length(max(d, 0.0)) + min(max(d.x, d.y), 0.0));
}

fragment float4 frag_liquid_optics(VOut in [[stage_in]],
                                   constant VisualUniforms& u [[buffer(0)]],
                                   texture2d<float> uImage [[texture(0)]],
                                   texture2d<float> uPrev [[texture(1)]],
                                   sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    float r = length(p);
    float a = atan2(p.y, p.x);
    float2 flow = uv;
    float swirl = fbm(float2(a * 0.9 + u.time * 0.035, r * 3.0 - u.time * 0.05));
    flow += normalize(p + 1e-4) * (0.045 * sin(swirl * 6.2831 + u.time * 0.22) + u.bassImpact * 0.025);
    flow += float2(fbm(p * 2.1 + u.time * 0.04), fbm(rot2(1.7) * p * 2.3 - u.time * 0.035)) * 0.075;

    float3 photo = sampleImage(uImage, uSamp, flow);
    float caustic = 0.0;
    for (int i = 0; i < 5; i++) {
        float fi = float(i);
        float n = fbmRidged(p * (2.0 + fi * 0.65) + float2(u.time * (0.06 + fi * 0.012), fi * 2.3));
        caustic += smoothstep(0.72, 0.96, n) * (0.25 + bandAt(u, fi / 5.0) * 0.9);
    }
    float aperture = ring(r, 0.28 + u.bassImpact * 0.055 + sin(a * 5.0 + u.time) * 0.006, 0.010);
    float lens = smoothstep(0.92, 0.08, r);
    float3 photoTint = paletteArtwork(photo, swirl, u);
    float3 col = photoTint * lens * 0.22;
    col += palVisual(swirl + u.spectralCentroid * 0.18, u) * caustic * (0.9 + u.onset * 2.2);
    col += peakWhite(palVisual(a / 6.2831 + 0.5, u), u.onset * 0.18) * aperture * (1.8 + u.bass * 4.0);
    col += palVisual(0.62, u) * ring(r, 0.55 + 0.04 * sin(a * 3.0 - u.time * 0.3), 0.003) * 0.9;
    col *= 1.0 - smoothstep(1.18, 1.75, r);
    col = feedbackTrail(suppressGreenCast(col), uPrev, uSamp, uv, min(u.trailDecay, 0.76), 0.006 + 0.008 * u.bassImpact, 0.006);
    col = suppressGreenCast(col);
    return float4(col, 1.0);
}

fragment float4 frag_volumetric_light(VOut in [[stage_in]],
                                      constant VisualUniforms& u [[buffer(0)]],
                                      texture2d<float> uImage [[texture(0)]],
                                      texture2d<float> uPrev [[texture(1)]],
                                      sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    float horizon = 0.54 + 0.03 * sin(u.time * 0.15);
    float3 col = float3(0.001, 0.002, 0.006);
    // NOTE: metal::pow(x, y) is undefined for x < 0 (fast-math exp2/log2 form
    // returns NaN); p.y + 0.08 is negative over most of the frame, so square
    // explicitly instead of pow(_, 2.0).
    float fogD = p.y + 0.08;
    float fog = exp(-(fogD * fogD) * 1.6) * smoothstep(1.5, 0.15, length(p));
    col += palVisual(0.58 + u.spectralCentroid * 0.2, u) * fog * 0.08;

    for (int i = 0; i < 9; i++) {
        float fi = float(i);
        float seed = hash11(fi * 11.7);
        float x = mix(-1.45, 1.45, (fi + 0.5) / 9.0);
        float sway = sin(u.time * (0.10 + seed * 0.08) + fi * 1.8) * 0.11;
        float apexY = -0.82 + seed * 0.22;
        float floorY = horizon + seed * 0.18;
        float2 apex = float2(x + sway, apexY);
        float2 dir = normalize(float2(sway * 0.6, floorY - apexY));
        float2 normal = float2(-dir.y, dir.x);
        float2 q = p - apex;
        float along = dot(q, dir);
        float across = abs(dot(q, normal));
        float coneWidth = 0.018 + along * mix(0.10, 0.19, seed);
        float cone = smoothstep(coneWidth, coneWidth * 0.18, across)
                   * smoothstep(0.0, 0.32, along)
                   * (1.0 - smoothstep(1.15, 1.95, along));
        float vein = smoothstep(0.62, 0.95, fbmRidged(float2(across * 18.0 + fi, along * 2.5 - u.time * 0.18)));
        float bandv = bandAt(u, fract(seed + 0.18 * fi));
        float3 c = palVisual(seed + fi * 0.08 + u.time * 0.012, u);
        col += c * cone * (0.16 + bandv * 0.55);
        col += peakWhite(c, vein * u.treble * 0.12) * cone * vein * (0.8 + u.onset * 2.0);
    }

    float floorLine = aaLine(p.y - horizon, 0.002) * smoothstep(1.35, 0.25, abs(p.x));
    col += palVisual(0.12, u) * floorLine * (0.35 + u.bassImpact * 1.2);
    col = feedbackTrail(col, uPrev, uSamp, uv, min(u.trailDecay, 0.62), 0.002, 0.001);
    return float4(col, 1.0);
}

fragment float4 frag_spectral_city(VOut in [[stage_in]],
                                   constant VisualUniforms& u [[buffer(0)]],
                                   texture2d<float> uImage [[texture(0)]],
                                   texture2d<float> uPrev [[texture(1)]],
                                   sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float floorY = 0.64;
    float3 col = float3(0.002, 0.004, 0.008);

    float road = smoothstep(floorY, 1.0, uv.y);
    if (road > 0.0) {
        float z = 1.0 / max(0.045, uv.y - floorY + 0.05);
        float lane = (uv.x - 0.5) * z;
        float grid = aaLine(fract(lane * 0.34) - 0.5, 0.018) + aaLine(fract(z * 0.10 - u.time * 0.22) - 0.5, 0.018);
        col += palVisual(fract(lane * 0.04 + z * 0.02), u) * grid * road * 0.26 * smoothstep(13.0, 1.2, z);
    }

    for (int i = 0; i < 72; i++) {
        float fi = float(i);
        float x = (fi + 0.5) / 72.0;
        float perspective = mix(0.50, 1.0, hash11(fi * 2.3));
        float bandv = bandAt(u, x);
        float w = mix(0.004, 0.014, perspective);
        float h = 0.07 + bandv * mix(0.28, 0.62, perspective) + hash11(fi * 5.1) * 0.12;
        float base = floorY - 0.01 * perspective;
        float2 bp = float2(uv.x - x, uv.y - (base - h * 0.5));
        float tower = boxFill(bp, float2(w, h * 0.5));
        float edge = aaLine(abs(bp.x) - w, 0.0014) * step(abs(bp.y), h * 0.52);
        float rows = aaLine(fract((uv.y - base + h) * 42.0) - 0.5, 0.03) * tower;
        float3 c = palVisual(x + u.time * 0.018, u);
        col += c * tower * (0.12 + bandv * 0.65);
        col += peakWhite(c, rows * u.treble * 0.08) * rows * (0.7 + bandv * 3.0);
        col += c * edge * (0.9 + u.onset * 2.0);

        float reflY = floorY + (base - h * 0.5 - uv.y) * 0.34;
        float refl = boxFill(float2(uv.x - x, uv.y - reflY), float2(w, h * 0.18))
                   * smoothstep(floorY, 1.0, uv.y) * (1.0 - smoothstep(floorY, 1.0, uv.y));
        col += c * refl * bandv * 0.45;
    }

    col += palVisual(0.55, u) * aaLine(uv.y - floorY, 0.0015) * 0.65;
    col = feedbackTrail(col, uPrev, uSamp, uv, min(u.trailDecay, 0.82), 0.002 + 0.004 * u.bassImpact, 0.0);
    return float4(col, 1.0);
}

fragment float4 frag_cymatics_prism(VOut in [[stage_in]],
                                    constant VisualUniforms& u [[buffer(0)]],
                                    texture2d<float> uImage [[texture(0)]],
                                    texture2d<float> uPrev [[texture(1)]],
                                    sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    p.y = (p.y + 0.18) / 0.48;
    p = rot2(u.time * 0.035 + u.beatPhase * 0.10) * p;
    float r = length(p);
    float a = atan2(p.y, p.x);
    float a01 = a / 6.2831853 + 0.5;
    float audio = bandAt(u, a01);
    float surface = 0.38 + audio * 0.22 + fbm(float2(a01 * 5.0, u.time * 0.10)) * 0.035;
    float3 col = float3(0.0);

    for (int k = -18; k <= 18; k++) {
        float fk = float(k);
        float rr = surface + fk * 0.014;
        float wire = ring(r, rr, 0.00075);
        float crest = 1.0 - min(1.0, abs(fk) / 18.0);
        float spike = smoothstep(0.70, 0.96, fbmRidged(float2(a01 * 18.0, fk * 0.1 + u.time * 0.12)));
        float3 c = palVisual(a01 + fk * 0.012 + u.spectralCentroid * 0.08, u);
        col += c * wire * (0.35 + crest * (audio * 7.0 + u.onset * 3.2));
        col += peakWhite(c, spike * crest * u.treble * 0.14) * wire * spike * crest * 2.6;
    }

    float aperture = ring(r, 0.18 + u.bassImpact * 0.035, 0.004);
    float outer = ring(r, 0.66 + u.bassImpact * 0.05, 0.004);
    col += palVisual(a01 + 0.33, u) * aperture * 2.4;
    col += palVisual(a01, u) * outer * (0.55 + u.bassImpact * 4.2 + u.beat * 2.4);
    float beam = exp(-abs(p.y) * 42.0) * smoothstep(0.92, 0.10, abs(p.x)) * (0.04 + u.bassImpact * 1.4 + u.beat * 0.9);
    col += palVisual(a01 + 0.08, u) * beam;
    col *= smoothstep(1.25, 0.42, r);
    col = feedbackTrail(col, uPrev, uSamp, uv, min(u.trailDecay, 0.86), 0.005 + 0.006 * u.bassImpact, 0.004);
    return float4(col, 1.0);
}

fragment float4 frag_event_horizon(VOut in [[stage_in]],
                                   constant VisualUniforms& u [[buffer(0)]],
                                   texture2d<float> uImage [[texture(0)]],
                                   texture2d<float> uPrev [[texture(1)]],
                                   sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    p.y *= 0.72;
    float r = length(p);
    float a = atan2(p.y, p.x) + u.time * 0.12;
    float3 col = float3(0.0);
    // pow(x, 2.0) is undefined for x < 0 in MSL (r < disk radius ⇒ negative);
    // square explicitly.
    float diskD = (r - 0.46 - 0.05 * sin(a * 2.0 + u.time * 0.5)) / 0.12;
    float disk = exp(-(diskD * diskD));
    float turbulence = fbmRidged(float2(a * 2.4 + u.time * 0.18, r * 6.0 - u.time * 0.28));
    float occlusion = smoothstep(0.26 + 0.035 * u.bassImpact, 0.31 + 0.04 * u.bassImpact, r);
    float lens = 1.0 / max(0.08, abs(r - 0.34));

    for (int i = 0; i < 24; i++) {
        float fi = float(i);
        float spokeA = a + fi * 0.2618 + sin(u.time * 0.1 + fi) * 0.10;
        float spoke = aaLine(fract(spokeA / 6.2831853 * 12.0) - 0.5, 0.018)
                    * smoothstep(0.22, 0.72, r) * smoothstep(1.35, 0.34, r);
        col += palVisual(fi / 24.0 + u.time * 0.02, u) * spoke * bandAt(u, fi / 24.0) * 0.55;
    }

    float hot = disk * smoothstep(0.42, 0.92, turbulence) * occlusion;
    col += palVisual(a / 6.2831853 + 0.5, u) * hot * (1.4 + u.level * 3.5);
    col += peakWhite(palVisual(0.10, u), u.onset * 0.22) * disk * lens * 0.10 * occlusion;
    col *= smoothstep(1.40, 0.20, length(p));
    col *= smoothstep(0.25, 0.34, r);
    col = feedbackTrail(col, uPrev, uSamp, uv, min(u.trailDecay, 0.88), 0.014 + 0.014 * u.bassImpact, 0.012);
    return float4(col, 1.0);
}

