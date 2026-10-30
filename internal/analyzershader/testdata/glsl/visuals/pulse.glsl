// Pulse — Lite — "Black-room Solar Core"
// Portable port of beamfall-apple-ui Shaders/Pulse.metal (keep in sync).
// param0 = Glow ; param1 = Impact ; param2 = Corona Count

vec3 visual(vec2 uv, VisualUniforms u) {
    vec2  p = motionCentered(uv, u);
    float r = length(p);
    float a = atan2_(p.y, p.x);

    float glowAmt   = mix(0.25, 1.40, u.param0);
    float impactAmt = mix(0.06, 0.26, u.param1);
    int   numCorona = int(mix(3.0, 5.0, u.param2) + 0.5);

    float breathe = 0.007 * sin(u.time * 1.1) + 0.005 * sin(u.time * 2.3 + 0.8);
    float coreR   = 0.12 + 0.06 * u.amplitude + breathe + impactAmt * u.bassImpact;

    vec3 col = vec3(0.0);
    {
        float rSafe = max(r, 1e-4);
        float inner = smoothstep(coreR, coreR * 0.40, rSafe);
        float rimW  = 0.012 + 0.006 * u.amplitude;
        float rim   = aaLine(rSafe - coreR, rimW);

        vec3 coreCol = mix(vec3(1.00, 0.20, 0.02), vec3(1.00, 0.05, 0.68),
                           smoothstep(0.2, 1.0, inner));
        coreCol = mix(coreCol, vec3(0.00, 0.78, 1.00), rim * 0.35);
        coreCol = peakWhite(coreCol, u.bassImpact * inner * 0.08);
        float exposure = 0.85 + 1.25 * u.level + 0.80 * u.bassImpact;
        col += coreCol * (inner * 2.1 + rim * 3.4) * exposure;

        float onsetFlash = u.onset * 5.0 * aaLine(rSafe - (coreR + 0.018), 0.008);
        col += vec3(1.8, 0.48, 0.08) * onsetFlash;
    }

    {
        float spacing = 0.065 + 0.020 * u.amplitude;
        // Contract rule 8: compile-time-constant loop bound (numCorona tops out
        // at 5); break on the dynamic count instead of looping on it directly.
        for (int i = 0; i < 5; ++i) {
            if (i >= numCorona) break;
            float fi    = float(i);
            float ringR = coreR + (fi + 1.0) * spacing
                        + impactAmt * u.bassImpact * (1.0 - fi * 0.18);

            float noiseT  = a * (0.8 + fi * 0.15);
            float noiseR  = r + fi * 0.07;
            float turb    = fbm(vec2(noiseT, noiseR + u.time * (0.08 + fi * 0.03)));
            float turbAmt = 0.018 + 0.010 * u.mid;
            float d       = r - ringR - turb * turbAmt;

            float halfW = 0.006 + 0.004 * u.amplitude - fi * 0.0008;
            halfW       = max(halfW, 0.002);
            float mask  = aaLine(d, halfW);

            float hue = fi * 0.20 + u.beatPhase * 0.35
                      + a / 6.2831853 * 0.12 + 0.08 * u.spectralCentroid;
            vec3 ringCol = palVisual(hue, u);
            ringCol      = accentize(ringCol, u.accent, 0.12);

            float bright = glowAmt * (1.5 - fi * 0.22) * (0.9 + u.amplitude * 0.5)
                         + 0.8 * u.bassImpact;
            bright = max(bright, 0.0);

            col += ringCol * mask * bright;
        }
    }

    {
        float trebleE = u.treble + u.flux * 0.5;
        for (int i = 0; i < 32; ++i) {
            float fi = float(i);
            vec2  h  = hash22(vec2(fi, 3.7));
            float bandOuter = coreR + float(numCorona) * (0.065 + 0.020 * u.amplitude) + 0.06;
            float sr = mix(coreR + 0.03, bandOuter, h.x);
            float sa = h.y * 6.2831853 + u.time * (0.4 + 0.6 * h.x) + u.beatPhase * 1.8;
            vec2  sp = vec2(cos(sa), sin(sa)) * sr;
            float dd = length(p - sp);
            float spark = 5e-5 / (dd * dd + 5e-5);
            spark = min(spark, 3.0);
            float flicker = hash11(fi + floor(u.time * 18.0)) * 0.5 + 0.5;
            vec3 scol = palVisual(h.x + 0.05 * u.spectralCentroid, u);
            scol = peakWhite(scol, u.onset * trebleE * 0.18);
            col += scol * spark * trebleE * flicker * 4.5;
        }
    }

    float outerFade = coreR + float(numCorona) * 0.09 + 0.12;
    col *= 1.0 - smoothstep(outerFade, outerFade + 0.18, r);

    float trailZoom = 0.008 + 0.014 * u.bassImpact;
    float trailRot  = 0.004 * u.beat + 0.001 * u.time * 0.01;
    col = feedbackTrail(col, uv, u.trailDecay, trailZoom, trailRot);

    return col; // LINEAR HDR — post chain tonemaps
}
