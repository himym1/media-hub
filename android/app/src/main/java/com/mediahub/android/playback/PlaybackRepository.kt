package com.mediahub.android.playback

import com.mediahub.android.core.auth.SecureSessionStore
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.DownloadedFile
import com.mediahub.android.core.network.MediaHubApi
import com.mediahub.android.core.network.MediaHubHttpClient
import java.io.File
import java.net.URI
import org.json.JSONObject

interface PlaybackRepository {
    suspend fun createDescriptor(request: PlaybackRequest): PlaybackDescriptor
    suspend fun fetchLocalSubtitle(itemId: String): DownloadedFile?
    suspend fun reportSession(sessionId: String, event: PlaybackSessionEvent, positionMs: Long, paused: Boolean)
}

enum class PlaybackSessionEvent(val wireValue: String) {
    Started("started"),
    Progress("progress"),
    Stopped("stopped"),
}

sealed interface PlaybackTarget {
    val key: String
}

data class Drive115Target(
    val parentId: String,
    val fileId: String,
) : PlaybackTarget {
    override val key: String get() = "drive115:$parentId:$fileId"
}

data class EmbyItemTarget(
    val itemId: String,
) : PlaybackTarget {
    override val key: String get() = "emby:$itemId"
}

data class PlaybackFallback(
    val appUrl: String?,
    val webUrl: String?,
)

data class PlaybackRequest(
    val target: PlaybackTarget,
    val title: String,
    val serverIdentity: String,
    val fallback: PlaybackFallback? = null,
) {
    val mediaId: String get() = "$serverIdentity:${target.key}"
}

data class PlaybackDescriptor(
    val streamUrl: String,
    val userAgent: String,
    val title: String,
    val expiresAt: String?,
    val sessionId: String? = null,
    val startPositionMs: Long = 0,
    val subtitle: LocalSubtitleFile? = null,
)

class NetworkPlaybackRepository(
    private val http: MediaHubHttpClient,
    private val sessionStore: SecureSessionStore,
    private val cacheDir: File,
) : PlaybackRepository {
    private val api = MediaHubApi(http)

    override suspend fun createDescriptor(request: PlaybackRequest): PlaybackDescriptor {
        val authorization = sessionStore.load()
            ?: throw ApiException(401, "authentication_required", "需要登录")
        val (path, body) = when (val target = request.target) {
            is Drive115Target -> "/api/v1/playback/descriptors/drive115" to JSONObject()
                .put("parentId", target.parentId)
                .put("fileId", target.fileId)
                .toString()
            is EmbyItemTarget -> "/api/v1/playback/descriptors/emby" to JSONObject()
                .put("itemId", target.itemId)
                .toString()
        }
        val descriptor = parsePlaybackDescriptor(http.request(
            path = path,
            method = "POST",
            body = body,
            token = authorization,
        ))
        val subtitle = (request.target as? EmbyItemTarget)?.let { item ->
            runCatching { cacheLocalSubtitle(item.itemId) }.getOrNull()
        }
        return descriptor.copy(subtitle = subtitle)
    }

    override suspend fun fetchLocalSubtitle(itemId: String): DownloadedFile? {
        val authorization = sessionStore.load()
            ?: throw ApiException(401, "authentication_required", "需要登录")
        return api.fetchLocalSubtitle(authorization, itemId)
    }

    internal suspend fun cacheLocalSubtitle(itemId: String): LocalSubtitleFile? {
        val downloaded = fetchLocalSubtitle(itemId) ?: return null
        return writeLocalSubtitleCache(cacheDir, itemId, downloaded.bytes, downloaded.contentType, downloaded.fileName)
    }

    override suspend fun reportSession(
        sessionId: String,
        event: PlaybackSessionEvent,
        positionMs: Long,
        paused: Boolean,
    ) {
        require(sessionId.matches(Regex("^[a-f0-9]{48}$")))
        val authorization = sessionStore.load()
            ?: throw ApiException(401, "authentication_required", "需要登录")
        http.request(
            path = "/api/v1/playback/sessions/$sessionId/events",
            method = "POST",
            body = JSONObject()
                .put("event", event.wireValue)
                .put("positionMs", positionMs.coerceAtLeast(0L))
                .put("paused", paused)
                .toString(),
            token = authorization,
        )
    }
}

internal fun parsePlaybackDescriptor(body: String): PlaybackDescriptor {
    val value = JSONObject(body)
    val streamUrl = value.getString("streamUrl").trim()
    val uri = URI(streamUrl)
    require(uri.scheme == "https" && !uri.host.isNullOrBlank() && uri.userInfo == null && uri.fragment == null) {
        "Invalid playback URL"
    }
    val userAgent = value.getString("userAgent").trim()
    require(userAgent.isNotEmpty() && userAgent.length <= 512 && '\n' !in userAgent && '\r' !in userAgent) {
        "Invalid playback User-Agent"
    }
    return PlaybackDescriptor(
        streamUrl = streamUrl,
        userAgent = userAgent,
        title = value.getString("title").trim().ifEmpty { "视频" },
        expiresAt = value.optString("expiresAt").trim().ifEmpty { null },
        sessionId = value.optString("sessionId").trim().takeIf { it.matches(Regex("^[a-f0-9]{48}$")) },
        startPositionMs = value.optLong("startPositionMs", 0L).coerceAtLeast(0L),
    )
}
