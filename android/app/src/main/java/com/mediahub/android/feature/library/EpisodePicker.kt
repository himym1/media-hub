package com.mediahub.android.feature.library

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Play
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubFilterChip
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.network.EmbyEpisode

@Composable
internal fun EpisodePicker(
    episodes: List<EmbyEpisode>,
    loading: Boolean,
    errorMessage: String?,
    onPlay: (EmbyEpisode) -> Unit,
) {
    val seasons = remember(episodes) { episodes.map { it.item.season.coerceAtLeast(0) }.distinct().sorted() }
    var selectedSeason by remember(seasons) { mutableIntStateOf(seasons.firstOrNull() ?: 0) }
    val visibleEpisodes = remember(episodes, selectedSeason) { episodes.filter { it.item.season == selectedSeason } }

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
                LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    items(seasons, key = { it }) { season ->
                        MediaHubFilterChip(
                            label = if (season > 0) "第 ${season} 季" else "特别篇",
                            selected = season == selectedSeason,
                            onClick = { selectedSeason = season },
                            role = Role.Tab,
                        )
                    }
                }
                MediaHubCard {
                    visibleEpisodes.forEachIndexed { index, episode ->
                        if (index > 0) MediaHubListDivider()
                        val progress = episodeProgressLabel(episode)
                        val playLabel = "${playbackActionLabel(episode.item)} ${episodeLabel(episode)}"
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .heightIn(min = 48.dp)
                                .clickable(role = Role.Button) { onPlay(episode) }
                                .semantics { contentDescription = playLabel }
                                .padding(horizontal = 14.dp, vertical = 12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Column(Modifier.weight(1f)) {
                                MediaHubText(
                                    text = episodeLabel(episode),
                                    color = MediaHubColors.TextPrimary,
                                    fontSize = 13.sp,
                                    fontWeight = FontWeight.Medium,
                                )
                                if (progress.isNotEmpty()) {
                                    MediaHubText(
                                        text = progress,
                                        color = MediaHubColors.Accent,
                                        fontSize = 12.sp,
                                        modifier = Modifier.padding(top = 2.dp),
                                    )
                                }
                            }
                            MediaHubIcon(
                                imageVector = Lucide.Play,
                                contentDescription = null,
                                tint = MediaHubColors.Accent,
                                modifier = Modifier.size(18.dp),
                            )
                        }
                    }
                }
            }
        }
    }
}

internal fun episodeLabel(episode: EmbyEpisode): String {
    val item = episode.item
    val number = item.episode.takeIf { it > 0 }?.let { "第 $it 集" }
    return listOfNotNull(number, item.name.takeIf(String::isNotBlank)).joinToString(" · ").ifBlank { "分集" }
}

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
