// Murmuration — Flagship — "Flock of Light"
// Analytic starling flock: 3 band-driven sub-flocks of point sprites on
// noise-perturbed orbits over a dark reflective floor. bassImpact = predator
// rupture (radial displacement, exp falloff). Feedback trails give the flock
// its smoke-like body; a dimmed mirror pass is the reflection.
// param0 = Flock Spread ; param1 = Rupture ; param2 = Wind

vec2 leaderPos(int f, float t, VisualUniforms u) {
    if (f == 0) return vec2(sin(t * 0.31) * 0.95,        -0.16 + u.bass   * 0.45 + sin(t * 0.70) * 0.08);
    if (f == 1) return vec2(sin(t * 0.23 + 2.1) * 1.05,   0.12 + u.mid    * 0.35 + cos(t * 0.50) * 0.13);
    return           vec2(sin(t * 0.41 + 4.2) * 0.80,     0.32 + u.treble * 0.32 + sin(t * 0.90 + 1.0) * 0.10);
}

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = centered(uv, u.resolution);
    p.y = -p.y; // y-up world
    float floorY = -0.62;

    float spread = mix(0.12, 0.36, u.param0);
    float rupAmt = mix(0.15, 0.85, u.param1);
    float wind   = mix(0.45, 1.60, u.param2);

    float t = u.time;

    // chorus-ish merge: slow LFO gated by level pulls flocks into one organism
    float merge = smoothstep(0.30, 0.85, 0.5 + 0.5 * sin(t * 0.045 + 1.2)) * saturate(u.level * 1.5);

    vec2 c0 = leaderPos(0, t, u);
    vec2 c1 = mix(leaderPos(1, t, u), c0, merge * 0.72);
    vec2 c2 = mix(leaderPos(2, t, u), c0, merge * 0.72);
    vec2 rupC = (c0 + c1 + c2) / 3.0;

    vec3 col = vec3(0.0);
    float wing = u.beatPhase * 6.2831853;

    for (int f = 0; f < 3; ++f) {
        vec2 cf = f == 0 ? c0 : (f == 1 ? c1 : c2);
        float bandE = f == 0 ? u.bass : (f == 1 ? u.mid : u.treble);
        vec3 flockCol;
        if (f == 0)      flockCol = palRoleBass(0.10, u);
        else if (f == 1) flockCol = palVisual(0.40 + 0.12 * u.spectralCentroid, u);
        else             flockCol = palRoleTreble(0.68, u);
        flockCol = accentize(flockCol, u.accent, 0.10);
        float ff = float(f);

        for (int i = 0; i < 40; ++i) {
            float fi = float(i);
            vec2 h = hash22(vec2(fi * 1.71 + ff * 61.3, ff * 7.7 + 3.1));

            // orbit around the sub-flock leader; radius breathes with noise so
            // birds drift between shells instead of tracing clean rings
            float radN = vnoise(vec2(fi * 0.73 + ff * 9.1, t * 0.16));
            float rad = spread * (0.30 + 1.35 * h.x) * (0.55 + 0.9 * radN) * (1.0 + bandE * 0.9);
            float sp  = (0.5 + h.y * 1.4) * wind;
            float dir = h.x > 0.5 ? 1.0 : -1.0;
            float ang = h.y * 6.2831853 + t * sp * dir + radN * 2.4;
            vec2 bp = cf + vec2(cos(ang), sin(ang) * 0.62) * rad;

            // curl-ish turbulence (two decorrelated value-noise reads)
            vec2 q = bp * 2.3 + vec2(t * 0.25 * wind, -t * 0.18 * wind) + ff * 17.0;
            bp += vec2(vnoise(q) - 0.5, vnoise(q + vec2(7.31, 3.7)) - 0.5) * (0.30 + u.mid * 0.34);

            // wingbeat shimmer, phase-locked to the beat
            bp.y += 0.014 * sin(wing + fi * 1.3 + ff * 2.1);

            // predator rupture: displace along (bird - center), exp falloff
            vec2 rv = bp - rupC;
            float rl = length(rv) + 1e-3;
            bp += (rv / rl) * u.bassImpact * rupAmt * (0.4 + 0.8 * h.x) * exp(-rl * 1.7);
            bp.y = max(bp.y, floorY + 0.015);

            float bright = 0.40 + bandE * 1.5 + u.bassImpact * 1.8 + u.beat * 0.4;
            float flick = 0.75 + 0.25 * sin(t * 9.0 + fi * 2.7 + ff * 5.0);

            // main sprite (soft HDR point)
            vec2 d = p - bp;
            float d2 = dot(d, d);
            col += flockCol * (8.5e-5 / (d2 + 5.0e-5)) * bright * flick;

            // reflection under the floor, dimmed + softened
            vec2 dm = p - vec2(bp.x, 2.0 * floorY - bp.y);
            float dm2 = dot(dm, dm);
            col += flockCol * (5.0e-5 / (dm2 + 3.0e-4)) * bright * 0.28;
        }
    }

    // faint reflective floor line + haze below horizon
    float floorGlow = exp(-abs(p.y - floorY) * 26.0);
    col += palRoleFog(0.5 + 0.1 * u.spectralCentroid, u) * floorGlow * (0.10 + 0.22 * u.amplitude);
    float below = smoothstep(floorY, floorY - 0.5, p.y);
    col *= 1.0 - below * 0.45;

    // rupture flash lifts the whole flock toward white, briefly
    col = peakWhite(col, u.bassImpact * u.bassImpact * 0.10);

    // strong trails = the murmuration's smoky body
    float decay = min(0.90, u.trailDecay + 0.07);
    float zoom = 0.010 + 0.020 * u.bassImpact;
    col = feedbackTrail(col, uv, decay, zoom, 0.0025 * sin(t * 0.21));

    return col; // LINEAR HDR
}
