// Beamfall libmpv JNI bridge — Engine 2 of the ADR-0017 §3 ladder.
//
// This binds the *modern* mpv client + render API (client.h + render.h/render_gl.h, mpv >= 0.29)
// rather than the deprecated opengl_cb path. It is the C half of the `LibmpvPlaybackSurface`
// contract: create/initialize the player, load a Stream-Token range URL, drive playback transport,
// select an embedded subtitle track (the reason this rung exists — libass fidelity), and render
// frames into a GL framebuffer supplied by the Android `GLSurfaceView` renderer.
//
// IMPORTANT BUILD STATE: this translation unit only compiles when `libmpv.so` (and its headers)
// are vendored for the target ABI and the kit is built with `-Pbeamfall.libmpv=true`. Without the
// header set, the kit's default build skips native compilation entirely (CMake guards on it), so
// `LibmpvClient.isAvailable()` stays honestly false and the gate stays green. See
// kit/src/main/cpp/CMakeLists.txt and kit/scripts/build_libmpv.sh.

#include <jni.h>
#include <stdlib.h>
#include <string.h>
#include <pthread.h>
#include <android/log.h>

#include <mpv/client.h>
#include <mpv/render.h>
#include <mpv/render_gl.h>

#define LOG_TAG "BeamfallMpv"
#define LOGE(...) __android_log_print(ANDROID_LOG_ERROR, LOG_TAG, __VA_ARGS__)
#define LOGI(...) __android_log_print(ANDROID_LOG_INFO, LOG_TAG, __VA_ARGS__)

// One native context per LibmpvPlaybackSurface instance. The Kotlin side holds the opaque handle.
typedef struct {
    mpv_handle *mpv;
    mpv_render_context *render;
    JavaVM *vm;
    jobject update_cb; // global ref to a Runnable invoked when a new frame is ready (GLSurfaceView.requestRender)
} bf_ctx;

static void *get_proc_address(void *ctx, const char *name) {
    // The GL function loader is supplied from the Kotlin/GL side via a registered resolver. The
    // EGL-backed implementation lives in the Java renderer; here we forward to eglGetProcAddress
    // which is the conventional Android resolver for a GLSurfaceView EGL context.
    (void) ctx;
    extern void *eglGetProcAddress(const char *procname);
    return eglGetProcAddress(name);
}

static void on_mpv_render_update(void *cb_ctx) {
    bf_ctx *c = (bf_ctx *) cb_ctx;
    if (!c || !c->update_cb) return;
    JNIEnv *env = NULL;
    int attached = 0;
    if ((*c->vm)->GetEnv(c->vm, (void **) &env, JNI_VERSION_1_6) == JNI_EDETACHED) {
        if ((*c->vm)->AttachCurrentThread(c->vm, &env, NULL) != 0) return;
        attached = 1;
    }
    jclass runnable = (*env)->GetObjectClass(env, c->update_cb);
    jmethodID run = (*env)->GetMethodID(env, runnable, "run", "()V");
    if (run) (*env)->CallVoidMethod(env, c->update_cb, run);
    (*env)->DeleteLocalRef(env, runnable);
    if (attached) (*c->vm)->DetachCurrentThread(c->vm);
}

// --- JNI entry points (declared as external functions on LibmpvNativeBridge) ---

JNIEXPORT jlong JNICALL
Java_com_beamfall_kit_LibmpvNativeBridge_nativeCreate(JNIEnv *env, jclass clazz) {
    (void) clazz;
    bf_ctx *c = calloc(1, sizeof(bf_ctx));
    if (!c) return 0;
    (*env)->GetJavaVM(env, &c->vm);
    c->mpv = mpv_create();
    if (!c->mpv) {
        free(c);
        return 0;
    }
    // Decode-to-PCM posture and libass: ADR-0017 §3/§4. Hardware decode where the SoC allows it.
    mpv_set_option_string(c->mpv, "hwdec", "auto-safe");
    mpv_set_option_string(c->mpv, "vo", "libmpv");
    mpv_set_option_string(c->mpv, "ao", "audiotrack");
    mpv_set_option_string(c->mpv, "sub-ass", "yes"); // libass styling — the load-bearing reason for Engine 2
    if (mpv_initialize(c->mpv) < 0) {
        mpv_terminate_destroy(c->mpv);
        free(c);
        return 0;
    }
    return (jlong) (intptr_t) c;
}

JNIEXPORT jint JNICALL
Java_com_beamfall_kit_LibmpvNativeBridge_nativeInitRender(JNIEnv *env, jclass clazz, jlong handle,
                                                          jobject update_cb) {
    (void) clazz;
    bf_ctx *c = (bf_ctx *) (intptr_t) handle;
    if (!c || !c->mpv) return -1;
    c->update_cb = (*env)->NewGlobalRef(env, update_cb);

    mpv_opengl_init_params gl_init = {.get_proc_address = get_proc_address, .get_proc_address_ctx = c};
    mpv_render_param params[] = {
        {MPV_RENDER_PARAM_API_TYPE, (void *) MPV_RENDER_API_TYPE_OPENGL},
        {MPV_RENDER_PARAM_OPENGL_INIT_PARAMS, &gl_init},
        {0},
    };
    int rc = mpv_render_context_create(&c->render, c->mpv, params);
    if (rc < 0) {
        LOGE("mpv_render_context_create failed: %s (%d)", mpv_error_string(rc), rc);
        // LIBMPV-002: the global ref taken above must not outlive this failed init, or every retry
        // (and the eventual nativeDestroy, which never runs because the caller treats this handle as
        // dead) leaks one JNI global ref.
        if (c->update_cb) {
            (*env)->DeleteGlobalRef(env, c->update_cb);
            c->update_cb = NULL;
        }
        return rc;
    }
    mpv_render_context_set_update_callback(c->render, on_mpv_render_update, c);
    return 0;
}

JNIEXPORT jint JNICALL
Java_com_beamfall_kit_LibmpvNativeBridge_nativeRender(JNIEnv *env, jclass clazz, jlong handle,
                                                      jint fbo, jint width, jint height) {
    (void) env;
    (void) clazz;
    bf_ctx *c = (bf_ctx *) (intptr_t) handle;
    if (!c || !c->render) return -1;
    mpv_opengl_fbo gl_fbo = {.fbo = (int) fbo, .w = (int) width, .h = (int) height};
    int flip_y = 1;
    mpv_render_param params[] = {
        {MPV_RENDER_PARAM_OPENGL_FBO, &gl_fbo},
        {MPV_RENDER_PARAM_FLIP_Y, &flip_y},
        {0},
    };
    return mpv_render_context_render(c->render, params);
}

JNIEXPORT jint JNICALL
Java_com_beamfall_kit_LibmpvNativeBridge_nativeCommand(JNIEnv *env, jclass clazz, jlong handle,
                                                       jobjectArray args) {
    (void) clazz;
    bf_ctx *c = (bf_ctx *) (intptr_t) handle;
    if (!c || !c->mpv) return -1;
    jsize n = (*env)->GetArrayLength(env, args);
    const char **cargs = calloc((size_t) n + 1, sizeof(char *));
    if (!cargs) return -2;
    // Keep the jstring local refs so we can pair each ReleaseStringUTFChars with the exact element
    // its chars came from (a fresh GetObjectArrayElement could return a different ref / NULL).
    jstring *elems = calloc((size_t) n, sizeof(jstring));
    if (!elems) {
        free(cargs);
        return -2;
    }
    // BL-P1-14 (RV-ANDUI-008): GetObjectArrayElement may yield NULL, and GetStringUTFChars returns
    // NULL with a pending OOM exception. Either case must bail with cleanup rather than handing a
    // NULL-containing argv to mpv_command (UB) or leaving a pending JNI exception across the call.
    int failed = 0;
    for (jsize i = 0; i < n; i++) {
        elems[i] = (jstring) (*env)->GetObjectArrayElement(env, args, i);
        if (elems[i] == NULL) {
            (*env)->ExceptionClear(env);
            failed = 1;
            break;
        }
        cargs[i] = (*env)->GetStringUTFChars(env, elems[i], NULL);
        if (cargs[i] == NULL) {
            (*env)->ExceptionClear(env);
            failed = 1;
            break;
        }
    }
    int rc;
    if (failed) {
        rc = -3;
    } else {
        rc = mpv_command(c->mpv, cargs);
    }
    for (jsize i = 0; i < n; i++) {
        if (elems[i] == NULL) {
            continue;
        }
        if (cargs[i] != NULL) {
            (*env)->ReleaseStringUTFChars(env, elems[i], cargs[i]);
        }
        (*env)->DeleteLocalRef(env, elems[i]);
    }
    free(elems);
    free(cargs);
    return rc;
}

JNIEXPORT jint JNICALL
Java_com_beamfall_kit_LibmpvNativeBridge_nativeSetOption(JNIEnv *env, jclass clazz, jlong handle,
                                                         jstring name, jstring value) {
    (void) clazz;
    bf_ctx *c = (bf_ctx *) (intptr_t) handle;
    if (!c || !c->mpv) return -1;
    // LIBMPV-001: GetStringUTFChars returns NULL (with a pending OOM exception) under memory
    // pressure. Passing that NULL straight into mpv_set_option_string is UB and has caused a SIGSEGV;
    // bail out and clear the pending exception instead.
    const char *n = (*env)->GetStringUTFChars(env, name, NULL);
    if (n == NULL) {
        (*env)->ExceptionClear(env);
        return -3;
    }
    const char *v = (*env)->GetStringUTFChars(env, value, NULL);
    if (v == NULL) {
        (*env)->ExceptionClear(env);
        (*env)->ReleaseStringUTFChars(env, name, n);
        return -3;
    }
    int rc = mpv_set_option_string(c->mpv, n, v);
    (*env)->ReleaseStringUTFChars(env, name, n);
    (*env)->ReleaseStringUTFChars(env, value, v);
    return rc;
}

JNIEXPORT jdouble JNICALL
Java_com_beamfall_kit_LibmpvNativeBridge_nativeGetPropertyDouble(JNIEnv *env, jclass clazz,
                                                                 jlong handle, jstring name) {
    (void) clazz;
    bf_ctx *c = (bf_ctx *) (intptr_t) handle;
    if (!c || !c->mpv) return 0.0;
    // LIBMPV-001: same NULL-under-OOM hazard as nativeSetOption — a NULL name would otherwise reach
    // mpv_get_property and SIGSEGV.
    const char *n = (*env)->GetStringUTFChars(env, name, NULL);
    if (n == NULL) {
        (*env)->ExceptionClear(env);
        return 0.0;
    }
    double out = 0.0;
    mpv_get_property(c->mpv, n, MPV_FORMAT_DOUBLE, &out);
    (*env)->ReleaseStringUTFChars(env, name, n);
    return out;
}

JNIEXPORT jstring JNICALL
Java_com_beamfall_kit_LibmpvNativeBridge_nativeGetPropertyString(JNIEnv *env, jclass clazz,
                                                                 jlong handle, jstring name) {
    (void) clazz;
    bf_ctx *c = (bf_ctx *) (intptr_t) handle;
    if (!c || !c->mpv) return NULL;
    // LIBMPV-001 discipline: GetStringUTFChars returns NULL (pending OOM exception) under memory
    // pressure; bail and clear rather than passing NULL into mpv_get_property.
    const char *n = (*env)->GetStringUTFChars(env, name, NULL);
    if (n == NULL) {
        (*env)->ExceptionClear(env);
        return NULL;
    }
    char *value = NULL;
    int rc = mpv_get_property(c->mpv, n, MPV_FORMAT_STRING, &value);
    (*env)->ReleaseStringUTFChars(env, name, n);
    if (rc < 0 || value == NULL) {
        return NULL;
    }
    jstring out = (*env)->NewStringUTF(env, value);
    mpv_free(value); // MPV_FORMAT_STRING allocates; the caller owns and must free it.
    return out;
}

JNIEXPORT void JNICALL
Java_com_beamfall_kit_LibmpvNativeBridge_nativeDestroy(JNIEnv *env, jclass clazz, jlong handle) {
    (void) clazz;
    bf_ctx *c = (bf_ctx *) (intptr_t) handle;
    if (!c) return;
    if (c->render) {
        mpv_render_context_free(c->render);
        c->render = NULL;
    }
    if (c->mpv) {
        mpv_terminate_destroy(c->mpv);
        c->mpv = NULL;
    }
    if (c->update_cb) {
        (*env)->DeleteGlobalRef(env, c->update_cb);
        c->update_cb = NULL;
    }
    free(c);
}
