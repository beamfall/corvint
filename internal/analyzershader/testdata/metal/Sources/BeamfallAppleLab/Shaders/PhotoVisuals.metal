#include "VisualShared.h"

static inline float3 photoSample(texture2d<float> img, sampler s, float2 uv) {
    float3 c = img.sample(s, clamp(uv, float2(0.0), float2(1.0))).rgb;
    return max(c, float3(0.0));
}

static inline float photoLuma(texture2d<float> img, sampler s, float2 uv) {
    return luma(photoSample(img, s, uv));
}

static inline float ringLine(float r, float target, float w) {
    return aaLine(r - target, w);
}

static inline float rectLine(float2 p, float2 halfSize, float w) {
    float2 d = abs(p) - halfSize;
    float outside = length(max(d, 0.0));
    float inside = min(max(d.x, d.y), 0.0);
    return aaLine(outside + inside, w);
}

static inline float roundedRectFill(float2 p, float2 b, float r) {
    float2 q = abs(p) - b + r;
    return aaFill(length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - r);
}

// Photo Prism Ring — refracted artwork shards orbit a spectrum-driven ring.
fragment float4 frag_photoprism(VOut in [[stage_in]],
                                constant VisualUniforms& u [[buffer(0)]],
                                texture2d<float> uImage [[texture(0)]],
                                texture2d<float> uPrev [[texture(1)]],
                                sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    p.y *= 0.88;
    float r = length(p);
    float a = atan2(p.y, p.x);
    float t = fract(a / 6.2831853 + 0.5 + u.time * 0.025);
    float b = bandAt(u, t);
    float ringR = 0.42 + b * 0.19 + u.bassImpact * 0.035;
    float ring = ringLine(r, ringR, 0.005 + b * 0.008);
    float shard = smoothstep(0.48, 0.98, fract(t * 44.0 + u.beatPhase * 0.65));
    shard *= ringLine(r, ringR + 0.03 + 0.08 * b, 0.012);
    float2 imgUV = float2(t, 0.52 + (r - ringR) * 1.3 + 0.04 * sin(a * 5.0));
    float3 photo = photoSample(uImage, uSamp, imgUV);
    float3 col = photo * (ring * 2.4 + shard * 1.4);
    col += palVisual(t, u) * (ring * (1.2 + b * 4.0) + shard * (0.8 + u.onset * 2.2));
    col += palVisual(t + 0.22, u) * ringLine(r, 0.20 + 0.03 * u.bassImpact, 0.003) * 1.8;
    col *= smoothstep(1.35, 0.25, r);
    col = feedbackTrail(col, uPrev, uSamp, uv, min(u.trailDecay, 0.80), 0.003 + 0.005 * u.bassImpact, 0.004);
    return float4(col, 1.0);
}

// Memory Tunnel — artwork tiles pulled into a deep luminous corridor.
fragment float4 frag_memorytunnel(VOut in [[stage_in]],
                                  constant VisualUniforms& u [[buffer(0)]],
                                  texture2d<float> uImage [[texture(0)]],
                                  texture2d<float> uPrev [[texture(1)]],
                                  sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    p.x *= 0.92;
    float z = 1.0 / max(0.13, abs(p.y) + 0.22);
    float lane = p.x * z;
    float depth = z + u.time * (0.38 + u.param0) + u.beatPhase * 0.5;
    float gridX = aaLineCyclic(lane * 2.0 + 0.5, 0.025);
    float gridZ = aaLineCyclic(depth, 0.035);
    float tile = max(gridX, gridZ) * smoothstep(0.0, 0.9, z) * smoothstep(5.6, 1.0, z);
    float2 imgUV = float2(fract(lane * 0.23 + 0.5), fract(depth * 0.22));
    float3 photo = photoSample(uImage, uSamp, imgUV);
    float3 col = photo * tile * (1.0 + 2.2 * u.level);
    col += palVisual(fract(depth * 0.07 + lane * 0.05), u) * tile * (0.7 + u.bass * 2.0);
    float horizon = exp(-abs(p.y) * 13.0) * (0.7 + u.bassImpact * 2.0);
    col += palVisual(0.58 + u.spectralCentroid * 0.2, u) * horizon;
    col *= smoothstep(1.5, 0.2, length(p));
    col = feedbackTrail(col, uPrev, uSamp, uv, min(u.trailDecay, 0.86), 0.009 + 0.010 * u.bassImpact, 0.002);
    return float4(col, 1.0);
}

// Chromatic Portrait Particles — artwork breaks into spectrum-lit dots.
fragment float4 frag_photoparticles(VOut in [[stage_in]],
                                    constant VisualUniforms& u [[buffer(0)]],
                                    texture2d<float> uImage [[texture(0)]],
                                    texture2d<float> uPrev [[texture(1)]],
                                    sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    float2 grid = floor(uv * float2(84.0, 48.0));
    float2 cell = fract(uv * float2(84.0, 48.0)) - 0.5;
    float h = hash21(grid);
    float freq = bandAt(u, h);
    float2 jitter = (hash22(grid + floor(u.beatCount)) - 0.5) * (0.018 + freq * 0.045 + u.bassImpact * 0.025);
    float lum = photoLuma(uImage, uSamp, uv + jitter);
    float dotR = 0.10 + lum * 0.28 + freq * 0.34;
    float dot = smoothstep(dotR, dotR * 0.45, length(cell));
    float reveal = smoothstep(0.05, 0.95, lum + freq * 0.45 + u.onset * 0.4);
    float3 photo = photoSample(uImage, uSamp, uv + jitter);
    float3 col = mix(palVisual(h + u.time * 0.02, u), photo * 1.4, 0.45 + lum * 0.35) * dot * reveal * 2.0 * (0.85 + u.bassImpact * 0.9 + u.onset * 0.6);
    col += palVisual(h + 0.33, u) * dot * freq * 1.5;
    col *= 1.0 - smoothstep(1.2, 1.75, length(p));
    col = feedbackTrail(col, uPrev, uSamp, uv, min(u.trailDecay, 0.72), 0.002, 0.003 * u.onset);
    return float4(col, 1.0);
}

// Glass Album Monoliths — photo-textured skyline towers on a glossy floor.
fragment float4 frag_photoglass(VOut in [[stage_in]],
                                constant VisualUniforms& u [[buffer(0)]],
                                texture2d<float> uImage [[texture(0)]],
                                texture2d<float> uPrev [[texture(1)]],
                                sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    float3 col = float3(0.003, 0.006, 0.012);
    float floorGlow = smoothstep(0.95, -0.35, p.y) * smoothstep(-0.05, -0.55, p.y);
    col += palVisual(0.62, u) * floorGlow * 0.08;
    for (int i = 0; i < 34; i++) {
        float fi = float(i);
        float x = mix(-1.55, 1.55, (fi + 0.5) / 34.0);
        float w = mix(0.020, 0.052, hash11(fi * 9.1));
        float b = bandAt(u, fi / 33.0);
        float h = 0.12 + b * 0.86 + hash11(fi * 2.7) * 0.32;
        float yTop = 0.62 - h - u.bassImpact * 0.05 * hash11(fi);
        float rect = roundedRectFill(float2(p.x - x, p.y - (0.62 + yTop) * 0.5), float2(w, max(0.02, (0.62 - yTop) * 0.5)), 0.012);
        float edge = rectLine(float2(p.x - x, p.y - (0.62 + yTop) * 0.5), float2(w, max(0.02, (0.62 - yTop) * 0.5)), 0.004);
        float2 imgUV = float2(fract((p.x - x) / max(w * 2.0, 0.01) * 0.5 + 0.5), fract((p.y - yTop) / max(h, 0.01)));
        float3 photo = photoSample(uImage, uSamp, imgUV);
        col += rect * photo * (0.12 + b * 0.6);
        col += edge * palVisual(fi / 34.0 + 0.05 * u.time, u) * (0.8 + b * 3.2);
        float refl = roundedRectFill(float2(p.x - x, -p.y - (0.62 + yTop) * 0.5), float2(w, max(0.02, (0.62 - yTop) * 0.5)), 0.012);
        col += refl * palVisual(fi / 34.0, u) * 0.15 * smoothstep(1.15, 0.2, abs(p.y));
    }
    col *= 1.0 - 0.55 * smoothstep(0.7, 1.6, length(p));
    return float4(col, 1.0);
}

// Liquid Photo Bloom — artwork melts into fluid neon ink veins.
fragment float4 frag_photoliquid(VOut in [[stage_in]],
                                 constant VisualUniforms& u [[buffer(0)]],
                                 texture2d<float> uImage [[texture(0)]],
                                 texture2d<float> uPrev [[texture(1)]],
                                 sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    float2 flow = uv;
    flow += 0.08 * float2(fbm(p * 2.2 + u.time * 0.05), fbm(rot2(1.2) * p * 2.0 - u.time * 0.04));
    flow += normalize(p + 1e-4) * u.bassImpact * 0.035;
    float3 photo = photoSample(uImage, uSamp, flow);
    float veins = 0.0;
    for (int i = 0; i < 5; i++) {
        float fi = float(i);
        float n = fbmRidged(p * (1.7 + fi * 0.55) + float2(u.time * (0.07 + fi * 0.01), -u.time * 0.04));
        veins += smoothstep(0.78, 0.96, n) * (0.35 + bandAt(u, fi / 5.0) * 0.9);
    }
    float bloom = smoothstep(0.72, 0.08, length(p)) * (0.24 + u.level * 0.6);
    float3 col = photo * bloom * 0.55;
    col += palVisual(photoLuma(uImage, uSamp, flow) + u.spectralCentroid * 0.1, u) * veins * (0.6 + u.onset * 1.8);
    col += palVisual(0.08 + u.beatPhase * 0.1, u) * ringLine(length(p), 0.26 + 0.06 * u.bassImpact, 0.020) * 0.6;
    col = feedbackTrail(col, uPrev, uSamp, uv, min(u.trailDecay, 0.66), 0.004, 0.002);
    return float4(col, 1.0);
}

// Neon Photo Topography — artwork heightfield rendered as contour waves.
fragment float4 frag_phototopo(VOut in [[stage_in]],
                               constant VisualUniforms& u [[buffer(0)]],
                               texture2d<float> uImage [[texture(0)]],
                               texture2d<float> uPrev [[texture(1)]],
                               sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    p.y += 0.15;
    float perspective = 1.0 / max(0.25, p.y + 1.25);
    float2 world = float2(p.x * perspective * 1.45, (p.y + u.time * 0.10) * perspective);
    float lum = photoLuma(uImage, uSamp, world * 0.22 + 0.5 + float2(0, u.time * 0.015));
    float h = lum * 0.38 + bandAt(u, fract(world.x * 0.22 + 0.5)) * 0.30;
    float contour = aaLineCyclic((world.y + h + u.beatPhase * 0.035) * 17.0, 0.045);
    float cross = aaLineCyclic(world.x * 12.0, 0.020) * 0.35;
    float mask = smoothstep(-0.65, 0.15, p.y) * smoothstep(1.25, 0.15, p.y) * smoothstep(1.8, 0.15, abs(p.x));
    float3 col = palVisual(fract(world.x * 0.10 + lum * 0.8), u) * (contour + cross) * mask * (0.7 + u.bass * 2.1);
    col += photoSample(uImage, uSamp, world * 0.18 + 0.5) * contour * mask * 0.28;
    col = feedbackTrail(col, uPrev, uSamp, uv, min(u.trailDecay, 0.70), 0.003, 0.001);
    return float4(col, 1.0);
}

// Shattered Lens Kaleidoscope — mirrored photo shards in a chromatic lens.
fragment float4 frag_photolens(VOut in [[stage_in]],
                               constant VisualUniforms& u [[buffer(0)]],
                               texture2d<float> uImage [[texture(0)]],
                               texture2d<float> uPrev [[texture(1)]],
                               sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    float r = length(p);
    float a = atan2(p.y, p.x) + u.time * (0.05 + u.param1 * 0.12) + u.bassImpact * 0.18 + u.onset * 0.12;
    float segs = 8.0 + floor(u.param0 * 8.0);
    float sector = 6.2831853 / segs;
    float ma = abs(fract(a / sector + 0.5) - 0.5) * sector;
    float2 q = float2(cos(ma), sin(ma)) * r;
    float2 imgUV = q * (0.65 + 0.16 * sin(u.time * 0.1) - u.bassImpact * 0.06) + 0.5;
    float3 photo = photoSample(uImage, uSamp, imgUV);
    float shard = aaLineCyclic(a * segs / 6.2831853, 0.025) + aaLineCyclic(r * 7.0 + u.beatPhase, 0.035);
    float lens = smoothstep(0.95, 0.20, r);
    float3 col = photo * lens * (0.18 + u.level * 0.35 + u.beat * 0.22);
    col += palVisual(fract(a / 6.2831853 + 0.5), u) * shard * lens * (0.65 + u.treble * 2.2);
    col += peakWhite(palVisual(0.6, u), u.onset * 0.6) * ringLine(r, 0.58 + 0.04 * u.bassImpact, 0.006) * (2.0 + u.onset * 3.0);
    col *= 1.0 - smoothstep(0.98, 1.30, r);
    col = feedbackTrail(col, uPrev, uSamp, uv, min(u.trailDecay, 0.82), 0.004, 0.006);
    return float4(col, 1.0);
}

// Hologram Gallery Drift — floating photo cards in 3D space.
fragment float4 frag_photogallery(VOut in [[stage_in]],
                                  constant VisualUniforms& u [[buffer(0)]],
                                  texture2d<float> uImage [[texture(0)]],
                                  texture2d<float> uPrev [[texture(1)]],
                                  sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    float3 col = float3(0.002, 0.004, 0.010);
    for (int i = 0; i < 7; i++) {
        float fi = float(i);
        float depth = mix(0.55, 1.25, hash11(fi * 4.7));
        float2 center = float2(mix(-1.1, 1.1, hash11(fi * 2.1)), mix(-0.42, 0.42, hash11(fi * 3.3)));
        center.x += sin(u.time * (0.08 + fi * 0.01) + fi) * 0.10;
        center.y += cos(u.time * (0.06 + fi * 0.01) + fi) * 0.06;
        float2 q = (p - center) / depth;
        q = rot2((hash11(fi) - 0.5) * 0.28 + sin(u.time * 0.05 + fi) * 0.06) * q;
        float fill = roundedRectFill(q, float2(0.19, 0.12), 0.020);
        float edge = rectLine(q, float2(0.19, 0.12), 0.004);
        float2 imgUV = q / float2(0.38, 0.24) + 0.5;
        float3 photo = photoSample(uImage, uSamp, imgUV + 0.02 * hash22(float2(fi, 1.0)));
        float b = bandAt(u, fi / 6.0);
        col += photo * fill * (0.16 + b * 0.42) / depth;
        col += palVisual(fi / 7.0 + 0.05 * u.time, u) * edge * (1.1 + b * 3.0 + u.onset) / depth;
    }
    col += palVisual(0.58, u) * exp(-abs(p.y + 0.58) * 10.0) * 0.25;
    return float4(col, 1.0);
}

// Spectral Photo Curtain — photo sampled into vertical color-separated curtains.
fragment float4 frag_photocurtain(VOut in [[stage_in]],
                                  constant VisualUniforms& u [[buffer(0)]],
                                  texture2d<float> uImage [[texture(0)]],
                                  texture2d<float> uPrev [[texture(1)]],
                                  sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    float x01 = fract(uv.x + 0.012 * sin(u.time * 0.2 + uv.y * 6.0));
    float b = bandAt(u, x01);
    float wave = sin((uv.y + u.time * 0.06) * 16.0 + x01 * 18.0) * (0.025 + b * 0.09);
    float2 imgUV = float2(x01 + wave, uv.y * 0.65 + 0.18);
    float3 photo = photoSample(uImage, uSamp, imgUV);
    float stripe = aaLineCyclic(x01 * 54.0, 0.055);
    float height = smoothstep(0.92, 0.20, abs(p.y - (0.18 - b * 0.45)));
    float3 col = photo * stripe * height * (0.45 + b * 1.3);
    col += palVisual(x01 + u.spectralCentroid * 0.08, u) * stripe * height * (0.55 + b * 2.5);
    col *= smoothstep(1.35, 0.30, length(p));
    col = feedbackTrail(col, uPrev, uSamp, uv, min(u.trailDecay, 0.78), 0.002, 0.001);
    return float4(col, 1.0);
}

// Aurora Photo Veil — translucent image texture embedded in sweeping ribbons.
fragment float4 frag_photoveil(VOut in [[stage_in]],
                               constant VisualUniforms& u [[buffer(0)]],
                               texture2d<float> uImage [[texture(0)]],
                               texture2d<float> uPrev [[texture(1)]],
                               sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;
    float2 p = motionCentered(uv, u);
    float3 col = float3(0.002, 0.004, 0.010);
    for (int i = 0; i < 7; i++) {
        float fi = float(i);
        float x = p.x;
        float y = p.y - mix(-0.45, 0.35, fi / 6.0);
        float freq = bandAt(u, fi / 6.0);
        float curve = sin(x * (1.2 + fi * 0.18) + u.time * (0.10 + fi * 0.018)) * (0.10 + freq * 0.18)
                    + sin(x * 2.7 - u.time * 0.08 + fi) * 0.04;
        float ribbon = aaLine(y - curve, 0.018 + freq * 0.020);
        float body = smoothstep(0.28 + freq * 0.22, 0.0, abs(y - curve));
        float2 imgUV = float2(fract(x * 0.18 + 0.5 + fi * 0.11), fract((y - curve) * 0.9 + 0.5));
        float3 photo = photoSample(uImage, uSamp, imgUV);
        float3 c = mix(palVisual(fi / 7.0 + u.time * 0.01, u), photo * 1.2, 0.35 + photoLuma(uImage, uSamp, imgUV) * 0.25);
        col += c * (ribbon * (1.0 + freq * 3.0) + body * 0.13) * smoothstep(1.5, 0.25, abs(x));
    }
    col += palVisual(0.55, u) * exp(-abs(p.y + 0.75) * 8.0) * 0.15;
    col = feedbackTrail(col, uPrev, uSamp, uv, min(u.trailDecay, 0.70), 0.002, 0.003 * u.bassImpact);
    return float4(col, 1.0);
}
