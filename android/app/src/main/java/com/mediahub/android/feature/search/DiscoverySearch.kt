package com.mediahub.android.feature.search

import com.mediahub.android.core.network.DiscoveryItem
import com.mediahub.android.core.network.SearchCandidate

internal fun discoverySearchQuery(item: DiscoveryItem): String {
    val title = item.title.trim()
    if (title.isEmpty()) return ""
    return if (item.year > 0) "$title ${item.year}" else title
}

internal fun discoveryFocusSubtitle(item: DiscoveryItem): String {
    val typeLabel = when (item.mediaType) {
        "tv", "series" -> "剧集"
        else -> "电影"
    }
    return if (item.year > 0) "${item.year} · $typeLabel" else typeLabel
}

internal fun prioritizeDiscoveryResults(
    results: List<SearchCandidate>,
    item: DiscoveryItem,
): List<SearchCandidate> {
    if (results.isEmpty()) return results
    val wantSeries = item.mediaType == "tv" || item.mediaType == "series"
    return results.sortedWith(
        compareByDescending<SearchCandidate> { candidate ->
            var score = 0
            if (!candidate.tmdbId.isNullOrBlank() && candidate.tmdbId == item.tmdbId) score += 100
            val isSeries = candidate.mediaType == "tv" || candidate.mediaType == "series"
            if (wantSeries == isSeries) score += 20
            if (item.year > 0 && candidate.year == item.year) score += 10
            if (candidate.transferToken != null) score += 5
            if (candidate.transferState == "available" || candidate.transferState == "downloadable") score += 3
            score
        }.thenBy { it.title },
    )
}

internal fun discoveryResultsHeading(
    searching: Boolean,
    focusTitle: String,
    resultCount: Int,
): String = when {
    searching && focusTitle.isNotBlank() -> "正在查找《$focusTitle》可获取版本…"
    searching -> "正在搜索…"
    focusTitle.isNotBlank() -> "《$focusTitle》可获取版本 ($resultCount)"
    else -> "搜索结果 $resultCount"
}
