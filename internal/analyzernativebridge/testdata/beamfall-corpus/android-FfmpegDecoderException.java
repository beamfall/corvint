/*
 * Copyright (C) 2016 The Android Open Source Project
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy at http://www.apache.org/licenses/LICENSE-2.0
 */
package androidx.media3.decoder.ffmpeg;

import androidx.media3.decoder.DecoderException;

final class FfmpegDecoderException extends DecoderException {
    FfmpegDecoderException(String message) {
        super(message);
    }

    FfmpegDecoderException(String message, Throwable cause) {
        super(message, cause);
    }
}
