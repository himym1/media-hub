package com.mediahub.android.feature.library

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.EmbyItem
import com.mediahub.android.core.network.EmbyItemDetail
import com.mediahub.android.core.network.MediaLibrary
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

internal const val LibraryPageSize = 24

data class LibraryUiState(
    val query: String = "",
    val submittedQuery: String = "",
    val libraries: List<MediaLibrary> = emptyList(),
    val selectedLibraryId: String? = null,
    val items: List<EmbyItem> = emptyList(),
    val total: Int = 0,
    val page: Int = 0,
    val selectedItemId: String? = null,
    val selectedItem: EmbyItemDetail? = null,
    val loadingLibraries: Boolean = false,
    val loadingItems: Boolean = false,
    val loadingDetail: Boolean = false,
    val refreshing: Boolean = false,
    val errorMessage: String? = null,
    val actionMessage: String? = null,
)

class LibraryViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(LibraryUiState())
    val uiState: StateFlow<LibraryUiState> = _uiState.asStateFlow()
    private var contentJob: Job? = null

    fun refreshLibraries() {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loadingLibraries = true, errorMessage = null)
            try {
                val libraries = repository.libraries()
                val selected = _uiState.value.selectedLibraryId?.takeIf { id -> libraries.any { it.id == id } }
                    ?: libraries.firstOrNull()?.id
                val browsing = _uiState.value.submittedQuery.isEmpty()
                _uiState.value = _uiState.value.copy(
                    libraries = libraries,
                    selectedLibraryId = selected,
                    items = if (browsing) emptyList() else _uiState.value.items,
                    total = if (browsing) 0 else _uiState.value.total,
                    page = if (browsing) 0 else _uiState.value.page,
                    selectedItemId = if (browsing) null else _uiState.value.selectedItemId,
                    selectedItem = if (browsing) null else _uiState.value.selectedItem,
                    loadingLibraries = false,
                )
                if (_uiState.value.submittedQuery.isEmpty() && selected != null) loadLibraryPage(selected, 0)
            } catch (error: ApiException) {
                fail(error.message ?: "媒体库读取失败", libraries = true)
            } catch (_: Exception) {
                fail("无法读取 Emby 媒体库", libraries = true)
            }
        }
    }

    fun selectLibrary(id: String) {
        if (_uiState.value.libraries.none { it.id == id }) return
        _uiState.value = _uiState.value.copy(
            selectedLibraryId = id,
            query = "",
            submittedQuery = "",
            page = 0,
            selectedItemId = null,
            selectedItem = null,
            actionMessage = null,
        )
        contentJob?.cancel()
        contentJob = viewModelScope.launch { loadLibraryPage(id, 0) }
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
        contentJob?.cancel()
        contentJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(
                submittedQuery = query,
                loadingItems = true,
                selectedItemId = null,
                selectedItem = null,
                errorMessage = null,
            )
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
        _uiState.value = _uiState.value.copy(
            query = "",
            submittedQuery = "",
            selectedItemId = null,
            selectedItem = null,
            page = 0,
            errorMessage = null,
        )
        if (libraryId != null) {
            contentJob?.cancel()
            contentJob = viewModelScope.launch { loadLibraryPage(libraryId, 0) }
        }
    }

    fun changePage(delta: Int) {
        if (_uiState.value.submittedQuery.isNotEmpty()) return
        val libraryId = _uiState.value.selectedLibraryId ?: return
        val page = nextLibraryPage(_uiState.value.page, delta, _uiState.value.total) ?: return
        _uiState.value = _uiState.value.copy(selectedItemId = null, selectedItem = null)
        contentJob?.cancel()
        contentJob = viewModelScope.launch { loadLibraryPage(libraryId, page) }
    }

    fun selectItem(id: String) {
        _uiState.value = _uiState.value.copy(selectedItemId = id, selectedItem = null, loadingDetail = true, errorMessage = null)
        viewModelScope.launch { loadItem(id) }
    }

    fun closeItem() {
        _uiState.value = _uiState.value.copy(selectedItemId = null, selectedItem = null, loadingDetail = false, errorMessage = null)
    }

    fun refreshSelectedLibrary() {
        val id = _uiState.value.selectedLibraryId ?: return
        if (_uiState.value.refreshing) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(refreshing = true, errorMessage = null, actionMessage = null)
            try {
                repository.refreshLibrary(id)
                _uiState.value = _uiState.value.copy(refreshing = false, actionMessage = "已请求 Emby 刷新媒体库")
                if (_uiState.value.submittedQuery.isEmpty()) loadLibraryPage(id, _uiState.value.page)
            } catch (error: ApiException) {
                fail(error.message ?: "媒体库刷新失败", refreshing = true)
            } catch (_: Exception) {
                fail("无法刷新 Emby 媒体库", refreshing = true)
            }
        }
    }

    fun refreshSelectedItem() {
        val id = _uiState.value.selectedItemId ?: return
        if (_uiState.value.refreshing) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(refreshing = true, errorMessage = null, actionMessage = null)
            try {
                repository.refreshItem(id)
                _uiState.value = _uiState.value.copy(refreshing = false, actionMessage = "已请求 Emby 刷新媒体元数据")
                loadItem(id)
            } catch (error: ApiException) {
                fail(error.message ?: "媒体元数据刷新失败", refreshing = true)
            } catch (_: Exception) {
                fail("无法刷新 Emby 媒体元数据", refreshing = true)
            }
        }
    }

    private suspend fun loadLibraryPage(libraryId: String, page: Int) {
        _uiState.value = _uiState.value.copy(loadingItems = true, errorMessage = null)
        try {
            val result = repository.libraryItems(libraryId, page * LibraryPageSize, LibraryPageSize)
            if (_uiState.value.selectedLibraryId == libraryId && _uiState.value.submittedQuery.isEmpty()) {
                _uiState.value = _uiState.value.copy(
                    items = result.items,
                    total = result.total,
                    page = page,
                    loadingItems = false,
                )
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

    private suspend fun loadItem(id: String) {
        try {
            val detail = repository.itemDetails(id)
            if (_uiState.value.selectedItemId == id) {
                _uiState.value = _uiState.value.copy(selectedItem = detail, loadingDetail = false)
            }
        } catch (error: ApiException) {
            if (_uiState.value.selectedItemId == id) fail(error.message ?: "媒体详情读取失败", detail = true)
        } catch (_: Exception) {
            if (_uiState.value.selectedItemId == id) fail("无法读取媒体详情", detail = true)
        }
    }

    private fun fail(message: String, libraries: Boolean = false, detail: Boolean = false, refreshing: Boolean = false) {
        _uiState.value = _uiState.value.copy(
            loadingLibraries = if (libraries) false else _uiState.value.loadingLibraries,
            loadingItems = false,
            loadingDetail = if (detail) false else _uiState.value.loadingDetail,
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
