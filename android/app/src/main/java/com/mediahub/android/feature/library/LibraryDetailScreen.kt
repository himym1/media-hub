package com.mediahub.android.feature.library

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.ArrowLeft
import com.composables.icons.lucide.ChevronDown
import com.composables.icons.lucide.ChevronUp
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Play
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.Trash2
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubConfirmDialog
import com.mediahub.android.core.designsystem.MediaHubDestructiveButton
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubPreferenceRow
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.image.PosterLoader
import com.mediahub.android.core.network.EmbyItemDetail
import com.mediahub.android.core.network.EmbyItem
import com.mediahub.android.playback.PlaybackFallback
import java.net.URI

@Composable
internal fun LibraryDetailScreen(state: LibraryDetailState, actions: LibraryDetailActions, posterLoader: PosterLoader) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 36.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item {
            Row(
                modifier = Modifier.fillMaxWidth().padding(top = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubIconButton(Lucide.ArrowLeft, "返回媒体列表", actions.onClose)
                MediaHubText(
                    text = "媒体详情",
                    modifier = Modifier.weight(1f).padding(start = 4.dp),
                    fontSize = 18.sp,
                    fontWeight = FontWeight.SemiBold,
                )
                MediaHubIconButton(
                    imageVector = Lucide.RefreshCw,
                    contentDescription = "刷新媒体元数据",
                    enabled = !state.refreshing,
                    onClick = actions.onRefresh,
                )
            }
            state.errorMessage?.let { DetailErrorLine(it) }
            state.actionMessage?.let {
                MediaHubText(text = it, color = MediaHubColors.Source, fontSize = 12.sp)
            }
        }
        if (state.loading) {
            item { MediaHubText(text = "正在读取媒体详情…", color = MediaHubColors.TextMuted, fontSize = 13.sp) }
        }
        state.item?.let { detail ->
            val fallback = PlaybackFallback(detail.appUrl, detail.externalUrl)
            item { DetailIdentity(detail, posterLoader) }
            if (detail.item.type == "Series") {
                item {
                    EpisodePicker(
                        episodes = state.episodes,
                        loading = state.loadingEpisodes,
                        errorMessage = state.episodesError,
                        onPlay = { episode ->
                            actions.onPlayItem(
                                episode.item,
                                PlaybackFallback(episode.appUrl, episode.externalUrl),
                            )
                        },
                    )
                }
            } else {
                item {
                    MediaHubButton(
                        label = playbackActionLabel(detail.item),
                        icon = Lucide.Play,
                        onClick = { actions.onPlayItem(detail.item, fallback) },
                        modifier = Modifier.fillMaxWidth().heightIn(min = 52.dp),
                    )
                }
            }
            item {
                MediaHubSmallTitle(text = "剧情简介")
                MediaHubCard(insideMargin = PaddingValues(16.dp)) {
                    MediaHubText(
                        text = detail.overview ?: "暂未提供简介。",
                        color = MediaHubColors.TextSecondary,
                        fontSize = 13.sp,
                    )
                }
            }
            item { TechnicalDetails(detail) }
            item { DeleteActions(state, actions) }
        }
    }
}

@Composable
private fun DetailIdentity(detail: EmbyItemDetail, posterLoader: PosterLoader) {
    val genres = detail.genres.map(::genreLabel).filter(String::isNotBlank).distinct()
    val facts = listOfNotNull(
        mediaTypeLabel(detail.item.type),
        detail.item.year?.toString(),
        detail.runtimeMinutes?.takeIf { it > 0 }?.let { "$it 分钟" },
        detail.communityRating?.let { "★ %.1f".format(it) },
    )
    MediaHubCard(insideMargin = PaddingValues(18.dp)) {
        Column(
            modifier = Modifier.fillMaxWidth(),
            verticalArrangement = Arrangement.spacedBy(14.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            EmbyPoster(
                itemId = detail.item.id,
                loader = posterLoader,
                contentDescription = "${detail.item.name} 封面",
                modifier = Modifier.width(150.dp),
            )
            MediaHubText(
                text = detail.item.name,
                fontSize = 20.sp,
                fontWeight = FontWeight.Bold,
                color = MediaHubColors.TextPrimary,
            )
            if (facts.isNotEmpty()) {
                MediaHubText(
                    text = facts.joinToString(" · "),
                    color = MediaHubColors.TextSecondary,
                    fontSize = 13.sp,
                    fontWeight = FontWeight.Medium,
                )
            }
            if (genres.isNotEmpty()) {
                MediaHubText(
                    text = genres.joinToString(" · "),
                    color = MediaHubColors.Accent,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.SemiBold,
                )
            }
        }
    }
}

@Composable
private fun DeleteActions(state: LibraryDetailState, actions: LibraryDetailActions) {
    val preview = state.deletePreview
    Column(modifier = Modifier.fillMaxWidth()) {
        MediaHubDestructiveButton(
            label = if (state.deleting) "正在删除…" else "从 Emby 删除",
            icon = Lucide.Trash2,
            enabled = !state.deleting && !state.refreshing,
            onClick = actions.onDelete,
            modifier = Modifier.fillMaxWidth(),
        )
        MediaHubConfirmDialog(
            visible = preview != null,
            title = "确认从 Emby 删除？",
            message = if (preview != null) "将从 Emby 移除「${preview.name}」${if (preview.type == "Series") "及全部分集" else ""}。\n\nNAS 上约 ${preview.fileCount} 个 STRM 库文件将被清理，115 网盘上的原始文件不会被删除。" else "",
            confirmLabel = if (state.deleting) "正在删除…" else "确认删除",
            cancelLabel = "取消",
            isDestructive = true,
            onConfirm = actions.onConfirmDelete,
            onDismiss = actions.onCancelDelete,
        )
    }
}

@Composable
private fun TechnicalDetails(detail: EmbyItemDetail) {
    var expanded by remember(detail.item.id) { mutableStateOf(false) }
    MediaHubCard {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(min = 48.dp)
                .semantics { stateDescription = if (expanded) "已展开" else "已收起" }
                .clickable(role = Role.Button) { expanded = !expanded }
                .padding(horizontal = 14.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(text = "媒体信息", modifier = Modifier.weight(1f), fontSize = 13.sp, fontWeight = FontWeight.Medium)
            MediaHubIcon(
                if (expanded) Lucide.ChevronUp else Lucide.ChevronDown,
                contentDescription = null,
                modifier = Modifier.size(17.dp),
            )
        }
        if (expanded) {
            MediaHubListDivider()
            visibleOriginalTitle(detail)?.let { DetailTechnicalLine("原名", it) }
            DetailTechnicalLine("TMDB 编号", detail.item.tmdbId ?: "未绑定")
            DetailTechnicalLine("媒体源", if (detail.item.type == "Series") "由分集提供" else "${detail.mediaSourceCount} 个")
            Spacer(Modifier.height(8.dp))
        }
    }
}

@Composable
private fun DetailTechnicalLine(label: String, value: String) {
    MediaHubPreferenceRow(title = "$label：$value")
}

@Composable
private fun DetailErrorLine(message: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(top = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MediaHubIcon(Lucide.CircleAlert, contentDescription = null, tint = MediaHubColors.Error, modifier = Modifier.size(16.dp))
        MediaHubText(text = message, modifier = Modifier.padding(start = 8.dp), color = MediaHubColors.Error, fontSize = 12.sp)
    }
}

private val EmbyIdentifierPattern = Regex("^[A-Za-z0-9_-]{1,128}$")

internal fun validatedEmbyAppUrl(value: String?): String? {
    val parsed = runCatching { URI(value?.trim().orEmpty()) }.getOrNull() ?: return null
    if (!parsed.scheme.equals("emby", ignoreCase = true) || !parsed.host.equals("items", ignoreCase = true) ||
        parsed.userInfo != null || parsed.port != -1 || parsed.query != null || parsed.fragment != null
    ) return null
    val segments = parsed.path.orEmpty().split('/').filter(String::isNotEmpty)
    if (segments.size != 2 || segments.any { !EmbyIdentifierPattern.matches(it) }) return null
    return "emby://items/${segments[0]}/${segments[1]}"
}
private fun visibleOriginalTitle(detail: EmbyItemDetail): String? {
    val value = detail.originalTitle?.trim()?.takeIf(String::isNotEmpty) ?: return null
    return value.takeUnless { it.lowercase() == detail.item.name.trim().lowercase() }
}

internal fun mediaTypeLabel(type: String): String = when (type) {
    "Movie" -> "电影"
    "Series" -> "剧集"
    else -> "媒体"
}

private fun genreLabel(genre: String): String = genreLabels[genre.trim().lowercase()] ?: genre.trim()

private val genreLabels = mapOf(
    "action" to "动作",
    "action & adventure" to "动作冒险",
    "adventure" to "冒险",
    "animation" to "动画",
    "biography" to "传记",
    "comedy" to "喜剧",
    "crime" to "犯罪",
    "documentary" to "纪录片",
    "drama" to "剧情",
    "family" to "家庭",
    "fantasy" to "奇幻",
    "film-noir" to "黑色电影",
    "history" to "历史",
    "horror" to "恐怖",
    "kids" to "儿童",
    "music" to "音乐",
    "musical" to "音乐剧",
    "mystery" to "悬疑",
    "news" to "新闻",
    "reality" to "真人秀",
    "romance" to "爱情",
    "sci-fi & fantasy" to "科幻奇幻",
    "science fiction" to "科幻",
    "short" to "短片",
    "soap" to "肥皂剧",
    "sport" to "体育",
    "talk" to "脱口秀",
    "thriller" to "惊悚",
    "tv movie" to "电视电影",
    "war" to "战争",
    "war & politics" to "战争政治",
    "western" to "西部",
)

internal fun playbackActionLabel(item: EmbyItem): String = when {
    item.played -> "重新播放"
    item.playbackPositionMs >= 30_000L -> "继续播放"
    else -> "播放"
}
