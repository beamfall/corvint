// Memory Tunnel — flying down a corridor whose floor and ceiling are paved
// with framed photo prints. Each tile shows the whole photo, lit by a pulsing
// horizon; beats send a wave of light racing down the corridor.
// Portable port of beamfall-apple-ui Shaders/PhotoVisuals.metal frag_memorytunnel (keep in sync).
// param0 = Flight ; param1 = Depth ; param2 = Pulse

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2 p = motionCentered(uv, u);
    p.x *= 0.92;
    float far = mix(4.2, 7.0, u.param1);
    float pulseAmt = mix(0.4, 1.5, u.param2);

    float z = 1.0 / max(0.13, abs(p.y) + 0.22);
    float lane = p.x * z;
    float depth = z + u.time * (0.38 + u.param0) + u.beatPhase * 0.35;

    // tile lattice — slim frames around each photo print
    float cellPhaseX = lane * 1.6 + 0.5;
    float cellX = fract(cellPhaseX);
    float cellPhaseZ = depth * 0.55;
    float cellZ = fract(cellPhaseZ);
    float frameX = aaLineCyclic(cellPhaseX, 0.012);
    float frameZ = aaLineCyclic(cellPhaseZ, 0.016);
    float frame = max(frameX, frameZ);

    // fog window: fade in near camera, out toward the horizon
    float fog = smoothstep(0.0, 0.9, z) * smoothstep(far, 1.0, z);

    // each tile carries the full photo (small inset margin)
    vec2 tileUV = clamp((vec2(cellX, cellZ) - 0.06) / 0.88, 0.0, 1.0);
    // ceiling tiles flip so prints hang the right way up
    if (p.y > 0.0) { tileUV.y = 1.0 - tileUV.y; }
    vec3 photo = max(IMG(clamp(tileUV, 0.001, 0.999)).rgb, vec3(0.0));

    // beat pulse: a bright wave races down the corridor from the horizon
    float wave = fract(depth * 0.15 - u.beatPhase);
    float pulse = exp(-wave * wave * 18.0) * (0.25 + 0.75 * u.beat) * pulseAmt;

    float interior = (1.0 - frame);
    vec3 col = photo * interior * fog
             * (0.22 + 0.12 * u.level + 0.16 * pulse + 0.05 * u.bass);

    // slim glowing frames between prints — never louder than the prints
    col += palVisual(fract(depth * 0.07 + lane * 0.05), u) * frame * fog
         * (0.06 + 0.22 * u.bass + 0.30 * pulse);

    // luminous horizon at the vanishing line
    float horizon = exp(-abs(p.y) * 20.0) * (0.5 + 1.2 * u.bassImpact) * pulseAmt;
    col += palVisual(0.58 + u.spectralCentroid * 0.2, u) * horizon * 0.35;
    // hot core right at the vanishing point
    col += peakWhite(palVisual(0.62, u), 0.3 + 0.4 * u.bassImpact)
         * exp(-(p.x * p.x * 6.0 + p.y * p.y * 90.0) * 2.5) * (0.35 + 0.9 * u.bassImpact);

    col *= 0.40 + 0.60 * smoothstep(1.65, 0.30, length(p));
    col = feedbackTrail(col, uv, min(u.trailDecay, 0.68),
                        0.004 + 0.006 * u.bassImpact, 0.001);
    return col; // LINEAR HDR — post chain tonemaps
}
