package com.mediahub.android.feature.search

import androidx.compose.foundation.background
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.BellPlus
import com.composables.icons.lucide.ChevronRight
import com.composables.icons.lucide.Film
import com.composables.icons.lucide.Flame
import com.composables.icons.lucide.LayoutGrid
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.Search
import com.composables.icons.lucide.Sparkles
import com.composables.icons.lucide.Star
import com.composables.icons.lucide.Tv
import com.composables.icons.lucide.Wifi
import com.mediahub.android.app.LocalTwoPane
import com.mediahub.android.core.designsystem.BadgeVariant
import com.mediahub.android.core.designsystem.MediaHubBadge
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubDialog
import com.mediahub.android.core.designsystem.MediaHubEmptyState
import com.mediahub.android.core.designsystem.MediaHubFilterChip
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubListDetail
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubPreferenceRow
import com.mediahub.android.core.designsystem.MediaHubSearchBar
import com.mediahub.android.core.designsystem.MediaHubSearchField
import com.mediahub.android.core.designsystem.MediaHubSecondaryButton
import com.mediahub.android.core.designsystem.MediaHubShimmerBox
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
import com.mediahub.android.core.designsystem.MediaHubTabRow
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTextButton
import com.mediahub.android.core.designsystem.MediaHubTextField
import com.mediahub.android.core.image.RemotePoster
import com.mediahub.android.core.network.DiscoveryGenre
import com.mediahub.android.core.network.DiscoveryItem
import com.mediahub.android.core.network.IntegrationHealth
import com.mediahub.android.core.network.SearchCandidate
import com.mediahub.android.core.network.SearchIdentity
import java.util.Locale

@Composable
internal fun SearchRoute(
    viewModel: SearchViewModel,
    onTransferCreated: () -> Unit,
    onSubscriptionRequested: (SearchCandidate) -> Unit,
    onOpenServices: () -> Unit = {},
) {
    val uiState by viewModel.uiState.collectAsState()

    // Single-time initial load, avoiding repeated flickering on tab navigation
    LaunchedEffect(Unit) {
        viewModel.loadInitialData()
    }

    LaunchedEffect(viewModel, onTransferCreated) {
        viewModel.events.collect { event ->
            when (event) {
                SearchEvent.TransferCreated -> onTransferCreated()
                is SearchEvent.SubscriptionRequested -> onSubscriptionRequested(event.candidate)
            }
        }
    }

    SearchScreen(
        uiState = uiState,
        onQueryChanged = viewModel::onQueryChanged,
        onSearch = viewModel::submitSearch,
        onRefreshOverview = viewModel::refreshAll,
        onTrendingSelected = viewModel::searchTrending,
        onCandidateSelected = viewModel::onCandidateSelected,
        onRecommendationSelected = viewModel::searchRecommendation,
        onShuffleRecommendations = viewModel::shuffleRecommendations,
        onCategorySelected = viewModel::onCategorySelected,
        onPipelineFilterSelected = viewModel::onPipelineFilterSelected,
        onGenreSelected = viewModel::selectGenre,
        onOpenServices = onOpenServices,
        onTransfer = viewModel::createTransfer,
        onSubscribe = viewModel::requestSubscription,
        onImportShare = viewModel::importShare,
    )
}

@Composable
internal fun SearchScreen(
    uiState: SearchUiState,
    onQueryChanged: (String) -> Unit,
    onSearch: () -> Unit,
    onRefreshOverview: () -> Unit,
    onTrendingSelected: (DiscoveryItem) -> Unit,
    onCandidateSelected: (String) -> Unit,
    onRecommendationSelected: (DiscoveryItem) -> Unit,
    onShuffleRecommendations: () -> Unit = {},
    onCategorySelected: (String) -> Unit = {},
    onPipelineFilterSelected: (String) -> Unit = {},
    onGenreSelected: (DiscoveryGenre?) -> Unit = {},
    onOpenServices: () -> Unit = {},
    onTransfer: (String) -> Unit = {},
    onSubscribe: (String) -> Unit = {},
    onImportShare: (String, String, String) -> Unit = { _, _, _ -> },
) {
    var showHealthDialog by remember { mutableStateOf(false) }

    if (showHealthDialog) {
        SystemHealthDetailDialog(
            integrations = uiState.integrations,
            refreshing = uiState.refreshing,
            onRefresh = onRefreshOverview,
            onOpenServices = {
                showHealthDialog = false
                onOpenServices()
            },
            onDismiss = { showHealthDialog = false },
        )
    }

    if (LocalTwoPane.current) {
        SearchTwoPane(
            uiState = uiState,
            onQueryChanged = onQueryChanged,
            onSearch = onSearch,
            onRefreshOverview = onRefreshOverview,
            onTrendingSelected = onTrendingSelected,
            onCandidateSelected = onCandidateSelected,
            onRecommendationSelected = onRecommendationSelected,
            onShuffleRecommendations = onShuffleRecommendations,
            onGenreSelected = onGenreSelected,
            onCategorySelected = onCategorySelected,
            onPipelineFilterSelected = onPipelineFilterSelected,
            onOpenServices = onOpenServices,
            onOpenHealthDetail = { showHealthDialog = true },
            onTransfer = onTransfer,
            onSubscribe = onSubscribe,
            onImportShare = onImportShare,
        )
        return
    }

    val selectedCandidate = uiState.selectedCandidate
    var expanded by remember {
        mutableStateOf(uiState.submittedQuery.isNotBlank() || uiState.results.isNotEmpty())
    }

    LaunchedEffect(uiState.submittedQuery, uiState.searching, uiState.results.size) {
        if (uiState.submittedQuery.isNotBlank() || uiState.searching || uiState.results.isNotEmpty()) {
            expanded = true
        }
    }

    Column(modifier = Modifier.fillMaxSize()) {
        MediaHubSearchBar(
            query = uiState.query,
            onQueryChange = onQueryChanged,
            onSearch = onSearch,
            expanded = expanded,
            onExpandedChange = { expanded = it },
            enabled = !uiState.searching,
            modifier = if (expanded) {
                Modifier.weight(1f).fillMaxWidth().padding(top = 4.dp)
            } else {
                Modifier.fillMaxWidth().padding(top = 4.dp)
            },
        ) {
            Column(modifier = Modifier.fillMaxSize().padding(horizontal = 12.dp)) {
                if (uiState.submittedQuery.isNotBlank() || uiState.searching) {
                    MediaHubSmallTitle(text = uiState.resultsHeading)
                    if (uiState.focusSubtitle.isNotBlank() && !uiState.searching) {
                        MediaHubText(
                            text = uiState.focusSubtitle,
                            color = MediaHubColors.TextMuted,
                            fontSize = 12.sp,
                            modifier = Modifier.padding(top = 2.dp, bottom = 4.dp),
                        )
                    }
                }
                uiState.errorMessage?.let { message ->
                    StatusMessage(message, MediaHubColors.Error)
                }
                uiState.sourceMessage?.let { message ->
                    StatusMessage(message, MediaHubColors.Warning)
                }
                uiState.transferMessage?.let { message ->
                    StatusMessage(message, MediaHubColors.Error)
                }
                LazyColumn(
                    modifier = Modifier.weight(1f),
                    contentPadding = PaddingValues(vertical = 8.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    if (uiState.titleIdentity != null) {
                        item(key = "identity") {
                            SearchIdentityCard(identity = uiState.titleIdentity)
                        }
                    }
                    if (uiState.results.isNotEmpty()) {
                        item(key = "pipeline-filter") {
                            PipelineFilterChips(
                                selected = uiState.pipelineFilter,
                                transferCount = uiState.results.count { !isDownloadable(it) },
                                downloadCount = uiState.results.count { isDownloadable(it) },
                                total = uiState.results.size,
                                onSelected = onPipelineFilterSelected,
                            )
                        }
                    }
                    if (uiState.visibleResults.isNotEmpty()) {
                        item(key = "results") {
                            MediaHubCard {
                                uiState.visibleResults.forEachIndexed { index, candidate ->
                                    if (index > 0) MediaHubListDivider()
                                    ReleaseRow(
                                        candidate = candidate,
                                        fallbackPoster = uiState.identities.singleOrNull()?.posterUrl,
                                        selected = selectedCandidate?.id == candidate.id,
                                        onClick = { onCandidateSelected(candidate.id) },
                                    )
                                }
                            }
                        }
                    }
                    if (uiState.recommendations.isNotEmpty()) {
                        item(key = "recommendations-heading") {
                            MediaHubSmallTitle(text = "相似内容")
                        }
                        item(key = "recommendations") {
                            LazyRow(
                                horizontalArrangement = Arrangement.spacedBy(10.dp),
                                contentPadding = PaddingValues(horizontal = 4.dp),
                            ) {
                                items(uiState.recommendations, key = { "${it.mediaType}-${it.tmdbId}" }) { item ->
                                    DiscoveryItemCard(item = item, onClick = { onRecommendationSelected(item) })
                                }
                            }
                        }
                    }
                    if (!uiState.searching && uiState.submittedQuery.isNotBlank() && uiState.results.isEmpty() && uiState.errorMessage == null) {
                        item {
                            MediaHubText(
                                text = if (uiState.focusTitle.isNotBlank()) {
                                    "没有找到《${uiState.focusTitle}》的可获取版本"
                                } else {
                                    "没有找到匹配资源"
                                },
                                modifier = Modifier.padding(vertical = 28.dp),
                                color = MediaHubColors.TextMuted,
                                fontSize = 13.sp,
                            )
                        }
                    }
                }
                if (selectedCandidate != null) {
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(bottom = 10.dp),
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                    ) {
                        MediaHubSecondaryButton(
                            label = "订阅",
                            icon = Lucide.BellPlus,
                            enabled = selectedCandidate.tmdbId != null && selectedCandidate.transferState != "identity_required",
                            modifier = Modifier.weight(0.35f),
                            onClick = { onSubscribe(selectedCandidate.id) },
                        )
                        MediaHubButton(
                            label = transferActionLabel(selectedCandidate, uiState.transferringCandidateId == selectedCandidate.id),
                            enabled = selectedCandidate.transferToken != null && uiState.transferringCandidateId == null,
                            modifier = Modifier.weight(0.65f),
                            onClick = { onTransfer(selectedCandidate.id) },
                        )
                    }
                }
            }
        }

        if (!expanded) {
            LazyColumn(
                modifier = Modifier
                    .weight(1f)
                    .fillMaxWidth(),
                contentPadding = PaddingValues(bottom = 24.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                // 1. Compact Sleek System Health Bar (Click opens detail dialog)
                item(key = "health-bar") {
                    SystemHealthPillBar(
                        integrations = uiState.integrations,
                        refreshing = uiState.refreshing,
                        onRefresh = onRefreshOverview,
                        onOpenDetail = { showHealthDialog = true },
                        modifier = Modifier.padding(start = 16.dp, end = 16.dp, top = 8.dp),
                    )
                }
                item(key = "share-import") {
                    ShareImportCard(
                        importing = uiState.shareImporting,
                        message = uiState.shareImportMessage,
                        onImport = onImportShare,
                        modifier = Modifier.padding(horizontal = 16.dp),
                    )
                }

                if (uiState.initialLoading) {
                    item(key = "skeleton") {
                        DiscoverySkeletonScreen(modifier = Modifier.padding(horizontal = 16.dp))
                    }
                } else {
                    // 2. Cinematic Hero Gallery Carousel
                    if (uiState.heroItems.isNotEmpty()) {
                        item(key = "hero-carousel") {
                            HeroGalleryCarousel(
                                items = uiState.heroItems,
                                onSelect = onTrendingSelected,
                                modifier = Modifier.padding(horizontal = 16.dp),
                            )
                        }
                    }

                    // 3. Category Filter Chips
                    item(key = "category-tabs") {
                        CategoryTabs(
                            selectedCategory = uiState.selectedCategory,
                            onCategorySelected = onCategorySelected,
                            modifier = Modifier.padding(horizontal = 16.dp),
                        )
                    }

                    if (uiState.movieGenres.isNotEmpty() &&
                        (uiState.selectedCategory == "all" || uiState.selectedCategory == "movie")
                    ) {
                        item(key = "genre-chips") {
                            GenreFilterChips(
                                genres = uiState.movieGenres,
                                selectedGenreId = uiState.selectedGenreId,
                                onGenreSelected = onGenreSelected,
                                modifier = Modifier.padding(horizontal = 16.dp),
                            )
                        }
                    }

                    if (uiState.selectedGenreId != null &&
                        (uiState.genreItems.isNotEmpty() || uiState.loadingGenre)
                    ) {
                        item(key = "genre-browse") {
                            DiscoveryGallerySection(
                                title = uiState.selectedGenreName?.let { "${it}片" } ?: "类型精选",
                                subtitle = if (uiState.loadingGenre) "正在加载…" else "",
                                icon = Lucide.LayoutGrid,
                                items = uiState.genreItems,
                                onSelect = onTrendingSelected,
                            )
                        }
                    }

                    if ((uiState.selectedCategory == "all" || uiState.selectedCategory == "movie") &&
                        uiState.topRatedMovies.isNotEmpty()
                    ) {
                        item(key = "top-rated-movies") {
                            DiscoveryGallerySection(
                                title = "高分电影",
                                subtitle = "TMDB 高分",
                                icon = Lucide.Star,
                                items = uiState.topRatedMovies,
                                onSelect = onTrendingSelected,
                            )
                        }
                    }

                    if ((uiState.selectedCategory == "all" || uiState.selectedCategory == "series") &&
                        uiState.popularSeries.isNotEmpty()
                    ) {
                        item(key = "popular-series") {
                            DiscoveryGallerySection(
                                title = "热门剧集",
                                subtitle = "TMDB 热度",
                                icon = Lucide.Tv,
                                items = uiState.popularSeries,
                                onSelect = onTrendingSelected,
                            )
                        }
                    }

                    // 4. Personalized / Library-based Recommendations (猜你喜欢)
                    if (uiState.selectedCategory == "all" || uiState.selectedCategory == "recommended") {
                        if (uiState.libraryRecommendations.isNotEmpty()) {
                            item(key = "library-recs") {
                                DiscoveryGallerySection(
                                    title = "猜你喜欢",
                                    subtitle = uiState.libraryRecommendationSeed?.let { "基于《$it》" } ?: "",
                                    icon = Lucide.Sparkles,
                                    items = uiState.libraryRecommendations,
                                    onSelect = onRecommendationSelected,
                                    actionLabel = if (uiState.shufflingRecommendations) "换一批…" else "换一批",
                                    onAction = onShuffleRecommendations,
                                    actionEnabled = !uiState.shufflingRecommendations,
                                )
                            }
                        } else if (uiState.selectedCategory == "recommended") {
                            item(key = "empty-recs") {
                                Column(
                                    modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp),
                                    horizontalAlignment = Alignment.CenterHorizontally,
                                ) {
                                    MediaHubEmptyState(
                                        title = "还没有推荐",
                                        message = "在片库播放几部影片，或点刷新后会出现",
                                        icon = Lucide.Sparkles,
                                    )
                                    MediaHubTextButton(
                                        label = "刷新推荐",
                                        icon = Lucide.RefreshCw,
                                        onClick = onRefreshOverview,
                                    )
                                }
                            }
                        }
                    }

                    // 5. Trending Movies (热门电影)
                    if ((uiState.selectedCategory == "all" || uiState.selectedCategory == "movie") &&
                        uiState.trendingMovies.isNotEmpty()
                    ) {
                        item(key = "trending-movies") {
                            DiscoveryGallerySection(
                                title = "热门电影",
                                subtitle = "",
                                icon = Lucide.Film,
                                items = uiState.trendingMovies,
                                onSelect = onTrendingSelected,
                            )
                        }
                    }

                    // 6. Trending Series (热门剧集)
                    if ((uiState.selectedCategory == "all" || uiState.selectedCategory == "series") &&
                        uiState.trendingSeries.isNotEmpty()
                    ) {
                        item(key = "trending-series") {
                            DiscoveryGallerySection(
                                title = "连载热播",
                                subtitle = "",
                                icon = Lucide.Tv,
                                items = uiState.trendingSeries,
                                onSelect = onTrendingSelected,
                            )
                        }
                    }

                    // 7. All Top Popular / Discovery feed
                    if (uiState.selectedCategory == "all" && uiState.trending.size > 5) {
                        item(key = "more-trending") {
                            DiscoveryGallerySection(
                                title = "本周热门",
                                subtitle = "",
                                icon = Lucide.Flame,
                                items = uiState.trending.drop(5),
                                onSelect = onTrendingSelected,
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun SystemHealthDetailDialog(
    integrations: List<IntegrationHealth>,
    refreshing: Boolean,
    onRefresh: () -> Unit,
    onOpenServices: () -> Unit,
    onDismiss: () -> Unit,
) {
    MediaHubDialog(
        title = "服务状态",
        onDismiss = onDismiss,
        confirmLabel = "服务设置",
        onConfirm = onOpenServices,
    ) {
        if (integrations.isEmpty()) {
            MediaHubText(
                text = "正在读取服务状态…",
                fontSize = 13.sp,
                color = MediaHubColors.TextMuted,
                modifier = Modifier.padding(vertical = 12.dp),
            )
        } else {
            integrations.forEachIndexed { index, item ->
                if (index > 0) MediaHubListDivider()
                val (badgeLabel, badgeVariant) = when (item.status) {
                    "healthy" -> "正常" to BadgeVariant.Success
                    "unconfigured" -> "未配置" to BadgeVariant.Warning
                    "unauthorized" -> "鉴权失效" to BadgeVariant.Error
                    "degraded" -> "异常" to BadgeVariant.Error
                    else -> item.status to BadgeVariant.Neutral
                }
                MediaHubPreferenceRow(
                    title = item.label,
                    summary = item.detail.ifBlank { integrationDescription(item.id) },
                    onClick = onOpenServices,
                    end = { MediaHubBadge(text = badgeLabel, variant = badgeVariant) },
                )
            }
        }
        MediaHubTextButton(
            label = if (refreshing) "检测中…" else "重新检测",
            icon = Lucide.RefreshCw,
            enabled = !refreshing,
            onClick = onRefresh,
        )
    }
}

/**
 * Compact, modern floating status capsule for external integrations
 */
@Composable
private fun SystemHealthPillBar(
    integrations: List<IntegrationHealth>,
    refreshing: Boolean,
    onRefresh: () -> Unit,
    onOpenDetail: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val healthyCount = integrations.count { it.status == "healthy" }
    val totalCount = integrations.size
    val allHealthy = totalCount > 0 && healthyCount == totalCount

    Row(
        modifier = modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .clip(RoundedCornerShape(20.dp))
            .background(MediaHubColors.SurfaceHigh)
            .clickable(onClick = onOpenDetail)
            .padding(horizontal = 14.dp, vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.weight(1f),
        ) {
            Box(
                modifier = Modifier
                    .size(8.dp)
                    .clip(CircleShape)
                    .background(if (allHealthy) MediaHubColors.Success else MediaHubColors.Warning),
            )
            MediaHubText(
                text = if (integrations.isEmpty()) {
                    "服务状态正在读取…"
                } else {
                    integrations.joinToString(" · ") { "${it.label} ${statusLabel(it.status)}" }
                },
                fontSize = 12.sp,
                color = MediaHubColors.TextSecondary,
                maxLines = 1,
                overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                modifier = Modifier.weight(1f, fill = false),
            )
            MediaHubIcon(
                imageVector = Lucide.ChevronRight,
                contentDescription = "查看服务详情",
                tint = MediaHubColors.TextMuted,
                modifier = Modifier.size(14.dp),
            )
        }

        MediaHubIconButton(
            imageVector = Lucide.RefreshCw,
            contentDescription = "刷新服务状态与发现",
            enabled = !refreshing,
            onClick = onRefresh,
        )
    }
}

/**
 * High-impact cinematic hero card with backdrop art & quick action
 */
@Composable
private fun HeroGalleryCarousel(
    items: List<DiscoveryItem>,
    onSelect: (DiscoveryItem) -> Unit,
    modifier: Modifier = Modifier,
) {
    var currentIndex by remember { mutableIntStateOf(0) }
    val safeIndex = currentIndex.coerceIn(0, (items.size - 1).coerceAtLeast(0))
    val currentItem = items.getOrNull(safeIndex) ?: return

    Column(
        modifier = modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(200.dp)
                .clip(RoundedCornerShape(18.dp))
                .background(MediaHubColors.CardBackground)
                .clickable { onSelect(currentItem) },
        ) {
            RemotePoster(
                url = currentItem.posterUrl,
                contentDescription = currentItem.title,
                modifier = Modifier.fillMaxSize(),
            )

            // Deep cinematic dark gradient overlay
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .background(
                        Brush.verticalGradient(
                            colors = listOf(
                                Color.Transparent,
                                Color(0xCC080A0D),
                                Color(0xF5080A0D),
                            ),
                            startY = 40f,
                        ),
                    ),
            )

            // Content Overlay
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(16.dp),
                verticalArrangement = Arrangement.SpaceBetween,
            ) {
                MediaHubBadge(
                    text = if (currentItem.mediaType == "movie") "电影" else "剧集",
                    variant = BadgeVariant.Neutral,
                )

                Column(modifier = Modifier.fillMaxWidth()) {
                    MediaHubText(
                        text = currentItem.title,
                        fontSize = 18.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = MediaHubColors.TextStrong,
                        maxLines = 2,
                        overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                    )
                    MediaHubText(
                        text = listOfNotNull(
                            currentItem.year.takeIf { it > 0 }?.toString(),
                            if (currentItem.mediaType == "movie") "电影" else "剧集",
                        ).joinToString(" · "),
                        fontSize = 12.sp,
                        color = MediaHubColors.TextSecondary,
                        modifier = Modifier.padding(top = 4.dp),
                    )
                }
            }
        }

        // Carousel Paging Indicator Dots
        if (items.size > 1) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.Center,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                items.indices.forEach { index ->
                    val isSelected = index == safeIndex
                    Box(
                        modifier = Modifier
                            .size(48.dp)
                            .clickable { currentIndex = index },
                        contentAlignment = Alignment.Center,
                    ) {
                        Box(
                            modifier = Modifier
                                .height(4.dp)
                                .width(if (isSelected) 18.dp else 6.dp)
                                .clip(RoundedCornerShape(2.dp))
                                .background(if (isSelected) MediaHubColors.Accent else MediaHubColors.BorderStrong),
                        )
                    }
                }
            }
        }
    }
}

/**
 * Filter pills for discovering specific media types
 */
@Composable
private fun GenreFilterChips(
    genres: List<DiscoveryGenre>,
    selectedGenreId: Int?,
    onGenreSelected: (DiscoveryGenre?) -> Unit,
    modifier: Modifier = Modifier,
) {
    LazyRow(
        modifier = modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        item(key = "genre-all") {
            MediaHubFilterChip(
                label = "全部类型",
                selected = selectedGenreId == null,
                onClick = { onGenreSelected(null) },
            )
        }
        items(genres.take(16), key = { it.id }) { genre ->
            MediaHubFilterChip(
                label = genre.name,
                selected = selectedGenreId == genre.id,
                onClick = {
                    if (selectedGenreId == genre.id) onGenreSelected(null)
                    else onGenreSelected(genre)
                },
            )
        }
    }
}

@Composable
private fun CategoryTabs(
    selectedCategory: String,
    onCategorySelected: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    MediaHubTabRow(
        options = listOf(
            "all" to "全部",
            "recommended" to "推荐",
            "movie" to "电影",
            "series" to "剧集",
        ),
        selected = selectedCategory,
        onSelected = onCategorySelected,
        modifier = modifier,
    )
}

/**
 * Horizontal gallery section with squircle poster cards
 */
@Composable
private fun DiscoveryGallerySection(
    title: String,
    subtitle: String = "",
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    items: List<DiscoveryItem>,
    onSelect: (DiscoveryItem) -> Unit,
    actionLabel: String? = null,
    onAction: (() -> Unit)? = null,
    actionEnabled: Boolean = true,
) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    MediaHubIcon(
                        imageVector = icon,
                        contentDescription = null,
                        tint = MediaHubColors.Accent,
                        modifier = Modifier.size(16.dp),
                    )
                    MediaHubText(
                        text = title,
                        fontSize = 16.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = MediaHubColors.TextStrong,
                    )
                }
                if (subtitle.isNotBlank()) {
                    MediaHubText(
                        text = subtitle,
                        modifier = Modifier.padding(top = 2.dp),
                        fontSize = 12.sp,
                        color = MediaHubColors.TextMuted,
                    )
                }
            }
            if (actionLabel != null && onAction != null) {
                Row(
                    modifier = Modifier
                        .heightIn(min = 48.dp)
                        .clickable(enabled = actionEnabled, onClick = onAction)
                        .padding(horizontal = 8.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(4.dp),
                ) {
                    MediaHubIcon(
                        imageVector = Lucide.RefreshCw,
                        contentDescription = null,
                        tint = if (actionEnabled) MediaHubColors.Accent else MediaHubColors.TextMuted,
                        modifier = Modifier.size(15.dp),
                    )
                    MediaHubText(
                        text = actionLabel,
                        fontSize = 13.sp,
                        fontWeight = FontWeight.SemiBold,
                        color = if (actionEnabled) MediaHubColors.Accent else MediaHubColors.TextMuted,
                    )
                }
            }
        }

        LazyRow(
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            contentPadding = PaddingValues(horizontal = 16.dp),
        ) {
            items(items, key = { "${it.mediaType}-${it.tmdbId}" }) { item ->
                DiscoveryItemCard(item = item, onClick = { onSelect(item) })
            }
        }
    }
}

/**
 * Poster card with 2:3 aspect ratio, elevation, and overlay tags
 */
@Composable
private fun DiscoveryItemCard(
    item: DiscoveryItem,
    onClick: () -> Unit,
) {
    Column(
        modifier = Modifier
            .width(136.dp)
            .clickable(onClick = onClick),
    ) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .aspectRatio(2f / 3f)
                .clip(RoundedCornerShape(12.dp))
                .background(MediaHubColors.CardBackground),
        ) {
            RemotePoster(
                url = item.posterUrl,
                contentDescription = item.title,
                modifier = Modifier.fillMaxSize(),
            )
        }

        MediaHubText(
            text = item.title,
            modifier = Modifier.padding(top = 7.dp, start = 2.dp, end = 2.dp),
            color = MediaHubColors.TextStrong,
            fontSize = 13.sp,
            fontWeight = FontWeight.SemiBold,
            maxLines = 1,
            overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
        )

        MediaHubText(
            text = listOfNotNull(
                item.year.takeIf { it > 0 }?.toString(),
                if (item.mediaType == "movie") "电影" else "剧集",
            ).joinToString(" · "),
            modifier = Modifier.padding(top = 2.dp, start = 2.dp, end = 2.dp),
            color = MediaHubColors.TextMuted,
            fontSize = 12.sp,
            maxLines = 1,
        )
    }
}

/**
 * Sleek pulsing shimmer skeleton while discovery data loads
 */
@Composable
private fun DiscoverySkeletonScreen(modifier: Modifier = Modifier) {
    Column(
        modifier = modifier.fillMaxWidth(),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        // Hero Skeleton
        MediaHubShimmerBox(
            modifier = Modifier
                .fillMaxWidth()
                .height(185.dp),
            shape = RoundedCornerShape(14.dp),
        )

        // Chips Skeleton
        Row(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier.fillMaxWidth(),
        ) {
            repeat(4) {
                MediaHubShimmerBox(
                    modifier = Modifier
                        .width(78.dp)
                        .height(32.dp),
                    shape = RoundedCornerShape(16.dp),
                )
            }
        }

        // Section Posters Skeleton
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            MediaHubShimmerBox(
                modifier = Modifier
                    .width(120.dp)
                    .height(18.dp),
                shape = RoundedCornerShape(4.dp),
            )
            Row(
                horizontalArrangement = Arrangement.spacedBy(12.dp),
                modifier = Modifier.fillMaxWidth(),
            ) {
                repeat(3) {
                    MediaHubShimmerBox(
                        modifier = Modifier
                            .width(136.dp)
                            .aspectRatio(2f / 3f),
                        shape = RoundedCornerShape(10.dp),
                    )
                }
            }
        }
    }
}

@Composable
private fun SearchTwoPane(
    uiState: SearchUiState,
    onQueryChanged: (String) -> Unit,
    onSearch: () -> Unit,
    onRefreshOverview: () -> Unit,
    onTrendingSelected: (DiscoveryItem) -> Unit,
    onCandidateSelected: (String) -> Unit,
    onRecommendationSelected: (DiscoveryItem) -> Unit,
    onShuffleRecommendations: () -> Unit = {},
    onGenreSelected: (DiscoveryGenre?) -> Unit = {},
    onCategorySelected: (String) -> Unit = {},
    onPipelineFilterSelected: (String) -> Unit = {},
    onOpenServices: () -> Unit = {},
    onOpenHealthDetail: () -> Unit = {},
    onTransfer: (String) -> Unit = {},
    onSubscribe: (String) -> Unit = {},
    onImportShare: (String, String, String) -> Unit = { _, _, _ -> },
) {
    val selectedCandidate = uiState.selectedCandidate
    val showingResults = uiState.submittedQuery.isNotBlank() || uiState.searching || uiState.results.isNotEmpty()

    MediaHubListDetail(
        detailOpen = selectedCandidate != null,
        emptyTitle = "选择一个资源",
        emptyMessage = "从左侧打开详情后转存或订阅",
        emptyIcon = Lucide.Search,
        list = {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = 12.dp)
                    .testTag("search-list-pane"),
            ) {
                MediaHubSearchField(
                    value = uiState.query,
                    onValueChange = onQueryChanged,
                    onSearch = onSearch,
                    enabled = !uiState.searching,
                    modifier = Modifier.fillMaxWidth().padding(top = 4.dp, bottom = 8.dp),
                )
                if (showingResults) {
                    if (uiState.submittedQuery.isNotBlank() || uiState.searching) {
                        MediaHubSmallTitle(text = uiState.resultsHeading)
                        if (uiState.focusSubtitle.isNotBlank() && !uiState.searching) {
                            MediaHubText(
                                text = uiState.focusSubtitle,
                                color = MediaHubColors.TextMuted,
                                fontSize = 12.sp,
                                modifier = Modifier.padding(top = 2.dp, bottom = 4.dp),
                            )
                        }
                    }
                    uiState.errorMessage?.let { StatusMessage(it, MediaHubColors.Error) }
                    uiState.sourceMessage?.let { StatusMessage(it, MediaHubColors.Warning) }
                    uiState.transferMessage?.let { StatusMessage(it, MediaHubColors.Error) }
                    LazyColumn(
                        modifier = Modifier.weight(1f),
                        contentPadding = PaddingValues(vertical = 8.dp),
                        verticalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        if (uiState.titleIdentity != null) {
                            item(key = "identity") {
                                SearchIdentityCard(identity = uiState.titleIdentity)
                            }
                        }
                        if (uiState.results.isNotEmpty()) {
                            item(key = "pipeline-filter") {
                                PipelineFilterChips(
                                    selected = uiState.pipelineFilter,
                                    transferCount = uiState.results.count { !isDownloadable(it) },
                                    downloadCount = uiState.results.count { isDownloadable(it) },
                                    total = uiState.results.size,
                                    onSelected = onPipelineFilterSelected,
                                )
                            }
                        }
                        if (uiState.visibleResults.isNotEmpty()) {
                            item(key = "results") {
                                MediaHubCard {
                                    uiState.visibleResults.forEachIndexed { index, candidate ->
                                        if (index > 0) MediaHubListDivider()
                                        ReleaseRow(
                                            candidate = candidate,
                                            fallbackPoster = uiState.identities.singleOrNull()?.posterUrl,
                                            selected = selectedCandidate?.id == candidate.id,
                                            onClick = { onCandidateSelected(candidate.id) },
                                        )
                                    }
                                }
                            }
                        }
                        if (!uiState.searching && uiState.submittedQuery.isNotBlank() && uiState.results.isEmpty() && uiState.errorMessage == null) {
                            item {
                                MediaHubText(
                                    text = if (uiState.focusTitle.isNotBlank()) {
                                        "没有找到《${uiState.focusTitle}》的可获取版本"
                                    } else {
                                        "没有找到匹配资源"
                                    },
                                    modifier = Modifier.padding(vertical = 28.dp),
                                    color = MediaHubColors.TextMuted,
                                    fontSize = 13.sp,
                                )
                            }
                        }
                    }
                } else {
                    SearchIdleOverview(
                        uiState = uiState,
                        onRefresh = onRefreshOverview,
                        onOpenDetail = onOpenHealthDetail,
                        onSelect = onTrendingSelected,
                        onShuffleRecommendations = onShuffleRecommendations,
                        onGenreSelected = onGenreSelected,
                        onCategorySelected = onCategorySelected,
                        onImportShare = onImportShare,
                    )
                }
            }
        },
        detail = {
            if (selectedCandidate != null) {
                SearchCandidateDetail(
                    uiState = uiState,
                    candidate = selectedCandidate,
                    onTransfer = onTransfer,
                    onSubscribe = onSubscribe,
                    onRecommendationSelected = onRecommendationSelected,
                )
            }
        },
    )
}

@Composable
private fun SearchIdleOverview(
    uiState: SearchUiState,
    onRefresh: () -> Unit,
    onOpenDetail: () -> Unit,
    onSelect: (DiscoveryItem) -> Unit,
    onShuffleRecommendations: () -> Unit,
    onGenreSelected: (DiscoveryGenre?) -> Unit = {},
    onCategorySelected: (String) -> Unit = {},
    onImportShare: (String, String, String) -> Unit = { _, _, _ -> },
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        item {
            SystemHealthPillBar(
                integrations = uiState.integrations,
                refreshing = uiState.refreshing,
                onRefresh = onRefresh,
                onOpenDetail = onOpenDetail,
            )
        }
        item {
            ShareImportCard(
                importing = uiState.shareImporting,
                message = uiState.shareImportMessage,
                onImport = onImportShare,
            )
        }
        if (uiState.heroItems.isNotEmpty()) {
            item {
                HeroGalleryCarousel(
                    items = uiState.heroItems,
                    onSelect = onSelect,
                )
            }
        }
        item {
            CategoryTabs(
                selectedCategory = uiState.selectedCategory,
                onCategorySelected = onCategorySelected,
                modifier = Modifier.padding(horizontal = 4.dp),
            )
        }
        if (uiState.movieGenres.isNotEmpty() &&
            (uiState.selectedCategory == "all" || uiState.selectedCategory == "movie")
        ) {
            item {
                GenreFilterChips(
                    genres = uiState.movieGenres,
                    selectedGenreId = uiState.selectedGenreId,
                    onGenreSelected = onGenreSelected,
                    modifier = Modifier.padding(horizontal = 16.dp),
                )
            }
        }
        if (uiState.selectedGenreId != null &&
            (uiState.genreItems.isNotEmpty() || uiState.loadingGenre)
        ) {
            item {
                DiscoveryGallerySection(
                    title = uiState.selectedGenreName?.let { "${it}片" } ?: "类型精选",
                    subtitle = if (uiState.loadingGenre) "正在加载…" else "",
                    icon = Lucide.LayoutGrid,
                    items = uiState.genreItems,
                    onSelect = onSelect,
                )
            }
        }
        if ((uiState.selectedCategory == "all" || uiState.selectedCategory == "movie") &&
            uiState.topRatedMovies.isNotEmpty()
        ) {
            item {
                DiscoveryGallerySection(
                    title = "高分电影",
                    subtitle = "TMDB 高分",
                    icon = Lucide.Star,
                    items = uiState.topRatedMovies,
                    onSelect = onSelect,
                )
            }
        }
        if ((uiState.selectedCategory == "all" || uiState.selectedCategory == "series") &&
            uiState.popularSeries.isNotEmpty()
        ) {
            item {
                DiscoveryGallerySection(
                    title = "热门剧集",
                    subtitle = "TMDB 热度",
                    icon = Lucide.Tv,
                    items = uiState.popularSeries,
                    onSelect = onSelect,
                )
            }
        }
        if ((uiState.selectedCategory == "all" || uiState.selectedCategory == "recommended") &&
            uiState.libraryRecommendations.isNotEmpty()
        ) {
            item {
                DiscoveryGallerySection(
                    title = "猜你喜欢",
                    subtitle = uiState.libraryRecommendationSeed?.let { "基于《$it》" } ?: "",
                    icon = Lucide.Sparkles,
                    items = uiState.libraryRecommendations,
                    onSelect = onSelect,
                    actionLabel = if (uiState.shufflingRecommendations) "换一批…" else "换一批",
                    onAction = onShuffleRecommendations,
                    actionEnabled = !uiState.shufflingRecommendations,
                )
            }
        }
        if (uiState.selectedCategory == "all" && uiState.trending.isNotEmpty()) {
            item {
                DiscoveryGallerySection(
                    title = "热门精选",
                    subtitle = "",
                    icon = Lucide.Flame,
                    items = uiState.trending,
                    onSelect = onSelect,
                )
            }
        }
    }
}

@Composable
private fun SearchCandidateDetail(
    uiState: SearchUiState,
    candidate: SearchCandidate,
    onTransfer: (String) -> Unit,
    onSubscribe: (String) -> Unit,
    onRecommendationSelected: (DiscoveryItem) -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 16.dp)
            .testTag("search-detail-pane"),
    ) {
        LazyColumn(
            modifier = Modifier.weight(1f),
            contentPadding = PaddingValues(top = 8.dp, bottom = 16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            item {
                MediaHubText(text = candidateDisplayTitle(candidate), fontSize = 20.sp, fontWeight = FontWeight.SemiBold)
                MediaHubText(
                    text = buildString {
                        append(pipelineLabel(candidate))
                        append(" · ")
                        append(if (candidate.mediaType == "movie") "电影" else "剧集")
                        if (candidate.year > 0) append(" · ").append(candidate.year)
                        append(" · ").append(candidate.provider ?: candidate.source)
                    },
                    modifier = Modifier.padding(top = 4.dp),
                    color = MediaHubColors.TextMuted,
                    fontSize = 13.sp,
                )
                val identity = uiState.titleIdentity
                val rating = candidate.rating ?: identity?.rating
                val overview = candidate.overview ?: identity?.overview
                if (rating != null) {
                    MediaHubText(
                        text = "TMDB ${String.format(Locale.ROOT, "%.1f", rating)}",
                        modifier = Modifier.padding(top = 8.dp),
                        color = MediaHubColors.Accent,
                        fontSize = 13.sp,
                        fontWeight = FontWeight.SemiBold,
                    )
                }
                if (!overview.isNullOrBlank()) {
                    MediaHubText(
                        text = overview,
                        modifier = Modifier.padding(top = 8.dp),
                        color = MediaHubColors.TextSecondary,
                        fontSize = 13.sp,
                    )
                }
                MediaHubText(
                    text = buildString {
                        append(candidate.release.resolution)
                        append(" · ")
                        append(candidate.release.videoCodec)
                        candidate.release.dynamicRange?.let { append(" · ").append(it) }
                        candidate.release.audio?.let { append(" · ").append(it) }
                        append(" · ")
                        append(formatBytes(candidate.release.sizeBytes))
                    },
                    modifier = Modifier.padding(top = 8.dp),
                    color = MediaHubColors.TextSecondary,
                    fontSize = 13.sp,
                )
                if (candidate.transferState.isNotEmpty() && candidate.transferState != "unknown") {
                    val (label, variant) = transferBadge(candidate.transferState)
                    MediaHubBadge(text = label, variant = variant, modifier = Modifier.padding(top = 8.dp))
                }
            }
            uiState.transferMessage?.let { message ->
                item { StatusMessage(message, MediaHubColors.Error) }
            }
            if (uiState.recommendations.isNotEmpty()) {
                item { MediaHubSmallTitle(text = "相似内容") }
                item {
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        items(uiState.recommendations, key = { "${it.mediaType}-${it.tmdbId}" }) { item ->
                            DiscoveryItemCard(item = item, onClick = { onRecommendationSelected(item) })
                        }
                    }
                }
            }
        }
        Row(
            modifier = Modifier.fillMaxWidth().padding(bottom = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            MediaHubSecondaryButton(
                label = "订阅",
                icon = Lucide.BellPlus,
                enabled = candidate.tmdbId != null && candidate.transferState != "identity_required",
                modifier = Modifier.weight(0.35f),
                onClick = { onSubscribe(candidate.id) },
            )
            MediaHubButton(
                label = transferActionLabel(candidate, uiState.transferringCandidateId == candidate.id),
                enabled = candidate.transferToken != null && uiState.transferringCandidateId == null,
                modifier = Modifier.weight(0.65f),
                onClick = { onTransfer(candidate.id) },
            )
        }
    }
}

@Composable
private fun StatusMessage(message: String, color: Color) {
    MediaHubText(
        text = message,
        modifier = Modifier
            .padding(top = 10.dp)
            .semantics { liveRegion = LiveRegionMode.Polite },
        color = color,
        fontSize = 12.sp,
    )
}

@Composable
private fun SearchIdentityCard(identity: SearchIdentity?) {
    if (identity == null) return
    MediaHubCard {
        Row(
            modifier = Modifier.fillMaxWidth().padding(12.dp),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            RemotePoster(
                url = identity.posterUrl,
                contentDescription = "${identity.title} 海报",
                modifier = Modifier
                    .width(72.dp)
                    .aspectRatio(2f / 3f)
                    .clip(RoundedCornerShape(8.dp)),
            )
            Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                MediaHubText(text = identity.title, fontSize = 16.sp, fontWeight = FontWeight.SemiBold)
                MediaHubText(
                    text = buildString {
                        if (identity.year > 0) append(identity.year).append(" · ")
                        append(if (identity.mediaType == "series") "剧集" else "电影")
                        identity.rating?.let { append(" · ").append(String.format(Locale.ROOT, "%.1f", it)) }
                    },
                    color = MediaHubColors.TextMuted,
                    fontSize = 12.sp,
                )
                identity.overview?.takeIf { it.isNotBlank() }?.let {
                    MediaHubText(text = it, color = MediaHubColors.TextSecondary, fontSize = 13.sp)
                }
            }
        }
    }
}

@Composable
private fun PipelineFilterChips(
    selected: String,
    transferCount: Int,
    downloadCount: Int,
    total: Int,
    onSelected: (String) -> Unit,
) {
    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        item {
            MediaHubFilterChip(label = "全部 $total", selected = selected == "all", onClick = { onSelected("all") })
        }
        item {
            MediaHubFilterChip(label = "115 转存 $transferCount", selected = selected == "transfer", onClick = { onSelected("transfer") })
        }
        item {
            MediaHubFilterChip(label = "PT 下载 $downloadCount", selected = selected == "download", onClick = { onSelected("download") })
        }
    }
}

@Composable
private fun ReleaseRow(
    candidate: SearchCandidate,
    selected: Boolean,
    onClick: () -> Unit,
    fallbackPoster: String? = null,
) {
    val facts = buildString {
        append(candidate.release.resolution)
        append(" · ")
        append(candidate.release.videoCodec)
        candidate.release.dynamicRange?.let { append(" · ").append(it) }
        candidate.release.audio?.let { append(" · ").append(it) }
        append(" · ")
        append(formatBytes(candidate.release.sizeBytes))
    }
    val transferBadge = if (candidate.transferState.isNotEmpty() && candidate.transferState != "unknown") {
        transferBadge(candidate.transferState)
    } else {
        null
    }
    val poster = candidate.posterUrl ?: fallbackPoster
    MediaHubPreferenceRow(
        title = candidateDisplayTitle(candidate),
        summary = facts,
        selected = selected,
        role = Role.RadioButton,
        onClick = onClick,
        start = {
            RemotePoster(
                url = poster,
                contentDescription = "${candidate.title} 海报",
                modifier = Modifier
                    .width(40.dp)
                    .aspectRatio(2f / 3f)
                    .clip(RoundedCornerShape(6.dp)),
            )
        },
        end = {
            Column(horizontalAlignment = Alignment.End, verticalArrangement = Arrangement.spacedBy(6.dp)) {
                MediaHubBadge(
                    text = pipelineLabel(candidate),
                    variant = if (isDownloadable(candidate)) BadgeVariant.Warning else BadgeVariant.Primary,
                )
                MediaHubBadge(
                    text = candidate.provider ?: candidate.source,
                    variant = BadgeVariant.Source,
                )
                transferBadge?.let { (label, variant) ->
                    MediaHubBadge(text = label, variant = variant)
                }
            }
        },
    )
}

private fun integrationDescription(key: String): String = when (key.lowercase()) {
    "115" -> "云盘授权与直链"
    "emby" -> "媒体库与播放"
    "strm" -> "STRM 写入"
    "tmdb" -> "影视信息与推荐"
    "wecom" -> "任务通知"
    else -> "外部服务"
}

private fun statusLabel(status: String): String = when (status) {
    "healthy" -> "正常"
    "unconfigured" -> "未配置"
    "unauthorized" -> "鉴权失效"
    "degraded" -> "异常"
    else -> "不可用"
}

private fun formatBytes(size: Long): String {
    if (size <= 0) return "大小未知"
    val units = listOf("B", "KB", "MB", "GB", "TB")
    var value = size.toDouble()
    var unit = 0
    while (value >= 1024 && unit < units.lastIndex) {
        value /= 1024
        unit += 1
    }
    return String.format(Locale.ROOT, if (unit == 0) "%.0f %s" else "%.1f %s", value, units[unit])
}

private fun candidateDisplayTitle(candidate: SearchCandidate): String {
    if (candidate.season <= 0) return candidate.title
    if (candidate.episodeStart <= 0) return "${candidate.title} · S${candidate.season}"
    val episodes = if (candidate.episodeStart == candidate.episodeEnd) {
        candidate.episodeStart.toString()
    } else {
        "${candidate.episodeStart}-${candidate.episodeEnd}"
    }
    return "${candidate.title} · S${candidate.season}E$episodes"
}

private fun transferActionLabel(candidate: SearchCandidate, creating: Boolean): String = when {
    creating -> "正在创建…"
    candidate.transferState == "identity_required" -> "身份待确认"
    candidate.transferToken == null -> "工作流不可用"
    isDownloadable(candidate) -> "开始下载"
    else -> "开始转存"
}

private fun transferBadge(state: String): Pair<String, BadgeVariant> = when (state) {
    "available" -> "可转存" to BadgeVariant.Success
    "downloadable" -> "可下载" to BadgeVariant.Success
    "transferring" -> "转存中" to BadgeVariant.Primary
    "downloading" -> "下载中" to BadgeVariant.Primary
    "transferred" -> "已转存" to BadgeVariant.Success
    "identity_required" -> "身份待确认" to BadgeVariant.Warning
    else -> state to BadgeVariant.Neutral
}

@Composable
private fun ShareImportCard(
    importing: Boolean,
    message: String?,
    onImport: (String, String, String) -> Unit,
    modifier: Modifier = Modifier,
) {
    var url by remember { mutableStateOf("") }
    var receiveCode by remember { mutableStateOf("") }
    var title by remember { mutableStateOf("") }
    MediaHubCard(modifier = modifier, insideMargin = PaddingValues(16.dp)) {
        Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
            MediaHubSmallTitle(text = "导入视频")
            MediaHubText(
                text = "贴 115 分享、磁力或视频直链。115 离线拉到成人库，文件不经过 Media Hub。",
                color = MediaHubColors.TextMuted,
                fontSize = 12.sp,
            )
            MediaHubTextField(value = url, onValueChange = { url = it }, placeholder = "115 分享、磁力或视频直链")
            MediaHubTextField(value = receiveCode, onValueChange = { receiveCode = it.take(8) }, placeholder = "提取码，可选")
            MediaHubTextField(value = title, onValueChange = { title = it }, placeholder = "番号或标题，可选")
            MediaHubButton(
                label = if (importing) "正在导入" else "导入成人库",
                enabled = !importing && canSubmitShareImport(url, receiveCode),
                modifier = Modifier.fillMaxWidth(),
                onClick = { onImport(url, receiveCode, title) },
            )
            if (!message.isNullOrBlank()) {
                MediaHubText(text = message, color = MediaHubColors.TextMuted, fontSize = 12.sp)
            }
        }
    }
}

