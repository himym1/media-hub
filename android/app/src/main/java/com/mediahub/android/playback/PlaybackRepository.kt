package com.mediahub.android.playback

import com.mediahub.android.core.auth.SecureSessionStore
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.MediaHubHttpClient
import java.net.URI
import org.json.JSONObject

interface PlaybackRepository {
    suspend fun createDescriptor(request: PlaybackRequest): PlaybackDescriptor
}

data class PlaybackRequest(
    val parentId: String,
    val fileId: String,
    val title: String,
    val serverIdentity: String,
) {
    val mediaId: String get() = "$serverIdentity:$parentId:$fileId"
}

data class PlaybackDescriptor(
    val streamUrl: String,
    val userAgent: String,
    val title: String,
    val expiresAt: String?,
)

class NetworkPlaybackRepository(
    private val http: MediaHubHttpClient,
    private val sessionStore: SecureSessionStore,
) : PlaybackRepository {
    override suspend fun createDescriptor(request: PlaybackRequest): PlaybackDescriptor {
        val authorization = sessionStore.load()
            ?: throw ApiException(401, "authentication_required", "需要登录")
        val body = JSONObject()
            .put("parentId", request.parentId)
            .put("fileId", request.fileId)
            .toString()
        return parsePlaybackDescriptor(http.request(
            path = "/api/v1/playback/descriptors",
            method = "POST",
            body = body,
            token = authorization,
        ))
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
        title = value.getString("title").trim().ifEmpty { "115 视频" },
        expiresAt = value.optString("expiresAt").trim().ifEmpty { null },
    )
}
