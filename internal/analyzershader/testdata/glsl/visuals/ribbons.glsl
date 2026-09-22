// Ribbons — layered silk-light ribbons with spectral edge highlights.
// Portable port of beamfall-apple-ui Shaders/Ribbons.metal (keep in sync).
// param0 Flow ; param1 Width ; param2 Layers

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    p = rot2(-0.18) * p;
    float flow = mix(0.12, 0.55, u.param0);
    float width = mix(0.014, 0.050, u.param1);
    int layers = int(mix(5.0, 12.0, u.param2) + 0.5);

    vec3 base = vec3(0.0);
    vec3 glow = vec3(0.0);

    for (int i = 0; i < 12; i++) {
        if (i >= layers) break;
        float fi = float(i);
        float y0 = mix(-0.70, 0.70, (fi + 0.5) / float(layers));
        float bandv = bandAtWrapped(u, fi / float(layers) + p.x * 0.08);
        float curve = y0
                    + sin(p.x * (1.2 + fi * 0.11) + u.time * flow + fi) * (0.11 + bandv * 0.16)
                    + sin(p.x * (2.4 + fi * 0.07) - u.time * flow * 0.7) * 0.045;
        float d = p.y - curve;
        float body = aaLine(d, width * 0.65) * (0.08 + bandv * 0.20);
        float edge = max(aaLine(d - width, 0.0025), aaLine(d + width, 0.0025));
        vec3 c = palVisual(fi / 12.0 + 0.12 * u.spectralCentroid + 0.06 * p.x, u);
        c = accentize(c, u.accent, 0.10);
        base += c * body * 0.04;
        glow += c * edge * (0.30 + bandv * 1.9 + max(0.0, u.level - 0.45) * 0.9 + u.onset * 1.2);

        float crestPhase = p.x * 0.35 + u.beatPhase + fi * 0.07;
        float crestWindow = pow(0.5 + 0.5 * cos(6.2831853 * crestPhase), 8.0);
        float crest = aaLine(d, 0.004) * crestWindow;
        glow += peakWhite(c, 0.24) * crest * (u.bassImpact + u.onset * 0.8) * 1.7;
    }

    vec3 trailedGlow = feedbackTrail(glow, uv,
                                     min(u.trailDecay, 0.82),
                                     0.0015 + 0.003 * u.bassImpact,
                                     0.0008);
    return base + trailedGlow;
}
