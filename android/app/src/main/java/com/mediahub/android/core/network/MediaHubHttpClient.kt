package com.mediahub.android.core.network

import com.mediahub.android.BuildConfig
import java.io.ByteArrayOutputStream
import java.io.IOException
import java.io.InputStream
import java.net.HttpURLConnection
import java.net.URL
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONObject

class MediaHubHttpClient(baseUrl: String) {
    val baseUrl: String = baseUrl.trimEnd('/').also { value ->
        val parsed = URL(value)
        require(parsed.protocol == "https" || (BuildConfig.DEBUG && parsed.protocol == "http")) {
            "Release builds require an HTTPS Media Hub API URL"
        }
        require(parsed.userInfo == null && parsed.query == null && parsed.ref == null) {
            "Media Hub API URL must not contain credentials, query, or fragment"
        }
    }

    suspend fun request(
        path: String,
        method: String = "GET",
        body: String? = null,
        token: String? = null,
        headers: Map<String, String> = emptyMap(),
    ): String = withContext(Dispatchers.IO) {
        require(path.startsWith('/')) { "Media Hub API path must be absolute" }
        val connection = URL(baseUrl + path).openConnection() as HttpURLConnection
        try {
            connection.requestMethod = method
            connection.connectTimeout = CONNECT_TIMEOUT_MS
            connection.readTimeout = READ_TIMEOUT_MS
            connection.instanceFollowRedirects = false
            connection.setRequestProperty("Accept", "application/json")
            connection.setRequestProperty("User-Agent", "Media-Hub-Android/${BuildConfig.VERSION_NAME}")
            if (token != null) connection.setRequestProperty("Authorization", "Bearer $token")
            headers.forEach(connection::setRequestProperty)
            if (body != null) {
                connection.doOutput = true
                connection.setRequestProperty("Content-Type", "application/json")
                connection.outputStream.use { output -> output.write(body.toByteArray(Charsets.UTF_8)) }
            }
            val status = connection.responseCode
            val responseBody = (if (status in 200..299) connection.inputStream else connection.errorStream)
                .readLimited(MAX_RESPONSE_BYTES)
            if (status !in 200..299) throw parseError(status, responseBody)
            responseBody
        } finally {
            connection.disconnect()
        }
    }

    suspend fun requestBytes(
        path: String,
        token: String,
        maxBytes: Int = 2 * 1024 * 1024,
    ): ByteArray = withContext(Dispatchers.IO) {
        require(path.startsWith('/')) { "Media Hub API path must be absolute" }
        val connection = URL(baseUrl + path).openConnection() as HttpURLConnection
        try {
            connection.requestMethod = "GET"
            connection.connectTimeout = CONNECT_TIMEOUT_MS
            connection.readTimeout = READ_TIMEOUT_MS
            connection.instanceFollowRedirects = false
            connection.setRequestProperty("Accept", "image/webp,image/png,image/jpeg")
            connection.setRequestProperty("Authorization", "Bearer $token")
            connection.setRequestProperty("User-Agent", "Media-Hub-Android/${BuildConfig.VERSION_NAME}")
            val status = connection.responseCode
            if (status !in 200..299) {
                val problem = connection.errorStream.readLimited(MAX_RESPONSE_BYTES)
                throw parseError(status, problem)
            }
            connection.inputStream.readBytesLimited(maxBytes)
        } finally {
            connection.disconnect()
        }
    }

    private fun parseError(status: Int, body: String): ApiException {
        val payload = runCatching { JSONObject(body) }.getOrNull()
        return ApiException(
            status = status,
            code = payload?.optString("code")?.takeIf(String::isNotBlank) ?: "request_failed",
            message = payload?.optString("title")?.takeIf(String::isNotBlank) ?: "请求失败",
        )
    }

    private fun InputStream.readBytesLimited(limit: Int): ByteArray = use { input ->
        val output = ByteArrayOutputStream()
        val buffer = ByteArray(8192)
        var total = 0
        while (true) {
            val count = input.read(buffer)
            if (count < 0) break
            total += count
            if (total > limit) throw IOException("Media Hub image response exceeded size limit")
            output.write(buffer, 0, count)
        }
        output.toByteArray()
    }

    private fun InputStream?.readLimited(limit: Int): String {
        if (this == null) return ""
        return use { input ->
            val output = ByteArrayOutputStream()
            val buffer = ByteArray(8192)
            var total = 0
            while (true) {
                val count = input.read(buffer)
                if (count < 0) break
                total += count
                if (total > limit) throw IOException("Media Hub API response exceeded size limit")
                output.write(buffer, 0, count)
            }
            output.toString(Charsets.UTF_8.name())
        }
    }

    private companion object {
        const val CONNECT_TIMEOUT_MS = 5_000
        const val READ_TIMEOUT_MS = 15_000
        const val MAX_RESPONSE_BYTES = 4 * 1024 * 1024
    }
}
