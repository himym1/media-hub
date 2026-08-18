package com.mediahub.android.feature.operations

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.Drive115Command
import com.mediahub.android.core.network.Drive115File
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

internal data class Drive115State(
    val loading: Boolean = true,
    val files: List<Drive115File> = emptyList(),
    val commands: List<Drive115Command> = emptyList(),
    val parentId: String = "0",
    val operation: String = "create_folder",
    val fileIds: String = "",
    val targetParentId: String = "0",
    val name: String = "",
    val invoking: Boolean = false,
    val error: String? = null,
    val initialized: Boolean = false,
)

class Drive115ViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(Drive115State())
    internal val uiState: StateFlow<Drive115State> = _uiState.asStateFlow()
    private var pollingJob: Job? = null
    private var refreshJob: Job? = null


    fun refresh() {
        refreshJob?.cancel()
        refreshJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true, error = null)
            try {
                val state = _uiState.value
                val files = repository.drive115Files(state.parentId)
                val commands = repository.drive115Commands()
                _uiState.value = _uiState.value.copy(
                    loading = false, initialized = true, files = files, commands = commands,
                )
                startPollingIfNeeded()
            } catch (error: ApiException) {
                fail(error.message ?: "无法读取 115 文件")
            } catch (_: Exception) {
                fail("无法读取 115 文件")
            }
        }
    }

    fun setParentId(value: String) {
        val clean = value.filter(Char::isDigit).take(20).ifEmpty { "0" }
        _uiState.value = _uiState.value.copy(parentId = clean, loading = true, error = null)
        refreshFiles(clean)
    }

    fun setOperation(value: String) {
        _uiState.value = _uiState.value.copy(operation = value, error = null)
    }

    fun setFileIds(value: String) {
        _uiState.value = _uiState.value.copy(fileIds = value.take(1024), error = null)
    }

    fun setTargetParentId(value: String) {
        _uiState.value = _uiState.value.copy(targetParentId = value.filter(Char::isDigit).take(20), error = null)
    }

    fun setName(value: String) {
        _uiState.value = _uiState.value.copy(name = value.take(255), error = null)
    }

    fun createCommand() {
        val state = _uiState.value
        val ids = state.fileIds.split(',').map(String::trim).filter(String::isNotEmpty)
        val params: Map<String, Any> = when (state.operation) {
            "create_folder" -> mapOf("parentId" to state.targetParentId, "name" to state.name.trim())
            "move" -> mapOf("fileIds" to ids, "targetParentId" to state.targetParentId)
            "rename" -> mapOf("fileId" to ids.firstOrNull().orEmpty(), "name" to state.name.trim())
            else -> mapOf("fileIds" to ids)
        }
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(invoking = true, error = null)
            try {
                repository.createDrive115Command(state.operation, params)
                val commands = repository.drive115Commands()
                _uiState.value = _uiState.value.copy(
                    invoking = false,
                    commands = commands,
                    fileIds = "",
                    name = "",
                )
                startPollingIfNeeded()
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(invoking = false, error = error.message ?: "115 命令创建失败")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(invoking = false, error = "115 命令创建失败")
            }
        }
    }

    fun confirmCommand(command: Drive115Command) {
        viewModelScope.launch {
            try {
                if (command.state == "awaiting_confirmation") {
                    repository.confirmDrive115Command(command)
                } else {
                    repository.retryDrive115Command(command)
                }
                _uiState.value = _uiState.value.copy(commands = repository.drive115Commands(), error = null)
                startPollingIfNeeded()
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(error = error.message ?: "无法更新 115 命令")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(error = "无法更新 115 命令")
            }
        }
    }

    private fun refreshFiles(parentId: String) {
        refreshJob?.cancel()
        refreshJob = viewModelScope.launch {
            try {
                val files = repository.drive115Files(parentId)
                if (_uiState.value.parentId == parentId) {
                    _uiState.value = _uiState.value.copy(files = files, loading = false, error = null)
                }
            } catch (_: Exception) {
                if (_uiState.value.parentId == parentId) fail("无法读取 115 目录")
            }
        }
    }

    private fun startPollingIfNeeded() {
        if (_uiState.value.commands.none(::activeCommand) || pollingJob?.isActive == true) return
        pollingJob = viewModelScope.launch {
            while (true) {
                delay(3_000)
                val commands = try {
                    repository.drive115Commands()
                } catch (_: Exception) {
                    _uiState.value = _uiState.value.copy(error = "无法刷新 115 命令")
                    break
                }
                _uiState.value = _uiState.value.copy(commands = commands)
                if (commands.none(::activeCommand)) break
            }
        }
    }

    private fun activeCommand(command: Drive115Command): Boolean =
        command.state == "queued" || command.state == "submitting"

    private fun fail(message: String) {
        _uiState.value = _uiState.value.copy(loading = false, initialized = true, error = message)
    }
}
