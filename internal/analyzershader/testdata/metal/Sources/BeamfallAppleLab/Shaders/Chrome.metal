#include "VisualShared.h"

// Chrome — Flagship / black-studio liquid metal.
// Dark metaballs reflect sparse neon softboxes. Most of the surface stays dark;
// only reflected strips, Fresnel rim, and transient glints enter HDR.
// param0 Blob Count      -> metaball count (3..7)
// param1 Reflection      -> strip reflection strength
// param2 Surface Ripple  -> micro-normal ripple amount

static float smin_chrome(float a, float b, float k) {
    float h = clamp(0.5 + 0.5 * (b - a) / max(k, 1e-5), 0.0, 1.0);
    return mix(b, a, h) - k * h * (1.0 - h);
}

static float sdf_chrome(float3 p, constant VisualUniforms& u, int nBlobs) {
    float orbitR = mix(0.32, 0.56, u.mid);
    float bassR = u.bassImpact * 0.06;
    float d = 1e9;

    for (int i = 0; i < 7; i++) {
        if (i >= nBlobs) break;

        float fi = float(i);
        float phase = fi * 1.618;
        float bandEnergy = band(u, i * 9);

        float3 center = float3(
            sin(u.time * 0.42 + phase) * orbitR * (0.65 + bandEnergy * 0.30),
            cos(u.time * 0.36 + fi * 1.3) * orbitR * 0.42,
            sin(u.time * 0.31 + fi * 0.9) * orbitR * 0.34
        );

        float radius = 0.32 + bandEnergy * 0.09 + bassR;
        float blob = length(p - center) - radius;
        d = (i == 0) ? blob : smin_chrome(d, blob, 0.32);
    }

    return d;
}

static float3 calcNormal_chrome(float3 p, constant VisualUniforms& u, int nBlobs) {
    const float e = 0.001;
    float3 n = float3(
        sdf_chrome(p + float3(e, 0.0, 0.0), u, nBlobs) - sdf_chrome(p - float3(e, 0.0, 0.0), u, nBlobs),
        sdf_chrome(p + float3(0.0, e, 0.0), u, nBlobs) - sdf_chrome(p - float3(0.0, e, 0.0), u, nBlobs),
        sdf_chrome(p + float3(0.0, 0.0, e), u, nBlobs) - sdf_chrome(p - float3(0.0, 0.0, e), u, nBlobs)
    );
    float len = length(n);
    return len > 1e-6 ? n / len : float3(0.0, 1.0, 0.0);
}

static float3 envChromeR2(float3 r, constant VisualUniforms& u) {
    float phi = atan2(r.z, r.x);
    float y = r.y;
    float3 env = float3(0.0);

    // Sparse vertical studio softboxes. The environment is black everywhere else.
    for (int i = 0; i < 3; i++) {
        float fi = float(i);
        float a = -1.95 + fi * 1.95 + 0.20 * sin(u.time * 0.22 + fi * 2.3);
        float ad = abs(atan2(sin(phi - a), cos(phi - a)));
        float strip = exp(-ad * ad * 260.0);
        strip *= smoothstep(-0.82, -0.20, y) * (1.0 - smoothstep(0.56, 0.86, y));
        env += palVisual(0.08 + fi * 0.24 + 0.09 * u.spectralCentroid, u) * strip * 1.8;
    }

    // A thin low horizon line, warmed by kick. Never let it become a filled band.
    float horizon = exp(-abs(y + 0.10) * 110.0);
    env += mix(float3(0.0, 0.72, 1.0), float3(1.55, 0.24, 0.02), u.bassImpact)
         * horizon * (0.55 + 1.45 * u.bassImpact);

    // Sparse pin lights for treble/flux.
    float sector = floor((phi / 6.2831853 + 0.5) * 42.0);
    float h = hash11(sector * 1.37 + floor(u.beatCount));
    float pinPhi = h * 6.2831853 - 3.14159265;
    float pinY = hash11(sector * 2.91 + 4.0) * 1.5 - 0.75;
    float pinD = abs(atan2(sin(phi - pinPhi), cos(phi - pinPhi)));
    float pin = exp(-pinD * pinD * 2200.0) * exp(-pow(y - pinY, 2.0) * 80.0);
    env += peakWhite(palVisual(h + 0.2 * u.spectralCentroid, u), pin * u.flux * 0.15)
         * pin * u.flux * (1.0 + u.treble * 2.5);

    return env;
}

fragment float4 frag_chrome(VOut in [[stage_in]],
                            constant VisualUniforms& u [[buffer(0)]],
                            texture2d<float> uImage [[texture(0)]],
                            texture2d<float> uPrev [[texture(1)]],
                            sampler uSamp [[sampler(0)]]) {
    float2 uv = motionCentered(in.uv, u);

    int nBlobs = int(mix(3.0, 7.0, u.param0));
    float reflStrength = mix(0.15, 0.55, u.param1);
    float rippleAmt = mix(0.0, 0.16, u.param2);

    float3 ro = float3(0.0, 0.10, 3.15);
    float3 rd = normalize(float3(uv * 0.86, -1.75));

    float t = 0.0;
    bool hit = false;
    for (int i = 0; i < 86; i++) {
        float3 p = ro + rd * t;
        float d = sdf_chrome(p, u, nBlobs);
        if (d < 0.001) {
            hit = true;
            break;
        }
        t += d;
        if (t > 12.0) break;
    }

    float3 col = float3(0.0);

    if (hit) {
        float3 pos = ro + rd * t;
        float3 nor = calcNormal_chrome(pos, u, nBlobs);

        float ripplePhase = length(pos.xy) * 11.0 - u.time * 4.6 + u.beatPhase * 6.2831853;
        float3 tangentPerturb = float3(
            sin(ripplePhase + pos.z * 3.0),
            cos(ripplePhase + pos.x * 2.6),
            sin(ripplePhase * 0.7 + pos.y * 4.0)
        );
        float ripple = sin(ripplePhase) * (0.25 + u.treble) * rippleAmt;
        float3 nPerturbed = normalize(nor + tangentPerturb * ripple);

        float3 refl = reflect(rd, nPerturbed);
        float3 env = envChromeR2(refl, u) * reflStrength;

        float NoV = max(dot(nPerturbed, -rd), 0.0);
        float fres = pow(1.0 - NoV, 4.8);

        float3 baseMetal = float3(0.006, 0.008, 0.011);
        float3 darkReflection = env * (0.25 + 0.55 * (1.0 - fres));
        float3 rim = mix(float3(0.0, 0.74, 1.05), float3(1.55, 0.24, 0.02), u.bassImpact)
                   * fres * (0.65 + 2.35 * u.bassImpact);

        float3 glints = float3(0.0);
        for (int i = 0; i < 8; i++) {
            float fi = float(i);
            float2 h = hash22(float2(fi * 1.7, floor(u.beatCount) + 2.0));
            float3 dir = normalize(float3(
                cos(h.x * 6.2831853) * sqrt(max(0.0, 1.0 - h.y * h.y)),
                h.y * 2.0 - 1.0,
                sin(h.x * 6.2831853) * sqrt(max(0.0, 1.0 - h.y * h.y))
            ));
            float spec = pow(max(dot(reflect(-dir, nPerturbed), -rd), 0.0), 80.0);
            glints += peakWhite(palVisual(h.x + u.spectralCentroid * 0.2, u), spec * 0.16)
                    * spec * (u.flux + u.treble * 0.35) * 4.0;
        }

        float3 base = baseMetal;
        float3 glow = darkReflection + rim + glints;
        float3 trail = feedbackTrail(glow, uPrev, uSamp, in.uv,
                                     min(u.trailDecay, 0.50),
                                     0.002 + 0.004 * u.bassImpact,
                                     0.0008 * u.beat);
        col = base + trail;
    }

    return float4(col, 1.0);
}
