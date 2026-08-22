package com.mediahub.android.feature.subscriptions

import androidx.activity.compose.BackHandler
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import com.composables.icons.lucide.ListPlus
import com.composables.icons.lucide.Lucide
import com.mediahub.android.core.designsystem.MediaHubListDetail
import com.mediahub.android.core.network.SearchCandidate

internal data class SubscriptionNavigator(
    val editorItemId: String?,
    val editorKey: Long?,
    val openEditor: (String?) -> Unit,
    val replaceEditor: (String, Long) -> Unit,
    val closeEditor: () -> Unit,
) {
    val editorOpen: Boolean get() = editorKey != null
}

internal sealed interface SubscriptionRouteResolution {
    data object Keep : SubscriptionRouteResolution
    data class Replace(val subscriptionId: String, val targetKey: Long) : SubscriptionRouteResolution
    data object Close : SubscriptionRouteResolution
}

internal fun resolveSubscriptionRoute(
    editorOpen: Boolean,
    editorItemId: String?,
    editorKey: Long?,
    initialized: Boolean,
    itemExists: Boolean,
    saved: SubscriptionMutationResult?,
    deleted: SubscriptionMutationResult?,
): SubscriptionRouteResolution {
    if (!editorOpen || editorKey == null) return SubscriptionRouteResolution.Keep
    if (deleted?.targetKey == editorKey && deleted.subscriptionId == editorItemId) {
        return SubscriptionRouteResolution.Close
    }
    if (saved?.targetKey == editorKey && (editorItemId == null || editorItemId == saved.subscriptionId)) {
        return SubscriptionRouteResolution.Replace(saved.subscriptionId, saved.targetKey)
    }
    if (editorItemId != null && initialized && !itemExists) return SubscriptionRouteResolution.Close
    return SubscriptionRouteResolution.Keep
}

@Composable
internal fun SubscriptionRoute(
    viewModel: SubscriptionViewModel,
    draft: SearchCandidate?,
    navigator: SubscriptionNavigator,
    onDraftConsumed: () -> Unit,
    active: Boolean = true,
) {
    val state by viewModel.uiState.collectAsState()
    DisposableEffect(viewModel, active) {
        if (active) {
            viewModel.startPolling()
            onDispose(viewModel::stopPolling)
        } else {
            viewModel.stopPolling()
            onDispose { }
        }
    }
    LaunchedEffect(draft, active) {
        if (active && draft != null) {
            viewModel.applyDraft(draft)
            if (!navigator.editorOpen) navigator.openEditor(null)
            onDraftConsumed()
        }
    }
    LaunchedEffect(
        state.saved,
        state.deleted,
        state.subscriptions,
        state.initialized,
        navigator.editorItemId,
        navigator.editorKey,
        navigator.editorOpen,
        active,
    ) {
        if (!active) return@LaunchedEffect
        when (val resolution = resolveSubscriptionRoute(
            editorOpen = navigator.editorOpen,
            editorItemId = navigator.editorItemId,
            editorKey = navigator.editorKey,
            initialized = state.initialized,
            itemExists = navigator.editorItemId == null || state.subscriptions.any { it.id == navigator.editorItemId },
            saved = state.saved,
            deleted = state.deleted,
        )) {
            SubscriptionRouteResolution.Keep -> Unit
            SubscriptionRouteResolution.Close -> {
                viewModel.closeEditor()
                navigator.closeEditor()
            }
            is SubscriptionRouteResolution.Replace -> {
                navigator.replaceEditor(resolution.subscriptionId, resolution.targetKey)
            }
        }
        state.saved?.let { viewModel.consumeSaved(it.targetKey) }
        state.deleted?.let { viewModel.consumeDeleted(it.targetKey) }
    }
    val closeEditor = {
        if (!state.saving) {
            viewModel.closeEditor()
            navigator.closeEditor()
        }
    }
    BackHandler(enabled = navigator.editorOpen && !state.saving, onBack = closeEditor)
    val editorActions = SubscriptionEditorActions(
        editorChanged = viewModel::updateEditor,
        back = closeEditor,
        backEnabled = !state.saving,
        save = { navigator.editorKey?.let(viewModel::save) },
        toggle = viewModel::toggleEnabled,
        run = viewModel::runNow,
        delete = { navigator.editorKey?.let(viewModel::delete) },
    )
    val listActions = SubscriptionListActions(
        create = {
            viewModel.createNew()
            navigator.openEditor(null)
        },
        select = { id ->
            viewModel.select(id)
            navigator.openEditor(id)
        },
        setAllEnabled = viewModel::setAllEnabled,
        export = viewModel::exportBackup,
        exportConsumed = viewModel::consumeExport,
        import = viewModel::importBackup,
        fileError = viewModel::reportFileError,
    )
    MediaHubListDetail(
        detailOpen = navigator.editorOpen,
        emptyTitle = "选择一条订阅",
        emptyMessage = "从左侧打开编辑",
        emptyIcon = Lucide.ListPlus,
        list = { SubscriptionListScreen(state = state, actions = listActions) },
        detail = { SubscriptionEditorScreen(state = state, actions = editorActions) },
    )
}
