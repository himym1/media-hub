package com.mediahub.android.app

import android.content.Context

class AndroidUpdatePromptStore(context: Context) {
    private val preferences = context.applicationContext.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)

    fun dismissedVersionCode(): Int = preferences.getInt(KEY_DISMISSED_VERSION_CODE, 0)

    fun rememberDismissed(versionCode: Int) {
        preferences.edit().putInt(KEY_DISMISSED_VERSION_CODE, versionCode).apply()
    }

    private companion object {
        const val PREFERENCES_NAME = "media_hub_updates"
        const val KEY_DISMISSED_VERSION_CODE = "dismissed_version_code"
    }
}
