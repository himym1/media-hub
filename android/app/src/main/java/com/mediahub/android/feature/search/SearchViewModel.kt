package com.mediahub.android.feature.search

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.DiscoveryItem
import com.mediahub.android.core.network.IntegrationHealth
import com.mediahub.android.core.network.SearchCandidate
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class SearchUiState(
    val query: String = "",
    val submittedQuery: String = "",
    val results: List<SearchCandidate> = emptyList(),
    val trending: List<DiscoveryItem> = emptyList(),
    val libraryRecommendations: List<DiscoveryItem> = emptyList(),
    val libraryRecommendationSeed: String? = null,
    val shufflingRecommendations: Boolean = false,
    val recommendations: List<DiscoveryItem> = emptyList(),
    val integrations: List<IntegrationHealth> = emptyList(),
    val selectedCategory: String = "all",
    val selectedCandidateId: String? = null,
    val searching: Boolean = false,
    val initialLoading: Boolean = false,
    val refreshing: Boolean = false,
    val errorMessage: String? = null,
    val sourceMessage: String? = null,
    val transferringCandidateId: String? = null,
    val transferMessage: String? = null,
) {
    val selectedCandidate: SearchCandidate?
        get() = results.firstOrNull { it.id == selectedCandidateId }

    val heroItems: List<DiscoveryItem>
        get() = trending.take(5)

    val trendingMovies: List<DiscoveryItem>
        get() = trending.filter { it.mediaType == "movie" }

    val trendingSeries: List<DiscoveryItem>
        get() = trending.filter { it.mediaType == "tv" || it.mediaType == "series" }
}

sealed interface SearchEvent {
    data object TransferCreated : SearchEvent
    data class SubscriptionRequested(val candidate: SearchCandidate) : SearchEvent
}

private data class RecommendationSeed(
    val mediaType: String,
    val tmdbId: String,
    val title: String,
)

class SearchViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(SearchUiState())
    val uiState: StateFlow<SearchUiState> = _uiState.asStateFlow()

    private val _events = MutableSharedFlow<SearchEvent>()
    val events: SharedFlow<SearchEvent> = _events.asSharedFlow()

    private var searchJob: Job? = null
    private var loadJob: Job? = null
    private var recommendationJob: Job? = null
    private var shuffleJob: Job? = null

    private var recommendationSeeds: List<RecommendationSeed> = emptyList()
    private var recommendationSeedIndex: Int = 0
    private var recommendationPool: List<DiscoveryItem> = emptyList()
    private var recommendationPage: Int = 0

    fun loadInitialData() {
        if (_uiState.value.trending.isNotEmpty() || _uiState.value.integrations.isNotEmpty()) {
            return
        }
        loadJob?.cancel()
        loadJob = viewModelScope.launch {
            val cached = repository.cachedTrending()
            if (cached != null) {
                _uiState.value = _uiState.value.copy(trending = cached, initialLoading = false)
                fetchDiscoveryData(forceNetwork = true)
            } else {
                _uiState.value = _uiState.value.copy(initialLoading = true)
                fetchDiscoveryData(forceNetwork = true)
                _uiState.value = _uiState.value.copy(initialLoading = false)
            }
        }
    }

    fun refreshAll() {
        loadJob?.cancel()
        loadJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(refreshing = true)
            fetchDiscoveryData(forceNetwork = true)
            _uiState.value = _uiState.value.copy(refreshing = false)
        }
    }

    private fun normalizeMediaType(raw: String): String =
        if (raw.equals("tv", ignoreCase = true) ||
            raw.equals("series", ignoreCase = true) ||
            raw.equals("Season", ignoreCase = true) ||
            raw.equals("Episode", ignoreCase = true)
        ) {
            "series"
        } else {
            "movie"
        }

    private suspend fun fetchDiscoveryData(forceNetwork: Boolean) {
        try {
            val integrations = repository.overview()
            _uiState.value = _uiState.value.copy(integrations = integrations)
        } catch (_: Exception) {
        }

        var trendingItems: List<DiscoveryItem> = emptyList()
        try {
            trendingItems = repository.trending(forceRefresh = forceNetwork)
            _uiState.value = _uiState.value.copy(trending = trendingItems)
        } catch (_: Exception) {
            if (trendingItems.isEmpty()) {
                trendingItems = _uiState.value.trending
            }
        }

        try {
            val seeds = mutableListOf<RecommendationSeed>()
            val seen = mutableSetOf<String>()

            val libraries = runCatching { repository.libraries() }.getOrNull().orEmpty()
            for (library in libraries) {
                val page = runCatching { repository.libraryItems(library.id, 0, 40) }.getOrNull() ?: continue
                for (item in page.items) {
                    val tmdbId = item.tmdbId?.takeIf { it.isNotBlank() } ?: continue
                    val key = "${normalizeMediaType(item.type)}:$tmdbId"
                    if (!seen.add(key)) continue
                    seeds += RecommendationSeed(
                        mediaType = normalizeMediaType(item.type),
                        tmdbId = tmdbId,
                        title = item.name,
                    )
                    if (seeds.size >= 12) break
                }
                if (seeds.size >= 12) break
            }

            if (seeds.isEmpty()) {
                for (topItem in trendingItems) {
                    if (topItem.tmdbId.isBlank()) continue
                    val mediaType = normalizeMediaType(topItem.mediaType)
                    val key = "$mediaType:${topItem.tmdbId}"
                    if (!seen.add(key)) continue
                    seeds += RecommendationSeed(mediaType, topItem.tmdbId, topItem.title)
                    if (seeds.size >= 8) break
                }
            }

            recommendationSeeds = seeds.shuffled()
            recommendationSeedIndex = 0
            recommendationPool = emptyList()
            recommendationPage = 0

            if (recommendationSeeds.isNotEmpty()) {
                loadRecommendationBatch(resetSeed = false)
            } else if (trendingItems.size >= 2) {
                publishRecommendationPage(
                    pool = trendingItems.shuffled(),
                    seedTitle = "全网热播精选",
                    page = 0,
                )
            }
        } catch (_: Exception) {
        }
    }

    fun shuffleRecommendations() {
        if (_uiState.value.shufflingRecommendations) return
        if (recommendationPool.isEmpty() && recommendationSeeds.isEmpty()) {
            refreshAll()
            return
        }
        shuffleJob?.cancel()
        shuffleJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(shufflingRecommendations = true)
            try {
                val nextPage = recommendationPage + 1
                val pageStart = nextPage * RECOMMENDATION_PAGE_SIZE
                when {
                    recommendationPool.size > pageStart -> {
                        publishRecommendationPage(
                            pool = recommendationPool,
                            seedTitle = _uiState.value.libraryRecommendationSeed,
                            page = nextPage,
                        )
                    }
                    recommendationSeeds.size > 1 -> {
                        recommendationSeedIndex = (recommendationSeedIndex + 1) % recommendationSeeds.size
                        recommendationPage = 0
                        loadRecommendationBatch(resetSeed = false)
                    }
                    recommendationPool.size > RECOMMENDATION_PAGE_SIZE -> {
                        publishRecommendationPage(
                            pool = recommendationPool.shuffled(),
                            seedTitle = _uiState.value.libraryRecommendationSeed,
                            page = 0,
                        )
                    }
                    else -> loadRecommendationBatch(resetSeed = true)
                }
            } finally {
                _uiState.value = _uiState.value.copy(shufflingRecommendations = false)
            }
        }
    }

    private suspend fun loadRecommendationBatch(resetSeed: Boolean) {
        if (recommendationSeeds.isEmpty()) return
        if (resetSeed) {
            recommendationSeeds = recommendationSeeds.shuffled()
            recommendationSeedIndex = 0
        }
        val start = recommendationSeedIndex
        for (offset in recommendationSeeds.indices) {
            val seed = recommendationSeeds[(start + offset) % recommendationSeeds.size]
            val result = runCatching {
                repository.recommendations(seed.mediaType, seed.tmdbId)
            }.getOrNull().orEmpty()
            if (result.isEmpty()) continue
            recommendationSeedIndex = (start + offset) % recommendationSeeds.size
            publishRecommendationPage(pool = result, seedTitle = seed.title, page = 0)
            return
        }
        val trendingFallback = _uiState.value.trending
        if (trendingFallback.size >= 2) {
            publishRecommendationPage(
                pool = trendingFallback.shuffled(),
                seedTitle = "全网热播精选",
                page = 0,
            )
        }
    }

    private fun publishRecommendationPage(
        pool: List<DiscoveryItem>,
        seedTitle: String?,
        page: Int,
    ) {
        recommendationPool = pool
        recommendationPage = page
        val start = page * RECOMMENDATION_PAGE_SIZE
        val visible = pool.drop(start).take(RECOMMENDATION_PAGE_SIZE).ifEmpty {
            recommendationPage = 0
            pool.take(RECOMMENDATION_PAGE_SIZE)
        }
        if (visible.isEmpty()) return
        _uiState.value = _uiState.value.copy(
            libraryRecommendations = visible,
            libraryRecommendationSeed = seedTitle,
        )
    }

    fun onCategorySelected(category: String) {
        _uiState.value = _uiState.value.copy(selectedCategory = category)
    }

    fun onQueryChanged(query: String) {
        if (query.length > 120) return
        _uiState.value = _uiState.value.copy(query = query, errorMessage = null)
    }

    fun submitSearch() {
        val query = _uiState.value.query.trim()
        if (query.isEmpty()) return
        searchJob?.cancel()
        searchJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(
                submittedQuery = query,
                searching = true,
                errorMessage = null,
                sourceMessage = null,
                recommendations = emptyList(),
            )
            try {
                val response = repository.search(query)
                _uiState.value = _uiState.value.copy(
                    searching = false,
                    results = response.results,
                    selectedCandidateId = response.results.firstOrNull()?.id,
                    sourceMessage = response.sourceErrors.joinToString(" · ") { it.message }.ifEmpty { null },
                )
                response.results.firstOrNull()?.let { onCandidateSelected(it.id) }
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(
                    searching = false,
                    errorMessage = error.message ?: "搜索失败",
                )
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(
                    searching = false,
                    errorMessage = "无法连接搜索服务",
                )
            }
        }
    }

    fun refreshOverview() = refreshAll()

    fun refreshTrending() = refreshAll()

    fun searchTrending(item: DiscoveryItem) {
        _uiState.value = _uiState.value.copy(query = item.title)
        submitSearch()
    }

    fun searchRecommendation(item: DiscoveryItem) = searchTrending(item)

    fun onCandidateSelected(candidateId: String) {
        val candidate = _uiState.value.results.firstOrNull { it.id == candidateId }
        _uiState.value = _uiState.value.copy(selectedCandidateId = candidateId, recommendations = emptyList())
        recommendationJob?.cancel()
        if (candidate?.tmdbId == null) return
        recommendationJob = viewModelScope.launch {
            try {
                val items = repository.recommendations(candidate.mediaType, candidate.tmdbId)
                if (_uiState.value.selectedCandidateId == candidateId) {
                    _uiState.value = _uiState.value.copy(recommendations = items)
                }
            } catch (_: Exception) {
                // Resource selection remains usable when recommendations are unavailable.
            }
        }
    }

    fun requestSubscription(candidateId: String) {
        val candidate = _uiState.value.results.firstOrNull { it.id == candidateId } ?: return
        if (candidate.tmdbId == null || candidate.transferState == "identity_required") {
            _uiState.value = _uiState.value.copy(transferMessage = "当前资源身份尚未验证")
            return
        }
        viewModelScope.launch { _events.emit(SearchEvent.SubscriptionRequested(candidate)) }
    }

    fun createTransfer(candidateId: String) {
        val candidate = _uiState.value.results.firstOrNull { it.id == candidateId } ?: return
        val token = candidate.transferToken
        if (token == null || _uiState.value.transferringCandidateId != null) {
            _uiState.value = _uiState.value.copy(transferMessage = "当前资源不能转存")
            return
        }
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(
                transferringCandidateId = candidateId,
                transferMessage = null,
            )
            try {
                repository.createTransfer(token)
                _uiState.value = _uiState.value.copy(transferringCandidateId = null)
                _events.emit(SearchEvent.TransferCreated)
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(
                    transferringCandidateId = null,
                    transferMessage = error.message ?: "转存任务创建失败",
                )
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(
                    transferringCandidateId = null,
                    transferMessage = "无法创建转存任务",
                )
            }
        }
    }

    companion object {
        private const val RECOMMENDATION_PAGE_SIZE = 8
    }
}
