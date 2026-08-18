package com.mediahub.android.feature.operations

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.ArchivePlan
import com.mediahub.android.core.network.ArchiveSuggestion
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

internal data class ArchiveState(
    val loading: Boolean = true,
    val suggestions: List<ArchiveSuggestion> = emptyList(),
    val plans: List<ArchivePlan> = emptyList(),
    val parentId: String = "0",
    val targetId: String = "",
    val selected: Set<String> = emptySet(),
    val names: Map<String, String> = emptyMap(),
    val error: String? = null,
)

class ArchiveViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(ArchiveState())
    internal val uiState: StateFlow<ArchiveState> = _uiState.asStateFlow()
    private var pollingJob: Job? = null

    init {
        refresh()
    }

    fun refresh() {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true, error = null)
            try {
                val plans = repository.archivePlans()
                _uiState.value = _uiState.value.copy(loading = false, plans = plans)
                startPollingIfNeeded()
            } catch (error: ApiException) {
                fail(error.message ?: "无法读取归档计划")
            } catch (_: Exception) {
                fail("无法读取归档计划")
            }
        }
    }

    fun setParentId(value: String) {
        _uiState.value = _uiState.value.copy(parentId = value.filter(Char::isDigit).take(20), error = null)
    }

    fun setTargetId(value: String) {
        _uiState.value = _uiState.value.copy(targetId = value.filter(Char::isDigit).take(20), error = null)
    }

    fun preview() {
        viewModelScope.launch {
            try {
                val values = repository.previewArchive(_uiState.value.parentId)
                _uiState.value = _uiState.value.copy(
                    suggestions = values,
                    names = values.associate { it.fileId to it.suggestedName },
                    selected = emptySet(),
                    error = null,
                )
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(error = error.message ?: "无法预览归档")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(error = "无法预览归档")
            }
        }
    }

    fun toggle(fileId: String) {
        val selected = _uiState.value.selected.toMutableSet()
        if (!selected.add(fileId)) selected.remove(fileId)
        _uiState.value = _uiState.value.copy(selected = selected)
    }

    fun setName(fileId: String, value: String) {
        _uiState.value = _uiState.value.copy(names = _uiState.value.names + (fileId to value.take(255)))
    }

    fun createPlan() {
        val state = _uiState.value
        val steps = buildArchiveSteps(
            suggestions = state.suggestions,
            selectedFileIds = state.selected,
            names = state.names,
            targetParentId = state.targetId,
        )
        if (steps.isEmpty()) {
            _uiState.value = state.copy(error = "至少选择一个更名或移动步骤")
            return
        }
        viewModelScope.launch {
            try {
                repository.createArchivePlan(steps)
                _uiState.value = _uiState.value.copy(plans = repository.archivePlans(), error = null)
                startPollingIfNeeded()
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(error = error.message ?: "无法创建归档计划")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(error = "无法创建归档计划")
            }
        }
    }

    fun confirm(plan: ArchivePlan) = changePlan(plan, retry = false)

    fun retry(plan: ArchivePlan) = changePlan(plan, retry = true)

    private fun changePlan(plan: ArchivePlan, retry: Boolean) {
        viewModelScope.launch {
            try {
                if (retry) repository.retryArchivePlan(plan) else repository.confirmArchivePlan(plan)
                _uiState.value = _uiState.value.copy(plans = repository.archivePlans(), error = null)
                startPollingIfNeeded()
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(error = error.message ?: "无法更新归档计划")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(error = "无法更新归档计划")
            }
        }
    }

    private fun startPollingIfNeeded() {
        if (_uiState.value.plans.none(::activePlan) || pollingJob?.isActive == true) return
        pollingJob = viewModelScope.launch {
            while (true) {
                delay(3_000)
                val plans = try {
                    repository.archivePlans()
                } catch (_: Exception) {
                    _uiState.value = _uiState.value.copy(error = "无法刷新归档计划")
                    break
                }
                _uiState.value = _uiState.value.copy(plans = plans)
                if (plans.none(::activePlan)) break
            }
        }
    }

    private fun activePlan(plan: ArchivePlan): Boolean = plan.state == "queued" || plan.state == "running"

    private fun fail(message: String) {
        _uiState.value = _uiState.value.copy(loading = false, error = message)
    }
}
