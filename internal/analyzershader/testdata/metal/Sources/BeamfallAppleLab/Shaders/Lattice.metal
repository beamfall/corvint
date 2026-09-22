#include "VisualShared.h"

// Lattice — Standard / "Prismatic Cymatics Ring"
// Layered rainbow contour-wire rings in perspective around a hollow center,
// sitting on a glossy floor with reflections, white-hot treble spikes along
// the crest, hue by frequency around the angle.
//
// param0  Contour Density   (default 0.64) — number of contour iso-rings
// param1  Height            (default 0.58) — audio displacement scale
// param2  Reflection        (default 0.46) — floor reflection strength
//
// HDR out — DO NOT tonemap here. Rim and treble spikes 4–7x.

fragment float4 frag_lattice(VOut in [[stage_in]],
                             constant VisualUniforms& u [[buffer(0)]],
                             texture2d<float> uImage    [[texture(0)]],
                             texture2d<float> uPrev     [[texture(1)]],
                             sampler uSamp              [[sampler(0)]]) {

    // ── params ──────────────────────────────────────────────────────────────
    float densityParam = mix(16.0, 64.0, u.param0);   // contour count
    float heightParam  = mix(0.04, 0.30, u.param1);   // max radial displacement
    float reflParam    = mix(0.0,  0.22, u.param2);   // floor reflection alpha

    // ── centered, aspect-correct coords ─────────────────────────────────────
    // p is in "screen space" with aspect applied, roughly ±1 on the short axis
    float2 p = motionCentered(in.uv, u);

    // Perspective tilt: squash y to create the oblique ring-in-3D illusion.
    // Shift the center DOWN so the ring sits above the horizon / floor seam.
    const float SQUASH   = 0.38;    // y compression factor (< 1 = flatter ellipse)
    const float LIFT     = 0.24;    // vertical center offset (positive = up in clip)
    float2 pRing = float2(p.x, (p.y + LIFT) / SQUASH);

    // Slow beat-phase rotation around the ring center
    float rotAngle = u.beatPhase * 0.25 + u.time * 0.04;
    pRing = rot2(rotAngle) * pRing;

    // ── polar coordinates ────────────────────────────────────────────────────
    float r   = length(pRing);
    float ang = atan2(pRing.y, pRing.x);        // -π … +π
    float ang01 = ang / 6.2831853 + 0.5;        // 0 … 1 around the ring

    // ── audio surface at this angle ──────────────────────────────────────────
    // bandAt() gives spectrum energy at a fractional frequency position.
    // We map ang01 → frequency so bass is outer, treble is at one sector.
    float audioH = bandAt(u, ang01);            // 0..1, raw spectral value
    float trebleH = bandAt(u, 0.75 + ang01 * 0.25); // hi-freq slice

    // FBM displacement — organic contour wobble, independent of audio
    float2 fbmUV = float2(ang01 * 4.0, u.time * 0.12);
    float noise  = fbm(fbmUV) * 0.5 - 0.25;    // centered, small

    // Base ring radius + audio-driven radial height + fbm
    const float BASE_R  = 0.40;
    const float SPACING = 0.016;                // gap between contour rings
    float heightScale = heightParam * (1.0 + u.bassImpact * 0.5);
    float surfaceR = BASE_R + audioH * heightScale + noise * 0.04;

    // Outer rim pulses with bass impact
    float outerRim = BASE_R + heightParam * 0.9 + u.bassImpact * 0.06;

    // ── contour ring drawing ─────────────────────────────────────────────────
    // Draw N concentric rings whose radii are offset from surfaceR by k*SPACING.
    // Each ring is an iso-line: bright where |r - ringRadius| is small.
    float3 col = float3(0.0);

    int nContours = int(densityParam);
    float halfN   = float(nContours) * 0.5;

    for (int k = 0; k < nContours; k++) {
        float fk = float(k) - halfN;            // signed offset: negative=inner, positive=outer
        float ringR = surfaceR + fk * SPACING;

        if (ringR < 0.05) continue;             // guard: skip collapsed rings near center

        // distance from current pixel radius to this iso-ring radius
        float d = r - ringR;

        // Anti-aliased line using fwidth — crisp even on perspective-skewed coords
        float wire = aaLine(d, 0.0005);

        if (wire < 0.001) continue;

        // Hue along angle; map band position to neon palette:
        //   bass (outer angle sector) → magenta/orange (t~0)
        //   mids                      → green (t~0.33)
        //   highs (inner)             → cyan/white (t~0.65)
        float hueFrac = fract(ang01 + 0.05 * u.spectralCentroid + 0.02 * u.time);
        float3 wireCol = palNeon(hueFrac);
        wireCol = accentize(wireCol, u.accent, 0.12);

        // Brightness: brighter at the crest (largest audio displacement) and at bass rings
        float crestProximity = 1.0 - abs(fk) / (halfN + 1.0); // 1 at crest, 0 at extremes
        float brightness = 1.5 + audioH * 3.0 * crestProximity;

        // White-hot treble spikes: where treble is high AND we're near the crest
        float trebleSpike = trebleH * crestProximity;
        float3 spikeTint  = mix(wireCol, float3(4.0, 4.0, 5.0), trebleSpike * trebleSpike);

        col += spikeTint * wire * brightness;
    }

    // ── outer rim pulse (bass impact) ────────────────────────────────────────
    {
        float rimD  = r - outerRim;
        float rimW  = aaLine(rimD, 0.003);
        float3 rimC = palNeon(ang01 * 0.5);   // bass side: magenta/orange
        rimC = accentize(rimC, u.accent, 0.15);
        col += rimC * rimW * (1.0 + u.bassImpact * 5.0);
    }

    // ── inner hollow edge (keeps center black) ───────────────────────────────
    {
        float innerR = BASE_R - heightParam * 0.35 - 0.02;
        innerR = max(innerR, 0.08);
        float innerD = r - innerR;
        float innerW = aaLine(innerD, 0.002);
        col += palNeon(ang01 + 0.5) * innerW * 0.6;
        // Fade everything inside the hollow center to black
        col *= smoothstep(innerR - 0.03, innerR + 0.01, r);
    }

    // ── treble spike ridge along the crest ───────────────────────────────────
    // Spiky white flares above/below the crest where treble is hot
    {
        float crD   = abs(r - surfaceR);
        float spike = exp(-crD * crD * 800.0) * trebleH * trebleH;
        col += float3(5.0, 5.5, 6.0) * spike * u.treble;
    }

    // ── fade extreme outer radius to black ───────────────────────────────────
    col *= 1.0 - smoothstep(BASE_R + heightParam + 0.15, BASE_R + heightParam + 0.35, r);

    // ── glossy floor reflection ───────────────────────────────────────────────
    // The "floor" is a horizontal mirror seam at the screen center (p.y == 0
    // in centered coords). We reflect p.y > 0 portion of the ring below y=0.
    // floor_p is the mirrored position in "ring space."
    if (reflParam > 0.001) {
        // Horizon line in screen-space centered coords sits at y = -LIFT
        // (where the ring center maps back to screen center).
        float horizY = -LIFT;
        float screenY = p.y;            // screen-space y, centered+aspect

        // Only draw reflection below the horizon
        if (screenY < horizY) {
            float below = horizY - screenY;  // distance below horizon (positive)

            // Mirror the ring geometry: reflect screen point through horizon
            float2 pMirror = float2(p.x, horizY + below);   // above-horizon equivalent
            float2 pMirRing = float2(pMirror.x, (pMirror.y + LIFT) / SQUASH);
            pMirRing = rot2(rotAngle) * pMirRing;

            float rM   = length(pMirRing);
            float angM = atan2(pMirRing.y, pMirRing.x);
            float angM01 = angM / 6.2831853 + 0.5;

            float audioHM = bandAt(u, angM01);
            float2 fbmUVM = float2(angM01 * 4.0, u.time * 0.12 + 0.7);
            float noiseM  = fbm(fbmUVM) * 0.5 - 0.25;
            float surfRM  = BASE_R + audioHM * heightScale + noiseM * 0.04;

            // Ripple distortion on the floor reflection (water-like shimmer)
            float ripple  = sin(below * 18.0 - u.time * 3.0 + ang * 2.0) * 0.012 * below;
            float rMR = rM + ripple;

            // Draw the same contour wires reflected
            float3 refCol = float3(0.0);
            for (int k = 0; k < nContours; k++) {
                float fk    = float(k) - halfN;
                float ringR = surfRM + fk * SPACING;
                if (ringR < 0.05) continue;

                float d    = rMR - ringR;
                float wire = aaLine(d, 0.0005);
                if (wire < 0.001) continue;

                float hueFrac = fract(angM01 + 0.05 * u.spectralCentroid + 0.02 * u.time);
                float3 wireCol = palNeon(hueFrac);
                wireCol = accentize(wireCol, u.accent, 0.12);
                float cP = 1.0 - abs(fk) / (halfN + 1.0);
                refCol += wireCol * wire * (1.2 + audioHM * 2.5 * cP);
            }

            // Fade by distance below horizon and bass impact brightens the flare
            float fadeD   = exp(-below * (3.5 - u.bassImpact * 1.5));
            float flare   = 1.0 + u.bassImpact * 2.0;
            refCol *= fadeD * reflParam * flare;

            col += refCol;
        }
    }

    // Suppress the long horizontal axis line outside the ring. The contour
    // geometry naturally overlaps there; without this mask the thumbnail reads
    // as a beam/eye instead of a prismatic topographic ring.
    float axis = exp(-pow(p.y + LIFT, 2.0) * 520.0);
    float outsideRing = smoothstep(0.42, 0.62, abs(p.x));
    col *= 1.0 - axis * outsideRing * 0.86;

    // ── feedback trails ───────────────────────────────────────────────────────
    // Subtle: tiny zoom-out + slight beat rotation
    col = feedbackTrail(col, uPrev, uSamp, in.uv,
                        u.trailDecay,
                        0.003 + 0.007 * u.bassImpact,
                        0.002 * u.beat);

    return float4(col, 1.0);   // LINEAR HDR — do NOT tonemap
}
