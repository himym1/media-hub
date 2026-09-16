package com.mediahub.android.feature.search

import java.net.URI

private val shareHosts = setOf("115.com", "www.115.com", "115cdn.com", "www.115cdn.com", "anxia.com", "www.anxia.com")

internal fun canSubmitShareImport(url: String, receiveCode: String = ""): Boolean {
    val raw = url.trim()
    if (raw.isEmpty()) return false
    return try {
        val parsed = URI(if (raw.contains("://")) raw else "https://${raw.removePrefix("//")}")
        val scheme = parsed.scheme?.lowercase()
        if (scheme != "http" && scheme != "https") return false
        val host = parsed.host?.lowercase() ?: return false
        if (host !in shareHosts) return false
        val segments = parsed.path.orEmpty().trim('/').split('/')
        if (segments.size < 2 || segments[0] != "s" || !segments[1].matches(Regex("^[A-Za-z0-9]{4,64}$"))) {
            return false
        }
        val queryCode = parsed.query.orEmpty()
            .split('&')
            .mapNotNull { pair ->
                val parts = pair.split('=', limit = 2)
                if (parts.size == 2 && (parts[0] == "password" || parts[0] == "pwd")) parts[1] else null
            }
            .firstOrNull()
            .orEmpty()
        val code = queryCode.ifBlank { receiveCode.trim() }
        code.matches(Regex("^[A-Za-z0-9]{0,8}$"))
    } catch (_: Exception) {
        false
    }
}
