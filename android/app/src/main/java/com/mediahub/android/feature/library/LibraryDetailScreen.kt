package com.mediahub.android.feature.library

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.ArrowLeft
import com.composables.icons.lucide.Captions
import com.composables.icons.lucide.ChevronDown
import com.composables.icons.lucide.ChevronUp
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Play
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.Trash2
import com.composables.icons.lucide.X
import com.mediahub.android.app.LocalTwoPane
import com.mediahub.android.core.designsystem.BadgeVariant
import com.mediahub.android.core.designsystem.MediaHubBadge
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubConfirmDialog
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubPreferenceRow
import com.mediahub.android.core.designsystem.MediaHubSecondaryButton
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTextButton
import com.mediahub.android.core.designsystem.MediaHubTopAppBar
import com.mediahub.android.core.image.PosterLoader
import com.mediahub.android.core.network.EmbyItemDetail
import com.mediahub.android.core.network.EmbyItem
import com.mediahub.android.core.network.EmbyRemoteSubtitle
import com.mediahub.android.feature.subtitles.RemoteSubtitleUiState
import com.mediahub.android.playback.PlaybackFallback
import java.net.URI

@Composable
internal fun LibraryDetailScreen(
    state: LibraryDetailState,
    subtitleState: RemoteSubtitleUiState,
    actions: LibraryDetailActions,
    posterLoader: PosterLoader,
) {
    val twoPane = LocalTwoPane.current
    Column(modifier = Modifier.fillMaxSize()) {
        MediaHubTopAppBar(
            title = state.item?.item?.name ?: "媒体详情",
            navigationIcon = {
                if (!twoPane) {
                    MediaHubIconButton(Lucide.ArrowLeft, "返回媒体列表", actions.onClose)
                }
            },
            actions = {
                MediaHubIconButton(
                    imageVector = Lucide.RefreshCw,
                    contentDescription = "刷新媒体元数据",
                    enabled = !state.refreshing,
                    onClick = actions.onRefresh,
                )
            },
        )
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 36.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            state.errorMessage?.let { item { DetailErrorLine(it) } }
            state.actionMessage?.let {
                item { MediaHubText(text = it, color = MediaHubColors.Source, fontSize = 12.sp) }
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
                            seriesTitle = detail.item.name,
                            onPlay = { episode ->
                                actions.onPlayItem(
                                    episode.item,
                                    PlaybackFallback(episode.appUrl, episode.externalUrl),
                                )
                            },
                            onSearchSubtitles = { episode ->
                                actions.onSearchSubtitles(episode.item.id, episodeLabel(episode, detail.item.name))
                            },
                        )
                    }
                } else {
                    item {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.spacedBy(10.dp),
                        ) {
                            MediaHubButton(
                                label = playbackActionLabel(detail.item),
                                icon = Lucide.Play,
                                onClick = { actions.onPlayItem(detail.item, fallback) },
                                modifier = Modifier.weight(1f),
                            )
                            MediaHubSecondaryButton(
                                label = if (subtitleState.searching && subtitleState.targetId == detail.item.id) {
                                    "搜索中…"
                                } else {
                                    "字幕"
                                },
                                icon = Lucide.Captions,
                                enabled = !subtitleState.searching && subtitleState.downloadingId == null && !state.refreshing,
                                onClick = { actions.onSearchSubtitles(detail.item.id, detail.item.name) },
                                modifier = Modifier.weight(1f),
                            )
                        }
                    }
                }
                if (subtitleState.targetId != null || subtitleState.remoteSubtitles.isNotEmpty() || subtitleState.searching) {
                    item {
                        RemoteSubtitlePanel(
                            state = subtitleState,
                            onDownload = actions.onDownloadSubtitle,
                            onClear = actions.onClearSubtitles,
                        )
                    }
                }
                item {
                    MediaHubSmallTitle(text = "剧情简介")
                    MediaHubText(
                        text = detail.overview ?: "暂未提供简介。",
                        color = MediaHubColors.TextSecondary,
                        fontSize = 14.sp,
                    )
                }
                item { TechnicalDetails(detail) }
                item { DeleteActions(state, actions) }
            }
        }
    }
}

@Composable
private fun RemoteSubtitlePanel(
    state: RemoteSubtitleUiState,
    onDownload: (String) -> Unit,
    onClear: () -> Unit,
) {
    Column(modifier = Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubSmallTitle(
                text = state.targetLabel?.let { "中文字幕 · $it" } ?: "中文字幕",
                modifier = Modifier.weight(1f),
            )
            MediaHubIconButton(Lucide.X, "关闭字幕结果", onClear)
        }
        state.message?.let {
            MediaHubText(text = it, color = MediaHubColors.TextMuted, fontSize = 12.sp)
        }
        if (state.searching) {
            MediaHubText(text = "正在搜索中文字幕…", color = MediaHubColors.TextMuted, fontSize = 13.sp)
        }
        if (state.remoteSubtitles.isNotEmpty()) {
            MediaHubCard {
                state.remoteSubtitles.forEachIndexed { index, subtitle ->
                    if (index > 0) MediaHubListDivider()
                    RemoteSubtitleRow(
                        subtitle = subtitle,
                        downloading = state.downloadingId == subtitle.id,
                        enabled = state.downloadingId == null,
                        onDownload = { onDownload(subtitle.id) },
                    )
                }
            }
        }
    }
}

@Composable
private fun RemoteSubtitleRow(
    subtitle: EmbyRemoteSubtitle,
    downloading: Boolean,
    enabled: Boolean,
    onDownload: () -> Unit,
) {
    val meta = listOfNotNull(
        subtitle.format.takeIf { it.isNotBlank() }?.uppercase(),
        subtitle.providerName.takeIf { it.isNotBlank() },
        if (subtitle.isHashMatch) "精确匹配" else null,
        if (subtitle.downloadCount > 0) "${subtitle.downloadCount} 次下载" else null,
    ).joinToString(" · ")
    MediaHubPreferenceRow(
        title = subtitle.name,
        summary = meta.takeIf { it.isNotBlank() },
        end = {
            MediaHubTextButton(
                label = if (downloading) "下载中…" else "下载",
                enabled = enabled,
                onClick = onDownload,
            )
        },
    )
}

@Composable
private fun DetailIdentity(detail: EmbyItemDetail, posterLoader: PosterLoader) {
    val genres = detail.genres.map(::genreLabel).filter(String::isNotBlank).distinct()
    val facts = listOfNotNull(
        mediaTypeLabel(detail.item.type),
        detail.item.year?.toString(),
        detail.runtimeMinutes?.takeIf { it > 0 }?.let { "$it 分钟" },
    )
    val rating = detail.communityRating?.takeIf { it > 0.0 }
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(16.dp),
        verticalAlignment = Alignment.Top,
    ) {
        Box(
            modifier = Modifier
                .width(112.dp)
                .clip(RoundedCornerShape(14.dp)),
        ) {
            EmbyPoster(
                itemId = detail.item.id,
                loader = posterLoader,
                contentDescription = "${detail.item.name} 封面",
                modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f),
            )
        }
        Column(
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            MediaHubText(
                text = detail.item.name,
                fontSize = 20.sp,
                fontWeight = FontWeight.SemiBold,
                color = MediaHubColors.TextStrong,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            if (facts.isNotEmpty()) {
                MediaHubText(
                    text = facts.joinToString(" · "),
                    color = MediaHubColors.TextMuted,
                    fontSize = 13.sp,
                )
            }
            if (rating != null) {
                MediaHubBadge(
                    text = "★ %.1f".format(rating),
                    variant = BadgeVariant.Warning,
                )
            }
            if (genres.isNotEmpty()) {
                MediaHubText(
                    text = genres.take(3).joinToString(" · "),
                    color = MediaHubColors.TextSecondary,
                    fontSize = 12.sp,
                )
            }
        }
    }
}

@Composable
private fun DeleteActions(state: LibraryDetailState, actions: LibraryDetailActions) {
    val preview = state.deletePreview
    Column(modifier = Modifier.fillMaxWidth()) {
        MediaHubTextButton(
            label = if (state.deleting) "正在删除…" else "从 Emby 删除",
            icon = Lucide.Trash2,
            destructive = true,
            enabled = !state.deleting && !state.refreshing,
            onClick = actions.onDelete,
            modifier = Modifier.fillMaxWidth(),
        )
        MediaHubConfirmDialog(
            visible = preview != null,
            title = "确认从 Emby 删除？",
            message = if (preview != null) {
                val versions = if (preview.versionCount > 1) "的 ${preview.versionCount} 个版本" else ""
                val series = if (preview.type == "Series") "及全部分集" else ""
                "将从 Emby 移除「${preview.name}」$versions$series。\n\nNAS 上约 ${preview.fileCount} 个 STRM 和同名字幕会被清掉，115 网盘上的原始文件不会被删除。本机字幕缓存也会一并删除。"
            } else "",
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
        MediaHubPreferenceRow(
            title = "媒体信息",
            summary = if (expanded) null else "原名、TMDB 与媒体源",
            modifier = Modifier.semantics { stateDescription = if (expanded) "已展开" else "已收起" },
            onClick = { expanded = !expanded },
            end = {
                MediaHubIcon(
                    if (expanded) Lucide.ChevronUp else Lucide.ChevronDown,
                    contentDescription = if (expanded) "收起" else "展开",
                    tint = MediaHubColors.TextMuted,
                    modifier = Modifier.size(18.dp),
                )
            },
        )
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
