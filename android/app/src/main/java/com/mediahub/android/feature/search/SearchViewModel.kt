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
    val recommendations: List<DiscoveryItem> = emptyList(),
    val integrations: List<IntegrationHealth> = emptyList(),
    val selectedCandidateId: String? = null,
    val searching: Boolean = false,
    val trendingLoading: Boolean = false,
    val refreshingOverview: Boolean = false,
    val errorMessage: String? = null,
    val sourceMessage: String? = null,
    val transferringCandidateId: String? = null,
    val transferMessage: String? = null,
) {
    val selectedCandidate: SearchCandidate?
        get() = results.firstOrNull { it.id == selectedCandidateId }
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
    private var overviewJob: Job? = null
    private var trendingJob: Job? = null
    private var recommendationJob: Job? = null

    fun refreshAll() {
        refreshOverview()
        refreshTrending()
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

    fun refreshOverview() {
        overviewJob?.cancel()
        overviewJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(refreshingOverview = true)
            try {
                val integrations = repository.overview()
                _uiState.value = _uiState.value.copy(
                    refreshingOverview = false,
                    integrations = integrations,
                )
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(refreshingOverview = false)
            }
        }
    }

    fun refreshTrending() {
        trendingJob?.cancel()
        trendingJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(trendingLoading = true)
            try {
                _uiState.value = _uiState.value.copy(
                    trending = repository.trending(),
                    trendingLoading = false,
                )
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(trendingLoading = false)
            }
        }
    }

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
