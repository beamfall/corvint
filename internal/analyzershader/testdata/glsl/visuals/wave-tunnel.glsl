// Wave Tunnel — Flagship — flying down a tunnel extruded from trace history.
// The wall cross-section IS the waveform: every depth ring is a past
// synthesized trace slice, wiggling the ring radius. beatPhase drives forward
// velocity lurches, onsets flash the nearest ring, spectralCentroid tilts the
// flight path. Depth fog, HDR ring cores, feedbackTrail speed streaks.
// param0 = Velocity ; param1 = Ring Density ; param2 = Wall Turbulence

// Synthesized waveform slice for depth ring `id`, sampled around angle t01.
float ringSlice(vec2 t01AndId, VisualUniforms u) {
    float t01 = t01AndId.x;
    float id = t01AndId.y;
    float seed = hash11(id * 3.71);
    // bandAtWrapped keeps the slice periodic in angle (no seam at the wrap)
    float bandA = bandAtWrapped(u, t01 + seed * 0.9);
    float bandB = bandAtWrapped(u, t01 * 2.0 + 0.22 + seed * 0.5);
    float w = (bandA - 0.45) * 0.9;
    w += sin(t01 * 6.2831853 * 3.0 + id * 1.7) * 0.28 * (0.4 + bandB);
    w += sin(t01 * 6.2831853 * 7.0 - id * 2.3 + u.time * 0.4) * 0.14;
    w += sin(t01 * 6.2831853 * 13.0 + id * 5.1) * 0.06 * u.treble;
    return w;
}

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);

    // spectralCentroid tilts the flight path; a slow weave keeps it alive.
    vec2 tilt = vec2((u.spectralCentroid - 0.45) * 0.42 + 0.10 * sin(u.time * 0.23),
                     0.09 * sin(u.time * 0.31 + 1.7));
    p -= tilt;

    float rS = max(length(p), 1e-3);
    float a = atan2_(p.y, p.x);
    float t01 = a / 6.2831853 + 0.5;

    float speed = mix(0.45, 1.9, u.param0);
    float density = mix(3.5, 8.0, u.param1);
    float turb = mix(0.10, 0.42, u.param2);

    // Forward travel: cruise + a lurch each beat (beatCount + eased beatPhase
    // is continuous, so the tunnel surges without snapping).
    float lurch = 1.0 - pow(1.0 - saturate(u.beatPhase), 3.0);
    float travel = u.time * speed + (u.beatCount + lurch) * 0.55;

    float z = 1.0 / rS;            // depth down the throat
    float q = z * density * 0.30 + travel; // ring coordinate
    float id = floor(q);
    float f = q - id;

    // Depth fog: the throat vanishes into black, walls near the eye are hot.
    float fog = smoothstep(0.03, 0.22, rS) / (1.0 + z * z * 0.16);
    float near = smoothstep(0.30, 0.95, rS); // ~1 at the nearest wall

    vec3 col = vec3(0.0);

    // ---- trace-extruded rings (this slice + the one behind it) ----
    for (int k = 0; k < 2; k++) {
        float rid = id + float(k);
        float ff = f - float(k);
        float slice = ringSlice(vec2(t01, rid), u) * turb;
        float d = ff - 0.5 + slice;
        // gaussian profile (not aaLine): spectrum-bin kinks in the slice make
        // fwidth spike, which dotted the rings at the near wall.
        float coreW = 0.030 + 0.020 * u.bassImpact;
        float core = exp(-d * d / max(coreW * coreW, 1e-5));
        float haloW = coreW * 4.5;
        float halo = exp(-d * d / max(haloW * haloW, 1e-5)) * 0.16;

        float age = fract(rid * 0.618); // stable per-ring variation
        float hue = fract(0.06 + age * 0.34 + 0.10 * u.spectralCentroid);
        vec3 rc = palRoleTrace(hue, u);
        rc = accentize(rc, u.accent, 0.07);

        float bandE = bandAtWrapped(u, t01 + age);
        float energy = (0.55 + 2.2 * bandE) * fog;
        col += rc * (core * (0.8 + 1.1 * near) + halo) * energy;

        // Onset: the nearest ring flashes white-hot.
        float nearest = smoothstep(0.60, 0.98, rS);
        col += peakWhite(rc, 0.45) * core * nearest * u.onset * 2.2;
    }

    // ---- wall body: dim spectrum wash between rings so the tube reads ----
    // circle-embedded fbm domain stays periodic in angle (no wrap seam)
    float wallBand = bandAtWrapped(u, t01 + 0.1);
    float wallShade = fbm(vec2(cos(a), sin(a)) * 1.8 + vec2(0.0, q * 0.45)) * 0.6 + 0.4;
    vec3 wallCol = palRoleFog(fract(0.5 + 0.10 * sin(a) + 0.08 * u.spectralCentroid), u);
    col += wallCol * fog * wallShade * (0.04 + 0.16 * wallBand) * (0.3 + 0.7 * near);

    // ---- longitudinal rails where the trace peaks, streaking toward us ----
    float railBand = bandAtWrapped(u, t01);
    float rail = pow(railBand, 3.0) * aaLineCyclic(t01 * 12.0 + 0.5, 0.025);
    col += palRoleTreble(fract(t01 + 0.2), u) * rail * fog * (0.3 + 1.2 * u.flux) * near;

    // ---- vanishing point: a faint distant coal, brighter on bass ----
    col += palRoleBass(0.06, u) * exp(-rS * 14.0) * (0.3 + 1.4 * u.bass) * 0.5;

    // ---- speed streaks: zooming feedback smears rings into motion lines ----
    vec3 outc = feedbackTrail(col, uv,
                              min(u.trailDecay, 0.86),
                              0.010 + 0.018 * u.bassImpact + 0.006 * speed,
                              0.0015);
    return outc; // LINEAR HDR
}
