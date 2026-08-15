package com.mediahub.android.feature.library

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Film
import com.composables.icons.lucide.LibraryBig
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubSearchField
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.network.EmbyItem

@Composable
internal fun LibraryRoute(viewModel: LibraryViewModel) {
    val uiState by viewModel.uiState.collectAsState()
    LaunchedEffect(viewModel) { viewModel.refreshLibraries() }
    LibraryScreen(
        uiState = uiState,
        onQueryChanged = viewModel::onQueryChanged,
        onSearch = viewModel::search,
        onRefresh = viewModel::refreshLibraries,
    )
}

@Composable
private fun LibraryScreen(
    uiState: LibraryUiState,
    onQueryChanged: (String) -> Unit,
    onSearch: () -> Unit,
    onRefresh: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MediaHubColors.Canvas)
            .padding(horizontal = 16.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 12.dp),
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
                contentDescription = "刷新媒体库",
                enabled = !uiState.loadingLibraries,
                onClick = onRefresh,
            )
        }
        MediaHubSearchField(
            value = uiState.query,
            onValueChange = onQueryChanged,
            onSearch = onSearch,
            enabled = !uiState.searching,
            placeholder = "搜索 Emby 媒体库",
            modifier = Modifier.fillMaxWidth(),
        )
        uiState.errorMessage?.let { message ->
            Row(
                modifier = Modifier.fillMaxWidth().padding(top = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubIcon(
                    imageVector = Lucide.CircleAlert,
                    contentDescription = null,
                    tint = MediaHubColors.Error,
                    modifier = Modifier.size(17.dp),
                )
                Spacer(Modifier.width(8.dp))
                MediaHubText(text = message, color = MediaHubColors.Error, fontSize = 12.sp)
            }
        }
        LazyColumn(
            modifier = Modifier.weight(1f),
            contentPadding = PaddingValues(top = 18.dp, bottom = 16.dp),
            verticalArrangement = Arrangement.spacedBy(9.dp),
        ) {
            item(key = "libraries-heading") {
                MediaHubText(
                    text = if (uiState.loadingLibraries) "正在读取媒体库" else "${uiState.libraries.size} 个媒体库",
                    color = MediaHubColors.TextMuted,
                    fontSize = 12.sp,
                )
            }
            items(uiState.libraries, key = { "library-${it.id}" }) { library ->
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .background(MediaHubColors.Surface, RoundedCornerShape(8.dp))
                        .padding(14.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    MediaHubIcon(imageVector = Lucide.LibraryBig, contentDescription = null, modifier = Modifier.size(17.dp))
                    Spacer(Modifier.width(10.dp))
                    MediaHubText(text = library.name, modifier = Modifier.weight(1f), fontSize = 13.sp)
                    MediaHubText(
                        text = collectionLabel(library.collectionType),
                        color = MediaHubColors.TextMuted,
                        fontSize = 12.sp,
                    )
                }
            }
            item(key = "results-heading") {
                MediaHubText(
                    text = if (uiState.searching) "正在搜索 Emby" else "媒体条目 ${uiState.items.size}",
                    modifier = Modifier.padding(top = 12.dp),
                    color = MediaHubColors.TextStrong,
                    fontSize = 15.sp,
                    fontWeight = FontWeight.Medium,
                )
            }
            items(uiState.items, key = { "item-${it.id}" }) { item -> EmbyItemRow(item) }
            if (!uiState.searching && uiState.items.isEmpty() && uiState.query.isNotBlank()) {
                item(key = "empty") {
                    MediaHubText(
                        text = "没有找到匹配条目",
                        modifier = Modifier.padding(vertical = 24.dp),
                        color = MediaHubColors.TextMuted,
                        fontSize = 12.sp,
                    )
                }
            }
        }
    }
}

@Composable
private fun EmbyItemRow(item: EmbyItem) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(MediaHubColors.Surface, RoundedCornerShape(8.dp))
            .padding(14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MediaHubIcon(imageVector = Lucide.Film, contentDescription = null, modifier = Modifier.size(17.dp))
        Spacer(Modifier.width(10.dp))
        Column(Modifier.weight(1f)) {
            MediaHubText(text = item.name, fontSize = 13.sp, fontWeight = FontWeight.Medium)
            MediaHubText(
                text = listOfNotNull(item.year?.toString(), item.tmdbId?.let { "TMDB $it" }).joinToString(" · "),
                modifier = Modifier.padding(top = 4.dp),
                color = MediaHubColors.TextMuted,
                fontSize = 12.sp,
            )
        }
        MediaHubText(
            text = if (item.type == "Movie") "电影" else "剧集",
            color = MediaHubColors.Accent,
            fontSize = 12.sp,
        )
    }
}

private fun collectionLabel(type: String?): String = when (type) {
    "movies" -> "电影"
    "tvshows" -> "剧集"
    else -> "媒体"
}
