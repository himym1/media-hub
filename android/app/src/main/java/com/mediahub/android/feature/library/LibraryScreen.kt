package com.mediahub.android.feature.library

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.BookOpen
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubEmptyState
import com.mediahub.android.core.designsystem.MediaHubListDetail
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubSearchField
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
import com.mediahub.android.core.designsystem.MediaHubTabRow
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.image.PosterLoader
import com.mediahub.android.core.network.EmbyItem
import com.mediahub.android.feature.subtitles.RemoteSubtitleViewModel
import com.mediahub.android.playback.PlaybackFallback

data class LibraryBrowseActions(
    val onQueryChanged: (String) -> Unit,
    val onSearch: () -> Unit,
    val onClearSearch: () -> Unit,
    val onRefreshLibraries: () -> Unit,
    val onRefreshSelectedLibrary: () -> Unit,
    val onSelectLibrary: (String) -> Unit,
    val onSelectItem: (String) -> Unit,
    val onLoadMore: () -> Unit,
)

data class LibraryDetailActions(
    val onClose: () -> Unit,
    val onRefresh: () -> Unit,
    val onPlayItem: (EmbyItem, PlaybackFallback) -> Unit,
    val onDelete: () -> Unit = {},
    val onConfirmDelete: () -> Unit = {},
    val onCancelDelete: () -> Unit = {},
    val onSearchSubtitles: (itemId: String?, label: String?) -> Unit = { _, _ -> },
    val onDownloadSubtitle: (String) -> Unit = {},
    val onClearSubtitles: () -> Unit = {},
)

@Composable
internal fun LibraryRoute(
    browseViewModel: LibraryViewModel,
    detailViewModel: LibraryDetailViewModel,
    subtitleViewModel: RemoteSubtitleViewModel,
    selectedItemId: String?,
    onSelectedItemChanged: (String?) -> Unit,
    onPlayItem: (EmbyItem, PlaybackFallback) -> Unit,
    posterLoader: PosterLoader,
    active: Boolean = true,
) {
    val browseState by browseViewModel.uiState.collectAsState()
    val detailState by detailViewModel.uiState.collectAsState()
    val subtitleState by subtitleViewModel.uiState.collectAsState()
    LaunchedEffect(browseViewModel, active) {
        if (active) browseViewModel.ensureLibrariesLoaded()
    }
    LaunchedEffect(selectedItemId) {
        subtitleViewModel.clear()
        selectedItemId?.let(detailViewModel::load)
    }
    LaunchedEffect(detailState.deleted) {
        if (detailState.deleted) {
            browseViewModel.reloadItems()
            onSelectedItemChanged(null)
            detailViewModel.clear()
        }
    }
    val closeDetail = {
        onSelectedItemChanged(null)
        detailViewModel.clear()
        subtitleViewModel.clear()
    }
    BackHandler(enabled = selectedItemId != null, onBack = closeDetail)
    val browseActions = LibraryBrowseActions(
        onQueryChanged = browseViewModel::onQueryChanged,
        onSearch = browseViewModel::search,
        onClearSearch = browseViewModel::clearSearch,
        onRefreshLibraries = browseViewModel::refreshLibraries,
        onRefreshSelectedLibrary = browseViewModel::refreshSelectedLibrary,
        onSelectLibrary = browseViewModel::selectLibrary,
        onSelectItem = { onSelectedItemChanged(it) },
        onLoadMore = browseViewModel::loadMore,
    )
    val detailActions = LibraryDetailActions(
        onClose = closeDetail,
        onRefresh = detailViewModel::refresh,
        onPlayItem = onPlayItem,
        onDelete = detailViewModel::requestDelete,
        onConfirmDelete = detailViewModel::confirmDelete,
        onCancelDelete = detailViewModel::cancelDelete,
        onSearchSubtitles = { itemId, label ->
            val detail = detailState.item ?: return@LibraryDetailActions
            val resolvedId = itemId?.takeIf { it.isNotBlank() } ?: detail.item.id
            val seriesBlocked = detail.item.type == "Series" && itemId.isNullOrBlank()
            subtitleViewModel.search(
                itemId = resolvedId,
                label = label ?: detail.item.name,
                allowSearch = !seriesBlocked,
                blockedMessage = if (seriesBlocked) "请选择某一集后再搜中文字幕" else null,
            )
        },
        onDownloadSubtitle = { subtitleId ->
            subtitleViewModel.download(subtitleId) {
                detailViewModel.refreshActionMessage("已下载字幕，Emby 正在刷新；重新播放后可选中文字幕")
            }
        },
        onClearSubtitles = subtitleViewModel::clear,
    )
    MediaHubListDetail(
        detailOpen = selectedItemId != null,
        emptyTitle = "选择一部影片",
        emptyMessage = "从左侧媒体库打开详情",
        emptyIcon = Lucide.BookOpen,
        list = { LibraryScreen(uiState = browseState, actions = browseActions, posterLoader = posterLoader) },
        detail = {
            LibraryDetailScreen(
                state = detailState,
                subtitleState = subtitleState,
                actions = detailActions,
                posterLoader = posterLoader,
            )
        },
    )
}

@Composable
internal fun LibraryScreen(
    uiState: LibraryBrowseState,
    actions: LibraryBrowseActions,
    posterLoader: PosterLoader,
) {
    val gridState = rememberLazyGridState()
    val nearEnd by remember {
        derivedStateOf {
            val last = gridState.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0
            val totalItems = gridState.layoutInfo.totalItemsCount
            totalItems > 0 && last >= totalItems - 6
        }
    }
    val canLoadMore = libraryHasMore(
        itemCount = uiState.items.size,
        total = uiState.total,
        searching = uiState.submittedQuery.isNotEmpty(),
    )
    LaunchedEffect(nearEnd, canLoadMore, uiState.loadingItems, uiState.loadingMore) {
        if (nearEnd && canLoadMore && !uiState.loadingItems && !uiState.loadingMore) {
            actions.onLoadMore()
        }
    }
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 12.dp),
    ) {
        val topLibraries = rootLibraries(uiState.libraries)
        val adultGroups = childLibraries(uiState.libraries, AdultLibraryId)
        val selectedRootId = libraryRootId(uiState.libraries, uiState.selectedLibraryId).orEmpty()
        if (topLibraries.size >= 2) {
            MediaHubTabRow(
                options = topLibraries.map { it.id to it.name },
                selected = selectedRootId,
                onSelected = actions.onSelectLibrary,
                modifier = Modifier.padding(top = 2.dp, bottom = 4.dp),
            )
        }
        if (selectedRootId == AdultLibraryId && adultGroups.isNotEmpty()) {
            MediaHubTabRow(
                options = listOf(AdultLibraryId to "全部") + adultGroups.map { it.id to it.name },
                selected = uiState.selectedLibraryId.orEmpty(),
                onSelected = actions.onSelectLibrary,
                modifier = Modifier.padding(bottom = 4.dp),
            )
        }
        MediaHubSearchField(
            value = uiState.query,
            onValueChange = {
                actions.onQueryChanged(it)
                if (it.isEmpty() && uiState.submittedQuery.isNotEmpty()) {
                    actions.onClearSearch()
                }
            },
            onSearch = actions.onSearch,
            enabled = !uiState.loadingItems,
            placeholder = "在媒体库中搜索",
            modifier = Modifier.fillMaxWidth(),
        )
        uiState.errorMessage?.let { ErrorLine(it) }
        uiState.actionMessage?.let {
            MediaHubText(text = it, modifier = Modifier.padding(top = 8.dp), color = MediaHubColors.Source, fontSize = 12.sp)
        }
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubSmallTitle(
                text = if (uiState.submittedQuery.isNotEmpty()) "搜索结果 (${uiState.total})" else "全部媒体 (${uiState.total})",
                modifier = Modifier.weight(1f),
            )
            if (uiState.submittedQuery.isEmpty() && uiState.selectedLibraryId != null) {
                MediaHubIconButton(Lucide.RefreshCw, "刷新当前媒体库", actions.onRefreshSelectedLibrary, enabled = !uiState.refreshing)
            }
        }
        LazyVerticalGrid(
            columns = GridCells.Fixed(3),
            modifier = Modifier.weight(1f),
            state = gridState,
            horizontalArrangement = Arrangement.spacedBy(10.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
            contentPadding = PaddingValues(bottom = 20.dp),
        ) {
            items(uiState.items, key = { it.id }) { item ->
                EmbyPosterCard(item = item, posterLoader = posterLoader, onClick = { actions.onSelectItem(item.id) })
            }
            if (!uiState.loadingItems && uiState.items.isEmpty()) {
                item(span = { GridItemSpan(maxLineSpan) }) {
                    MediaHubEmptyState(
                        title = if (uiState.submittedQuery.isNotEmpty()) "没有找到匹配的媒体内容" else "此媒体库暂无内容",
                        message = if (uiState.submittedQuery.isNotEmpty()) "尝试缩短或更换搜索关键词" else "可在「发现」页搜索并转存新内容到此媒体库",
                        icon = Lucide.BookOpen,
                        modifier = Modifier.padding(vertical = 24.dp),
                    )
                }
            }
            if (uiState.loadingMore || (canLoadMore && uiState.loadingItems && uiState.items.isNotEmpty())) {
                item(span = { GridItemSpan(maxLineSpan) }) {
                    MediaHubText(
                        text = "正在加载更多…",
                        modifier = Modifier.fillMaxWidth().padding(vertical = 12.dp),
                        color = MediaHubColors.TextMuted,
                        fontSize = 12.sp,
                    )
                }
            }
        }
    }
}

@Composable
private fun EmbyPosterCard(
    item: EmbyItem,
    posterLoader: PosterLoader,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .clickable(role = Role.Button, onClick = onClick)
            .heightIn(min = 48.dp),
    ) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .aspectRatio(2f / 3f)
                .clip(RoundedCornerShape(14.dp)),
        ) {
            EmbyPoster(
                itemId = item.id,
                loader = posterLoader,
                contentDescription = "${item.name} 海报",
                modifier = Modifier.fillMaxSize(),
            )
            libraryPlaybackStatus(item)?.let { status ->
                Box(
                    modifier = Modifier
                        .align(Alignment.TopEnd)
                        .padding(6.dp)
                        .background(
                            if (item.played) MediaHubColors.Success else Color(0xD90F172A),
                            RoundedCornerShape(6.dp),
                        )
                        .padding(horizontal = 6.dp, vertical = 2.dp),
                ) {
                    MediaHubText(
                        text = status,
                        color = Color.White,
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Bold,
                    )
                }
            }
        }
        MediaHubText(
            text = item.name,
            fontSize = 13.sp,
            fontWeight = FontWeight.SemiBold,
            color = MediaHubColors.TextStrong,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(top = 6.dp, start = 2.dp, end = 2.dp),
        )
        MediaHubText(
            text = listOfNotNull(
                mediaTypeLabel(item.type),
                item.year?.toString(),
            ).joinToString(" · "),
            fontSize = 12.sp,
            color = MediaHubColors.TextMuted,
            maxLines = 1,
            modifier = Modifier.padding(top = 1.dp, start = 2.dp, end = 2.dp),
        )
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

internal fun libraryPlaybackStatus(item: EmbyItem): String? = when {
    item.played -> "已看"
    item.playbackPositionMs >= 30_000L -> "继续 ${formatPlaybackPosition(item.playbackPositionMs)}"
    else -> null
}
