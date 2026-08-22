package com.mediahub.android.feature.subtitles

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.EmbyRemoteSubtitle
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class RemoteSubtitleUiState(
    val targetId: String? = null,
    val targetLabel: String? = null,
    val remoteSubtitles: List<EmbyRemoteSubtitle> = emptyList(),
    val searching: Boolean = false,
    val downloadingId: String? = null,
    val message: String? = null,
)

class RemoteSubtitleViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(RemoteSubtitleUiState())
    val uiState: StateFlow<RemoteSubtitleUiState> = _uiState.asStateFlow()

    fun search(
        itemId: String,
        label: String,
        allowSearch: Boolean = true,
        blockedMessage: String? = null,
    ) {
        if (!allowSearch) {
            _uiState.value = _uiState.value.copy(message = blockedMessage)
            return
        }
        if (_uiState.value.searching) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(
                searching = true,
                targetId = itemId,
                targetLabel = label,
                remoteSubtitles = emptyList(),
                message = null,
            )
            try {
                val items = repository.searchRemoteSubtitles(itemId, "chi")
                if (_uiState.value.targetId == itemId) {
                    _uiState.value = _uiState.value.copy(
                        searching = false,
                        remoteSubtitles = items,
                        message = if (items.isEmpty()) "未找到可用中文字幕（需 Emby 已配置字幕插件）" else null,
                    )
                }
            } catch (error: ApiException) {
                if (_uiState.value.targetId == itemId) {
                    _uiState.value = _uiState.value.copy(
                        searching = false,
                        message = error.message ?: "字幕搜索失败",
                    )
                }
            } catch (_: Exception) {
                if (_uiState.value.targetId == itemId) {
                    _uiState.value = _uiState.value.copy(
                        searching = false,
                        message = "无法搜索 Emby 字幕",
                    )
                }
            }
        }
    }

    fun download(subtitleId: String, onSuccess: (() -> Unit)? = null) {
        val itemId = _uiState.value.targetId ?: return
        if (_uiState.value.downloadingId != null) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(downloadingId = subtitleId, message = null)
            try {
                repository.downloadRemoteSubtitle(itemId, subtitleId)
                if (_uiState.value.targetId == itemId) {
                    _uiState.value = _uiState.value.copy(
                        downloadingId = null,
                        message = "下载成功，重新打开音轨/字幕可选中文字幕",
                    )
                    onSuccess?.invoke()
                }
            } catch (error: ApiException) {
                if (_uiState.value.targetId == itemId) {
                    _uiState.value = _uiState.value.copy(
                        downloadingId = null,
                        message = error.message ?: "字幕下载失败",
                    )
                }
            } catch (_: Exception) {
                if (_uiState.value.targetId == itemId) {
                    _uiState.value = _uiState.value.copy(
                        downloadingId = null,
                        message = "无法下载 Emby 字幕",
                    )
                }
            }
        }
    }

    fun clear() {
        _uiState.value = RemoteSubtitleUiState()
    }
}
