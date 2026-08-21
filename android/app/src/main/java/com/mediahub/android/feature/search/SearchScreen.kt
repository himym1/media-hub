package com.mediahub.android.feature.search

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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.selection.selectable
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
import androidx.compose.ui.window.Dialog
import com.composables.icons.lucide.BellPlus
import com.composables.icons.lucide.ChevronRight
import com.composables.icons.lucide.Film
import com.composables.icons.lucide.Flame
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.Search
import com.composables.icons.lucide.Settings2
import com.composables.icons.lucide.Sparkles
import com.composables.icons.lucide.Tv
import com.composables.icons.lucide.Wifi
import com.composables.icons.lucide.X
import com.mediahub.android.app.LocalTwoPane
import com.mediahub.android.core.designsystem.BadgeVariant
import com.mediahub.android.core.designsystem.MediaHubBadge
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubListDetail
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubSearchBar
import com.mediahub.android.core.designsystem.MediaHubSearchField
import com.mediahub.android.core.designsystem.MediaHubSecondaryButton
import com.mediahub.android.core.designsystem.MediaHubShimmerBox
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.image.RemotePoster
import com.mediahub.android.core.network.DiscoveryItem
import com.mediahub.android.core.network.IntegrationHealth
import com.mediahub.android.core.network.SearchCandidate
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
        onCategorySelected = viewModel::onCategorySelected,
        onOpenServices = onOpenServices,
        onTransfer = viewModel::createTransfer,
        onSubscribe = viewModel::requestSubscription,
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
    onCategorySelected: (String) -> Unit = {},
    onOpenServices: () -> Unit = {},
    onTransfer: (String) -> Unit = {},
    onSubscribe: (String) -> Unit = {},
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
            onCategorySelected = onCategorySelected,
            onOpenServices = onOpenServices,
            onOpenHealthDetail = { showHealthDialog = true },
            onTransfer = onTransfer,
            onSubscribe = onSubscribe,
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
                    MediaHubSmallTitle(text = if (uiState.searching) "正在搜索…" else "搜索结果 ${uiState.results.size}")
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
                    if (uiState.results.isNotEmpty()) {
                        item(key = "results") {
                            MediaHubCard {
                                uiState.results.forEachIndexed { index, candidate ->
                                    if (index > 0) MediaHubListDivider()
                                    ReleaseRow(
                                        candidate = candidate,
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
                                text = "没有找到匹配资源",
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
                        MediaHubButton(
                            label = "订阅",
                            icon = Lucide.BellPlus,
                            enabled = selectedCandidate.tmdbId != null && selectedCandidate.transferState != "identity_required",
                            modifier = Modifier.weight(0.35f),
                            onClick = { onSubscribe(selectedCandidate.id) },
                        )
                        MediaHubButton(
                            label = when {
                                uiState.transferringCandidateId == selectedCandidate.id -> "正在创建…"
                                selectedCandidate.transferState == "identity_required" -> "身份待确认"
                                selectedCandidate.transferToken == null -> "工作流不可用"
                                else -> "开始转存"
                            },
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
                    item(key = "category-chips") {
                        CategoryFilterChips(
                            selectedCategory = uiState.selectedCategory,
                            onCategorySelected = onCategorySelected,
                            modifier = Modifier.padding(horizontal = 16.dp),
                        )
                    }

                    // 4. Personalized / Library-based Recommendations (猜你喜欢)
                    if (uiState.selectedCategory == "all" || uiState.selectedCategory == "recommended") {
                        if (uiState.libraryRecommendations.isNotEmpty()) {
                            item(key = "library-recs") {
                                DiscoveryGallerySection(
                                    title = "猜你喜欢",
                                    subtitle = uiState.libraryRecommendationSeed?.let { "基于《$it》推荐" } ?: "根据你的媒体库精选推荐",
                                    icon = Lucide.Sparkles,
                                    items = uiState.libraryRecommendations,
                                    onSelect = onRecommendationSelected,
                                )
                            }
                        } else if (uiState.selectedCategory == "recommended") {
                            item(key = "empty-recs") {
                                MediaHubCard(
                                    modifier = Modifier.padding(horizontal = 16.dp),
                                    insideMargin = PaddingValues(20.dp),
                                ) {
                                    Column(
                                        modifier = Modifier.fillMaxWidth(),
                                        horizontalAlignment = Alignment.CenterHorizontally,
                                        verticalArrangement = Arrangement.spacedBy(8.dp),
                                    ) {
                                        MediaHubIcon(Lucide.Sparkles, contentDescription = null, tint = MediaHubColors.Accent, modifier = Modifier.size(28.dp))
                                        MediaHubText(text = "正在准备猜你喜欢内容", fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = MediaHubColors.TextStrong)
                                        MediaHubText(
                                            text = "在 Emby 媒体库收录影片或点击下方刷新，系统将自动生成深度关联推荐",
                                            fontSize = 12.sp,
                                            color = MediaHubColors.TextMuted,
                                        )
                                        MediaHubButton(
                                            label = "刷新推荐",
                                            icon = Lucide.RefreshCw,
                                            onClick = onRefreshOverview,
                                            modifier = Modifier.padding(top = 4.dp),
                                        )
                                    }
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
                                title = "院线热映",
                                subtitle = "全网热度最高的电影资源",
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
                                subtitle = "正在热播的剧集与动漫",
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
                                title = "本周爆款榜",
                                subtitle = "全网热搜排名前列",
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

/**
 * Dedicated System Health Details Modal Dialog
 */
@Composable
private fun SystemHealthDetailDialog(
    integrations: List<IntegrationHealth>,
    refreshing: Boolean,
    onRefresh: () -> Unit,
    onOpenServices: () -> Unit,
    onDismiss: () -> Unit,
) {
    Dialog(onDismissRequest = onDismiss) {
        MediaHubCard(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 4.dp),
            insideMargin = PaddingValues(20.dp),
        ) {
            Column(
                modifier = Modifier.fillMaxWidth(),
                verticalArrangement = Arrangement.spacedBy(14.dp),
            ) {
                // Header
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Column {
                        MediaHubText(
                            text = "系统服务集成状态",
                            fontSize = 18.sp,
                            fontWeight = FontWeight.Bold,
                            color = MediaHubColors.TextStrong,
                        )
                        MediaHubText(
                            text = "核心驱动与媒体控制平面健康检查",
                            fontSize = 12.sp,
                            color = MediaHubColors.TextMuted,
                            modifier = Modifier.padding(top = 2.dp),
                        )
                    }
                    MediaHubIconButton(
                        imageVector = Lucide.X,
                        contentDescription = "关闭",
                        onClick = onDismiss,
                        modifier = Modifier.size(32.dp),
                    )
                }

                MediaHubListDivider()

                // Integration list
                Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    if (integrations.isEmpty()) {
                        MediaHubText(
                            text = "正在检测系统服务健康状态…",
                            fontSize = 13.sp,
                            color = MediaHubColors.TextMuted,
                            modifier = Modifier.padding(vertical = 12.dp),
                        )
                    } else {
                        integrations.forEach { item ->
                            val isHealthy = item.status == "healthy"
                            val (badgeLabel, badgeVariant) = when (item.status) {
                                "healthy" -> "正常" to BadgeVariant.Success
                                "unconfigured" -> "未配置" to BadgeVariant.Warning
                                "unauthorized" -> "鉴权失效" to BadgeVariant.Error
                                "degraded" -> "异常" to BadgeVariant.Error
                                else -> item.status to BadgeVariant.Neutral
                            }
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .clip(RoundedCornerShape(8.dp))
                                    .background(MediaHubColors.CardBackground.copy(alpha = 0.5f))
                                    .border(1.dp, MediaHubColors.BorderSubtle, RoundedCornerShape(8.dp))
                                    .padding(horizontal = 12.dp, vertical = 10.dp),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                                    modifier = Modifier.weight(1f),
                                ) {
                                    Box(
                                        modifier = Modifier
                                            .size(8.dp)
                                            .clip(CircleShape)
                                            .background(if (isHealthy) MediaHubColors.Success else MediaHubColors.Warning),
                                    )
                                    Column {
                                        MediaHubText(
                                            text = item.label,
                                            fontSize = 14.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            color = MediaHubColors.TextStrong,
                                        )
                                        MediaHubText(
                                            text = item.detail.ifBlank { integrationDescription(item.id) },
                                            fontSize = 11.sp,
                                            color = MediaHubColors.TextMuted,
                                            modifier = Modifier.padding(top = 1.dp),
                                        )
                                    }
                                }
                                MediaHubBadge(text = badgeLabel, variant = badgeVariant)
                            }
                        }
                    }
                }

                MediaHubListDivider()

                // Actions
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    MediaHubSecondaryButton(
                        label = if (refreshing) "检测中…" else "重新检测",
                        icon = Lucide.RefreshCw,
                        enabled = !refreshing,
                        modifier = Modifier.weight(0.42f),
                        onClick = onRefresh,
                    )
                    MediaHubButton(
                        label = "服务设置",
                        icon = Lucide.Settings2,
                        modifier = Modifier.weight(0.58f),
                        onClick = onOpenServices,
                    )
                }
            }
        }
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
            .clip(RoundedCornerShape(20.dp))
            .background(MediaHubColors.CardBackground.copy(alpha = 0.85f))
            .border(1.dp, MediaHubColors.BorderSubtle, RoundedCornerShape(20.dp))
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
            modifier = Modifier.size(32.dp),
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
                .height(185.dp)
                .clip(RoundedCornerShape(14.dp))
                .background(MediaHubColors.CardBackground)
                .border(1.dp, MediaHubColors.BorderSubtle, RoundedCornerShape(14.dp))
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
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    MediaHubBadge(
                        text = "🔥 本周热播 TOP ${safeIndex + 1}",
                        variant = BadgeVariant.Primary,
                    )
                    MediaHubBadge(
                        text = if (currentItem.mediaType == "movie") "电影" else "剧集",
                        variant = BadgeVariant.Neutral,
                    )
                }

                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.Bottom,
                ) {
                    Column(modifier = Modifier.weight(1f).padding(end = 12.dp)) {
                        MediaHubText(
                            text = currentItem.title,
                            fontSize = 18.sp,
                            fontWeight = FontWeight.Bold,
                            color = MediaHubColors.TextStrong,
                            maxLines = 1,
                            overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
                        )
                        MediaHubText(
                            text = buildString {
                                if (currentItem.year > 0) append("${currentItem.year}年 · ")
                                append(if (currentItem.mediaType == "movie") "电影频道" else "连载剧集")
                                append(" · 全网高热度")
                            },
                            fontSize = 12.sp,
                            color = MediaHubColors.TextSecondary,
                            modifier = Modifier.padding(top = 2.dp),
                        )
                    }

                    MediaHubButton(
                        label = "立即搜源",
                        icon = Lucide.Search,
                        onClick = { onSelect(currentItem) },
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
                            .padding(horizontal = 3.dp)
                            .height(4.dp)
                            .width(if (isSelected) 18.dp else 6.dp)
                            .clip(RoundedCornerShape(2.dp))
                            .background(if (isSelected) MediaHubColors.Accent else MediaHubColors.BorderStrong)
                            .clickable { currentIndex = index },
                    )
                }
            }
        }
    }
}

/**
 * Filter pills for discovering specific media types
 */
@Composable
private fun CategoryFilterChips(
    selectedCategory: String,
    onCategorySelected: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    val categories = listOf(
        "all" to "✨ 全部发现",
        "recommended" to "🎯 猜你喜欢",
        "movie" to "🎬 热门电影",
        "series" to "📺 热门剧集",
    )

    LazyRow(
        modifier = modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        items(categories) { (key, label) ->
            val isSelected = selectedCategory == key
            Box(
                modifier = Modifier
                    .clip(RoundedCornerShape(16.dp))
                    .background(
                        if (isSelected) MediaHubColors.Accent.copy(alpha = 0.18f) else MediaHubColors.CardBackground,
                    )
                    .border(
                        1.dp,
                        if (isSelected) MediaHubColors.Accent else MediaHubColors.BorderSubtle,
                        RoundedCornerShape(16.dp),
                    )
                    .clickable { onCategorySelected(key) }
                    .padding(horizontal = 14.dp, vertical = 7.dp),
                contentAlignment = Alignment.Center,
            ) {
                MediaHubText(
                    text = label,
                    fontSize = 13.sp,
                    fontWeight = if (isSelected) FontWeight.SemiBold else FontWeight.Normal,
                    color = if (isSelected) MediaHubColors.Accent else MediaHubColors.TextSecondary,
                )
            }
        }
    }
}

/**
 * Horizontal gallery section with squircle poster cards
 */
@Composable
private fun DiscoveryGallerySection(
    title: String,
    subtitle: String,
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    items: List<DiscoveryItem>,
    onSelect: (DiscoveryItem) -> Unit,
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
                    fontWeight = FontWeight.Bold,
                    color = MediaHubColors.TextStrong,
                )
            }
            MediaHubText(
                text = subtitle,
                fontSize = 12.sp,
                color = MediaHubColors.TextMuted,
            )
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
                .clip(RoundedCornerShape(10.dp))
                .background(MediaHubColors.CardBackground)
                .border(1.dp, MediaHubColors.BorderSubtle, RoundedCornerShape(10.dp)),
        ) {
            RemotePoster(
                url = item.posterUrl,
                contentDescription = item.title,
                modifier = Modifier.fillMaxSize(),
            )

            // Top Floating Tag
            Box(
                modifier = Modifier
                    .align(Alignment.TopStart)
                    .padding(6.dp)
                    .clip(RoundedCornerShape(4.dp))
                    .background(Color(0xCC0B0D10))
                    .padding(horizontal = 5.dp, vertical = 2.dp),
            ) {
                MediaHubText(
                    text = if (item.mediaType == "movie") "电影" else "剧集",
                    fontSize = 10.sp,
                    fontWeight = FontWeight.Medium,
                    color = MediaHubColors.TextSecondary,
                )
            }
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
            text = if (item.year > 0) "${item.year} 年" else "热播中",
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
    onCategorySelected: (String) -> Unit = {},
    onOpenServices: () -> Unit = {},
    onOpenHealthDetail: () -> Unit = {},
    onTransfer: (String) -> Unit = {},
    onSubscribe: (String) -> Unit = {},
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
                        MediaHubSmallTitle(text = if (uiState.searching) "正在搜索…" else "搜索结果 ${uiState.results.size}")
                    }
                    uiState.errorMessage?.let { StatusMessage(it, MediaHubColors.Error) }
                    uiState.sourceMessage?.let { StatusMessage(it, MediaHubColors.Warning) }
                    uiState.transferMessage?.let { StatusMessage(it, MediaHubColors.Error) }
                    LazyColumn(
                        modifier = Modifier.weight(1f),
                        contentPadding = PaddingValues(vertical = 8.dp),
                        verticalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        if (uiState.results.isNotEmpty()) {
                            item(key = "results") {
                                MediaHubCard {
                                    uiState.results.forEachIndexed { index, candidate ->
                                        if (index > 0) MediaHubListDivider()
                                        ReleaseRow(
                                            candidate = candidate,
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
                                    text = "没有找到匹配资源",
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
                        onCategorySelected = onCategorySelected,
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
    onCategorySelected: (String) -> Unit,
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
        if (uiState.heroItems.isNotEmpty()) {
            item {
                HeroGalleryCarousel(
                    items = uiState.heroItems,
                    onSelect = onSelect,
                )
            }
        }
        if (uiState.libraryRecommendations.isNotEmpty()) {
            item {
                DiscoveryGallerySection(
                    title = "猜你喜欢",
                    subtitle = uiState.libraryRecommendationSeed?.let { "基于《$it》" } ?: "精选推荐",
                    icon = Lucide.Sparkles,
                    items = uiState.libraryRecommendations,
                    onSelect = onSelect,
                )
            }
        }
        if (uiState.trending.isNotEmpty()) {
            item {
                DiscoveryGallerySection(
                    title = "热门精选",
                    subtitle = "实时全网热榜",
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
                        append(if (candidate.mediaType == "movie") "电影" else "剧集")
                        if (candidate.year > 0) append(" · ").append(candidate.year)
                        append(" · ").append(candidate.provider ?: candidate.source)
                    },
                    modifier = Modifier.padding(top = 4.dp),
                    color = MediaHubColors.TextMuted,
                    fontSize = 13.sp,
                )
            }
            item {
                MediaHubCard(insideMargin = PaddingValues(16.dp)) {
                    MediaHubText(
                        text = buildString {
                            append(candidate.release.resolution)
                            append(" · ")
                            append(candidate.release.videoCodec)
                            candidate.release.dynamicRange?.let { append(" · ").append(it) }
                            candidate.release.audio?.let { append(" · ").append(it) }
                        },
                        color = MediaHubColors.TextSecondary,
                        fontSize = 13.sp,
                    )
                    MediaHubText(
                        text = formatBytes(candidate.release.sizeBytes),
                        modifier = Modifier.padding(top = 8.dp),
                        color = MediaHubColors.TextMuted,
                        fontSize = 12.sp,
                    )
                    if (candidate.transferState.isNotEmpty() && candidate.transferState != "unknown") {
                        val (label, variant) = when (candidate.transferState) {
                            "available" -> "可转存" to BadgeVariant.Success
                            "transferring" -> "转存中" to BadgeVariant.Primary
                            "transferred" -> "已转存" to BadgeVariant.Success
                            "identity_required" -> "身份待确认" to BadgeVariant.Warning
                            else -> candidate.transferState to BadgeVariant.Neutral
                        }
                        MediaHubBadge(text = label, variant = variant, modifier = Modifier.padding(top = 8.dp))
                    }
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
            MediaHubButton(
                label = "订阅",
                icon = Lucide.BellPlus,
                enabled = candidate.tmdbId != null && candidate.transferState != "identity_required",
                modifier = Modifier.weight(0.35f),
                onClick = { onSubscribe(candidate.id) },
            )
            MediaHubButton(
                label = when {
                    uiState.transferringCandidateId == candidate.id -> "正在创建…"
                    candidate.transferState == "identity_required" -> "身份待确认"
                    candidate.transferToken == null -> "工作流不可用"
                    else -> "开始转存"
                },
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
private fun ReleaseRow(
    candidate: SearchCandidate,
    selected: Boolean,
    onClick: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 52.dp)
            .selectable(
                selected = selected,
                onClick = onClick,
                role = Role.RadioButton,
            )
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(
                text = candidateDisplayTitle(candidate),
                modifier = Modifier.weight(1f).padding(end = 10.dp),
                color = MediaHubColors.TextStrong,
                fontSize = 15.sp,
                fontWeight = FontWeight.SemiBold,
            )
            MediaHubBadge(
                text = candidate.provider ?: candidate.source,
                variant = BadgeVariant.Source,
            )
        }
        MediaHubText(
            text = buildString {
                append(candidate.release.resolution)
                append(" · ")
                append(candidate.release.videoCodec)
                candidate.release.dynamicRange?.let { append(" · ").append(it) }
                candidate.release.audio?.let { append(" · ").append(it) }
            },
            color = MediaHubColors.TextSecondary,
            fontSize = 12.sp,
        )
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(
                text = formatBytes(candidate.release.sizeBytes),
                color = MediaHubColors.TextMuted,
                fontSize = 12.sp,
            )
            if (candidate.transferState.isNotEmpty() && candidate.transferState != "unknown") {
                val (label, variant) = when (candidate.transferState) {
                    "available" -> "可转存" to BadgeVariant.Success
                    "transferring" -> "转存中" to BadgeVariant.Primary
                    "transferred" -> "已转存" to BadgeVariant.Success
                    "identity_required" -> "身份待确认" to BadgeVariant.Warning
                    else -> candidate.transferState to BadgeVariant.Neutral
                }
                MediaHubBadge(text = label, variant = variant)
            }
        }
    }
}

private fun integrationDescription(key: String): String = when (key.lowercase()) {
    "115" -> "PKCE 授权驱动 · 云端直链与存储调度"
    "emby" -> "媒体库同步 · 视频回放与 STRM 指向"
    "qmediasync", "qms" -> "STRM 直链生成 · 挂载文件实时同步"
    "tmdb" -> "影视信息刮削 · 猜你喜欢与热门推荐"
    "wecom" -> "企业微信通知 · 任务完成与异常提醒"
    else -> "后台自动化服务与媒体控制平面"
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

