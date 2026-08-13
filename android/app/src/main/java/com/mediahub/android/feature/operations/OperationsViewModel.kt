package com.mediahub.android.feature.operations

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.ArchivePlan
import com.mediahub.android.core.network.ArchiveStep
import com.mediahub.android.core.network.Drive115Command
import com.mediahub.android.core.network.Drive115File
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

data class OperationsUiState(
    val loading: Boolean = true,
    val driveFiles: List<Drive115File> = emptyList(),
    val driveCommands: List<Drive115Command> = emptyList(),
    val driveParentId: String = "0",
    val driveOperation: String = "create_folder",
    val driveFileIds: String = "",
    val driveTargetParentId: String = "0",
    val driveName: String = "",
    val driveInvoking: Boolean = false,
    val localRoots: List<LocalUploadRoot> = emptyList(),
    val localEntries: List<LocalUploadEntry> = emptyList(),
    val localUploads: List<LocalUploadJob> = emptyList(),
    val localRootId: String = "",
    val localPath: String = "",
    val localSelectedFile: String = "",
    val localDestinationId: String = "",
    val localInvoking: Boolean = false,
    val archiveSuggestions: List<com.mediahub.android.core.network.ArchiveSuggestion> = emptyList(),
    val archivePlans: List<ArchivePlan> = emptyList(),
    val archiveParentId: String = "0",
    val archiveTargetId: String = "",
    val archiveSelected: Set<String> = emptySet(),
    val archiveNames: Map<String, String> = emptyMap(),
    val error: String? = null,
)

class OperationsViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(OperationsUiState())
    val uiState: StateFlow<OperationsUiState> = _uiState.asStateFlow()
    private var pollingJob: Job? = null

    init {
        refresh()
    }

    fun refresh() {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true, error = null)
            try {
                val driveFiles = runCatching { repository.drive115Files(_uiState.value.driveParentId) }.getOrDefault(emptyList())
                val driveCommands = runCatching { repository.drive115Commands() }.getOrDefault(emptyList())
                val localRoots = runCatching { repository.localUploadRoots() }.getOrDefault(emptyList())
                val localRootId = _uiState.value.localRootId.ifBlank { localRoots.firstOrNull()?.id.orEmpty() }
                val localEntries = if (localRootId.isNotBlank()) runCatching { repository.localUploadFiles(localRootId, _uiState.value.localPath) }.getOrDefault(emptyList()) else emptyList()
                val localUploads = runCatching { repository.localUploads() }.getOrDefault(emptyList())
                val archivePlans = runCatching { repository.archivePlans() }.getOrDefault(emptyList())
                _uiState.value = _uiState.value.copy(
                    loading = false,
                    driveFiles = driveFiles,
                    driveCommands = driveCommands,
                    localRoots = localRoots, localRootId = localRootId, localEntries = localEntries, localUploads = localUploads, archivePlans = archivePlans,
                )
                startPollingIfNeeded()
            } catch (error: ApiException) {
                fail(error.message ?: "无法读取运营状态")
            } catch (_: Exception) {
                fail("无法读取运营状态")
            }
        }
    }


    fun setDriveParentId(value: String) {
        val clean = value.filter(Char::isDigit).take(20).ifEmpty { "0" }
        _uiState.value = _uiState.value.copy(driveParentId = clean)
        viewModelScope.launch {
            val files = runCatching { repository.drive115Files(clean) }.getOrElse { _uiState.value = _uiState.value.copy(error = "无法读取 115 目录"); return@launch }
            _uiState.value = _uiState.value.copy(driveFiles = files, error = null)
        }
    }
    fun setDriveOperation(value: String) { _uiState.value = _uiState.value.copy(driveOperation = value, error = null) }
    fun setDriveFileIds(value: String) { _uiState.value = _uiState.value.copy(driveFileIds = value.take(1024), error = null) }
    fun setDriveTargetParentId(value: String) { _uiState.value = _uiState.value.copy(driveTargetParentId = value.filter(Char::isDigit).take(20), error = null) }
    fun setDriveName(value: String) { _uiState.value = _uiState.value.copy(driveName = value.take(255), error = null) }

    fun createDriveCommand() {
        val state = _uiState.value
        val ids = state.driveFileIds.split(',').map(String::trim).filter(String::isNotEmpty)
        val params: Map<String, Any> = when (state.driveOperation) {
            "create_folder" -> mapOf("parentId" to state.driveTargetParentId, "name" to state.driveName.trim())
            "move" -> mapOf("fileIds" to ids, "targetParentId" to state.driveTargetParentId)
            "rename" -> mapOf("fileId" to ids.firstOrNull().orEmpty(), "name" to state.driveName.trim())
            else -> mapOf("fileIds" to ids)
        }
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(driveInvoking = true, error = null)
            try {
                repository.createDrive115Command(state.driveOperation, params)
                val commands = repository.drive115Commands()
                _uiState.value = _uiState.value.copy(driveInvoking = false, driveCommands = commands, driveFileIds = "", driveName = "")
                startPollingIfNeeded()
            } catch (error: ApiException) { _uiState.value = _uiState.value.copy(driveInvoking = false, error = error.message ?: "115 命令创建失败") }
        }
    }

    fun confirmDriveCommand(command: Drive115Command) {
        viewModelScope.launch {
            try {
                if (command.state == "awaiting_confirmation") repository.confirmDrive115Command(command) else repository.retryDrive115Command(command)
                _uiState.value = _uiState.value.copy(driveCommands = repository.drive115Commands(), error = null)
                startPollingIfNeeded()
            } catch (error: ApiException) { _uiState.value = _uiState.value.copy(error = error.message ?: "无法确认删除命令") }
        }
    }

    fun setLocalRoot(value: String) { _uiState.value = _uiState.value.copy(localRootId = value, localPath = "", localSelectedFile = ""); refreshLocalFiles() }
    fun setLocalPath(value: String) { _uiState.value = _uiState.value.copy(localPath = value.take(2048), localSelectedFile = ""); refreshLocalFiles() }
    fun selectLocalEntry(entry: LocalUploadEntry) { if (entry.directory) setLocalPath(entry.path) else _uiState.value = _uiState.value.copy(localSelectedFile = entry.path) }
    fun setLocalDestination(value: String) { _uiState.value = _uiState.value.copy(localDestinationId = value.filter(Char::isDigit).take(20)) }
    fun createLocalUpload() {
        val state = _uiState.value
        if (state.localRootId.isBlank() || state.localSelectedFile.isBlank() || state.localDestinationId.isBlank()) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(localInvoking = true, error = null)
            try {
                repository.createLocalUpload(state.localRootId, state.localSelectedFile, state.localDestinationId)
                _uiState.value = _uiState.value.copy(localInvoking = false, localSelectedFile = "", localUploads = repository.localUploads())
                startPollingIfNeeded()
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(localInvoking = false, error = error.message ?: "无法创建上传任务")
            }
        }
    }
    fun retryLocalUpload(upload: LocalUploadJob) {
        viewModelScope.launch {
            try {
                repository.retryLocalUpload(upload)
                _uiState.value = _uiState.value.copy(localUploads = repository.localUploads(), error = null)
                startPollingIfNeeded()
            } catch (error: ApiException) { _uiState.value = _uiState.value.copy(error = error.message ?: "无法重试上传") }
        }
    }
    private fun refreshLocalFiles() {
        val state = _uiState.value
        if (state.localRootId.isBlank()) return
        viewModelScope.launch {
            val values = runCatching { repository.localUploadFiles(state.localRootId, state.localPath) }.getOrElse {
                _uiState.value = _uiState.value.copy(error = "无法读取本地目录")
                return@launch
            }
            _uiState.value = _uiState.value.copy(localEntries = values, error = null)
        }
    }

    fun setArchiveParentId(value: String) { _uiState.value = _uiState.value.copy(archiveParentId = value.filter(Char::isDigit).take(20)) }
    fun setArchiveTargetId(value: String) { _uiState.value = _uiState.value.copy(archiveTargetId = value.filter(Char::isDigit).take(20)) }
    fun previewArchive() {
        viewModelScope.launch {
            try {
                val values = repository.previewArchive(_uiState.value.archiveParentId)
                _uiState.value = _uiState.value.copy(
                    archiveSuggestions = values, archiveNames = values.associate { it.fileId to it.suggestedName }, archiveSelected = emptySet(), error = null,
                )
            } catch (error: ApiException) { _uiState.value = _uiState.value.copy(error = error.message ?: "无法预览归档") }
        }
    }
    fun toggleArchive(fileId: String) {
        val current = _uiState.value.archiveSelected.toMutableSet()
        if (!current.add(fileId)) current.remove(fileId)
        _uiState.value = _uiState.value.copy(archiveSelected = current)
    }
    fun setArchiveName(fileId: String, value: String) { _uiState.value = _uiState.value.copy(archiveNames = _uiState.value.archiveNames + (fileId to value.take(255))) }
    fun createArchivePlan() {
        val state = _uiState.value
        val steps = buildArchiveSteps(
            suggestions = state.archiveSuggestions,
            selectedFileIds = state.archiveSelected,
            names = state.archiveNames,
            targetParentId = state.archiveTargetId,
        )
        if (steps.isEmpty()) { _uiState.value = state.copy(error = "至少选择一个更名或移动步骤"); return }
        viewModelScope.launch {
            try {
                repository.createArchivePlan(steps)
                _uiState.value = _uiState.value.copy(archivePlans = repository.archivePlans(), error = null)
                startPollingIfNeeded()
            } catch (error: ApiException) { _uiState.value = _uiState.value.copy(error = error.message ?: "无法创建归档计划") }
        }
    }
    fun confirmArchive(plan: ArchivePlan) = changeArchivePlan(plan, retry = false)
    fun retryArchive(plan: ArchivePlan) = changeArchivePlan(plan, retry = true)
    private fun changeArchivePlan(plan: ArchivePlan, retry: Boolean) {
        viewModelScope.launch {
            try {
                if (retry) repository.retryArchivePlan(plan) else repository.confirmArchivePlan(plan)
                _uiState.value = _uiState.value.copy(archivePlans = repository.archivePlans(), error = null)
                startPollingIfNeeded()
            } catch (error: ApiException) { _uiState.value = _uiState.value.copy(error = error.message ?: "无法更新归档计划") }
        }
    }


    private fun startPollingIfNeeded() {
        val state = _uiState.value
        val active = state.driveCommands.any { it.state == "queued" || it.state == "submitting" } ||
            state.localUploads.any { it.state in setOf("queued", "hashing", "submitting_init", "uploading") } ||
            state.archivePlans.any { it.state == "queued" || it.state == "running" }
        if (!active || pollingJob?.isActive == true) return
        pollingJob = viewModelScope.launch {
            while (true) {
                delay(3_000)
                val driveCommands = runCatching { repository.drive115Commands() }.getOrDefault(_uiState.value.driveCommands)
                val localUploads = runCatching { repository.localUploads() }.getOrDefault(_uiState.value.localUploads)
                val archivePlans = runCatching { repository.archivePlans() }.getOrDefault(_uiState.value.archivePlans)
                _uiState.value = _uiState.value.copy(driveCommands = driveCommands, localUploads = localUploads, archivePlans = archivePlans)
                if (driveCommands.none { it.state == "queued" || it.state == "submitting" } &&
                    localUploads.none { it.state in setOf("queued", "hashing", "submitting_init", "uploading") } &&
                    archivePlans.none { it.state == "queued" || it.state == "running" }) break
            }
        }
    }

    private fun fail(message: String) {
        _uiState.value = _uiState.value.copy(loading = false, error = message)
    }
}
