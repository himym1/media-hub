package com.mediahub.android.feature.library

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.Captions
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Lucide
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubPreferenceRow
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
import com.mediahub.android.core.designsystem.MediaHubTabRow
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.network.EmbyEpisode
import com.mediahub.android.core.network.EmbyItem

@Composable
internal fun EpisodePicker(
    episodes: List<EmbyEpisode>,
    loading: Boolean,
    errorMessage: String?,
    seriesTitle: String = "",
    onPlay: (EmbyEpisode) -> Unit,
    onSearchSubtitles: (EmbyEpisode) -> Unit = {},
) {
    val seasons = remember(episodes) { episodes.map { displaySeason(it.item.season) }.distinct().sorted() }
    var selectedSeason by remember(seasons) { mutableIntStateOf(seasons.firstOrNull() ?: 0) }
    val visibleEpisodes = remember(episodes, selectedSeason) {
        episodes.filter { displaySeason(it.item.season) == selectedSeason }
            .sortedBy { displayEpisodeNumber(it.item) }
    }

    Column(modifier = Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        MediaHubSmallTitle(text = "选集")
        when {
            loading -> MediaHubText(text = "正在读取剧集列表…", color = MediaHubColors.TextMuted, fontSize = 13.sp)
            errorMessage != null -> Row(verticalAlignment = Alignment.CenterVertically) {
                MediaHubIcon(Lucide.CircleAlert, contentDescription = null, tint = MediaHubColors.Error, modifier = Modifier.size(16.dp))
                MediaHubText(text = errorMessage, modifier = Modifier.padding(start = 8.dp), color = MediaHubColors.Error, fontSize = 12.sp)
            }
            episodes.isEmpty() -> MediaHubText(text = "Emby 暂未提供可播放分集", color = MediaHubColors.TextMuted, fontSize = 13.sp)
            else -> {
                if (seasons.size > 1) {
                    MediaHubTabRow(
                        options = seasons.map { season ->
                            season.toString() to if (season > 0) "第 ${season} 季" else "特别篇"
                        },
                        selected = selectedSeason.toString(),
                        onSelected = { selectedSeason = it.toInt() },
                    )
                }
                MediaHubCard {
                    visibleEpisodes.forEachIndexed { index, episode ->
                        if (index > 0) MediaHubListDivider()
                        val progress = episodeProgressLabel(episode)
                        val playLabel = "${playbackActionLabel(episode.item)} ${episodeLabel(episode, seriesTitle)}"
                        MediaHubPreferenceRow(
                            title = episodeLabel(episode, seriesTitle),
                            summary = progress.takeIf { it.isNotEmpty() },
                            contentDescription = playLabel,
                            onClick = { onPlay(episode) },
                            end = {
                                MediaHubIconButton(
                                    imageVector = Lucide.Captions,
                                    contentDescription = "搜中文字幕 ${episodeLabel(episode, seriesTitle)}",
                                    onClick = { onSearchSubtitles(episode) },
                                )
                            },
                        )
                    }
                }
            }
        }
    }
}

internal fun displaySeason(season: Int): Int = if (season in 1900..2100) 1 else season.coerceAtLeast(0)

internal fun displayEpisodeNumber(item: EmbyItem): Int {
    if (item.episode > 0) return item.episode
    return episodeNumberFromName(item.name)
}

internal fun episodeNumberFromName(name: String): Int {
    Regex("""(?i)S(\d{1,2})E(\d{1,3})(?:[^A-Za-z0-9]|$)""").find(name)?.groupValues?.getOrNull(2)
        ?.toIntOrNull()?.takeIf { it > 0 }?.let { return it }
    Regex("""(?i)(?:^|[^A-Za-z0-9])E(\d{1,3})(?:[^A-Za-z0-9]|$)""").find(name)?.groupValues?.getOrNull(1)
        ?.toIntOrNull()?.takeIf { it > 0 }?.let { return it }
    return Regex("""第\s*([0-9]+)\s*集""").find(name)?.groupValues?.getOrNull(1)?.toIntOrNull() ?: 0
}

internal fun looksLikeReleaseName(name: String): Boolean =
    releaseNameToken.containsMatchIn(name) || releaseYearEpisode.containsMatchIn(name)

internal fun episodeTitle(name: String, seriesTitle: String = ""): String? {
    val trimmed = name.trim()
    if (trimmed.isEmpty() || looksLikeReleaseName(trimmed) || genericEpisodeName.matches(trimmed)) {
        return null
    }
    if (seriesTitle.isNotBlank() && trimmed.equals(seriesTitle.trim(), ignoreCase = true)) {
        return null
    }
    return trimmed
}

internal fun episodeLabel(episode: EmbyEpisode, seriesTitle: String = ""): String {
    val number = displayEpisodeNumber(episode.item).takeIf { it > 0 }?.let { "第 $it 集" }
    return listOfNotNull(number, episodeTitle(episode.item.name, seriesTitle)).joinToString(" · ").ifBlank { "分集" }
}

private val releaseNameToken = Regex(
    """(?i)(?:UHDTV|WEB-?DL|WEBRip|Blu-?Ray|HDTV|HEVC|x264|x265|\bAVC\b|10bit|8bit|2160p|1080p|720p|480p|HDR10|\bHDR\b|Dolby|TrueHD|Atmos|\bDTS\b|DD[25]\.[01]|\bAAC\b|\d{2}fps)""",
)
private val releaseYearEpisode = Regex(
    """(?i)\b(?:19|20)\d{2}\b.*\bE\d{1,3}\b|\bE\d{1,3}\b.*\b(?:19|20)\d{2}\b""",
)
private val genericEpisodeName = Regex(
    """(?i)^(?:第\s*[0-9一二三四五六七八九十百]+\s*集|(?:episode|ep\.?|e)\s*0*[0-9]+)$""",
)

internal fun episodeProgressLabel(episode: EmbyEpisode): String = when {
    episode.item.played -> "已看"
    episode.item.playbackPositionMs >= 30_000L -> "继续 ${formatPlaybackPosition(episode.item.playbackPositionMs)}"
    else -> ""
}

internal fun formatPlaybackPosition(positionMs: Long): String {
    val totalSeconds = positionMs.coerceAtLeast(0L) / 1_000
    val hours = totalSeconds / 3_600
    val minutes = (totalSeconds % 3_600) / 60
    val seconds = totalSeconds % 60
    return if (hours > 0) "%d:%02d:%02d".format(hours, minutes, seconds) else "%d:%02d".format(minutes, seconds)
}
