// Petals — spectral flower/speaker-cone rosette.
// Portable port of beamfall-apple-ui Shaders/Petals.metal (keep in sync).
// param0 Petal Count ; param1 Bloom ; param2 Detail

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    float r = length(p);
    float a = atan2_(p.y, p.x);
    float ang = a / 6.2831853 + 0.5;

    float petals = mix(6.0, 18.0, u.param0);
    float bloom = mix(0.24, 0.62, u.param1) * (1.0 + u.bassImpact * 0.22);
    float detail = mix(0.4, 1.6, u.param2);

    float wave = abs(cos(a * petals * 0.5 + u.time * 0.12));
    float bandv = bandAt(u, fract(ang + 0.05 * u.beatPhase));
    float petalR = bloom * (0.58 + wave * (0.45 + bandv * 0.55));
    float edge = aaLine(r - petalR, 0.006);
    float inner = aaLine(r - petalR * 0.62, 0.0035);
    float vein = aaLineCyclic(ang * petals + r * 2.0, 0.020) *
                 smoothstep(0.05, petalR, r) * (1.0 - smoothstep(petalR, petalR + 0.08, r));

    vec3 c = palVisual(ang + 0.12 * u.spectralCentroid, u);
    c = accentize(c, u.accent, 0.10);
    vec3 glow = c * edge * (1.0 + bandv * 2.8 + u.bassImpact * 2.0);
    glow += palVisual(ang + 0.2, u) * inner * (0.4 + u.level);
    glow += c * vein * detail * (0.25 + bandv);
    glow += palRolePeak(0.58, u) * exp(-r * r / 0.030) * (0.25 + u.level * 0.70);

    vec3 col = feedbackTrail(glow, uv,
                             min(u.trailDecay, 0.82),
                             0.0015 + 0.004 * u.bassImpact,
                             0.001 * u.beat);
    return col;
}
