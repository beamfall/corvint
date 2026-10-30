#include "VisualShared.h"

// Tunnel — Flagship "Event Horizon". A tilted polar tunnel: concentric rings fly
// toward a black core, a circular spectrum equalizer rings the void, bass throws
// radial spikes, embers stream outward, feedback zoom makes warp trails.
// param0 Flight Speed   param1 Twist   param2 Ring Detail
// HDR out — no tonemap here.
fragment float4 frag_tunnel(VOut in [[stage_in]],
                            constant VisualUniforms& u [[buffer(0)]],
                            texture2d<float> uImage [[texture(0)]],
                            texture2d<float> uPrev [[texture(1)]],
                            sampler uSamp [[sampler(0)]]) {
    float2 p = motionCentered(in.uv, u);
    p = rot2(0.20) * p;     // tilt the disc
    p.y /= 0.80;            // perspective squash

    float r = length(p);
    float a = atan2(p.y, p.x);
    float ang01 = a / 6.2831853 + 0.5;        // 0..1 around the ring

    float speed  = mix(0.3, 1.8, u.param0);
    float twist  = mix(0.0, 2.5, u.param1);
    float detail = mix(8.0, 28.0, u.param2);

    float3 col = float3(0.0);

    // radius window: keep the action in a ring band, fade to black elsewhere
    float band01 = smoothstep(0.12, 0.22, r) * (1.0 - smoothstep(0.9, 1.3, r));

    // ---- flythrough concentric rings ----
    float depth = 1.0 / max(r, 0.05) + u.time * speed + u.beatPhase * 0.6;
    float ringLine = aaLineCyclic(depth, 0.08) * band01;
    float3 ringCol = palVisual(fract(depth * 0.12 + ang01 * 0.1 + 0.03 * u.time), u);
    col += ringCol * ringLine * (0.4 + 1.2 * u.amplitude);

    // ---- circular spectrum equalizer around a base radius ----
    float baseR = 0.40;
    int   bi = int(ang01 * float(NBANDS)) % NBANDS;
    float bandH = band(u, bi);
    float tickOuter = baseR + 0.02 + bandH * 0.40;
    float tickMask  = step(baseR, r) * step(r, tickOuter);
    float cell = fract(ang01 * detail * 2.0);
    tickMask *= smoothstep(0.0, 0.12, cell) * smoothstep(1.0, 0.88, cell);
    float3 tickCol = palVisual(ang01 + 0.1 * u.spectralCentroid, u);
    tickCol = accentize(tickCol, u.accent, 0.15);
    col += tickCol * tickMask * (1.2 + bandH * 5.0);

    // base rim line at the equalizer radius
    col += palVisual(ang01, u) * aaLine(r - baseR, 0.004) * 2.5;

    // ---- bass radial spikes ----
    float spikeCell = fract(ang01 * 24.0);
    float spike = smoothstep(0.10, 0.0, abs(spikeCell - 0.5)) * step(baseR, r);
    col += palVisual(ang01 + 0.25, u) * spike * (1.0 - smoothstep(baseR, 1.5, r)) * u.bassImpact * 6.2;

    // ---- ember sparks streaming outward ----
    for (int i = 0; i < 44; i++) {
        float fi = float(i);
        float2 h = hash22(float2(fi, 7.0));
        float sa = h.x * 6.2831853 + 0.3 * sin(u.time * 0.2 + fi);
        float life = fract(h.y + u.time * (0.15 + 0.45 * h.x) + u.beatCount * 0.07);
        float sr = mix(baseR, 1.7, life);
        float2 sp = float2(cos(sa), sin(sa)) * sr;
        float d = length(p - sp);
        float spark = min(2.0, 0.0004 / (d * d + 4e-4));
        float3 sparkCol = palVisual(h.x, u);
        sparkCol = peakWhite(sparkCol, u.treble * (1.0 - life) * 0.18);
        col += sparkCol * spark * (1.0 - life) * (0.5 + u.treble * 2.2);
    }

    // ---- black event-horizon core + inner rim glow ----
    col *= smoothstep(0.0, 0.18, r);
    col += palVisual(0.72, u) * aaLine(r - 0.16, 0.015) * (0.6 + u.bass) * 1.8;

    // ---- stable feedback warp trails (subtle zoom -> content flies outward) ----
    col = feedbackTrail(col, uPrev, uSamp, in.uv,
                        min(u.trailDecay, 0.88), 0.005 + 0.012 * u.bassImpact, 0.003 * twist + 0.008 * u.beat);

    return float4(col, 1.0);
}
