#include "VisualShared.h"

// Kaleidoscope — Radial spectrum mandala that collapses into a dark event
// horizon on big beats. Angular wedge folding produces nested mirrored rings;
// spectrum bins map to radial teeth; bassImpact expands the center void;
// onset fires white fracture rays; feedbackTrail with zoom-in rotation
// creates tunnel trails toward the void.
//
// param0  Segments   mirror-fold count (2..16)
// param1  Spin       base rotation speed, nudged by beatPhase
// param2  Vortex     radial swirl / depth of the inward tunnel
//
// HDR out — no tonemap. Teeth caps + center rim 4-6x.

fragment float4 frag_kaleidoscope(VOut in [[stage_in]],
                                  constant VisualUniforms& u [[buffer(0)]],
                                  texture2d<float> uImage [[texture(0)]],
                                  texture2d<float> uPrev  [[texture(1)]],
                                  sampler uSamp [[sampler(0)]]) {

    float2 p = motionCentered(in.uv, u);

    // ---- parameters ----
    // param0 Segments: 2..16 folds, integer so petals are always whole
    float nSegs  = max(floor(mix(2.0, 16.0, u.param0) + 0.5), 2.0);
    float spinSpd = mix(0.04, 0.40, u.param1);
    float vortex  = u.param2;

    // ---- polar coords ----
    float r = length(p);
    float a = atan2(p.y, p.x);   // -pi..pi

    // ---- bassImpact drives center event-horizon void radius ----
    float voidR = 0.10 + 0.14 * u.bassImpact;

    // ---- param2 vortex: radius-dependent angle swirl ----
    float vortexTwist = vortex * 1.4 * 0.04 / max(r, 0.05);

    // ---- rotation: continuous spin + beatPhase nudge ----
    float rotAngle = u.time * spinSpd + u.beatPhase * 0.25 * spinSpd;
    float aTwisted = a + rotAngle + vortexTwist;

    // ---- kaleidoscope angular fold into one mirrored wedge ----
    float wedge  = M_PI_F / nSegs;                    // half-angle of one segment
    float aWrap  = fmod(abs(aTwisted), 2.0 * wedge);  // [0, 2*wedge)
    float aFolded = (aWrap > wedge) ? (2.0 * wedge - aWrap) : aWrap; // mirror
    // aFolded in [0, wedge]; normalise to 0..1 within wedge
    float ang01 = aFolded / max(wedge, 1e-5);

    // ---- spectrum at this folded angle ----
    float bSmooth = bandAt(u, ang01);         // continuous read for teeth
    float bHi     = bandAt(u, ang01 * 0.5 + 0.5); // upper-half for veins

    // spectralCentroid shifts the palette hue globally
    float hueShift = 0.18 * u.spectralCentroid;

    float3 col = float3(0.0);

    // ====================================================================
    // LAYER 1 — Radial teeth (spectrum bars from voidR outward)
    // ====================================================================
    float toothBase  = voidR + 0.02;
    float toothLen   = 0.08 + bSmooth * 0.38 + u.bassImpact * 0.06;
    float toothOuter = toothBase + toothLen;

    // Soft angular taper: fade at wedge boundaries to avoid hard seam
    float angEdge = smoothstep(0.0, 0.06, ang01) * smoothstep(1.0, 0.94, ang01);
    float radMask = step(toothBase, r) * step(r, toothOuter);

    float toothT   = ang01 + hueShift + 0.05 * u.time;
    float3 toothCol = palVisual(toothT, u);
    toothCol = accentize(toothCol, u.accent, 0.20 + 0.25 * u.bassImpact);

    // Caps at tooth tip glow 4-6x (HDR)
    float capProx   = smoothstep(toothOuter - 0.025, toothOuter, r);
    float toothBright = (0.38 + bSmooth * 1.55) * (1.0 + capProx * 1.15);
    col += toothCol * radMask * angEdge * toothBright;

    // ====================================================================
    // LAYER 2 — Nested concentric ring lines
    // ====================================================================
    for (int ri = 1; ri <= 5; ri++) {
        float fRi   = float(ri);
        float ringR = voidR + (1.0 - voidR) * (fRi / 5.5);
        float rBand = bandAt(u, fRi / 6.0);
        float rLine = aaLine(r - ringR, 0.003 + 0.006 * rBand);
        float rT    = ang01 + hueShift + fRi * 0.13 + 0.04 * u.time;
        float3 rCol = palVisual(rT, u);
        // High-frequency rings lean cyan/white
        float hiFreq = fRi / 5.0;
        rCol = mix(rCol, float3(0.0, 0.78, 1.0), hiFreq * 0.45 * u.treble);
        rCol = accentize(rCol, u.accent, 0.12);
        col += rCol * rLine * (0.70 + rBand * 2.1) * angEdge;
    }

    // ====================================================================
    // LAYER 3 — Center rim glow (inner edge of the event horizon)
    // ====================================================================
    float rimWidth = 0.008 + 0.012 * u.bassImpact;
    float rimLine  = aaLine(r - voidR, rimWidth);
    // Rim: base night palette at cyan/teal, then gold on bassImpact
    float3 rimCol  = palVisual(hueShift + 0.62, u) * (1.25 + u.bassImpact * 1.45);
    rimCol = mix(rimCol, float3(1.6, 0.24, 0.02), u.bassImpact * 0.55);
    col += rimCol * rimLine;

    // ====================================================================
    // LAYER 4 — onset: white fracture rays at fold-mirror seams
    // ====================================================================
    if (u.onset > 0.02) {
        // Rays appear at the fold boundaries (ang01 near 0 or 1)
        float rayNear0 = 1.0 - smoothstep(0.0,  0.08, ang01);
        float rayNear1 = 1.0 - smoothstep(0.92, 1.0,  ang01);
        float rayMask  = max(rayNear0, rayNear1);
        float rayRadial = smoothstep(voidR, voidR + 0.06, r)
                        * (1.0 - smoothstep(0.5, 1.0, r));
        float rayThin = pow(rayMask, 4.0);
        float peak = saturate(u.onset * u.bassImpact * 0.22);
        col += peakWhite(palVisual(0.04 + hueShift, u), peak) * rayThin * rayRadial * u.onset * 0.75;
    }

    // ====================================================================
    // LAYER 5 — treble/flux fine veins (crisp lines from upper spectrum)
    // ====================================================================
    {
        float veinR   = voidR + 0.04 + bHi * 0.12;
        float veinLine = aaLine(r - veinR, 0.004);
        float3 veinCol = palVisual(0.76 + hueShift, u) * (1.5 + u.treble * 2.4);
        col += veinCol * veinLine * u.treble * angEdge;
    }

    // ====================================================================
    // BLACK EVENT HORIZON core — hard mask inside voidR
    // ====================================================================
    col *= smoothstep(0.0, voidR, r);

    // ====================================================================
    // FEEDBACK TRAILS — zoom-in rotation produces collapsing tunnel trails
    // ====================================================================
    float fbZoom = 0.006 + 0.010 * u.bassImpact;
    float fbRot  = 0.004 * spinSpd + 0.006 * u.beat;
    col = feedbackTrail(col, uPrev, uSamp, in.uv, min(u.trailDecay, 0.76), fbZoom, fbRot);

    return float4(col, 1.0);   // LINEAR HDR — DO NOT call tonemap()
}
