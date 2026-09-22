/*
 * Copyright (C) 2016 The Android Open Source Project
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy at http://www.apache.org/licenses/LICENSE-2.0
 */
/* Test-source copy of the Media3 1.10.1 FFmpeg extension API. */
package androidx.media3.decoder.ffmpeg;

import androidx.annotation.Nullable;
import androidx.media3.common.C;
import androidx.media3.common.MimeTypes;

public final class FfmpegLibrary {
    static {
        System.loadLibrary("ffmpegJNI");
    }

    private FfmpegLibrary() {}

    public static boolean isAvailable() {
        return true;
    }

    @Nullable
    public static String getVersion() {
        return ffmpegGetVersion();
    }

    public static int getInputBufferPaddingSize() {
        int size = ffmpegGetInputBufferPaddingSize();
        return size == C.LENGTH_UNSET ? 0 : size;
    }

    public static boolean supportsFormat(String mimeType) {
        String codecName = getCodecName(mimeType);
        return codecName != null && ffmpegHasDecoder(codecName);
    }

    @Nullable
    static String getCodecName(String mimeType) {
        switch (mimeType) {
            case MimeTypes.AUDIO_AC3:
                return "ac3";
            case MimeTypes.AUDIO_E_AC3:
            case MimeTypes.AUDIO_E_AC3_JOC:
                return "eac3";
            case MimeTypes.AUDIO_DTS:
            case MimeTypes.AUDIO_DTS_HD:
                return "dca";
            case MimeTypes.AUDIO_TRUEHD:
                return "truehd";
            default:
                return null;
        }
    }

    public static native String ffmpegGetVersion();

    private static native int ffmpegGetInputBufferPaddingSize();

    public static native boolean ffmpegHasDecoder(String codecName);
}
