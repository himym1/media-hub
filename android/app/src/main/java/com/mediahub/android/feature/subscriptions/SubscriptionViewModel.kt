package com.mediahub.android.feature.subscriptions

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.MediaSubscription
import com.mediahub.android.core.network.ProviderSourceSettings
import com.mediahub.android.core.network.SearchCandidate
import com.mediahub.android.core.network.SubscriptionInput
import com.mediahub.android.core.network.SubscriptionPreferences
import com.mediahub.android.core.network.SubscriptionRun
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlin.math.roundToLong

internal data class SubscriptionEditorState(
    val tmdbId: String = "",
    val title: String = "",
    val originalTitle: String = "",
    val year: String = "",
    val mediaType: String = "movie",
    val season: String = "0",
    val policy: String = "once",
    val enabled: Boolean = true,
    val intervalMinutes: String = "60",
    val sourceIds: List<String> = emptyList(),
    val qualityPreset: String = "standard",
    val resolutions: String = "",
    val videoCodecs: String = "",
    val dynamicRanges: String = "",
    val audioContains: String = "",
    val preferredSources: String = "",
    val minSizeGiB: String = "",
    val maxSizeGiB: String = "",
    val allowUnknownSize: Boolean = false,
    val preferSmaller: Boolean = false,
)

internal data class SubscriptionMutationResult(
    val targetKey: Long,
    val subscriptionId: String,
)

internal data class SubscriptionUiState(
    val subscriptions: List<MediaSubscription> = emptyList(),
    val selectedId: String? = null,
    val editor: SubscriptionEditorState = SubscriptionEditorState(),
    val runs: List<SubscriptionRun> = emptyList(),
    val availableSources: List<ProviderSourceSettings> = emptyList(),
    val loading: Boolean = false,
    val saving: Boolean = false,
    val errorMessage: String? = null,
    val exportPayload: String? = null,
    val actionMessage: String? = null,
    val deleted: SubscriptionMutationResult? = null,
    val saved: SubscriptionMutationResult? = null,
    val initialized: Boolean = false,
)


class SubscriptionViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(SubscriptionUiState())
    internal val uiState: StateFlow<SubscriptionUiState> = _uiState.asStateFlow()
    private var pollingJob: Job? = null

    fun startPolling() {
        if (pollingJob != null) return
        pollingJob = viewModelScope.launch {
            var first = true
            while (true) {
                load(first)
                first = false
                delay(15_000)
            }
        }
    }

    fun stopPolling() {
        pollingJob?.cancel()
        pollingJob = null
    }

    fun applyDraft(candidate: SearchCandidate) {
        val tmdbId = candidate.tmdbId ?: return
        _uiState.value = _uiState.value.copy(
            selectedId = null,
            editor = SubscriptionEditorState(
                tmdbId = tmdbId,
                title = candidate.title,
                year = candidate.year.takeIf { it > 0 }?.toString() ?: "",
                mediaType = candidate.mediaType,
                season = candidate.season.toString(),
                sourceIds = listOf(candidate.sourceId),
            ),
            runs = emptyList(),
            errorMessage = null,
        )
    }

    fun createNew() {
        _uiState.value = _uiState.value.copy(
            selectedId = null,
            editor = SubscriptionEditorState(),
            runs = emptyList(),
            errorMessage = null,
        )
    }

    fun select(id: String) {
        val item = _uiState.value.subscriptions.firstOrNull { it.id == id } ?: return
        _uiState.value = _uiState.value.copy(
            selectedId = id,
            editor = editorFrom(item),
            errorMessage = null,
        )
        viewModelScope.launch { loadRuns(id) }
    }

    fun closeEditor() {
        _uiState.value = _uiState.value.copy(
            selectedId = null,
            editor = SubscriptionEditorState(),
            runs = emptyList(),
            errorMessage = null,
        )
    }

    internal fun updateEditor(editor: SubscriptionEditorState) {
        _uiState.value = _uiState.value.copy(editor = editor, errorMessage = null)
    }

    fun save(targetKey: Long) {
        if (_uiState.value.saving) return
        val input = inputFrom(_uiState.value.editor)
        if (input == null) {
            _uiState.value = _uiState.value.copy(errorMessage = "请检查 TMDB ID、标题、年份、季号和轮询间隔")
            return
        }
        val selectedId = _uiState.value.selectedId
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(saving = true, errorMessage = null)
            try {
                val item = if (selectedId == null) {
                    repository.createSubscription(input)
                } else {
                    repository.updateSubscription(selectedId, input)
                }
                load(false)
                select(item.id)
                _uiState.value = _uiState.value.copy(
                    saving = false,
                    saved = SubscriptionMutationResult(targetKey, item.id),
                )
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(saving = false, errorMessage = error.message ?: "订阅保存失败")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(saving = false, errorMessage = "无法保存订阅")
            }
        }
    }

    fun consumeSaved(targetKey: Long) {
        if (_uiState.value.saved?.targetKey == targetKey) {
            _uiState.value = _uiState.value.copy(saved = null)
        }
    }

    fun toggleEnabled() {
        val id = _uiState.value.selectedId ?: return
        val enabled = !_uiState.value.editor.enabled
        perform("订阅状态更新失败") {
            repository.setSubscriptionEnabled(id, enabled)
            load(false)
            select(id)
        }
    }

    fun runNow() {
        val id = _uiState.value.selectedId ?: return
        perform("订阅运行失败") {
            repository.runSubscription(id)
            loadRuns(id)
        }
    }

    fun delete(targetKey: Long) {
        val id = _uiState.value.selectedId ?: return
        perform("订阅删除失败") {
            repository.deleteSubscription(id)
            _uiState.value = _uiState.value.copy(
                selectedId = null,
                editor = SubscriptionEditorState(),
                runs = emptyList(),
                deleted = SubscriptionMutationResult(targetKey, id),
            )
            load(false)
        }
    }

    fun consumeDeleted(targetKey: Long) {
        if (_uiState.value.deleted?.targetKey == targetKey) {
            _uiState.value = _uiState.value.copy(deleted = null)
        }
    }

    fun setAllEnabled(enabled: Boolean) {
        val ids = _uiState.value.subscriptions.map { it.id }
        if (ids.isEmpty()) return
        perform("批量更新订阅失败") {
            repository.setSubscriptionsEnabled(ids, enabled)
            _uiState.value = _uiState.value.copy(actionMessage = if (enabled) "订阅已全部启用" else "订阅已全部暂停")
            load(false)
        }
    }

    fun exportBackup() {
        perform("导出订阅备份失败") {
            _uiState.value = _uiState.value.copy(exportPayload = repository.exportSubscriptions())
        }
    }

    fun consumeExport() {
        _uiState.value = _uiState.value.copy(exportPayload = null)
    }

    fun importBackup(backup: String) {
        if (backup.toByteArray().size > 2 * 1024 * 1024) {
            _uiState.value = _uiState.value.copy(errorMessage = "订阅备份超过 2 MiB")
            return
        }
        perform("导入订阅备份失败") {
            val (created, skipped) = repository.importSubscriptions(backup)
            _uiState.value = _uiState.value.copy(actionMessage = "已导入 $created 条，跳过 $skipped 条")
            load(false)
        }
    }


    fun reportFileError(message: String) {
        _uiState.value = _uiState.value.copy(errorMessage = message)
    }

    private fun perform(fallback: String, block: suspend () -> Unit) {
        if (_uiState.value.saving) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(saving = true, errorMessage = null)
            try {
                block()
                _uiState.value = _uiState.value.copy(saving = false)
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(saving = false, errorMessage = error.message ?: fallback)
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(saving = false, errorMessage = fallback)
            }
        }
    }

    private suspend fun load(initial: Boolean) {
        if (initial) _uiState.value = _uiState.value.copy(loading = true)
        try {
            val previousSelectedId = _uiState.value.selectedId
            val subscriptions = repository.subscriptions()
            val selectedId = _uiState.value.selectedId?.takeIf { id -> subscriptions.any { it.id == id } }
            val selectionRemoved = previousSelectedId != null && selectedId == null
            val sources = if (_uiState.value.availableSources.isEmpty()) {
                runCatching { repository.providerSettings().sources }.getOrDefault(emptyList())
            } else {
                _uiState.value.availableSources
            }
            _uiState.value = _uiState.value.copy(
                subscriptions = subscriptions,
                selectedId = selectedId,
                editor = if (selectionRemoved) SubscriptionEditorState() else _uiState.value.editor,
                runs = if (selectionRemoved) emptyList() else _uiState.value.runs,
                availableSources = sources,
                loading = false,
                errorMessage = null,
                initialized = true,
            )
            if (selectedId != null) loadRuns(selectedId)
        } catch (error: ApiException) {
            _uiState.value = _uiState.value.copy(loading = false, errorMessage = error.message ?: "订阅读取失败")
        } catch (_: Exception) {
            _uiState.value = _uiState.value.copy(loading = false, errorMessage = "无法读取订阅")
        }
    }

    private suspend fun loadRuns(id: String) {
        runCatching { repository.subscriptionRuns(id) }.onSuccess { runs ->
            if (_uiState.value.selectedId == id) _uiState.value = _uiState.value.copy(runs = runs)
        }
    }
}

private fun editorFrom(item: MediaSubscription): SubscriptionEditorState {
    val editor = SubscriptionEditorState(
        tmdbId = item.tmdbId,
        title = item.title,
        originalTitle = item.originalTitle,
        year = item.year.takeIf { it > 0 }?.toString() ?: "",
        mediaType = item.mediaType,
        season = item.season.toString(),
        policy = item.policy,
        enabled = item.enabled,
        intervalMinutes = item.intervalMinutes.toString(),
        sourceIds = item.sourceIds,
        qualityPreset = "custom",
        resolutions = item.preferences.resolutions.joinToString(", "),
        videoCodecs = item.preferences.videoCodecs.joinToString(", "),
        dynamicRanges = item.preferences.dynamicRanges.joinToString(", "),
        audioContains = item.preferences.audioContains.joinToString(", "),
        preferredSources = item.preferences.preferredSources.joinToString(", "),
        minSizeGiB = bytesToGiB(item.preferences.minSizeBytes),
        maxSizeGiB = bytesToGiB(item.preferences.maxSizeBytes),
        allowUnknownSize = item.preferences.allowUnknownSize,
        preferSmaller = item.preferences.preferSmaller,
    )
    return editor.copy(qualityPreset = inferQualityPreset(editor))
}

private fun inputFrom(editor: SubscriptionEditorState): SubscriptionInput? {
    val tmdbId = editor.tmdbId.trim()
    val title = editor.title.trim()
    val year = editor.year.ifBlank { "0" }.toIntOrNull() ?: return null
    val season = if (editor.mediaType == "movie") 0 else editor.season.ifBlank { "0" }.toIntOrNull() ?: return null
    val interval = editor.intervalMinutes.toIntOrNull() ?: return null
    if (!tmdbId.matches(Regex("^[1-9][0-9]{0,19}$")) || title.isEmpty() || year !in 0..2100 || season !in 0..100 || interval !in 15..10080) return null
    return SubscriptionInput(
        tmdbId = tmdbId,
        title = title,
        originalTitle = editor.originalTitle.trim(),
        year = year,
        mediaType = editor.mediaType,
        season = season,
        policy = editor.policy,
        enabled = editor.enabled,
        intervalMinutes = interval,
        sourceIds = editor.sourceIds,
        preferences = SubscriptionPreferences(
            resolutions = splitValues(editor.resolutions),
            videoCodecs = splitValues(editor.videoCodecs),
            dynamicRanges = splitValues(editor.dynamicRanges),
            audioContains = splitValues(editor.audioContains),
            preferredSources = splitValues(editor.preferredSources),
            minSizeBytes = gibToBytes(editor.minSizeGiB),
            maxSizeBytes = gibToBytes(editor.maxSizeGiB),
            allowUnknownSize = editor.allowUnknownSize,
            preferSmaller = editor.preferSmaller,
        ),
    )
}

internal fun applyQualityPreset(editor: SubscriptionEditorState, preset: String): SubscriptionEditorState {
    if (preset == "custom") return editor.copy(qualityPreset = preset)
    val next = editor.copy(
        qualityPreset = preset,
        resolutions = "", videoCodecs = "", dynamicRanges = "", audioContains = "", preferredSources = "",
        minSizeGiB = "", maxSizeGiB = "", allowUnknownSize = false, preferSmaller = false,
    )
    return when (preset) {
        "space" -> next.copy(resolutions = "1080p", videoCodecs = "HEVC", maxSizeGiB = "20", preferSmaller = true)
        "balanced" -> next.copy(resolutions = "2160p, 1080p", videoCodecs = "HEVC, AVC", maxSizeGiB = "40", preferSmaller = true)
        "quality" -> next.copy(resolutions = "2160p", dynamicRanges = "Dolby Vision, HDR10", minSizeGiB = "15")
        else -> next.copy(qualityPreset = "standard")
    }
}

internal fun inferQualityPreset(editor: SubscriptionEditorState): String {
    val fields: (SubscriptionEditorState) -> List<Any> = { value -> listOf(
        value.resolutions, value.videoCodecs, value.dynamicRanges, value.audioContains, value.preferredSources,
        value.minSizeGiB, value.maxSizeGiB, value.allowUnknownSize, value.preferSmaller,
    ) }
    return listOf("standard", "space", "balanced", "quality")
        .firstOrNull { fields(applyQualityPreset(editor, it)) == fields(editor) } ?: "custom"
}


private fun splitValues(value: String) = value.split(',').map(String::trim).filter(String::isNotEmpty).distinct()

private fun gibToBytes(value: String): Long {
    val number = value.ifBlank { "0" }.toDoubleOrNull() ?: return 0
    return if (number < 0) 0 else (number * 1024 * 1024 * 1024).roundToLong()
}

private fun bytesToGiB(value: Long) = if (value > 0) "%.1f".format(value.toDouble() / 1024 / 1024 / 1024) else ""
