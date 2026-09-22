package com.beamfall.mobile

import com.beamfall.kit.ClientBuildInfo

internal fun packagedClientBuildInfo(): ClientBuildInfo =
    ClientBuildInfo(
        appVersion = BuildConfig.VERSION_NAME,
        appBuildID = BuildConfig.VERSION_CODE.toString(),
    )
