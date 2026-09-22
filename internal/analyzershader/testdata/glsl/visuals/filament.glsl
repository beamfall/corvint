// Filament — Standard — a single elastic string spanning the screen, plucked
// by the music. Displacement is a sum of standing-wave harmonics whose weights
// come from band energies (fundamental=bass … high harmonics=treble), with a
// per-beat pluck-decay envelope; glowing harmonic-node beads ride the string;
// bassImpact bows the whole span. Calm-capable: quiet audio leaves a near-still
// glowing thread.
// param0 Tension ; param1 Glow ; param2 Bead Shine

float filamentY(float x01, VisualUniforms u, float tension, float pluck) {
    float y = 0.0;
    for (int n = 1; n <= 8; ++n) {
        float fn = float(n);
        float w = bandAt(u, (fn - 1.0) / 7.0) * (1.25 - 0.09 * fn); // physical rolloff
        float om = tension * (2.0 + 1.35 * fn);
        float shape = sin(fn * 3.14159265 * x01);
        float osc = cos(om * u.time + fn * 1.7);
        float amp = w * pluck * (0.52 / fn) * (0.35 + 0.95 * u.amplitude);
        y += shape * osc * amp;
    }
    // bass bows the whole string in one arc
    y += sin(3.14159265 * x01) * 0.20 * u.bassImpact;
    return y;
}

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    float aspect = u.resolution.x / max(u.resolution.y, 1.0);
    float x01 = saturate(uv.x);

    float tension = mix(0.7, 2.0, u.param0);
    float glowAmt = mix(0.6, 1.9, u.param1);
    float beadAmt = mix(0.3, 1.7, u.param2);

    // pluck envelope: excitation decays between beats, re-armed on each beat
    float pluck = exp(-2.4 * u.beatPhase) * (0.40 + 0.60 * saturate(u.onset)) + 0.16;

    float y = filamentY(x01, u, tension, pluck);
    float d = p.y - y;

    // temporal echo: the string a moment ago, as a dim ghost (depth cue)
    float yGhost = filamentY(x01, u, tension, exp(-2.4 * fract(u.beatPhase + 0.22)) * 0.5 + 0.16);
    float dGhost = p.y - mix(yGhost, y, 0.35);

    // string velocity proxy (finite difference in x -> local slope energy)
    float y2 = filamentY(x01 + 0.004, u, tension, pluck);
    float slope = abs(y2 - y) * 250.0;
    float energy = saturate(slope * 0.35 + u.amplitude * 0.6);

    // ---- the string: hot core + inner halo + wide bloom ----
    float th = 0.0035 * (1.0 + 0.8 * u.amplitude + 0.5 * u.bassImpact);
    float core = aaLine(d, th);
    float halo = aaLine(d, th * 5.0) * 0.22;
    float bloom = exp(-d * d / (0.007 + 0.012 * u.bassImpact + 1e-5)) * 0.14;

    vec3 cTrace = palRoleTrace(x01 * 0.55 + 0.10 * u.spectralCentroid, u);
    cTrace = accentize(cTrace, u.accent, 0.08);
    vec3 col = cTrace * (core * (2.2 + 3.2 * energy) * glowAmt
                       + halo * (0.7 + 1.6 * energy)
                       + bloom * glowAmt);

    // ghost pass: dim, wider, cooler — reads as motion history
    vec3 cGhost = palRoleFog(x01 * 0.55 + 0.35, u);
    col += cGhost * aaLine(dGhost, th * 2.6) * 0.20 * (0.3 + 0.7 * u.amplitude);

    // pluck flash races along the string on the beat
    float flashX = u.beatPhase;
    float flash = exp(-(x01 - flashX)*(x01 - flashX) * 140.0) * exp(-3.0 * u.beatPhase);
    col += palRolePeak(0.5, u) * flash * core * u.onset * 3.5;

    // ---- harmonic-node beads at k/8 span positions ----
    for (int k = 1; k <= 7; ++k) {
        float fk = float(k);
        float xk = fk / 8.0;
        float yk = filamentY(xk, u, tension, pluck);
        vec2 db = vec2((uv.x - xk) * aspect * 2.0, p.y - yk);
        float r2 = dot(db, db);
        float wk = bandAt(u, (fk - 1.0) / 7.0);
        float bead = exp(-r2 / (0.00010 + 0.00030 * wk + 1e-6));
        float beadHalo = exp(-r2 / 0.0035) * 0.20;
        vec3 cb = palRoleTreble(fk / 7.0, u);
        col += cb * (bead * (1.2 + 4.5 * wk) + beadHalo * wk) * beadAmt;
    }

    // anchors: dim endpoints ground the string physically
    float endL = exp(-dot(vec2(p.x + aspect, p.y), vec2(p.x + aspect, p.y)) / 0.004);
    float endR = exp(-dot(vec2(p.x - aspect, p.y), vec2(p.x - aspect, p.y)) / 0.004);
    col += palRoleFog(0.3, u) * (endL + endR) * 0.5;

    // faint ambient fog so calm mode isn't a void
    col += palRoleFog(0.62, u) * exp(-p.y * p.y * 3.0) * 0.015 * (0.4 + 0.6 * u.amplitude);

    return feedbackTrail(col, uv, min(u.trailDecay, 0.70),
                         0.0012 + 0.0025 * u.bassImpact, 0.0);
}
