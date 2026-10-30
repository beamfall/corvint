// Cymatics/Prism — Standard / frequency + beat.
// Portable port of frag_cymatics_prism in beamfall-apple-ui
// Shaders/CuratedRuntime.metal (keep in sync).
// A prismatic cymatics ring with audio-carved contours.
// param0 = Density ; param1 = Displace ; param2 = Glow

float ring(float r, float at, float w) {
    return aaLine(r - at, w);
}

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    p.y = (p.y + 0.18) / 0.48;
    p = rot2(u.time * 0.035 + u.beatPhase * 0.10) * p;
    float r = length(p);
    float a = atan2_(p.y, p.x);
    float a01 = a / 6.2831853 + 0.5;
    float audio = bandAtWrapped(u, a01);
    float surface = 0.38 + audio * 0.22 + fbm(vec2(a01 * 5.0, u.time * 0.10)) * 0.035;
    vec3 col = vec3(0.0);

    for (int k = -18; k <= 18; k++) {
        float fk = float(k);
        float rr = surface + fk * 0.014;
        float wire = ring(r, rr, 0.00075);
        float crest = 1.0 - min(1.0, abs(fk) / 18.0);
        float spike = smoothstep(0.70, 0.96, fbmRidged(vec2(a01 * 18.0, fk * 0.1 + u.time * 0.12)));
        vec3 c = palVisual(a01 + fk * 0.012 + u.spectralCentroid * 0.08, u);
        col += c * wire * (0.35 + crest * (audio * 7.0 + u.onset * 3.2));
        col += peakWhite(c, spike * crest * u.treble * 0.14) * wire * spike * crest * 2.6;
    }

    float aperture = ring(r, 0.18 + u.bassImpact * 0.035, 0.004);
    float outer = ring(r, 0.66 + u.bassImpact * 0.05, 0.004);
    col += palVisual(a01 + 0.33, u) * aperture * 2.4;
    col += palVisual(a01, u) * outer * (0.55 + u.bassImpact * 4.2 + u.beat * 2.4);
    float beam = exp(-abs(p.y) * 42.0) * smoothstep(0.92, 0.10, abs(p.x)) * (0.04 + u.bassImpact * 1.4 + u.beat * 0.9);
    col += palVisual(a01 + 0.08, u) * beam;
    col *= smoothstep(1.25, 0.42, r);
    col = feedbackTrail(col, uv, min(u.trailDecay, 0.86), 0.005 + 0.006 * u.bassImpact, 0.004);
    return col; // LINEAR HDR — post chain tonemaps
}
