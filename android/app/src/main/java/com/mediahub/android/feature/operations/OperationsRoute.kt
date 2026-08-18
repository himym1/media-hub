package com.mediahub.android.feature.operations

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubSegmentedControl
import com.mediahub.android.core.designsystem.MediaHubText

private val operationSections = listOf(
    "drive" to "115 文件",
    "upload" to "本地上传",
    "archive" to "归档整理",
)

@Composable
internal fun OperationsRoute(
    driveViewModel: Drive115ViewModel,
    localUploadViewModel: LocalUploadViewModel,
    archiveViewModel: ArchiveViewModel,
    onPlayDriveFile: (com.mediahub.android.core.network.Drive115File, String) -> Unit,
) {
    val driveState by driveViewModel.uiState.collectAsState()
    val localUploadState by localUploadViewModel.uiState.collectAsState()
    val archiveState by archiveViewModel.uiState.collectAsState()
    OperationsScreen(
        driveState = driveState,
        localUploadState = localUploadState,
        archiveState = archiveState,
        driveActions = Drive115Actions(
            refresh = driveViewModel::refresh,
            parentChanged = driveViewModel::setParentId,
            operationChanged = driveViewModel::setOperation,
            fileIdsChanged = driveViewModel::setFileIds,
            targetChanged = driveViewModel::setTargetParentId,
            nameChanged = driveViewModel::setName,
            create = driveViewModel::createCommand,
            confirm = driveViewModel::confirmCommand,
            play = { file -> onPlayDriveFile(file, driveState.parentId) },
        ),
        localUploadActions = LocalUploadActions(
            refresh = localUploadViewModel::refresh,
            rootChanged = localUploadViewModel::setRoot,
            pathChanged = localUploadViewModel::setPath,
            entrySelected = localUploadViewModel::selectEntry,
            destinationChanged = localUploadViewModel::setDestination,
            create = localUploadViewModel::createUpload,
            retry = localUploadViewModel::retryUpload,
        ),
        archiveActions = ArchiveActions(
            refresh = archiveViewModel::refresh,
            parentChanged = archiveViewModel::setParentId,
            targetChanged = archiveViewModel::setTargetId,
            preview = archiveViewModel::preview,
            toggle = archiveViewModel::toggle,
            nameChanged = archiveViewModel::setName,
            create = archiveViewModel::createPlan,
            confirm = archiveViewModel::confirm,
            retry = archiveViewModel::retry,
        ),
    )
}

@Composable
internal fun OperationsScreen(
    driveState: Drive115State,
    localUploadState: LocalUploadState,
    archiveState: ArchiveState,
    driveActions: Drive115Actions,
    localUploadActions: LocalUploadActions,
    archiveActions: ArchiveActions,
) {
    var section by rememberSaveable { mutableStateOf("drive") }
    val loading = when (section) {
        "drive" -> driveState.loading
        "upload" -> localUploadState.loading
        else -> archiveState.loading
    }
    val error = when (section) {
        "drive" -> driveState.error
        "upload" -> localUploadState.error
        else -> archiveState.error
    }
    val refresh = when (section) {
        "drive" -> driveActions.refresh
        "upload" -> localUploadActions.refresh
        else -> archiveActions.refresh
    }

    LazyColumn(
        modifier = Modifier
            .fillMaxSize()
            .background(MediaHubColors.Canvas)
            .padding(horizontal = 16.dp),
        contentPadding = PaddingValues(bottom = 18.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item {
            Row(
                modifier = Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 6.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubText(
                    text = "115 文件操作、上传与归档",
                    modifier = Modifier.weight(1f),
                    color = MediaHubColors.TextMuted,
                    fontSize = 13.sp,
                )
                MediaHubIconButton(Lucide.RefreshCw, "刷新当前工具", refresh)
            }
        }
        item {
            MediaHubSegmentedControl(
                options = operationSections,
                selected = section,
                onSelected = { section = it },
                modifier = Modifier.fillMaxWidth(),
            )
        }
        error?.let { message ->
            item { MediaHubText(message, color = MediaHubColors.Error, fontSize = 12.sp) }
        }
        if (loading) {
            item { MediaHubText("正在读取当前工具...", color = MediaHubColors.TextMuted, fontSize = 13.sp) }
        } else {
            item {
                Column(Modifier.fillMaxWidth()) {
                    when (section) {
                        "drive" -> Drive115Screen(driveState, driveActions)
                        "upload" -> LocalUploadScreen(localUploadState, localUploadActions)
                        else -> ArchiveScreen(archiveState, archiveActions)
                    }
                }
            }
        }
    }
}
