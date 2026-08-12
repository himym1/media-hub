package com.mediahub.android.feature.library

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.EmbyItem
import com.mediahub.android.core.network.MediaLibrary
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class LibraryUiState(
    val query: String = "",
    val libraries: List<MediaLibrary> = emptyList(),
    val items: List<EmbyItem> = emptyList(),
    val loadingLibraries: Boolean = false,
    val searching: Boolean = false,
    val errorMessage: String? = null,
)

class LibraryViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(LibraryUiState())
    val uiState: StateFlow<LibraryUiState> = _uiState.asStateFlow()

    private var searchJob: Job? = null

    fun refreshLibraries() {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loadingLibraries = true, errorMessage = null)
            try {
                _uiState.value = _uiState.value.copy(
                    libraries = repository.libraries(),
                    loadingLibraries = false,
                )
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(
                    loadingLibraries = false,
                    errorMessage = error.message ?: "媒体库读取失败",
                )
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(
                    loadingLibraries = false,
                    errorMessage = "无法读取 Emby 媒体库",
                )
            }
        }
    }

    fun onQueryChanged(query: String) {
        if (query.length <= 120) _uiState.value = _uiState.value.copy(query = query, errorMessage = null)
    }

    fun search() {
        val query = _uiState.value.query.trim()
        if (query.isEmpty()) return
        searchJob?.cancel()
        searchJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(searching = true, errorMessage = null)
            try {
                _uiState.value = _uiState.value.copy(
                    items = repository.items(query),
                    searching = false,
                )
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(
                    searching = false,
                    errorMessage = error.message ?: "Emby 搜索失败",
                )
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(
                    searching = false,
                    errorMessage = "无法搜索 Emby 媒体库",
                )
            }
        }
    }
}
