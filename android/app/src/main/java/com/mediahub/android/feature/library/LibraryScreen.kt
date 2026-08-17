package com.mediahub.android.feature.library

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.height
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
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
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
                onClick = actions.onRefreshLibraries,
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
                            .selectable(selected = selected, role = Role.RadioButton) { actions.onSelectLibrary(library.id) }
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
                onValueChange = actions.onQueryChanged,
                onSearch = actions.onSearch,
                enabled = !uiState.loadingItems,
                placeholder = "搜索电影或剧集",
                modifier = Modifier.weight(1f),
            )
            if (uiState.submittedQuery.isNotEmpty()) {
                Spacer(Modifier.width(8.dp))
                MediaHubIconButton(Lucide.X, "清除搜索", actions.onClearSearch)
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
                MediaHubIconButton(Lucide.RefreshCw, "刷新当前媒体库", actions.onRefreshSelectedLibrary, enabled = !uiState.refreshing)
            }
        }
        LazyColumn(
            modifier = Modifier.weight(1f),
            contentPadding = PaddingValues(bottom = 16.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            items(uiState.items, key = { it.id }) { item ->
                EmbyItemRow(item = item, posterLoader = posterLoader, onClick = { actions.onSelectItem(item.id) })
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
private fun EmbyItemRow(item: EmbyItem, posterLoader: PosterLoader, onClick: () -> Unit) {
    Column(modifier = Modifier.fillMaxWidth()) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(min = 76.dp)
                .clickable(role = Role.Button, onClick = onClick)
                .padding(horizontal = 4.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            EmbyPoster(
                itemId = item.id,
                loader = posterLoader,
                contentDescription = "${item.name} 封面",
                modifier = Modifier.width(48.dp),
            )
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                MediaHubText(text = item.name, fontSize = 14.sp, fontWeight = FontWeight.Medium)
                MediaHubText(
                    text = listOfNotNull(mediaTypeLabel(item.type), item.year?.toString()).joinToString(" · "),
                    modifier = Modifier.padding(top = 4.dp),
                    color = MediaHubColors.TextMuted,
                    fontSize = 12.sp,
                )
            }
            MediaHubIcon(
                imageVector = Lucide.ChevronRight,
                contentDescription = null,
                modifier = Modifier.size(18.dp),
                tint = MediaHubColors.TextFaint,
            )
        }
        Box(Modifier.fillMaxWidth().height(1.dp).background(MediaHubColors.Border))
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
