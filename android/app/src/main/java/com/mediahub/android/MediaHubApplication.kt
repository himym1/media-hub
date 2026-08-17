package com.mediahub.android

import android.app.Application
import com.mediahub.android.app.AppContainer

class MediaHubApplication : Application() {
    val container: AppContainer by lazy(LazyThreadSafetyMode.SYNCHRONIZED) { AppContainer(this) }
}
