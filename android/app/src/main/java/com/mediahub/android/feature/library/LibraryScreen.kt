package com.mediahub.android.feature.library

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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.ArrowLeft
import com.composables.icons.lucide.BookOpen
import com.composables.icons.lucide.ChevronLeft
import com.composables.icons.lucide.ChevronRight
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.ExternalLink
import com.composables.icons.lucide.Film
import com.composables.icons.lucide.LibraryBig
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.X
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubSearchField
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.network.EmbyItem
import com.mediahub.android.core.network.EmbyItemDetail


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
                            MediaHubText(text = collectionLabel(library.collectionType), color = MediaHubColors.TextMuted, fontSize = 12.sp)
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
private fun LibraryDetailScreen(state: LibraryUiState, onBack: () -> Unit, onRefresh: () -> Unit) {
    val uriHandler = LocalUriHandler.current
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
                Column(Modifier.weight(1f)) {
                    MediaHubText(text = "媒体详情", fontSize = 21.sp, fontWeight = FontWeight.SemiBold)
                    MediaHubText(text = "来自 Emby 的安全元数据", color = MediaHubColors.TextMuted, fontSize = 12.sp)
                }
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
                    text = detail.overview ?: "Emby 暂未提供简介。",
                    color = MediaHubColors.TextSecondary,
                    fontSize = 13.sp,
                )
            }
            item {
                Column(
                    modifier = Modifier.fillMaxWidth().background(MediaHubColors.Surface, RoundedCornerShape(8.dp)).padding(14.dp),
                    verticalArrangement = Arrangement.spacedBy(5.dp),
                ) {
                    MediaHubText(
                        text = if (detail.item.type == "Series") "剧集播放由分集媒体源提供" else "已关联 ${detail.mediaSourceCount} 个媒体源",
                        fontSize = 13.sp,
                        fontWeight = FontWeight.Medium,
                    )
                    MediaHubText(text = "Media Hub 不代理或删除媒体文件。", color = MediaHubColors.TextMuted, fontSize = 12.sp)
                }
            }
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
                    label = "在 Emby 中打开",
                    icon = Lucide.ExternalLink,
                    onClick = { uriHandler.openUri(detail.externalUrl) },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }
    }
}

@Composable
private fun DetailIdentity(detail: EmbyItemDetail) {
    Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
        MediaHubText(text = detail.item.name, fontSize = 22.sp, fontWeight = FontWeight.SemiBold)
        detail.originalTitle?.let { MediaHubText(text = it, color = MediaHubColors.TextMuted, fontSize = 12.sp) }
        MediaHubText(
            text = listOfNotNull(
                if (detail.item.type == "Movie") "电影" else "剧集",
                detail.item.year?.toString(),
                detail.item.tmdbId?.let { "TMDB $it" },
                detail.runtimeMinutes?.takeIf { it > 0 }?.let { "$it 分钟" },
                detail.communityRating?.let { "评分 %.1f".format(it) },
            ).joinToString(" · "),
            color = MediaHubColors.TextSecondary,
            fontSize = 13.sp,
        )
        if (detail.genres.isNotEmpty()) {
            MediaHubText(text = detail.genres.joinToString(" · "), color = MediaHubColors.Accent, fontSize = 12.sp)
        }
    }
}

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
            MediaHubText(
                text = listOfNotNull(item.year?.toString(), item.tmdbId?.let { "TMDB $it" }).joinToString(" · "),
                modifier = Modifier.padding(top = 4.dp),
                color = MediaHubColors.TextMuted,
                fontSize = 12.sp,
            )
        }
        MediaHubText(text = if (item.type == "Movie") "电影" else "剧集", color = MediaHubColors.Accent, fontSize = 12.sp)
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
