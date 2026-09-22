// Constellation — Standard — frequency + beat.
// Portable port of beamfall-apple-ui Shaders/Constellation.metal (keep in sync).
//
// Audio-reactive node network, not a starfield.
// 30 nodes + local edges; bounded analytic loops.
// param0 Node Count, param1 Link Range, param2 Pulse

vec2 constellationNode(float i, VisualUniforms u) {
    vec2 h = hash22(vec2(i * 1.31, i * 2.17 + 4.0));
    float x = mix(-1.55, 1.55, h.x) + 0.05 * sin(u.time * 0.18 + i);
    float y = mix(-0.78, 0.78, h.y) + 0.04 * cos(u.time * 0.15 + i * 1.7);
    return vec2(x, y);
}

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    float count = mix(22.0, 30.0, u.param0);
    float linkRange = mix(0.38, 0.68, u.param1);
    float pulse = mix(0.4, 1.6, u.param2);

    vec3 glow = vec3(0.0);

    for (int i = 0; i < 30; i++) {
        if (float(i) >= count) break;
        float fi = float(i);
        vec2 a = constellationNode(fi, u);
        float bandv = bandAt(u, fract(fi / 30.0 + 0.04 * u.beatPhase));
        float nodeD = length(p - a);
        float node = min(1.8, 0.00035 / (nodeD * nodeD + 0.00025));
        vec3 c = palVisual(fi / 30.0 + 0.18 * u.spectralCentroid, u);
        glow += c * node * (0.35 + bandv * 1.5 + u.flux * pulse);

        for (int j = 1; j <= 5; j++) {
            int jj = (i + j * 5 + 3) % 30;
            vec2 b = constellationNode(float(jj), u);
            vec2 ab = b - a;
            float t = clamp(dot(p - a, ab) / max(dot(ab, ab), 1e-4), 0.0, 1.0);
            float d = length(p - (a + ab * t));
            float distAB = length(ab);
            float link = aaLine(d, 0.0012) * (1.0 - smoothstep(linkRange, linkRange + 0.10, distAB));
            float travel = exp(-(t - u.beatPhase)*(t - u.beatPhase) * 80.0);
            glow += c * link * (0.18 + bandv * 0.9 + travel * u.bassImpact * 1.8);
        }
    }

    vec3 col = feedbackTrail(glow, uv,
                             min(u.trailDecay, 0.84),
                             0.0015 + 0.003 * u.bassImpact,
                             0.001 * sin(u.time * 0.3));
    return col; // LINEAR HDR — post chain tonemaps
}
