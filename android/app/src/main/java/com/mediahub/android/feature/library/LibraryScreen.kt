package com.mediahub.android.feature.library

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.BookOpen
import com.composables.icons.lucide.ChevronLeft
import com.composables.icons.lucide.ChevronRight
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Film
import com.composables.icons.lucide.LibraryBig
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.X
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubSearchField
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.image.PosterLoader
import com.mediahub.android.core.network.EmbyItem
import com.mediahub.android.playback.PlaybackFallback

data class LibraryBrowseActions(
    val onQueryChanged: (String) -> Unit,
    val onSearch: () -> Unit,
    val onClearSearch: () -> Unit,
    val onRefreshLibraries: () -> Unit,
    val onRefreshSelectedLibrary: () -> Unit,
    val onSelectLibrary: (String) -> Unit,
    val onSelectItem: (String) -> Unit,
    val onChangePage: (Int) -> Unit,
)

data class LibraryDetailActions(
    val onClose: () -> Unit,
    val onRefresh: () -> Unit,
    val onPlayItem: (EmbyItem, PlaybackFallback) -> Unit,
    val onDelete: () -> Unit = {},
    val onConfirmDelete: () -> Unit = {},
    val onCancelDelete: () -> Unit = {},
)

@Composable
internal fun LibraryRoute(
    browseViewModel: LibraryViewModel,
    detailViewModel: LibraryDetailViewModel,
    selectedItemId: String?,
    onSelectedItemChanged: (String?) -> Unit,
    onPlayItem: (EmbyItem, PlaybackFallback) -> Unit,
    posterLoader: PosterLoader,
) {
    val browseState by browseViewModel.uiState.collectAsState()
    val detailState by detailViewModel.uiState.collectAsState()
    LaunchedEffect(browseViewModel) { browseViewModel.refreshLibraries() }
    LaunchedEffect(selectedItemId) { selectedItemId?.let(detailViewModel::load) }
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
        onChangePage = browseViewModel::changePage,
    )
    val detailActions = LibraryDetailActions(
        onClose = closeDetail,
        onRefresh = detailViewModel::refresh,
        onPlayItem = onPlayItem,
        onDelete = detailViewModel::requestDelete,
        onConfirmDelete = detailViewModel::confirmDelete,
        onCancelDelete = detailViewModel::cancelDelete,
    )
    if (selectedItemId != null) {
        LibraryDetailScreen(state = detailState, actions = detailActions, posterLoader = posterLoader)
    } else {
        LibraryScreen(uiState = browseState, actions = browseActions, posterLoader = posterLoader)
    }
}

@Composable
internal fun LibraryScreen(
    uiState: LibraryBrowseState,
    actions: LibraryBrowseActions,
    posterLoader: PosterLoader,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MediaHubColors.Canvas)
            .padding(horizontal = 16.dp),
    ) {
        if (uiState.libraries.isNotEmpty()) {
            LazyRow(
                modifier = Modifier.fillMaxWidth().padding(top = 4.dp, bottom = 10.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                items(uiState.libraries, key = { it.id }) { library ->
                    val selected = library.id == uiState.selectedLibraryId
                    val collection = collectionLabel(library.collectionType)
                    Row(
                        modifier = Modifier
                            .heightIn(min = 40.dp)
                            .background(if (selected) MediaHubColors.SurfaceSelected else MediaHubColors.Surface, RoundedCornerShape(20.dp))
                            .border(width = 1.dp, color = if (selected) MediaHubColors.Accent else MediaHubColors.Border, shape = RoundedCornerShape(20.dp))
                            .selectable(selected = selected, role = Role.RadioButton) { actions.onSelectLibrary(library.id) }
                            .padding(horizontal = 14.dp, vertical = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        MediaHubIcon(
                            imageVector = Lucide.LibraryBig,
                            contentDescription = null,
                            tint = if (selected) MediaHubColors.Accent else MediaHubColors.TextMuted,
                            modifier = Modifier.size(15.dp),
                        )
                        Spacer(Modifier.width(6.dp))
                        MediaHubText(
                            text = library.name,
                            color = if (selected) MediaHubColors.TextPrimary else MediaHubColors.TextSecondary,
                            fontSize = 13.sp,
                            fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
                        )
                    }
                }
            }
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
            modifier = Modifier.fillMaxWidth().padding(top = 14.dp, bottom = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(
                text = if (uiState.submittedQuery.isNotEmpty()) "搜索结果 (${uiState.total})" else "全部媒体 (${uiState.total})",
                modifier = Modifier.weight(1f),
                color = MediaHubColors.TextStrong,
                fontSize = 15.sp,
                fontWeight = FontWeight.SemiBold,
            )
            if (uiState.submittedQuery.isEmpty() && uiState.selectedLibraryId != null) {
                MediaHubIconButton(Lucide.RefreshCw, "刷新当前媒体库", actions.onRefreshSelectedLibrary, enabled = !uiState.refreshing)
            }
        }
        LazyVerticalGrid(
            columns = GridCells.Fixed(3),
            modifier = Modifier.weight(1f),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
            contentPadding = PaddingValues(bottom = 20.dp),
        ) {
            items(uiState.items, key = { it.id }) { item ->
                EmbyPosterCard(item = item, posterLoader = posterLoader, onClick = { actions.onSelectItem(item.id) })
            }
            if (!uiState.loadingItems && uiState.items.isEmpty()) {
                item(span = { GridItemSpan(maxLineSpan) }) {
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(vertical = 40.dp)
                            .background(MediaHubColors.Surface, RoundedCornerShape(12.dp))
                            .border(width = 1.dp, color = MediaHubColors.Border, shape = RoundedCornerShape(12.dp))
                            .padding(vertical = 36.dp, horizontal = 20.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                    ) {
                        MediaHubIcon(Lucide.BookOpen, contentDescription = null, tint = MediaHubColors.TextMuted, modifier = Modifier.size(36.dp))
                        MediaHubText(
                            text = if (uiState.submittedQuery.isNotEmpty()) "没有找到匹配的媒体内容" else "此媒体库暂无内容",
                            modifier = Modifier.padding(top = 14.dp),
                            color = MediaHubColors.TextSecondary,
                            fontSize = 14.sp,
                            fontWeight = FontWeight.Medium,
                        )
                        MediaHubText(
                            text = if (uiState.submittedQuery.isNotEmpty()) "尝试缩短或更换搜索关键词" else "可在「发现」页搜索并转存新内容到此媒体库",
                            modifier = Modifier.padding(top = 6.dp),
                            color = MediaHubColors.TextMuted,
                            fontSize = 12.sp,
                        )
                    }
                }
            }
            if (uiState.submittedQuery.isEmpty() && uiState.total > LibraryPageSize) {
                item(span = { GridItemSpan(maxLineSpan) }) {
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
                        horizontalArrangement = Arrangement.Center,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        MediaHubIconButton(Lucide.ChevronLeft, "上一页", { actions.onChangePage(-1) }, enabled = uiState.page > 0 && !uiState.loadingItems)
                        MediaHubText(
                            text = "${uiState.page + 1} / ${libraryPageCount(uiState.total)}",
                            modifier = Modifier.padding(horizontal = 14.dp),
                            color = MediaHubColors.TextMuted,
                            fontSize = 12.sp,
                        )
                        MediaHubIconButton(Lucide.ChevronRight, "下一页", { actions.onChangePage(1) }, enabled = (uiState.page + 1) * LibraryPageSize < uiState.total && !uiState.loadingItems)
                    }
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
            .clip(RoundedCornerShape(8.dp))
            .clickable(role = Role.Button, onClick = onClick),
    ) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .aspectRatio(2f / 3f)
                .background(MediaHubColors.Surface, RoundedCornerShape(8.dp))
                .border(width = 1.dp, color = MediaHubColors.Border, shape = RoundedCornerShape(8.dp))
                .clip(RoundedCornerShape(8.dp)),
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
                        .padding(4.dp)
                        .background(
                            if (item.played) MediaHubColors.Accent else MediaHubColors.Surface.copy(alpha = 0.92f),
                            RoundedCornerShape(4.dp),
                        )
                        .padding(horizontal = 5.dp, vertical = 2.dp),
                ) {
                    MediaHubText(
                        text = status,
                        color = if (item.played) MediaHubColors.Canvas else MediaHubColors.TextPrimary,
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Bold,
                    )
                }
            }
        }
        MediaHubText(
            text = item.name,
            fontSize = 13.sp,
            fontWeight = FontWeight.Medium,
            color = MediaHubColors.TextPrimary,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.padding(top = 5.dp, start = 2.dp, end = 2.dp),
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

private fun collectionLabel(type: String?): String = when (type) {
    "movies" -> "电影"
    "tvshows" -> "剧集"
    else -> "媒体"
}

internal fun libraryPlaybackStatus(item: EmbyItem): String? = when {
    item.played -> "已看"
    item.playbackPositionMs >= 30_000L -> "继续 ${formatPlaybackPosition(item.playbackPositionMs)}"
    else -> null
}
