#include "VisualShared.h"

// Plasma — electric arcs in a black chamber.
// Multiple analytic lightning paths, sparse HDR cores.
// param0 Arc Count, param1 Turbulence, param2 Branches

fragment float4 frag_plasma(VOut in [[stage_in]],
                            constant VisualUniforms& u [[buffer(0)]],
                            texture2d<float> uImage [[texture(0)]],
                            texture2d<float> uPrev [[texture(1)]],
                            sampler uSamp [[sampler(0)]]) {
    float2 p = motionCentered(in.uv, u);
    float arcs = mix(4.0, 9.0, u.param0);
    float turb = mix(0.08, 0.34, u.param1);
    float branches = mix(0.2, 1.4, u.param2);

    float3 glow = float3(0.0);

    for (int i = 0; i < 9; i++) {
        if (float(i) >= arcs) break;
        float fi = float(i);
        float y0 = mix(-0.62, 0.62, hash11(fi * 2.31));
        float slope = mix(-0.22, 0.22, hash11(fi * 5.17));
        float x = p.x;
        float bandv = bandAt(u, fract(fi / 9.0 + 0.15 * u.beatPhase));
        float path = y0 + slope * x
                   + sin(x * (2.3 + fi) + u.time * (0.8 + fi * 0.05)) * turb * (0.4 + bandv)
                   + (fbm(float2(x * 1.8 + fi, u.time * 0.6)) - 0.5) * turb;
        float d = p.y - path;
        float core = aaLine(d, 0.0022);
        float halo = aaLine(d, 0.020) * 0.20;
        float window = smoothstep(-1.65, -1.10, x) * (1.0 - smoothstep(1.10, 1.65, x));
        float3 c = palNeon(fi / 9.0 + 0.16 * u.spectralCentroid);
        glow += c * (core * (1.0 + bandv * 3.0) + halo * (0.4 + bandv)) * window;

        // Branches: short vertical forks on flux.
        for (int b = 0; b < 3; b++) {
            float fb = float(b);
            float bx = mix(-1.2, 1.2, hash11(fi * 8.1 + fb));
            float by = y0 + slope * bx + sin(bx * (2.3 + fi) + u.time * (0.8 + fi * 0.05)) * turb;
            float len = (0.06 + 0.20 * hash11(fi * 4.2 + fb)) * branches;
            float ddx = abs(p.x - bx);
            float iny = step(by, p.y) * step(p.y, by + len);
            float fork = (1.0 - smoothstep(0.001, 0.008, ddx)) * iny;
            glow += mix(c, float3(1.0), 0.45) * fork * u.flux * 2.2;
        }
    }

    float3 col = feedbackTrail(glow, uPrev, uSamp, in.uv,
                               min(u.trailDecay, 0.86),
                               0.002 + 0.005 * u.bassImpact,
                               0.001);
    return float4(col, 1.0);
}
