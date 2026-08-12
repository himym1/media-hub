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
    val loading: Boolean = false,
    val refreshing: Boolean = false,
    val retrying: Boolean = false,
    val notificationRetrying: Boolean = false,
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
        _uiState.value = _uiState.value.copy(
            loading = initial && _uiState.value.jobs.isEmpty(),
            refreshing = !initial,
        )
        try {
            val jobs = repository.transfers()
            val notifications = repository.transferNotifications()
            val selectedId = _uiState.value.selectedId?.takeIf { id -> jobs.any { it.id == id } }
                ?: jobs.firstOrNull()?.id
            _uiState.value = _uiState.value.copy(
                jobs = jobs,
                notifications = notifications,
                selectedId = selectedId,
                loading = false,
                refreshing = false,
                retrying = false,
                notificationRetrying = false,
                errorMessage = null,
            )
            if (selectedId != null) loadDetail(selectedId)
        } catch (error: ApiException) {
            _uiState.value = _uiState.value.copy(
                loading = false,
                refreshing = false,
                retrying = false,
                notificationRetrying = false,
                errorMessage = error.message ?: "任务读取失败",
            )
        } catch (_: Exception) {
            _uiState.value = _uiState.value.copy(
                loading = false,
                refreshing = false,
                retrying = false,
                notificationRetrying = false,
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
