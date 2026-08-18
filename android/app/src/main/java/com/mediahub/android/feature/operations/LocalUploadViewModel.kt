package com.mediahub.android.feature.operations

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.LocalUploadEntry
import com.mediahub.android.core.network.LocalUploadJob
import com.mediahub.android.core.network.LocalUploadRoot
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

internal data class LocalUploadState(
    val loading: Boolean = true,
    val roots: List<LocalUploadRoot> = emptyList(),
    val entries: List<LocalUploadEntry> = emptyList(),
    val uploads: List<LocalUploadJob> = emptyList(),
    val rootId: String = "",
    val path: String = "",
    val selectedFile: String = "",
    val destinationId: String = "",
    val invoking: Boolean = false,
    val error: String? = null,
)

class LocalUploadViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(LocalUploadState())
    internal val uiState: StateFlow<LocalUploadState> = _uiState.asStateFlow()
    private var pollingJob: Job? = null
    private var filesJob: Job? = null

    init {
        refresh()
    }

    fun refresh() {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true, error = null)
            try {
                val roots = repository.localUploadRoots()
                val rootId = _uiState.value.rootId.ifBlank { roots.firstOrNull()?.id.orEmpty() }
                val entries = if (rootId.isBlank()) emptyList() else repository.localUploadFiles(rootId, _uiState.value.path)
                val uploads = repository.localUploads()
                _uiState.value = _uiState.value.copy(
                    loading = false,
                    roots = roots,
                    rootId = rootId,
                    entries = entries,
                    uploads = uploads,
                )
                startPollingIfNeeded()
            } catch (error: ApiException) {
                fail(error.message ?: "无法读取本地上传")
            } catch (_: Exception) {
                fail("无法读取本地上传")
            }
        }
    }

    fun setRoot(value: String) {
        _uiState.value = _uiState.value.copy(rootId = value, path = "", selectedFile = "", error = null)
        refreshFiles()
    }

    fun setPath(value: String) {
        _uiState.value = _uiState.value.copy(path = value.take(2048), selectedFile = "", error = null)
        refreshFiles()
    }

    fun selectEntry(entry: LocalUploadEntry) {
        if (entry.directory) setPath(entry.path) else _uiState.value = _uiState.value.copy(selectedFile = entry.path)
    }

    fun setDestination(value: String) {
        _uiState.value = _uiState.value.copy(destinationId = value.filter(Char::isDigit).take(20), error = null)
    }

    fun createUpload() {
        val state = _uiState.value
        if (state.rootId.isBlank() || state.selectedFile.isBlank() || state.destinationId.isBlank()) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(invoking = true, error = null)
            try {
                repository.createLocalUpload(state.rootId, state.selectedFile, state.destinationId)
                _uiState.value = _uiState.value.copy(
                    invoking = false,
                    selectedFile = "",
                    uploads = repository.localUploads(),
                )
                startPollingIfNeeded()
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(invoking = false, error = error.message ?: "无法创建上传任务")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(invoking = false, error = "无法创建上传任务")
            }
        }
    }

    fun retryUpload(upload: LocalUploadJob) {
        viewModelScope.launch {
            try {
                repository.retryLocalUpload(upload)
                _uiState.value = _uiState.value.copy(uploads = repository.localUploads(), error = null)
                startPollingIfNeeded()
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(error = error.message ?: "无法重试上传")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(error = "无法重试上传")
            }
        }
    }

    private fun refreshFiles() {
        filesJob?.cancel()
        val rootId = _uiState.value.rootId
        val path = _uiState.value.path
        if (rootId.isBlank()) {
            _uiState.value = _uiState.value.copy(entries = emptyList())
            return
        }
        filesJob = viewModelScope.launch {
            try {
                val entries = repository.localUploadFiles(rootId, path)
                if (_uiState.value.rootId == rootId && _uiState.value.path == path) {
                    _uiState.value = _uiState.value.copy(entries = entries, error = null)
                }
            } catch (_: Exception) {
                if (_uiState.value.rootId == rootId && _uiState.value.path == path) {
                    _uiState.value = _uiState.value.copy(error = "无法读取本地目录")
                }
            }
        }
    }

    private fun startPollingIfNeeded() {
        if (_uiState.value.uploads.none(::activeUpload) || pollingJob?.isActive == true) return
        pollingJob = viewModelScope.launch {
            while (true) {
                delay(3_000)
                val uploads = try {
                    repository.localUploads()
                } catch (_: Exception) {
                    _uiState.value = _uiState.value.copy(error = "无法刷新上传任务")
                    break
                }
                _uiState.value = _uiState.value.copy(uploads = uploads)
                if (uploads.none(::activeUpload)) break
            }
        }
    }

    private fun activeUpload(upload: LocalUploadJob): Boolean =
        upload.state in setOf("queued", "hashing", "submitting_init", "uploading")

    private fun fail(message: String) {
        _uiState.value = _uiState.value.copy(loading = false, error = message)
    }
}
