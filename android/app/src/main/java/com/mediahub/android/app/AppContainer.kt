package com.mediahub.android.app

import android.content.Context
import com.mediahub.android.BuildConfig
import com.mediahub.android.core.auth.SecureSessionStore
import com.mediahub.android.core.config.ServerUrlStore
import com.mediahub.android.core.image.EmbyPosterLoader
import com.mediahub.android.core.network.MediaHubApi
import com.mediahub.android.core.network.MediaHubHttpClient
import com.mediahub.android.data.DiscoveryCache
import com.mediahub.android.data.MediaHubRepository
import com.mediahub.android.playback.NetworkPlaybackRepository
import com.mediahub.android.playback.PlaybackRepository
import com.mediahub.android.playback.clearLocalSubtitleCache
import com.mediahub.android.playback.subtitleOffsetStore
import java.security.MessageDigest

class AppContainer(context: Context) {
    val serverUrlStore = ServerUrlStore(context)
    val sessionStore = SecureSessionStore(context)
    private val appContext = context.applicationContext
    private val discoveryCache = DiscoveryCache(appContext)

    @Volatile
    private var configured: ConfiguredDependencies? = null
    private var generationCounter = 0L

    fun initialServerUrl(): String = serverUrlStore.load(BuildConfig.API_BASE_URL)

    @Synchronized
    fun configureServer(serverUrl: String): ConfiguredDependencies {
        configured?.takeIf { it.serverUrl == serverUrl.trimEnd('/') }?.let { return it }
        val transport = MediaHubHttpClient(serverUrl)
        return ConfiguredDependencies(
            serverUrl = transport.baseUrl,
            generation = ++generationCounter,
            serverIdentity = serverIdentity(transport.baseUrl),
            repository = MediaHubRepository(MediaHubApi(transport), sessionStore, discoveryCache),
            playbackRepository = NetworkPlaybackRepository(transport, sessionStore),
            posterLoader = EmbyPosterLoader(appContext, transport, sessionStore),
        ).also { configured = it }
    }

    fun requireConfigured(): ConfiguredDependencies = configured
        ?: configureServer(initialServerUrl())

    fun evictItemCaches(itemId: String) {
        clearLocalSubtitleCache(appContext.cacheDir, itemId)
        configured?.posterLoader?.evict(itemId)
        subtitleOffsetStore(appContext).clearItem(itemId)
    }

    @Synchronized
    fun clearConfiguration() {
        sessionStore.clear()
        serverUrlStore.clear()
        discoveryCache.clear()
        generationCounter++
        configured = null
    }
}

data class ConfiguredDependencies(
    val serverUrl: String,
    val generation: Long,
    val serverIdentity: String,
    val repository: MediaHubRepository,
    val playbackRepository: PlaybackRepository,
    val posterLoader: EmbyPosterLoader,
)

private fun serverIdentity(serverUrl: String): String =
    MessageDigest.getInstance("SHA-256")
        .digest(serverUrl.toByteArray(Charsets.UTF_8))
        .joinToString("") { byte -> "%02x".format(byte) }
