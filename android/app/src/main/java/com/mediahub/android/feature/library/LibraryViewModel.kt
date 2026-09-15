package com.mediahub.android.feature.library

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.EmbyItem
import com.mediahub.android.core.network.MediaLibrary
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

internal const val LibraryPageSize = 24

data class LibraryBrowseState(
    val query: String = "",
    val submittedQuery: String = "",
    val libraries: List<MediaLibrary> = emptyList(),
    val selectedLibraryId: String? = null,
    val items: List<EmbyItem> = emptyList(),
    val total: Int = 0,
    val page: Int = 0,
    val loadingLibraries: Boolean = false,
    val loadingItems: Boolean = false,
    val loadingMore: Boolean = false,
    val refreshing: Boolean = false,
    val errorMessage: String? = null,
    val actionMessage: String? = null,
)

class LibraryViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(LibraryBrowseState())
    val uiState: StateFlow<LibraryBrowseState> = _uiState.asStateFlow()
    private var contentJob: Job? = null
    private var moreJob: Job? = null
    private var librariesJob: Job? = null

    private fun cancelContent() {
        contentJob?.cancel()
        moreJob?.cancel()
    }

    /** First open loads; revisits keep the current grid and only soft-refresh in background. */
    fun ensureLibrariesLoaded() {
        val state = _uiState.value
        if (state.libraries.isNotEmpty() && (state.items.isNotEmpty() || state.loadingItems || state.submittedQuery.isNotEmpty())) {
            softRefreshLibraries()
            return
        }
        refreshLibraries(force = true)
    }

    fun refreshLibraries(force: Boolean = true) {
        librariesJob?.cancel()
        moreJob?.cancel()
        librariesJob = viewModelScope.launch {
            val keepGrid = !force && _uiState.value.items.isNotEmpty() && _uiState.value.submittedQuery.isEmpty()
            _uiState.value = _uiState.value.copy(
                loadingLibraries = _uiState.value.libraries.isEmpty(),
                errorMessage = null,
            )
            try {
                val libraries = repository.libraries()
                val previousSelected = _uiState.value.selectedLibraryId
                val selected = previousSelected?.takeIf { id -> libraries.any { it.id == id } }
                    ?: libraries.firstOrNull()?.id
                val browsing = _uiState.value.submittedQuery.isEmpty()
                val selectionChanged = selected != previousSelected
                _uiState.value = _uiState.value.copy(
                    libraries = libraries,
                    selectedLibraryId = selected,
                    loadingLibraries = false,
                    page = if (browsing && selectionChanged) 0 else _uiState.value.page,
                )
                if (browsing && selected != null) {
                    val page = if (selectionChanged) 0 else _uiState.value.page
                    loadLibraryPage(
                        libraryId = selected,
                        page = page,
                        keepExisting = keepGrid && !selectionChanged,
                    )
                }
            } catch (cancelled: CancellationException) {
                throw cancelled
            } catch (error: ApiException) {
                fail(error.message ?: "媒体库读取失败", libraries = true)
            } catch (_: Exception) {
                fail("无法读取 Emby 媒体库", libraries = true)
            }
        }
    }

    private fun softRefreshLibraries() {
        refreshLibraries(force = false)
    }

    fun selectLibrary(id: String) {
        if (_uiState.value.libraries.none { it.id == id }) return
        if (id == _uiState.value.selectedLibraryId && _uiState.value.submittedQuery.isEmpty()) return
        _uiState.value = _uiState.value.copy(
            selectedLibraryId = id,
            query = "",
            submittedQuery = "",
            page = 0,
            actionMessage = null,
        )
        cancelContent()
        contentJob = viewModelScope.launch { loadLibraryPage(id, 0, keepExisting = false) }
    }

    fun onQueryChanged(query: String) {
        if (query.length <= 120) _uiState.value = _uiState.value.copy(query = query, errorMessage = null)
    }

    fun search() {
        val query = _uiState.value.query.trim()
        if (query.isEmpty()) {
            clearSearch()
            return
        }
        cancelContent()
        contentJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(submittedQuery = query, loadingItems = true, loadingMore = false, errorMessage = null)
            try {
                val items = repository.items(query)
                _uiState.value = _uiState.value.copy(items = items, total = items.size, page = 0, loadingItems = false)
            } catch (cancelled: CancellationException) {
                throw cancelled
            } catch (error: ApiException) {
                fail(error.message ?: "Emby 搜索失败")
            } catch (_: Exception) {
                fail("无法搜索 Emby 媒体库")
            }
        }
    }

    fun clearSearch() {
        val libraryId = _uiState.value.selectedLibraryId
        _uiState.value = _uiState.value.copy(query = "", submittedQuery = "", page = 0, errorMessage = null)
        if (libraryId != null) {
            cancelContent()
            contentJob = viewModelScope.launch { loadLibraryPage(libraryId, 0, keepExisting = false) }
        }
    }

    fun loadMore() {
        val state = _uiState.value
        if (state.submittedQuery.isNotEmpty() || state.loadingItems || state.loadingMore) return
        val libraryId = state.selectedLibraryId ?: return
        if (!libraryHasMore(state.items.size, state.total, searching = false)) return
        val page = nextLibraryPage(state.page, 1, state.total) ?: return
        moreJob?.cancel()
        moreJob = viewModelScope.launch { loadLibraryPage(libraryId, page, append = true) }
    }

    fun reloadItems() {
        val submitted = _uiState.value.submittedQuery
        if (submitted.isNotEmpty()) {
            search()
            return
        }
        val libraryId = _uiState.value.selectedLibraryId ?: return
        cancelContent()
        contentJob = viewModelScope.launch {
            loadLibraryPage(libraryId, 0, keepExisting = _uiState.value.items.isNotEmpty())
        }
    }

    fun refreshSelectedLibrary() {
        val id = _uiState.value.selectedLibraryId ?: return
        if (_uiState.value.refreshing) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(refreshing = true, errorMessage = null, actionMessage = null)
            try {
                repository.refreshLibrary(id)
                _uiState.value = _uiState.value.copy(refreshing = false, actionMessage = "已请求 Emby 刷新媒体库")
                if (_uiState.value.submittedQuery.isEmpty()) {
                    loadLibraryPage(id, 0, keepExisting = true)
                }
            } catch (error: ApiException) {
                fail(error.message ?: "媒体库刷新失败", refreshing = true)
            } catch (_: Exception) {
                fail("无法刷新 Emby 媒体库", refreshing = true)
            }
        }
    }

    private suspend fun loadLibraryPage(
        libraryId: String,
        page: Int,
        keepExisting: Boolean = false,
        append: Boolean = false,
    ) {
        val showLoading = !append && (!keepExisting || _uiState.value.items.isEmpty())
        _uiState.value = _uiState.value.copy(
            loadingItems = showLoading,
            loadingMore = append,
            errorMessage = null,
        )
        try {
            val start = if (keepExisting && !append && _uiState.value.items.isNotEmpty()) 0 else page * LibraryPageSize
            val limit = if (keepExisting && !append && _uiState.value.items.isNotEmpty()) {
                _uiState.value.items.size.coerceAtLeast(LibraryPageSize)
            } else {
                LibraryPageSize
            }
            val result = repository.libraryItems(libraryId, start, limit)
            if (_uiState.value.selectedLibraryId == libraryId && _uiState.value.submittedQuery.isEmpty()) {
                val items = if (append) {
                    val seen = _uiState.value.items.map { it.id }.toHashSet()
                    _uiState.value.items + result.items.filter { it.id !in seen }
                } else {
                    result.items
                }
                val resolvedPage = if (append) page else ((items.size - 1).coerceAtLeast(0) / LibraryPageSize)
                val unchanged = !append &&
                    _uiState.value.total == result.total &&
                    _uiState.value.items.map { it.id } == items.map { it.id }
                if (unchanged) {
                    _uiState.value = _uiState.value.copy(loadingItems = false, loadingMore = false)
                } else {
                    _uiState.value = _uiState.value.copy(
                        items = items,
                        total = result.total,
                        page = resolvedPage,
                        loadingItems = false,
                        loadingMore = false,
                    )
                }
            }
        } catch (cancelled: CancellationException) {
            throw cancelled
        } catch (error: ApiException) {
            if (_uiState.value.selectedLibraryId == libraryId && _uiState.value.submittedQuery.isEmpty()) {
                fail(error.message ?: "媒体内容读取失败")
            }
        } catch (_: Exception) {
            if (_uiState.value.selectedLibraryId == libraryId && _uiState.value.submittedQuery.isEmpty()) {
                fail("无法读取媒体库内容")
            }
        }
    }

    private fun fail(message: String, libraries: Boolean = false, refreshing: Boolean = false) {
        _uiState.value = _uiState.value.copy(
            loadingLibraries = if (libraries) false else _uiState.value.loadingLibraries,
            loadingItems = false,
            loadingMore = false,
            refreshing = if (refreshing) false else _uiState.value.refreshing,
            errorMessage = message,
        )
    }
}

internal fun libraryPageCount(total: Int): Int = maxOf(1, (total.coerceAtLeast(0) + LibraryPageSize - 1) / LibraryPageSize)

internal fun nextLibraryPage(current: Int, delta: Int, total: Int): Int? {
    val next = current + delta
    return next.takeIf { it >= 0 && it < libraryPageCount(total) }
}

internal fun libraryHasMore(itemCount: Int, total: Int, searching: Boolean): Boolean =
    !searching && itemCount < total.coerceAtLeast(0)

internal const val AdultLibraryId = "adult"

internal fun rootLibraries(libraries: List<MediaLibrary>) =
    libraries.filter { it.parentId.isNullOrBlank() }

internal fun childLibraries(libraries: List<MediaLibrary>, parentId: String) =
    libraries.filter { it.parentId == parentId }

internal fun libraryRootId(libraries: List<MediaLibrary>, selectedId: String?): String? {
    val selected = libraries.firstOrNull { it.id == selectedId }
    return if (selectedId == AdultLibraryId || selected?.parentId == AdultLibraryId) {
        AdultLibraryId
    } else {
        selectedId
    }
}
