// Waveform — Lite / prismatic oscilloscope threads.
// Portable port of beamfall-apple-ui Shaders/Waveform.metal (keep in sync).
// param0 Thickness ; param1 Trail Length ; param2 Trace Stack

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);

    float thickness = mix(0.0015, 0.0050, u.param0);
    int traces = int(mix(7.0, 17.0, u.param2) + 0.5);
    float trailDec = clamp(mix(0.64, 0.86, u.param1), 0.0, 0.88);

    vec3 glow = vec3(0.0);
    float baseY = 0.0;

    for (int ti = 0; ti < 17; ti++) {
        if (ti >= traces) break;
        float fi = float(ti);
        float layer = (fi - float(traces - 1) * 0.5) / max(float(traces), 1.0);
        float yOff = layer * mix(0.18, 0.62, u.param2);

        float x01 = uv.x;
        float bandA = bandAt(u, fract(x01 * 0.78 + fi * 0.037));
        float bandB = bandAt(u, fract(x01 * 0.33 + 0.22 + fi * 0.051));
        float amp = (0.11 + 0.34 * u.amplitude) * (0.45 + 0.75 * bandA);

        float wave = sin(x01 * (7.0 + fi * 0.9) + u.time * (0.75 + fi * 0.04)) * 0.36;
        wave += sin(x01 * (19.0 + fi * 0.6) - u.time * 1.7 + bandB * 2.0) * 0.16;
        wave += sin(x01 * 47.0 + u.time * 4.8) * 0.035 * u.treble;

        float y = baseY + yOff + wave * amp;
        float d = p.y - y;
        float line = aaLine(d, thickness * (1.0 + 0.7 * ((ti == traces / 2) ? 1.0 : 0.0)));
        float halo = aaLine(d, thickness * 5.5) * 0.18;

        float hue = fract(x01 * 0.86 + fi / 17.0 * 0.35 + 0.12 * u.spectralCentroid);
        vec3 c = palRoleTrace(hue, u);
        c = accentize(c, u.accent, 0.08);

        float focus = 1.0 - min(1.0, abs(layer) * 1.9);
        glow += c * (line * (1.3 + bandA * 2.5) + halo * (0.4 + bandA)) * (0.28 + 0.72 * focus);
    }

    // Beat pulse travels along the waveform as white-hot short segments.
    float pulseX = u.beatPhase;
    float pulse = exp(-(uv.x - pulseX)*(uv.x - pulseX) * 90.0);
    glow += palRoleBass(0.10, u) * pulse * u.bassImpact * aaLine(p.y, 0.055) * 2.2;

    // Sparse vertical transient pins.
    for (int i = 0; i < 22; i++) {
        float fi = float(i);
        vec2 h = hash22(vec2(fi, floor(u.beatCount) + 3.0));
        float x = h.x;
        float len = (0.05 + 0.18 * h.y) * (u.flux + 0.35 * u.treble);
        float dx = abs(uv.x - x);
        float y0 = 0.5 + 0.18 * sin(x * 8.0 + u.time);
        float inY = step(y0 - len, uv.y) * step(uv.y, y0 + len * 0.2);
        float pin = (1.0 - smoothstep(0.0015, 0.006, dx)) * inY;
        glow += palRoleTreble(x + 0.18 * u.spectralCentroid, u) * pin * (u.flux + u.onset) * 2.8;
    }

    vec3 col = feedbackTrail(glow, uv,
                             trailDec,
                             0.002 + 0.004 * u.bassImpact,
                             0.0);
    return col;
}
