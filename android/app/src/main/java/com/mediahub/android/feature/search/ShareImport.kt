package com.mediahub.android.feature.search

import java.net.URI

private val shareHosts = setOf("115.com", "www.115.com", "115cdn.com", "www.115cdn.com", "anxia.com", "www.anxia.com")
private val otherCloudHosts = setOf(
    "pan.quark.cn",
    "www.pan.quark.cn",
    "aliyundrive.com",
    "www.aliyundrive.com",
    "alipan.com",
    "www.alipan.com",
    "pan.baidu.com",
    "www.pan.baidu.com",
    "yun.baidu.com",
    "123pan.com",
    "www.123pan.com",
)
private val videoExt = Regex("\\.(mp4|mkv|avi|ts|m2ts|mts|wmv|flv|mov|iso|rmvb|rm|m4v|webm|mpeg|mpg|vob|f4v|asf|3gp)$", RegexOption.IGNORE_CASE)

internal fun canSubmitShareImport(url: String, receiveCode: String = ""): Boolean {
    val raw = url.trim()
    if (raw.isEmpty() || raw.length > 8192) return false
    val lower = raw.lowercase()
    if (lower.startsWith("magnet:")) return lower.contains("xt=urn:btih:")
    if (lower.startsWith("ed2k://")) return raw.length >= 16
    return try {
        val parsed = URI(if (raw.contains("://")) raw else "https://${raw.removePrefix("//")}")
        if (!parsed.userInfo.isNullOrEmpty()) return false
        val scheme = parsed.scheme?.lowercase()
        if (scheme != "http" && scheme != "https") return false
        val host = parsed.host?.lowercase() ?: return false
        if (host in otherCloudHosts) return false
        if (host in shareHosts) {
            val segments = parsed.path.orEmpty().trim('/').split('/')
            if (segments.size >= 2 && segments[0] == "s" && segments[1].matches(Regex("^[A-Za-z0-9]{4,64}$"))) {
                val queryCode = parsed.query.orEmpty()
                    .split('&')
                    .mapNotNull { pair ->
                        val parts = pair.split('=', limit = 2)
                        if (parts.size == 2 && (parts[0] == "password" || parts[0] == "pwd")) parts[1] else null
                    }
                    .firstOrNull()
                    .orEmpty()
                val code = queryCode.ifBlank { receiveCode.trim() }
                return code.matches(Regex("^[A-Za-z0-9]{0,8}$"))
            }
            return videoExt.containsMatchIn(parsed.path.orEmpty())
        }
        true
    } catch (_: Exception) {
        false
    }
}
