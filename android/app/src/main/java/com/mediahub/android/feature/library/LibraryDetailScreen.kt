package com.mediahub.android.feature.library

import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.widget.Toast
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
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
import androidx.compose.ui.platform.LocalContext
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
import com.composables.icons.lucide.ExternalLink
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Play
import com.composables.icons.lucide.RefreshCw
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubSecondaryButton
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.image.PosterLoader
import com.mediahub.android.core.network.EmbyItemDetail
import com.mediahub.android.playback.PlaybackFallback
import java.net.URI

@Composable
internal fun LibraryDetailScreen(state: LibraryDetailState, actions: LibraryDetailActions, posterLoader: PosterLoader) {
    val context = LocalContext.current
    val selectedAppUrl = state.item?.appUrl
    val opensInApp = remember(context, selectedAppUrl) { canOpenEmbyApp(context, selectedAppUrl) }
    LazyColumn(
        modifier = Modifier.fillMaxSize().background(MediaHubColors.Canvas),
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 32.dp),
        verticalArrangement = Arrangement.spacedBy(18.dp),
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
                    fontSize = 19.sp,
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
                        label = "直接播放",
                        icon = Lucide.Play,
                        onClick = { actions.onPlayItem(detail.item, fallback) },
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
            item {
                MediaHubSecondaryButton(
                    label = embyPlayLabel(opensInApp),
                    icon = Lucide.ExternalLink,
                    onClick = { openEmby(context, detail) },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            item {
                MediaHubText(
                    text = detail.overview ?: "暂未提供简介。",
                    color = MediaHubColors.TextSecondary,
                    fontSize = 14.sp,
                )
            }
            item { TechnicalDetails(detail) }
        }
    }
}

@Composable
private fun DetailIdentity(detail: EmbyItemDetail, posterLoader: PosterLoader) {
    val genres = detail.genres.map(::genreLabel).filter(String::isNotBlank).distinct()
    Column(modifier = Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(14.dp)) {
        EmbyPoster(
            itemId = detail.item.id,
            loader = posterLoader,
            contentDescription = "${detail.item.name} 封面",
            modifier = Modifier.width(144.dp).align(Alignment.CenterHorizontally),
        )
        Column(modifier = Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
            MediaHubText(text = detail.item.name, fontSize = 24.sp, fontWeight = FontWeight.SemiBold)
            MediaHubText(
                text = listOfNotNull(
                    mediaTypeLabel(detail.item.type),
                    detail.item.year?.toString(),
                    detail.runtimeMinutes?.takeIf { it > 0 }?.let { "$it 分钟" },
                    detail.communityRating?.let { "评分 %.1f".format(it) },
                ).joinToString(" · "),
                color = MediaHubColors.TextSecondary,
                fontSize = 13.sp,
            )
            if (genres.isNotEmpty()) {
                MediaHubText(text = genres.joinToString(" · "), color = MediaHubColors.Accent, fontSize = 12.sp)
            }
        }
    }
}

@Composable
private fun TechnicalDetails(detail: EmbyItemDetail) {
    var expanded by remember(detail.item.id) { mutableStateOf(false) }
    Column(modifier = Modifier.fillMaxWidth()) {
        Box(Modifier.fillMaxWidth().height(1.dp).background(MediaHubColors.Border))
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(min = 48.dp)
                .semantics { stateDescription = if (expanded) "已展开" else "已收起" }
                .clickable(role = Role.Button) { expanded = !expanded },
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
            Column(
                modifier = Modifier.fillMaxWidth().padding(bottom = 14.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                visibleOriginalTitle(detail)?.let { DetailTechnicalLine("原名", it) }
                DetailTechnicalLine("TMDB 编号", detail.item.tmdbId ?: "未绑定")
                DetailTechnicalLine("媒体源", if (detail.item.type == "Series") "由分集提供" else "${detail.mediaSourceCount} 个")
            }
        }
        Box(Modifier.fillMaxWidth().height(1.dp).background(MediaHubColors.Border))
    }
}

@Composable
private fun DetailTechnicalLine(label: String, value: String) {
    MediaHubText(text = "$label：$value", color = MediaHubColors.TextSecondary, fontSize = 12.sp)
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

private const val EmbyAndroidPackage = "com.mb.android"
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

internal fun embyPlayLabel(appAvailable: Boolean): String =
    if (appAvailable) "使用 Emby 播放" else "在 Emby 网页中播放"

internal fun canOpenEmbyApp(context: Context, appUrl: String?): Boolean {
    val validatedUrl = validatedEmbyAppUrl(appUrl) ?: return false
    return try {
        embyAppIntent(validatedUrl).resolveActivity(context.packageManager) != null
    } catch (_: SecurityException) {
        false
    }
}

private fun openEmby(context: Context, detail: EmbyItemDetail) {
    val appUrl = validatedEmbyAppUrl(detail.appUrl)
    if (appUrl != null && canOpenEmbyApp(context, appUrl) && tryStartActivity(context, embyAppIntent(appUrl))) return

    val webUrl = validatedEmbyWebUrl(detail.externalUrl)
    if (webUrl == null || !tryStartActivity(context, Intent(Intent.ACTION_VIEW, Uri.parse(webUrl)))) {
        Toast.makeText(context, "无法打开 Emby", Toast.LENGTH_SHORT).show()
    }
}

private fun embyAppIntent(appUrl: String): Intent =
    Intent(Intent.ACTION_VIEW, Uri.parse(appUrl)).setPackage(EmbyAndroidPackage)

private fun validatedEmbyWebUrl(value: String): String? {
    val parsed = runCatching { URI(value.trim()) }.getOrNull() ?: return null
    if (parsed.host.isNullOrBlank() || (parsed.scheme != "http" && parsed.scheme != "https")) return null
    return parsed.toASCIIString()
}

private fun tryStartActivity(context: Context, intent: Intent): Boolean = try {
    context.startActivity(intent)
    true
} catch (_: ActivityNotFoundException) {
    false
} catch (_: SecurityException) {
    false
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
