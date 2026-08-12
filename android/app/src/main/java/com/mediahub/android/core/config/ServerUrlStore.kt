package com.mediahub.android.core.config

import android.content.Context

class ServerUrlStore(context: Context) {
    private val preferences = context.getSharedPreferences(PREFERENCES_NAME, Context.MODE_PRIVATE)

    fun load(defaultValue: String): String = preferences.getString(KEY_SERVER_URL, null)
        ?.trim()
        ?.takeIf(String::isNotEmpty)
        ?: defaultValue.trim()

    fun save(value: String) {
        preferences.edit().putString(KEY_SERVER_URL, value.trimEnd('/')).apply()
    }

    fun clear() {
        preferences.edit().remove(KEY_SERVER_URL).apply()
    }

    private companion object {
        const val PREFERENCES_NAME = "media_hub_configuration"
        const val KEY_SERVER_URL = "server_url"
    }
}
