// Lattice — Standard / "Prismatic Cymatics Ring"
// Portable port of beamfall-apple-ui Shaders/Lattice.metal (keep in sync).
// param0 Contour Density ; param1 Height ; param2 Reflection

vec3 visual(vec2 uv, VisualUniforms u) {
    // ── params ──
    float densityParam = mix(16.0, 64.0, u.param0);
    float heightParam  = mix(0.04, 0.30, u.param1);
    float reflParam    = mix(0.0,  0.22, u.param2);

    // ── centered, aspect-correct coords ──
    vec2 p = motionCentered(uv, u);

    const float SQUASH = 0.38;
    const float LIFT   = 0.24;
    vec2 pRing = vec2(p.x, (p.y + LIFT) / SQUASH);

    float rotAngle = u.beatPhase * 0.25 + u.time * 0.04;
    pRing = rot2(rotAngle) * pRing;

    // ── polar coordinates ──
    float r   = length(pRing);
    float ang = atan2_(pRing.y, pRing.x);
    float ang01 = ang / 6.2831853 + 0.5;

    // ── audio surface at this angle ──
    float audioH = bandAt(u, ang01);
    float trebleH = bandAt(u, 0.75 + ang01 * 0.25);

    vec2 fbmUV = vec2(ang01 * 4.0, u.time * 0.12);
    float noise  = fbm(fbmUV) * 0.5 - 0.25;

    const float BASE_R  = 0.40;
    const float SPACING = 0.016;
    float heightScale = heightParam * (1.0 + u.bassImpact * 0.5);
    float surfaceR = BASE_R + audioH * heightScale + noise * 0.04;

    float outerRim = BASE_R + heightParam * 0.9 + u.bassImpact * 0.06;

    // ── contour ring drawing ──
    vec3 col = vec3(0.0);

    int nContours = int(densityParam);
    float halfN   = float(nContours) * 0.5;

    for (int k = 0; k < 64; k++) {
        if (k >= nContours) break;
        float fk = float(k) - halfN;
        float ringR = surfaceR + fk * SPACING;

        if (ringR < 0.05) continue;

        float d = r - ringR;
        float wire = aaLine(d, 0.0005);

        if (wire < 0.001) continue;

        float hueFrac = fract(ang01 + 0.05 * u.spectralCentroid + 0.02 * u.time);
        vec3 wireCol = palVisual(hueFrac, u);
        wireCol = accentize(wireCol, u.accent, 0.12);

        float crestProximity = 1.0 - abs(fk) / (halfN + 1.0);
        float brightness = 1.5 + audioH * 3.0 * crestProximity;

        float trebleSpike = trebleH * crestProximity;
        vec3 spikeTint  = mix(wireCol, vec3(4.0, 4.0, 5.0), trebleSpike * trebleSpike);

        col += spikeTint * wire * brightness;
    }

    // ── outer rim pulse (bass impact) ──
    {
        float rimD  = r - outerRim;
        float rimW  = aaLine(rimD, 0.003);
        vec3 rimC = palVisual(ang01 * 0.5, u);
        rimC = accentize(rimC, u.accent, 0.15);
        col += rimC * rimW * (1.0 + u.bassImpact * 5.0);
    }

    // ── inner hollow edge (keeps center black) ──
    {
        float innerR = BASE_R - heightParam * 0.35 - 0.02;
        innerR = max(innerR, 0.08);
        float innerD = r - innerR;
        float innerW = aaLine(innerD, 0.002);
        col += palVisual(ang01 + 0.5, u) * innerW * 0.6;
        col *= smoothstep(innerR - 0.03, innerR + 0.01, r);
    }

    // ── treble spike ridge along the crest ──
    {
        float crD   = abs(r - surfaceR);
        float spike = exp(-crD * crD * 800.0) * trebleH * trebleH;
        col += vec3(5.0, 5.5, 6.0) * spike * u.treble;
    }

    // ── fade extreme outer radius to black ──
    col *= 1.0 - smoothstep(BASE_R + heightParam + 0.15, BASE_R + heightParam + 0.35, r);

    // ── glossy floor reflection ──
    if (reflParam > 0.001) {
        float horizY = -LIFT;
        float screenY = p.y;

        if (screenY < horizY) {
            float below = horizY - screenY;

            vec2 pMirror = vec2(p.x, horizY + below);
            vec2 pMirRing = vec2(pMirror.x, (pMirror.y + LIFT) / SQUASH);
            pMirRing = rot2(rotAngle) * pMirRing;

            float rM   = length(pMirRing);
            float angM = atan2_(pMirRing.y, pMirRing.x);
            float angM01 = angM / 6.2831853 + 0.5;

            float audioHM = bandAt(u, angM01);
            vec2 fbmUVM = vec2(angM01 * 4.0, u.time * 0.12 + 0.7);
            float noiseM  = fbm(fbmUVM) * 0.5 - 0.25;
            float surfRM  = BASE_R + audioHM * heightScale + noiseM * 0.04;

            float ripple  = sin(below * 18.0 - u.time * 3.0 + ang * 2.0) * 0.012 * below;
            float rMR = rM + ripple;

            vec3 refCol = vec3(0.0);
            for (int k = 0; k < 64; k++) {
                if (k >= nContours) break;
                float fk    = float(k) - halfN;
                float ringR = surfRM + fk * SPACING;
                if (ringR < 0.05) continue;

                float d    = rMR - ringR;
                float wire = aaLine(d, 0.0005);
                if (wire < 0.001) continue;

                float hueFrac = fract(angM01 + 0.05 * u.spectralCentroid + 0.02 * u.time);
                vec3 wireCol = palVisual(hueFrac, u);
                wireCol = accentize(wireCol, u.accent, 0.12);
                float cP = 1.0 - abs(fk) / (halfN + 1.0);
                refCol += wireCol * wire * (1.2 + audioHM * 2.5 * cP);
            }

            float fadeD   = exp(-below * (3.5 - u.bassImpact * 1.5));
            float flare   = 1.0 + u.bassImpact * 2.0;
            refCol *= fadeD * reflParam * flare;

            col += refCol;
        }
    }

    // Suppress the long horizontal axis line outside the ring.
    float axis = exp(-(p.y + LIFT)*(p.y + LIFT) * 520.0);
    float outsideRing = smoothstep(0.42, 0.62, abs(p.x));
    col *= 1.0 - axis * outsideRing * 0.86;

    // ── feedback trails ──
    col = feedbackTrail(col, uv,
                        u.trailDecay,
                        0.003 + 0.007 * u.bassImpact,
                        0.002 * u.beat);

    return col; // LINEAR HDR — do NOT tonemap
}
