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

    fun loadInitialData() {
        if (_uiState.value.trending.isNotEmpty() || _uiState.value.integrations.isNotEmpty()) {
            return
        }
        loadJob?.cancel()
        loadJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(initialLoading = true)
            fetchDiscoveryData()
            _uiState.value = _uiState.value.copy(initialLoading = false)
        }
    }

    fun refreshAll() {
        loadJob?.cancel()
        loadJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(refreshing = true)
            fetchDiscoveryData()
            _uiState.value = _uiState.value.copy(refreshing = false)
        }
    }

    private fun normalizeMediaType(raw: String): String =
        if (raw.equals("tv", ignoreCase = true) ||
            raw.equals("series", ignoreCase = true) ||
            raw.equals("Season", ignoreCase = true) ||
            raw.equals("Episode", ignoreCase = true)
        ) "series" else "movie"

    private suspend fun fetchDiscoveryData() {
        // 1. Fetch overview health
        try {
            val integrations = repository.overview()
            _uiState.value = _uiState.value.copy(integrations = integrations)
        } catch (_: Exception) {}

        // 2. Fetch trending items
        var trendingItems: List<DiscoveryItem> = emptyList()
        try {
            trendingItems = repository.trending()
            _uiState.value = _uiState.value.copy(trending = trendingItems)
        } catch (_: Exception) {}

        // 3. Fetch library-based or seed recommendations
        try {
            var seedTitle: String? = null
            var recs: List<DiscoveryItem> = emptyList()

            // Try to find a movie/show in user's Emby library
            val libraries = runCatching { repository.libraries() }.getOrNull().orEmpty()
            for (library in libraries) {
                if (recs.isNotEmpty()) break
                val page = runCatching { repository.libraryItems(library.id, 0, 20) }.getOrNull()
                val seedItem = page?.items?.firstOrNull { it.tmdbId != null && it.tmdbId.isNotBlank() }
                if (seedItem != null && seedItem.tmdbId != null) {
                    val mediaType = normalizeMediaType(seedItem.type)
                    val result = runCatching { repository.recommendations(mediaType, seedItem.tmdbId) }.getOrNull().orEmpty()
                    if (result.isNotEmpty()) {
                        recs = result
                        seedTitle = seedItem.name
                    }
                }
            }

            // Fallback to top trending items if library recommendation is empty
            if (recs.isEmpty() && trendingItems.isNotEmpty()) {
                for (topItem in trendingItems.take(3)) {
                    if (recs.isNotEmpty()) break
                    if (topItem.tmdbId.isNotBlank()) {
                        val mediaType = normalizeMediaType(topItem.mediaType)
                        val result = runCatching { repository.recommendations(mediaType, topItem.tmdbId) }.getOrNull().orEmpty()
                        if (result.isNotEmpty()) {
                            recs = result
                            seedTitle = topItem.title
                        }
                    }
                }
            }

            // Curated fallback if TMDB upstream recommendation endpoint is unavailable or returns empty
            if (recs.isEmpty() && trendingItems.size >= 2) {
                recs = trendingItems.shuffled().take(8)
                seedTitle = "全网热播精选"
            }

            if (recs.isNotEmpty()) {
                _uiState.value = _uiState.value.copy(
                    libraryRecommendations = recs,
                    libraryRecommendationSeed = seedTitle,
                )
            }
        } catch (_: Exception) {}
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
}
