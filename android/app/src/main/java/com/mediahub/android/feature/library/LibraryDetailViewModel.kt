package com.mediahub.android.feature.library

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.EmbyDeletePreview
import com.mediahub.android.core.network.EmbyItem
import com.mediahub.android.core.network.EmbyEpisode
import com.mediahub.android.core.network.EmbyItemDetail
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class LibraryDetailState(
    val itemId: String? = null,
    val item: EmbyItemDetail? = null,
    val episodes: List<EmbyEpisode> = emptyList(),
    val loading: Boolean = false,
    val loadingEpisodes: Boolean = false,
    val refreshing: Boolean = false,
    val deleting: Boolean = false,
    val deletePreview: EmbyDeletePreview? = null,
    val deleted: Boolean = false,
    val errorMessage: String? = null,
    val episodesError: String? = null,
    val actionMessage: String? = null,
)

class LibraryDetailViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(LibraryDetailState())
    val uiState: StateFlow<LibraryDetailState> = _uiState.asStateFlow()
    private var loadJob: Job? = null
    private var refreshJob: Job? = null
    private var deleteJob: Job? = null
    private var generation = 0L

    fun load(itemId: String) {
        if (_uiState.value.itemId == itemId && _uiState.value.item != null) return
        loadJob?.cancel()
        refreshJob?.cancel()
        deleteJob?.cancel()
        val requestGeneration = ++generation
        loadJob = viewModelScope.launch {
            _uiState.value = LibraryDetailState(itemId = itemId, loading = true)
            try {
                val detail = repository.itemDetails(itemId)
                if (_uiState.value.itemId != itemId || generation != requestGeneration) return@launch
                _uiState.value = _uiState.value.copy(
                    item = detail,
                    loading = false,
                    loadingEpisodes = detail.item.type == "Series",
                )
                if (detail.item.type == "Series") loadEpisodes(itemId, requestGeneration)
            } catch (error: ApiException) {
                fail(itemId, requestGeneration, error.message ?: "媒体详情读取失败")
            } catch (_: Exception) {
                fail(itemId, requestGeneration, "无法读取媒体详情")
            }
        }
    }

    fun refresh() {
        val itemId = _uiState.value.itemId ?: return
        if (_uiState.value.refreshing) return
        refreshJob?.cancel()
        val requestGeneration = generation
        refreshJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(refreshing = true, errorMessage = null, actionMessage = null)
            try {
                repository.refreshItem(itemId)
                if (_uiState.value.itemId != itemId || generation != requestGeneration) return@launch
                _uiState.value = _uiState.value.copy(
                    refreshing = false,
                    actionMessage = "已请求 Emby 刷新媒体元数据",
                    item = null,
                )
                refreshJob = null
                load(itemId)
            } catch (error: ApiException) {
                if (_uiState.value.itemId == itemId && generation == requestGeneration) {
                    _uiState.value = _uiState.value.copy(
                        refreshing = false,
                        errorMessage = error.message ?: "媒体元数据刷新失败",
                    )
                }
            } catch (_: Exception) {
                if (_uiState.value.itemId == itemId && generation == requestGeneration) {
                    _uiState.value = _uiState.value.copy(refreshing = false, errorMessage = "无法刷新 Emby 媒体元数据")
                }
            }
        }
    }

    fun requestDelete() {
        val itemId = _uiState.value.itemId ?: return
        if (_uiState.value.deleting || _uiState.value.deletePreview != null) return
        deleteJob?.cancel()
        val requestGeneration = generation
        deleteJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(deleting = true, errorMessage = null, actionMessage = null)
            try {
                val preview = repository.previewItemDelete(itemId)
                if (_uiState.value.itemId == itemId && generation == requestGeneration) {
                    _uiState.value = _uiState.value.copy(deleting = false, deletePreview = preview)
                }
            } catch (error: ApiException) {
                if (_uiState.value.itemId == itemId && generation == requestGeneration) {
                    _uiState.value = _uiState.value.copy(deleting = false, errorMessage = error.message ?: "无法预览删除")
                }
            } catch (_: Exception) {
                if (_uiState.value.itemId == itemId && generation == requestGeneration) {
                    _uiState.value = _uiState.value.copy(deleting = false, errorMessage = "无法预览 Emby 删除")
                }
            }
        }
    }

    fun confirmDelete() {
        val itemId = _uiState.value.itemId ?: return
        val preview = _uiState.value.deletePreview ?: return
        if (!preview.id.equals(itemId, ignoreCase = true) || _uiState.value.deleting) return
        deleteJob?.cancel()
        val requestGeneration = generation
        deleteJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(deleting = true, errorMessage = null)
            try {
                repository.deleteItem(itemId)
                if (_uiState.value.itemId == itemId && generation == requestGeneration) {
                    _uiState.value = _uiState.value.copy(deleting = false, deleted = true, deletePreview = null)
                }
            } catch (error: ApiException) {
                if (_uiState.value.itemId == itemId && generation == requestGeneration) {
                    _uiState.value = _uiState.value.copy(deleting = false, errorMessage = error.message ?: "删除失败")
                }
            } catch (_: Exception) {
                if (_uiState.value.itemId == itemId && generation == requestGeneration) {
                    _uiState.value = _uiState.value.copy(deleting = false, errorMessage = "无法从 Emby 删除")
                }
            }
        }
    }

    fun cancelDelete() {
        deleteJob?.cancel()
        _uiState.value = _uiState.value.copy(deleting = false, deletePreview = null)
    }

    fun refreshActionMessage(message: String) {
        _uiState.value = _uiState.value.copy(actionMessage = message, errorMessage = null)
    }

    fun clear() {
        generation++
        loadJob?.cancel()
        refreshJob?.cancel()
        deleteJob?.cancel()
        _uiState.value = LibraryDetailState()
    }

    private suspend fun loadEpisodes(seriesId: String, requestGeneration: Long) {
        try {
            val episodes = repository.episodes(seriesId)
            if (_uiState.value.itemId == seriesId && generation == requestGeneration) {
                _uiState.value = _uiState.value.copy(episodes = episodes, loadingEpisodes = false, episodesError = null)
            }
        } catch (error: ApiException) {
            if (_uiState.value.itemId == seriesId && generation == requestGeneration) {
                _uiState.value = _uiState.value.copy(
                    loadingEpisodes = false,
                    episodesError = error.message ?: "无法读取剧集列表",
                )
            }
        } catch (_: Exception) {
            if (_uiState.value.itemId == seriesId && generation == requestGeneration) {
                _uiState.value = _uiState.value.copy(loadingEpisodes = false, episodesError = "无法读取剧集列表")
            }
        }
    }

    private fun fail(itemId: String, requestGeneration: Long, message: String) {
        if (_uiState.value.itemId == itemId && generation == requestGeneration) {
            _uiState.value = _uiState.value.copy(loading = false, loadingEpisodes = false, errorMessage = message)
        }
    }
}
