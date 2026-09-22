// Rain — Standard — frequency + beat.
// Portable port of beamfall-apple-ui Shaders/Rain.metal (keep in sync).
//
// Neon spectrum rain over a glossy black floor.
// 64 deterministic drops; no texture dependency.
// param0 Density, param1 Fall Speed, param2 Reflection

vec3 visual(vec2 uv, VisualUniforms u) {
    float density = mix(28.0, 64.0, u.param0);
    float speed   = mix(0.35, 1.45, u.param1);
    float refl    = mix(0.15, 0.65, u.param2);

    float floorY = 0.72;
    vec3 glow = vec3(0.0);

    for (int i = 0; i < 64; i++) {
        if (float(i) >= density) break;
        float fi = float(i);
        vec2 h = hash22(vec2(fi * 2.71, 8.0));
        float phase = beatRadians(u, h.x);
        float x = fract(h.x + sin(phase) * 0.006);
        float bandv = bandAtWrapped(u, h.x + u.beatPhase * 0.12);
        float fall = fract(h.y + u.time * speed * (0.22 + 0.65 * h.x) + u.beatCount * 0.015 + u.beatPhase * (0.10 + 0.10 * h.y));
        float y = mix(-0.12, floorY + 0.08, fall);
        float phaseStreak = phasePulse(u, h.y, 0.22);
        float len = mix(0.035, 0.18, bandv) * (0.7 + u.bassImpact * 0.8 + phaseStreak * 0.35);
        float dx = abs(uv.x - x);
        float inDrop = (1.0 - smoothstep(0.0014, 0.0065, dx)) *
                       step(y - len, uv.y) * step(uv.y, y);
        float head = min(1.5, 0.00008 / (((uv.x - x) * 1.7)*((uv.x - x) * 1.7) + (uv.y - y)*(uv.y - y) + 0.00008));
        vec3 c = palRoleTrace(x + 0.12 * u.spectralCentroid + u.beatPhase * 0.10, u);
        glow += c * (inDrop * (0.4 + bandv * 1.8 + phaseStreak * 0.38) + head * (0.35 + u.flux + phaseStreak * 0.20));

        // Floor splash and reflection.
        float splash = exp(-(uv.y - floorY)*(uv.y - floorY) * 1800.0) * exp(-(uv.x - x)*(uv.x - x) * 900.0);
        glow += c * splash * bandv * max(u.bassImpact, phaseStreak * 0.35) * 2.2;

        if (uv.y > floorY) {
            float ry = floorY + (floorY - y) * 0.42;
            float refLine = (1.0 - smoothstep(0.001, 0.005, dx)) * aaLine(uv.y - ry, len * 0.22);
            glow += c * refLine * refl * (1.0 - smoothstep(floorY, 1.0, uv.y)) * bandv;
        }
    }

    float floorLine = aaLine(uv.y - floorY, 0.0016);
    glow += palRoleFog(0.56, u) * floorLine * 0.28;

    vec3 col = feedbackTrail(glow, uv,
                             min(u.trailDecay, 0.82),
                             0.001 + 0.003 * u.bassImpact,
                             0.0);
    return col; // LINEAR HDR — post chain tonemaps
}
