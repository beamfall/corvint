// Ink — Flagship — "Luminous Ink in Dark Water"
// No fluid sim: the feedback buffer IS the fluid. Each frame samples PREV
// through a curl-noise displaced uv (advection), decays it (energy bleed),
// and injects analytic dye splats on onsets whose color follows the spectral
// centroid. bassImpact adds an upward advection bias (buoyancy plume).
// param0 = Flow ; param1 = Ink Budget ; param2 = Buoyancy

float inkNs(vec2 p) {
    return vnoise(p) + 0.5 * vnoise(p * 2.13 + 7.7);
}

vec2 inkCurl(vec2 p) {
    float e = 0.05;
    float a = inkNs(p + vec2(0.0, e)) - inkNs(p - vec2(0.0, e));
    float b = inkNs(p + vec2(e, 0.0)) - inkNs(p - vec2(e, 0.0));
    return vec2(a, -b) / (2.0 * e);
}

vec3 visual(vec2 uv, VisualUniforms u) {
    float aspect = u.resolution.x / max(u.resolution.y, 1.0);
    float flow   = mix(0.45, 1.70, u.param0);
    float budget = mix(0.55, 1.90, u.param1);
    float buoy   = mix(0.25, 1.30, u.param2);

    // ---- advection: curl-noise velocity field in uv space (y-down screen) ----
    vec2 q = uv * vec2(aspect, 1.0) * 3.1 + vec2(0.0, u.time * 0.045);
    vec2 vel = inkCurl(q) * (0.55 + u.mid * 1.1);
    vel += inkCurl(q * 2.7 + 31.4) * 0.35 * (0.3 + u.treble);

    // beat swirl around center (vorticity "turns over" on the beat)
    vec2 rc = (uv - 0.5) * vec2(aspect, 1.0);
    vel += vec2(-rc.y, rc.x) * (u.beat * 1.4 + 0.15) * phasePulse(u, 0.0, 0.25);

    // buoyancy: sample from below => dye rises (uv.y grows downward)
    float rise = (0.0022 + 0.0060 * u.bassImpact + 0.0018 * u.bass) * buoy;

    vec2 sampleUV = uv + vel * 0.0045 * flow + vec2(0.0, rise);
    vec3 prev = PREV(sampleUV).rgb;

    // energy must decay: base bleed + whiteout guard at high luminance
    float pl = luma(prev);
    float keep = 0.978 - 0.10 * smoothstep(2.2, 6.0, pl);
    prev = max(prev * keep - 0.0016, vec3(0.0));

    // ---- dye injection ----
    vec3 dye = vec3(0.0);
    float k = u.beatCount * 2.0 + step(0.5, u.beatPhase); // splat site key
    for (int j = 0; j < 3; ++j) {
        float fj = float(j);
        vec2 h = hash22(vec2(k * 0.371 + fj * 13.7, fj * 5.13 + 2.7));
        vec2 c = vec2(0.16 + 0.68 * h.x, 0.22 + 0.48 * h.y);
        vec2 d = (uv - c) * vec2(aspect, 1.0);
        float g = exp(-dot(d, d) / (0.0016 + 0.0045 * u.flux + 1e-5));
        vec3 dcol = palVisual(u.spectralCentroid * 0.85 + fj * 0.14 + h.y * 0.10, u);
        dcol = peakWhite(dcol, u.onset * u.treble * 0.10);
        dye += dcol * g * u.onset * budget * 2.0;
    }

    // kick plume: hot charge near the floor, buoyancy carries it up
    vec2 db = (uv - vec2(0.5 + 0.12 * sin(u.time * 0.7), 0.90)) * vec2(aspect, 1.0);
    vec3 bassCol = palRoleBass(0.06 + 0.05 * u.spectralCentroid, u);
    dye += bassCol * exp(-dot(db, db) / 0.0045) * u.bassImpact * budget * 2.6;

    vec3 col = prev + dye;

    // faint caustic floor light in the empty water (self-limiting)
    float caustic = fbm(uv * vec2(aspect, 1.0) * 6.0 + u.time * 0.03);
    col += palRoleFog(0.62, u) * caustic * 0.006 * (1.0 - saturate(pl * 1.5)) * (0.4 + u.amplitude);

    // subtle chroma shear at plume fronts: brighten dye edges
    float dl = luma(dye);
    col += dye * dl * 0.35;

    return col; // LINEAR HDR — this exact value is next frame's fluid state
}
