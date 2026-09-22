#include "VisualShared.h"

// Terrain — Flagship / Pulse Array City skyline stage.
// R2 replaces the raymarched flythrough. The old heightfield placed the camera
// inside broad bright planes, which failed at frame 1. This version is a
// screen-space pseudo-3D stage: black sky, dark glass skyline, dim lower-third
// floor grid, and a thin hot orange waveform river.
// param0 Floor Speed
// param1 Skyline Height
// param2 Floor/Grid Visibility

fragment float4 frag_terrain(VOut in [[stage_in]],
                             constant VisualUniforms& u [[buffer(0)]],
                             texture2d<float> uImage [[texture(0)]],
                             texture2d<float> uPrev [[texture(1)]],
                             sampler uSamp [[sampler(0)]]) {
    float2 uv = in.uv;

    float shake = u.bassImpact * 0.004;
    float2 cameraNoise = float2(
        sin(u.time * 15.0 + 1.9 * sin(u.time * 2.5)),
        sin(u.time * 11.0 + 1.4 * cos(u.time * 2.1) + 5.0)
    );
    uv += shake * float2(cameraNoise.x, cameraNoise.y * 0.55);
    uv = clamp(uv, 0.001, 0.999);

    float horizon = 0.64;
    float heightScale = mix(0.85, 1.85, u.param1);
    float floorVis = mix(0.18, 0.58, u.param2);

    float3 base = float3(0.0);
    float3 glow = float3(0.0);

    // ---------------------------------------------------------------------
    // Floor: lower-third support only. Dim circuit lines, never HDR.
    // ---------------------------------------------------------------------
    if (uv.y > horizon) {
        float ft = (uv.y - horizon) / max(1.0 - horizon, 0.001);
        float depth = 1.0 / max(ft * 2.45 + 0.025, 0.025);
        float scroll = u.time * mix(0.12, 0.75, u.param0) + u.beatPhase * 0.36;
        float wx = (uv.x - 0.5 + sin(beatRadians(u, ft * 0.20)) * 0.0035) * depth * 1.55;
        float wz = depth + scroll;

        float cellX = wx * 0.75 + 0.5;
        float cellZ = wz * 0.34;
        float vLine = aaLineCyclic(cellX, 0.0035);
        float hLine = aaLineCyclic(cellZ, 0.0024);
        float grid = max(vLine, hLine);

        float nearHorizonFade = smoothstep(0.02, 0.20, ft);
        float farFade = exp(-ft * 3.8);
        float sideFade = smoothstep(0.78, 0.18, abs(uv.x - 0.5));
        float floorFade = nearHorizonFade * farFade * sideFade * floorVis;
        float3 floorCol = mix(palRoleFog(0.56 + 0.08 * u.spectralCentroid, u),
                              palRoleTrace(uv.x + 0.08 * u.spectralCentroid, u),
                              0.45);
        glow += floorCol * grid * floorFade * (0.32 + 0.24 * u.mid);

        // Very dim reflected ticks directly beneath active columns.
        float tickCell = fract(uv.x * 40.0);
        float tick = smoothstep(0.08, 0.18, tickCell) * smoothstep(0.92, 0.82, tickCell);
        float b = bandAt(u, uv.x);
        float tickY = step(ft, b * 0.26);
        glow += palRoleTrace(uv.x + 0.05 * u.spectralCentroid, u) * tick * tickY * floorFade * b * 0.25;
    }

    // ---------------------------------------------------------------------
    // Skyline: dark glass bodies with bright edges and caps.
    // ---------------------------------------------------------------------
    const int NCOLS = 56;
    for (int i = 0; i < NCOLS; i++) {
        float fi = float(i);
        float x = (fi + 0.5) / float(NCOLS);
        float lane = abs(uv.x - x);
        float phaseOffset = fi / float(NCOLS - 1);
        float rawB = bandAtWrapped(u, phaseOffset + u.beatPhase * 0.032);
        float seed = hash11(fi * 2.91);
        float b = max(rawB, 0.10 + 0.18 * seed);
        float phaseLift = phasePulse(u, phaseOffset, 0.19);

        float group = hash11(floor(fi / 3.0) * 2.3);
        float peak = pow(seed, 5.0) * 0.36;
        float h = pow(b, 0.68) * heightScale * (0.16 + 0.34 * group) + peak;
        h *= (1.0 + 0.16 * u.bassImpact + phaseLift * 0.075);
        h = clamp(h, 0.045, 0.60);
        float yTop = horizon - h;

        float colW = mix(0.0028, 0.0070, smoothstep(0.05, 1.0, b));
        float insideX = 1.0 - smoothstep(colW, colW + 0.002, lane);
        float insideY = step(yTop, uv.y) * step(uv.y, horizon);
        float3 c = palRoleTrace(x + 0.06 * u.spectralCentroid, u);
        c = accentize(c, u.accent, 0.08);

        float body = insideX * insideY * b * 0.035;
        float edge = aaLine(lane - colW, 0.0018) * insideY;
        float cap = aaLine(uv.y - yTop, 0.0035) * insideX;

        base += c * body;
        glow += c * edge * (0.85 + 1.75 * b);
        glow += mix(c, float3(1.0), 0.45) * cap * (1.2 + 2.9 * u.treble + 1.1 * b);

        // Tall particle shafts above active towers, like the city reference.
        float shaft = exp(-lane * lane * 22000.0)
                    * smoothstep(yTop - 0.28, yTop, uv.y)
                    * (1.0 - smoothstep(yTop, yTop + 0.05, uv.y));
        float shaftMask = step(0.72, seed) * (0.2 + 0.8 * b);
        glow += c * shaft * shaftMask * (0.35 + 0.65 * u.flux + phaseLift * 0.22);

        // Sparse upward sparks on transient flux.
        float sparkSeed = hash11(fi * 5.19 + floor(u.beatCount));
        if (sparkSeed > 0.72) {
            float life = fract(sparkSeed + u.time * (0.55 + sparkSeed) + u.beatPhase * 0.14);
            float sparkY = horizon - h * life;
            float sparkD = length(float2((uv.x - x) * 1.8, uv.y - sparkY));
            float spark = min(1.2, 0.00018 / (sparkD * sparkD + 0.00045));
            glow += palRoleTreble(x + 0.06 * u.spectralCentroid, u) * spark * u.flux * (1.0 - life);
        }
    }

    // ---------------------------------------------------------------------
    // Foreground waveform river: the visual hero, thin and hot.
    // ---------------------------------------------------------------------
    float riverBand = bandAt(u, uv.x);
    float riverY = horizon + 0.080
                 + 0.028 * (riverBand - 0.5)
                 + 0.012 * sin(uv.x * 12.0 + u.time * 1.4 + beatRadians(u, 0.0) * 0.16)
                 + 0.006 * sin(uv.x * 31.0 - u.time * 2.7 - beatRadians(u, 0.25) * 0.14);
    float riverW = 0.0038 + 0.0030 * u.bassImpact;
    float river = aaLine(uv.y - riverY, riverW);
    float riverHalo = aaLine(uv.y - riverY, riverW * 4.5) * 0.22;
    float riverWindow = smoothstep(horizon + 0.02, horizon + 0.09, riverY)
                      * smoothstep(0.96, 0.72, uv.y);
    float3 riverCol = palRoleBass(0.08, u);
    glow += riverCol * (river + riverHalo) * riverWindow * (1.8 + 2.3 * riverBand + 2.2 * u.bassImpact);

    // Thin horizon separator: narrow, not a fog bar.
    float hLine = aaLine(uv.y - horizon, 0.0018);
    glow += palRoleFog(0.60 + 0.08 * u.spectralCentroid, u) * hLine * 0.20;

    float3 trailedGlow = feedbackTrail(glow, uPrev, uSamp, in.uv,
                                       min(u.trailDecay, 0.56),
                                       0.0015 + 0.003 * u.bassImpact,
                                       0.0);
    return float4(base + trailedGlow, 1.0);
}
