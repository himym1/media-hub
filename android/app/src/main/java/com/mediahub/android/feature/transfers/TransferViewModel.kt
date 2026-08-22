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
    val deleting: Boolean = false,
    val errorMessage: String? = null,
    val initialized: Boolean = false,
    val archivedCompletedId: String? = null,
    val deletedCompletedId: String? = null,
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
        viewModelScope.launch { load(initial = false, userRefresh = true) }
    }

    fun select(id: String) {
        if (_uiState.value.selectedId == id && _uiState.value.selected != null) return
        _uiState.value = _uiState.value.copy(selectedId = id, selected = null)
        viewModelScope.launch { loadDetail(id) }
    }

    fun closeDetail() {
        _uiState.value = _uiState.value.copy(selectedId = null, selected = null, errorMessage = null)
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
        if (!allowed || _uiState.value.archiving || _uiState.value.deleting) return
        val archived = !_uiState.value.archived
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(archiving = true, errorMessage = null, archivedCompletedId = null)
            try {
                repository.setTransferArchived(selected.id, archived)
                load(false)
                _uiState.value = _uiState.value.copy(archivedCompletedId = selected.id)
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(archiving = false, errorMessage = error.message ?: "任务归档失败")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(archiving = false, errorMessage = "无法更新任务归档状态")
            }
        }
    }

    fun deleteSelected() {
        val selected = _uiState.value.selected ?: return
        if (!canDeleteTransfer(selected) || _uiState.value.deleting || _uiState.value.archiving) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(deleting = true, errorMessage = null, deletedCompletedId = null)
            try {
                repository.deleteTransfer(selected.id)
                load(false)
                _uiState.value = _uiState.value.copy(deletedCompletedId = selected.id)
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(deleting = false, errorMessage = error.message ?: "任务删除失败")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(deleting = false, errorMessage = "无法删除任务")
            }
        }
    }

    fun consumeArchivedCompletion(id: String) {
        if (_uiState.value.archivedCompletedId == id) {
            _uiState.value = _uiState.value.copy(archivedCompletedId = null)
        }
    }

    fun consumeDeletedCompletion(id: String) {
        if (_uiState.value.deletedCompletedId == id) {
            _uiState.value = _uiState.value.copy(deletedCompletedId = null)
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

    private suspend fun load(initial: Boolean, userRefresh: Boolean = false) {
        val archived = _uiState.value.archived
        _uiState.value = _uiState.value.copy(
            loading = initial && _uiState.value.jobs.isEmpty(),
            refreshing = userRefresh,
        )
        try {
            val jobs = repository.transfers(archived)
            val notifications = if (archived) emptyList() else repository.transferNotifications()
            if (_uiState.value.archived != archived) return
            val selectedId = _uiState.value.selectedId?.takeIf { id -> jobs.any { it.id == id } }
            val previousJobs = _uiState.value.jobs
            val previousNotifications = _uiState.value.notifications
            val jobsUnchanged = previousJobs.map { it.id to it.state } == jobs.map { it.id to it.state }
            val notificationsUnchanged = previousNotifications.map { it.id to it.state } ==
                notifications.map { it.id to it.state }
            _uiState.value = _uiState.value.copy(
                jobs = if (jobsUnchanged) previousJobs else jobs,
                notifications = if (notificationsUnchanged) previousNotifications else notifications,
                selectedId = selectedId,
                selected = _uiState.value.selected.takeIf { it?.id == selectedId },
                loading = false,
                refreshing = false,
                retrying = false,
                notificationRetrying = false,
                archiving = false,
                deleting = false,
                errorMessage = null,
                initialized = true,
            )
            if (selectedId != null && (_uiState.value.selected == null || !jobsUnchanged)) {
                loadDetail(selectedId)
            }
        } catch (error: ApiException) {
            if (_uiState.value.archived != archived) return
            _uiState.value = _uiState.value.copy(
                loading = false,
                refreshing = false,
                retrying = false,
                notificationRetrying = false,
                archiving = false,
                deleting = false,
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
                deleting = false,
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

internal fun canArchiveTransfer(job: TransferJob): Boolean = job.state == "completed"

internal fun canDeleteTransfer(job: TransferJob): Boolean =
    job.state == "failed" || job.state == "needs_attention"
