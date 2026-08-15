package com.mediahub.android.feature.transfers

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.TransferJob
import com.mediahub.android.core.network.TransferNotification
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class TransferUiState(
    val jobs: List<TransferJob> = emptyList(),
    val selectedId: String? = null,
    val selected: TransferJob? = null,
    val notifications: List<TransferNotification> = emptyList(),
    val archived: Boolean = false,
    val loading: Boolean = false,
    val refreshing: Boolean = false,
    val retrying: Boolean = false,
    val notificationRetrying: Boolean = false,
    val archiving: Boolean = false,
    val errorMessage: String? = null,
)

class TransferViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(TransferUiState())
    val uiState: StateFlow<TransferUiState> = _uiState.asStateFlow()

    private var pollingJob: Job? = null

    fun startPolling() {
        if (pollingJob != null) return
        pollingJob = viewModelScope.launch {
            var first = true
            while (true) {
                load(first)
                first = false
                delay(5_000)
            }
        }
    }

    fun stopPolling() {
        pollingJob?.cancel()
        pollingJob = null
    }

    fun refresh() {
        viewModelScope.launch { load(false) }
    }

    fun select(id: String) {
        _uiState.value = _uiState.value.copy(selectedId = id, selected = null)
        viewModelScope.launch { loadDetail(id) }
    }

    fun showArchived(archived: Boolean) {
        if (_uiState.value.archived == archived) return
        _uiState.value = _uiState.value.copy(
            archived = archived, jobs = emptyList(), selectedId = null, selected = null, loading = true, errorMessage = null,
        )
        viewModelScope.launch { load(true) }
    }

    fun setSelectedArchived() {
        val selected = _uiState.value.selected ?: return
        val allowed = canArchiveTransfer(selected)
        if (!allowed || _uiState.value.archiving) return
        val archived = !_uiState.value.archived
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(archiving = true, errorMessage = null)
            try {
                repository.setTransferArchived(selected.id, archived)
                load(false)
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(archiving = false, errorMessage = error.message ?: "任务归档失败")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(archiving = false, errorMessage = "无法更新任务归档状态")
            }
        }
    }


    fun retry() {
        val id = _uiState.value.selectedId ?: return
        if (_uiState.value.retrying) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(retrying = true, errorMessage = null)
            try {
                repository.retryTransfer(id)
                load(false)
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(
                    retrying = false,
                    errorMessage = error.message ?: "任务重试失败",
                )
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(
                    retrying = false,
                    errorMessage = "无法重试任务",
                )
            }
        }
    }

    fun retryNotification(notification: TransferNotification) {
        if (_uiState.value.notificationRetrying || notification.state != "needs_attention") return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(notificationRetrying = true, errorMessage = null)
            try {
                repository.retryTransferNotification(notification)
                load(false)
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(
                    notificationRetrying = false,
                    errorMessage = error.message ?: "通知重发失败",
                )
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(notificationRetrying = false, errorMessage = "无法重发通知")
            }
        }
    }

    private suspend fun load(initial: Boolean) {
        val archived = _uiState.value.archived
        _uiState.value = _uiState.value.copy(
            loading = initial && _uiState.value.jobs.isEmpty(),
            refreshing = !initial,
        )
        try {
            val jobs = repository.transfers(archived)
            val notifications = if (archived) emptyList() else repository.transferNotifications()
            if (_uiState.value.archived != archived) return
            val selectedId = _uiState.value.selectedId?.takeIf { id -> jobs.any { it.id == id } }
                ?: jobs.firstOrNull()?.id
            _uiState.value = _uiState.value.copy(
                jobs = jobs,
                notifications = notifications,
                selectedId = selectedId,
                selected = _uiState.value.selected.takeIf { it?.id == selectedId },
                loading = false,
                refreshing = false,
                retrying = false,
                notificationRetrying = false,
                archiving = false,
                errorMessage = null,
            )
            if (selectedId != null) loadDetail(selectedId)
        } catch (error: ApiException) {
            if (_uiState.value.archived != archived) return
            _uiState.value = _uiState.value.copy(
                loading = false,
                refreshing = false,
                retrying = false,
                notificationRetrying = false,
                archiving = false,
                errorMessage = error.message ?: "任务读取失败",
            )
        } catch (_: Exception) {
            if (_uiState.value.archived != archived) return
            _uiState.value = _uiState.value.copy(
                loading = false,
                refreshing = false,
                retrying = false,
                notificationRetrying = false,
                archiving = false,
                errorMessage = "无法读取转存任务",
            )
        }
    }

    private suspend fun loadDetail(id: String) {
        try {
            val detail = repository.transfer(id)
            if (_uiState.value.selectedId == id) {
                _uiState.value = _uiState.value.copy(selected = detail)
            }
        } catch (_: Exception) {
            // The list remains usable while a detail refresh is temporarily unavailable.
        }
    }
}

internal fun canArchiveTransfer(job: TransferJob): Boolean =
    job.state == "completed" || (job.state == "failed" && !job.retryable)
