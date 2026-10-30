// Spectrum — Standard / neon glass skyline.
// Portable port of beamfall-apple-ui Shaders/Spectrum.metal (keep in sync).
// param0 Perspective ; param1 Column Glass ; param2 Reflection

vec3 visual(vec2 uv, VisualUniforms u) {
    // Camera shake (suv = shaken coords; keep uv pristine for the trail).
    float shake = u.bassImpact * 0.004;
    vec2 cameraNoise = vec2(
        sin(u.time * 17.0 + 2.2 * sin(u.time * 3.1)),
        sin(u.time * 13.0 + 1.7 * cos(u.time * 2.7) + 4.0)
    );
    vec2 suv = uv + shake * vec2(cameraNoise.x, cameraNoise.y * 0.45);
    suv = clamp(suv, 0.001, 0.999);

    float horizon = 0.62;
    float persp = mix(1.35, 2.75, u.param0);
    float glass = mix(0.48, 1.00, u.param1);
    float refl = mix(0.10, 0.42, u.param2);

    vec3 base = vec3(0.0);
    vec3 glow = vec3(0.0);

    // Floor grid/reflection, lower half only.
    if (suv.y > horizon) {
        float ft = (suv.y - horizon) / max(1.0 - horizon, 0.001);
        float depth = 1.0 / max(ft * persp + 0.025, 0.025);
        float wx = (suv.x - 0.5 + sin(beatRadians(u, ft * 0.25)) * 0.004) * depth * 1.8;
        float wz = depth + u.time * 0.20 + u.beatPhase * 0.42;

        float gx = wx * 0.82 + 0.5;
        float gz = wz * 0.36;
        float grid = max(aaLineCyclic(gx, 0.0032), aaLineCyclic(gz, 0.0022));
        float fade = smoothstep(0.02, 0.20, ft) * exp(-ft * 3.4) * smoothstep(0.86, 0.16, abs(suv.x - 0.5));
        glow += mix(palRoleFog(0.56, u), palRoleTrace(suv.x, u), 0.45) * grid * fade * (0.16 + 0.12 * u.mid);
    }

    const int NCOLS = 58;
    for (int i = 0; i < NCOLS; i++) {
        float fi = float(i);
        float x = (fi + 0.5) / float(NCOLS);
        float lane = abs(suv.x - x);
        float phaseOffset = fi / float(NCOLS - 1);
        float raw = bandAtWrapped(u, phaseOffset + u.beatPhase * 0.035);
        float seed = hash11(fi * 3.37);
        float b = max(raw, 0.06 + 0.10 * seed);
        float phaseLift = phasePulse(u, phaseOffset, 0.18);
        float cluster = hash11(floor(fi / 4.0) * 2.11);

        float h = pow(b, 0.72) * mix(0.20, 0.60, cluster) * glass;
        h += pow(seed, 6.0) * 0.26;
        h *= (1.0 + 0.18 * u.bassImpact + phaseLift * 0.08);
        h = clamp(h, 0.035, 0.56);

        float yTop = horizon - h;
        float colW = mix(0.0028, 0.0068, smoothstep(0.06, 1.0, b));
        float inX = 1.0 - smoothstep(colW, colW + 0.002, lane);
        float inY = step(yTop, suv.y) * step(suv.y, horizon);
        vec3 c = palRoleTrace(x + 0.08 * u.spectralCentroid, u);
        c = accentize(c, u.accent, 0.08);

        // Transparent body plus glass edges/caps/segment rings.
        base += c * inX * inY * b * 0.024;
        float edge = aaLine(lane - colW, 0.0016) * inY;
        float cap = aaLine(suv.y - yTop, 0.0032) * inX;
        float seg = aaLineCyclic((suv.y - yTop) / max(h, 0.001) * 5.0 - u.beatPhase * 0.55, 0.035) * inX * inY;
        glow += c * edge * (0.58 + 1.10 * b);
        glow += mix(c, vec3(1.0), 0.45) * cap * (0.82 + 1.90 * u.treble + 0.70 * b);
        glow += c * seg * (0.12 + 0.38 * b + phaseLift * 0.18);

        // Reflection as thin mirrored ticks, not a smear.
        if (suv.y > horizon) {
            float ft = (suv.y - horizon) / max(1.0 - horizon, 0.001);
            float mirrorY = horizon + (horizon - yTop) * (1.0 - ft * 0.62);
            float rLine = aaLine(suv.y - mirrorY, 0.0035) * inX;
            glow += c * rLine * refl * (1.0 - ft) * (0.20 + 0.55 * b);
        }

        // Particle shafts above selected towers.
        float shaft = exp(-lane * lane * 23000.0)
                    * smoothstep(yTop - 0.22, yTop - 0.02, suv.y)
                    * (1.0 - smoothstep(yTop - 0.02, yTop + 0.04, suv.y));
        glow += c * shaft * step(0.78, seed) * (0.18 + 0.50 * u.flux) * (0.25 + 0.65 * b);

        // Tiny spark caps.
        float sparkLife = fract(seed + u.time * (0.5 + seed) + u.beatCount * 0.03 + u.beatPhase * 0.12);
        float sparkY = yTop - sparkLife * (0.06 + 0.22 * seed);
        float sparkD = length(vec2((suv.x - x) * 1.9, suv.y - sparkY));
        float spark = min(1.5, 0.00014 / (sparkD * sparkD + 0.00032));
        glow += palRoleTreble(x + 0.08 * u.spectralCentroid, u) * spark * u.flux * (1.0 - sparkLife);
    }

    // Molten waveform ribbon across the city front.
    float bline = bandAt(u, suv.x);
    float ribbonY = horizon + 0.055
                  + 0.035 * (bline - 0.5)
                  + 0.014 * sin(suv.x * 12.0 + u.time * 1.5 + beatRadians(u, 0.0) * 0.18)
                  + 0.006 * sin(suv.x * 35.0 - u.time * 2.6 - beatRadians(u, 0.25) * 0.15);
    float ribbonW = 0.0034 + 0.0028 * u.bassImpact;
    float ribbon = aaLine(suv.y - ribbonY, ribbonW);
    float halo = aaLine(suv.y - ribbonY, ribbonW * 5.0) * 0.20;
    glow += palRoleBass(0.08, u) * (ribbon + halo) * (1.15 + 1.55 * bline + 1.35 * u.bassImpact);

    vec3 trailedGlow = feedbackTrail(glow, uv,
                                     min(u.trailDecay, 0.80),
                                     0.002 + 0.004 * u.bassImpact,
                                     0.0006 * u.beat);
    return base + trailedGlow;
}
