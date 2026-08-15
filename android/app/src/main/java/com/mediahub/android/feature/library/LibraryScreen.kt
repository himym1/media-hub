package com.mediahub.android.feature.library

import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.widget.Toast
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
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
import com.composables.icons.lucide.BookOpen
import com.composables.icons.lucide.ChevronDown
import com.composables.icons.lucide.ChevronLeft
import com.composables.icons.lucide.ChevronRight
import com.composables.icons.lucide.ChevronUp
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.ExternalLink
import com.composables.icons.lucide.Film
import com.composables.icons.lucide.LibraryBig
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.Play
import com.composables.icons.lucide.X
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubSearchField
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.network.EmbyItem
import com.mediahub.android.core.network.EmbyItemDetail
import java.net.URI


@Composable
internal fun LibraryRoute(viewModel: LibraryViewModel) {
    val uiState by viewModel.uiState.collectAsState()
    LaunchedEffect(viewModel) { viewModel.refreshLibraries() }
    BackHandler(enabled = uiState.selectedItemId != null, onBack = viewModel::closeItem)
    if (uiState.selectedItemId != null) {
        LibraryDetailScreen(
            state = uiState,
            onBack = viewModel::closeItem,
            onRefresh = viewModel::refreshSelectedItem,
        )
    } else {
        LibraryScreen(
            uiState = uiState,
            onQueryChanged = viewModel::onQueryChanged,
            onSearch = viewModel::search,
            onClearSearch = viewModel::clearSearch,
            onRefreshLibraries = viewModel::refreshLibraries,
            onRefreshSelectedLibrary = viewModel::refreshSelectedLibrary,
            onSelectLibrary = viewModel::selectLibrary,
            onSelectItem = viewModel::selectItem,
            onChangePage = viewModel::changePage,
        )
    }
}

@Composable
internal fun LibraryScreen(
    uiState: LibraryUiState,
    onQueryChanged: (String) -> Unit,
    onSearch: () -> Unit,
    onClearSearch: () -> Unit,
    onRefreshLibraries: () -> Unit,
    onRefreshSelectedLibrary: () -> Unit,
    onSelectLibrary: (String) -> Unit,
    onSelectItem: (String) -> Unit,
    onChangePage: (Int) -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MediaHubColors.Canvas)
            .padding(horizontal = 16.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(
                text = if (uiState.loadingLibraries) "正在读取媒体库…" else "${uiState.libraries.size} 个媒体库",
                modifier = Modifier.weight(1f),
                color = MediaHubColors.TextMuted,
                fontSize = 13.sp,
            )
            MediaHubIconButton(
                imageVector = Lucide.RefreshCw,
                contentDescription = "刷新媒体库列表",
                enabled = !uiState.loadingLibraries,
                onClick = onRefreshLibraries,
            )
        }
        if (uiState.libraries.isNotEmpty()) {
            LazyRow(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                contentPadding = PaddingValues(bottom = 12.dp),
            ) {
                items(uiState.libraries, key = { it.id }) { library ->
                    val selected = library.id == uiState.selectedLibraryId
                    val collection = collectionLabel(library.collectionType)
                    Row(
                        modifier = Modifier
                            .heightIn(min = 48.dp)
                            .background(if (selected) MediaHubColors.SurfaceSelected else MediaHubColors.Surface, RoundedCornerShape(8.dp))
                            .selectable(selected = selected, role = Role.RadioButton) { onSelectLibrary(library.id) }
                            .padding(horizontal = 12.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        MediaHubIcon(Lucide.LibraryBig, contentDescription = null, modifier = Modifier.size(17.dp))
                        Spacer(Modifier.width(8.dp))
                        Column {
                            MediaHubText(text = library.name, fontSize = 13.sp, fontWeight = FontWeight.Medium)
                            if (!library.name.equals(collection, ignoreCase = true)) {
                                MediaHubText(text = collection, color = MediaHubColors.TextMuted, fontSize = 12.sp)
                            }
                        }
                    }
                }
            }
        }
        Row(verticalAlignment = Alignment.CenterVertically) {
            MediaHubSearchField(
                value = uiState.query,
                onValueChange = onQueryChanged,
                onSearch = onSearch,
                enabled = !uiState.loadingItems,
                placeholder = "搜索电影或剧集",
                modifier = Modifier.weight(1f),
            )
            if (uiState.submittedQuery.isNotEmpty()) {
                Spacer(Modifier.width(8.dp))
                MediaHubIconButton(Lucide.X, "清除搜索", onClearSearch)
            }
        }
        uiState.errorMessage?.let { ErrorLine(it) }
        uiState.actionMessage?.let {
            MediaHubText(text = it, modifier = Modifier.padding(top = 10.dp), color = MediaHubColors.Source, fontSize = 12.sp)
        }
        Row(
            modifier = Modifier.fillMaxWidth().padding(top = 16.dp, bottom = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(
                text = if (uiState.submittedQuery.isNotEmpty()) "搜索结果 ${uiState.total}" else "媒体内容 ${uiState.total}",
                modifier = Modifier.weight(1f),
                color = MediaHubColors.TextStrong,
                fontSize = 15.sp,
                fontWeight = FontWeight.Medium,
            )
            if (uiState.submittedQuery.isEmpty() && uiState.selectedLibraryId != null) {
                MediaHubIconButton(Lucide.RefreshCw, "刷新当前媒体库", onRefreshSelectedLibrary, enabled = !uiState.refreshing)
            }
        }
        LazyColumn(
            modifier = Modifier.weight(1f),
            contentPadding = PaddingValues(bottom = 16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            items(uiState.items, key = { it.id }) { item ->
                EmbyItemRow(item = item, onClick = { onSelectItem(item.id) })
            }
            if (!uiState.loadingItems && uiState.items.isEmpty()) {
                item(key = "empty") {
                    Column(
                        modifier = Modifier.fillMaxWidth().padding(vertical = 54.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                    ) {
                        MediaHubIcon(Lucide.BookOpen, contentDescription = null, modifier = Modifier.size(28.dp))
                        MediaHubText(
                            text = if (uiState.submittedQuery.isNotEmpty()) "没有匹配的媒体" else "此媒体库暂无内容",
                            modifier = Modifier.padding(top = 12.dp),
                            color = MediaHubColors.TextMuted,
                            fontSize = 13.sp,
                        )
                    }
                }
            }
            if (uiState.submittedQuery.isEmpty() && uiState.total > LibraryPageSize) {
                item(key = "pagination") {
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
                        horizontalArrangement = Arrangement.Center,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        MediaHubIconButton(Lucide.ChevronLeft, "上一页", { onChangePage(-1) }, enabled = uiState.page > 0 && !uiState.loadingItems)
                        MediaHubText(
                            text = "${uiState.page + 1} / ${libraryPageCount(uiState.total)}",
                            modifier = Modifier.padding(horizontal = 14.dp),
                            color = MediaHubColors.TextMuted,
                            fontSize = 12.sp,
                        )
                        MediaHubIconButton(Lucide.ChevronRight, "下一页", { onChangePage(1) }, enabled = (uiState.page + 1) * LibraryPageSize < uiState.total && !uiState.loadingItems)
                    }
                }
            }
        }
    }
}

@Composable
internal fun LibraryDetailScreen(state: LibraryUiState, onBack: () -> Unit, onRefresh: () -> Unit) {
    val context = LocalContext.current
    val selectedAppUrl = state.selectedItem?.appUrl
    val opensInApp = remember(context, selectedAppUrl) { canOpenEmbyApp(context, selectedAppUrl) }
    LazyColumn(
        modifier = Modifier.fillMaxSize().background(MediaHubColors.Canvas),
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 24.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        item {
            Row(
                modifier = Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubIconButton(Lucide.ArrowLeft, "返回媒体列表", onBack)
                MediaHubText(text = "媒体详情", modifier = Modifier.weight(1f), fontSize = 21.sp, fontWeight = FontWeight.SemiBold)
            }
            state.errorMessage?.let { ErrorLine(it) }
            state.actionMessage?.let { MediaHubText(text = it, color = MediaHubColors.Source, fontSize = 12.sp) }
        }
        if (state.loadingDetail) {
            item { MediaHubText(text = "正在读取媒体详情…", color = MediaHubColors.TextMuted, fontSize = 13.sp) }
        }
        state.selectedItem?.let { detail ->
            item { DetailIdentity(detail) }
            item {
                MediaHubText(
                    text = detail.overview ?: "暂未提供简介。",
                    color = MediaHubColors.TextSecondary,
                    fontSize = 13.sp,
                )
            }
            item { TechnicalDetails(detail) }
            item {
                MediaHubButton(
                    label = if (state.refreshing) "已提交刷新" else "刷新元数据",
                    icon = Lucide.RefreshCw,
                    enabled = !state.refreshing,
                    onClick = onRefresh,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            item {
                MediaHubButton(
                    label = embyOpenLabel(opensInApp),
                    icon = if (opensInApp) Lucide.Play else Lucide.ExternalLink,
                    onClick = { openEmby(context, detail) },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }
    }
}

@Composable
private fun DetailIdentity(detail: EmbyItemDetail) {
    val genres = detail.genres.map(::genreLabel).filter(String::isNotBlank).distinct()
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        MediaHubText(text = detail.item.name, fontSize = 22.sp, fontWeight = FontWeight.SemiBold)
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

@Composable
private fun TechnicalDetails(detail: EmbyItemDetail) {
    var expanded by remember(detail.item.id) { mutableStateOf(false) }
    val originalTitle = visibleOriginalTitle(detail)
    Column(modifier = Modifier.fillMaxWidth().background(MediaHubColors.Surface, RoundedCornerShape(8.dp))) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(min = 48.dp)
                .semantics { stateDescription = if (expanded) "已展开" else "已收起" }
                .clickable(role = Role.Button) { expanded = !expanded }
                .padding(horizontal = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(text = "更多信息", modifier = Modifier.weight(1f), fontSize = 13.sp, fontWeight = FontWeight.Medium)
            MediaHubIcon(if (expanded) Lucide.ChevronUp else Lucide.ChevronDown, contentDescription = null, modifier = Modifier.size(17.dp))
        }
        if (expanded) {
            Column(
                modifier = Modifier.fillMaxWidth().padding(start = 12.dp, end = 12.dp, bottom = 12.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                originalTitle?.let { DetailTechnicalLine("原名", it) }
                DetailTechnicalLine("TMDB 编号", detail.item.tmdbId ?: "未绑定")
                DetailTechnicalLine("媒体源", if (detail.item.type == "Series") "由分集提供" else "${detail.mediaSourceCount} 个")
            }
        }
    }
}

@Composable
private fun DetailTechnicalLine(label: String, value: String) {
    MediaHubText(text = "$label：$value", color = MediaHubColors.TextSecondary, fontSize = 12.sp)
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

internal fun embyOpenLabel(appAvailable: Boolean): String =
    if (appAvailable) "在 Emby App 中打开" else "打开 Emby 网页"

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

private fun mediaTypeLabel(type: String): String = when (type) {
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

@Composable
private fun EmbyItemRow(item: EmbyItem, onClick: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 64.dp)
            .background(MediaHubColors.Surface, RoundedCornerShape(8.dp))
            .clickable(role = Role.Button, onClick = onClick)
            .padding(14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MediaHubIcon(imageVector = Lucide.Film, contentDescription = null, modifier = Modifier.size(17.dp))
        Spacer(Modifier.width(10.dp))
        Column(Modifier.weight(1f)) {
            MediaHubText(text = item.name, fontSize = 14.sp, fontWeight = FontWeight.Medium)
            item.year?.let {
                MediaHubText(
                    text = it.toString(),
                    modifier = Modifier.padding(top = 4.dp),
                    color = MediaHubColors.TextMuted,
                    fontSize = 12.sp,
                )
            }
        }
        MediaHubText(text = mediaTypeLabel(item.type), color = MediaHubColors.Accent, fontSize = 12.sp)
    }
}

@Composable
private fun ErrorLine(message: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MediaHubIcon(Lucide.CircleAlert, contentDescription = null, tint = MediaHubColors.Error, modifier = Modifier.size(16.dp))
        MediaHubText(text = message, modifier = Modifier.padding(start = 8.dp), color = MediaHubColors.Error, fontSize = 12.sp)
    }
}

private fun collectionLabel(type: String?): String = when (type) {
    "movies" -> "电影"
    "tvshows" -> "剧集"
    else -> "媒体"
}
