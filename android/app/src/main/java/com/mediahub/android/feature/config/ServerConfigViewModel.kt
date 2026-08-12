package com.mediahub.android.feature.config

import android.net.Uri
import androidx.lifecycle.ViewModel
import com.mediahub.android.core.config.ServerUrlStore
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

class ServerConfigViewModel(
    private val store: ServerUrlStore,
    initialValue: String,
) : ViewModel() {
    private val _serverUrl = MutableStateFlow(initialValue)
    val serverUrl: StateFlow<String> = _serverUrl.asStateFlow()

    fun update(value: String) {
        _serverUrl.value = value.take(2048)
    }

    fun save(): String? {
        val value = _serverUrl.value.trim().trimEnd('/')
        val uri = runCatching { Uri.parse(value) }.getOrNull()
        if (uri?.scheme != "https" || uri.host.isNullOrBlank() || uri.userInfo != null || uri.query != null || uri.fragment != null) {
            return null
        }
        store.save(value)
        return value
    }
}
