package com.mediahub.android.feature.operations

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.LogOut
import com.composables.icons.lucide.ArrowRightLeft
import com.composables.icons.lucide.FolderOpen
import com.composables.icons.lucide.HardDrive
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.Upload
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTextField


@Composable
internal fun OperationsRoute(
    viewModel: OperationsViewModel,
    onLogout: () -> Unit,
) {
    val state by viewModel.uiState.collectAsState()
    OperationsScreen(
        state = state,
        onRefresh = viewModel::refresh,
        onDriveParentChanged = viewModel::setDriveParentId,
        onDriveOperationChanged = viewModel::setDriveOperation,
        onDriveFileIdsChanged = viewModel::setDriveFileIds,
        onDriveTargetChanged = viewModel::setDriveTargetParentId,
        onDriveNameChanged = viewModel::setDriveName,
        onCreateDriveCommand = viewModel::createDriveCommand,
        onConfirmDriveCommand = viewModel::confirmDriveCommand,
        onLocalRootChanged = viewModel::setLocalRoot,
        onLocalPathChanged = viewModel::setLocalPath,
        onLocalEntrySelected = viewModel::selectLocalEntry,
        onLocalDestinationChanged = viewModel::setLocalDestination,
        onCreateLocalUpload = viewModel::createLocalUpload,
        onRetryLocalUpload = viewModel::retryLocalUpload,
        onArchiveParentChanged = viewModel::setArchiveParentId,
        onArchiveTargetChanged = viewModel::setArchiveTargetId,
        onPreviewArchive = viewModel::previewArchive,
        onToggleArchive = viewModel::toggleArchive,
        onArchiveNameChanged = viewModel::setArchiveName,
        onCreateArchivePlan = viewModel::createArchivePlan,
        onConfirmArchive = viewModel::confirmArchive,
        onRetryArchive = viewModel::retryArchive,
        onMigrateSubscriptions = viewModel::migrateSubscriptions,
        onRetryMigrationCommand = viewModel::retryMigrationCommand,
        onLogout = onLogout,
    )
}

@Composable
private fun OperationsScreen(
    state: OperationsUiState,
    onRefresh: () -> Unit,
    onDriveParentChanged: (String) -> Unit,
    onDriveOperationChanged: (String) -> Unit,
    onDriveFileIdsChanged: (String) -> Unit,
    onDriveTargetChanged: (String) -> Unit,
    onDriveNameChanged: (String) -> Unit,
    onCreateDriveCommand: () -> Unit,
    onConfirmDriveCommand: (com.mediahub.android.core.network.Drive115Command) -> Unit,
    onLocalRootChanged: (String) -> Unit,
    onLocalPathChanged: (String) -> Unit,
    onLocalEntrySelected: (com.mediahub.android.core.network.LocalUploadEntry) -> Unit,
    onLocalDestinationChanged: (String) -> Unit,
    onCreateLocalUpload: () -> Unit,
    onRetryLocalUpload: (com.mediahub.android.core.network.LocalUploadJob) -> Unit,
    onArchiveParentChanged: (String) -> Unit,
    onArchiveTargetChanged: (String) -> Unit,
    onPreviewArchive: () -> Unit,
    onToggleArchive: (String) -> Unit,
    onArchiveNameChanged: (String, String) -> Unit,
    onCreateArchivePlan: () -> Unit,
    onConfirmArchive: (com.mediahub.android.core.network.ArchivePlan) -> Unit,
    onRetryArchive: (com.mediahub.android.core.network.ArchivePlan) -> Unit,
    onMigrateSubscriptions: () -> Unit,
    onRetryMigrationCommand: (com.mediahub.android.core.network.MigrationSourceCommand) -> Unit,
    onLogout: () -> Unit,
) {
    LazyColumn(
        modifier = Modifier
            .fillMaxSize()
            .background(MediaHubColors.Canvas)
            .statusBarsPadding()
            .navigationBarsPadding()
            .padding(horizontal = 20.dp),
        contentPadding = PaddingValues(bottom = 84.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        item {
            Row(
                modifier = Modifier.fillMaxWidth().padding(top = 18.dp, bottom = 6.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column {
                    MediaHubText(text = "运维", color = MediaHubColors.TextPrimary, fontSize = 25.sp, fontWeight = FontWeight.SemiBold)
                    MediaHubText(
                        text = if (state.configured) "SubX 迁移源可用" else "原生运营模式",
                        color = if (state.configured) MediaHubColors.Source else MediaHubColors.TextMuted,
                        fontSize = 11.sp,
                    )
                }
                Row {
                    MediaHubIconButton(imageVector = Lucide.RefreshCw, contentDescription = "刷新", onClick = onRefresh)
                    MediaHubIconButton(imageVector = Lucide.LogOut, contentDescription = "退出登录", onClick = onLogout)
                }
            }
        }
        state.error?.let { message ->
            item { MediaHubText(text = message, color = MediaHubColors.Error, fontSize = 12.sp) }
        }
        if (state.loading) {
            item { MediaHubText(text = "正在读取运维能力...", color = MediaHubColors.TextMuted, fontSize = 13.sp) }
        } else {
            state.migration?.let { migration ->
                item {
                    Column(
                        modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        Row(
                            modifier = Modifier.fillMaxWidth(),
                            horizontalArrangement = Arrangement.SpaceBetween,
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Column(Modifier.weight(1f)) {
                                MediaHubText(
                                    text = if (migration.canStopSubX) "已满足 SubX 停用门槛" else "SubX 不可停用",
                                    color = if (migration.canStopSubX) MediaHubColors.Source else MediaHubColors.Warning,
                                    fontSize = 13.sp,
                                    fontWeight = FontWeight.SemiBold,
                                )
                                MediaHubText(
                                    text = "${migration.nativeSubscriptions} 个原生订阅 · ${migration.nativeSourceCount} 个原生来源 · ${migration.delegatedOperations} 个委托操作",
                                    color = MediaHubColors.TextSecondary,
                                    fontSize = 11.sp,
                                )
                                MediaHubText(
                                    text = "核心配置 ${if (migration.coreConfigurationReady) "完整" else "未完成"} · 并行验收 ${if (migration.parallelValidationCompleted) "已确认" else "未确认"}",
                                    color = MediaHubColors.TextMuted,
                                    fontSize = 11.sp,
                                )
                            }
                            MediaHubButton(
                                label = if (state.migrating) "迁移中" else "迁移订阅",
                                onClick = onMigrateSubscriptions,
                                enabled = migration.subXConfigured && !state.migrating,
                                icon = Lucide.ArrowRightLeft,
                            )
                        }
                        migration.blockers.firstOrNull()?.let { blocker ->
                            MediaHubText(text = blocker, color = MediaHubColors.TextMuted, fontSize = 11.sp)
                        }
                        state.migrationCommands.forEach { command ->
                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                Column(Modifier.weight(1f)) {
                                    MediaHubText(command.state, color = commandStateColor(command.state), fontSize = 11.sp, fontWeight = FontWeight.Bold)
                                    MediaHubText("${command.operationId} · ${command.id}", color = MediaHubColors.TextMuted, fontSize = 9.sp)
                                    if (command.errorMessage.isNotBlank()) MediaHubText(command.errorMessage, color = MediaHubColors.Warning, fontSize = 10.sp)
                                }
                                if (command.state == "needs_attention") {
                                    MediaHubButton("核对后重试", onClick = { onRetryMigrationCommand(command) }, enabled = migration.fallbackSourceEnabled)
                                }
                            }
                        }
                        state.migrationResult?.let { result ->
                            MediaHubText(
                                text = "新增 ${result.created} · 已有 ${result.skipped} · 无法识别 ${result.rejected}",
                                color = MediaHubColors.Source,
                                fontSize = 11.sp,
                            )
                        }
                    }
                }
            }
            item {
                Drive115Section(
                    state = state,
                    onParentChanged = onDriveParentChanged,
                    onOperationChanged = onDriveOperationChanged,
                    onFileIdsChanged = onDriveFileIdsChanged,
                    onTargetChanged = onDriveTargetChanged,
                    onNameChanged = onDriveNameChanged,
                    onCreate = onCreateDriveCommand,
                    onConfirm = onConfirmDriveCommand,
                )
            }
            if (state.localRoots.isNotEmpty()) item {
				LocalUploadSection(state,onLocalRootChanged,onLocalPathChanged,onLocalEntrySelected,onLocalDestinationChanged,onCreateLocalUpload,onRetryLocalUpload)
			}
            item { ArchiveSection(state,onArchiveParentChanged,onArchiveTargetChanged,onPreviewArchive,onToggleArchive,onArchiveNameChanged,onCreateArchivePlan,onConfirmArchive,onRetryArchive) }
        }
    }
}

@Composable
private fun Drive115Section(
    state: OperationsUiState,
    onParentChanged: (String) -> Unit,
    onOperationChanged: (String) -> Unit,
    onFileIdsChanged: (String) -> Unit,
    onTargetChanged: (String) -> Unit,
    onNameChanged: (String) -> Unit,
    onCreate: () -> Unit,
    onConfirm: (com.mediahub.android.core.network.Drive115Command) -> Unit,
 ) {
    Column(modifier = Modifier.fillMaxWidth().padding(vertical = 12.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            com.mediahub.android.core.designsystem.MediaHubIcon(imageVector = Lucide.HardDrive, contentDescription = null, tint = MediaHubColors.Accent)
            Spacer(Modifier.padding(horizontal = 4.dp))
            MediaHubText(text = "115 文件", color = MediaHubColors.TextPrimary, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
        }
        MediaHubTextField(value = state.driveParentId, onValueChange = onParentChanged, placeholder = "当前目录 ID", modifier = Modifier.fillMaxWidth(), keyboardType = KeyboardType.Number)
        state.driveFiles.take(50).forEach { file ->
            Row(modifier = Modifier.fillMaxWidth().clickable(enabled = file.kind == "folder") { if (file.kind == "folder") onParentChanged(file.id) }.padding(vertical = 9.dp), verticalAlignment = Alignment.CenterVertically) {
                com.mediahub.android.core.designsystem.MediaHubIcon(imageVector = Lucide.FolderOpen, contentDescription = null, tint = if (file.kind == "folder") MediaHubColors.Accent else MediaHubColors.TextMuted)
                Spacer(Modifier.padding(horizontal = 5.dp))
                Column(Modifier.weight(1f)) {
                    MediaHubText(text = file.name, color = MediaHubColors.TextStrong, fontSize = 12.sp)
                    MediaHubText(text = if (file.kind == "folder") "目录 ${file.id}" else "${formatBytes(file.size)} · ${file.id}", color = MediaHubColors.TextMuted, fontSize = 9.sp)
                }
            }
        }
        Row(modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            listOf("create_folder" to "建目录", "move" to "移动", "rename" to "重命名", "delete" to "删除").forEach { (value,label) -> ChoiceChip(label, state.driveOperation == value) { onOperationChanged(value) } }
        }
        if (state.driveOperation != "create_folder") MediaHubTextField(value = state.driveFileIds, onValueChange = onFileIdsChanged, placeholder = if (state.driveOperation == "rename") "文件 ID" else "文件 ID，逗号分隔", modifier = Modifier.fillMaxWidth())
        if (state.driveOperation == "create_folder" || state.driveOperation == "move") MediaHubTextField(value = state.driveTargetParentId, onValueChange = onTargetChanged, placeholder = if (state.driveOperation == "move") "目标目录 ID" else "父目录 ID", modifier = Modifier.fillMaxWidth(), keyboardType = KeyboardType.Number)
        if (state.driveOperation == "create_folder" || state.driveOperation == "rename") MediaHubTextField(value = state.driveName, onValueChange = onNameChanged, placeholder = "名称", modifier = Modifier.fillMaxWidth())
        MediaHubButton(label = if (state.driveInvoking) "正在持久化" else if (state.driveOperation == "delete") "创建待确认删除命令" else "创建命令", onClick = onCreate, enabled = !state.driveInvoking, modifier = Modifier.fillMaxWidth())
        state.driveCommands.take(8).forEach { command ->
            Row(modifier = Modifier.fillMaxWidth().padding(vertical = 7.dp), horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                Column(Modifier.weight(1f)) { MediaHubText(text = command.operation, color = MediaHubColors.TextStrong, fontSize = 12.sp); MediaHubText(text = command.id, color = MediaHubColors.TextMuted, fontSize = 9.sp) }
                MediaHubText(text = commandStateLabel(command.state), color = commandStateColor(command.state), fontSize = 10.sp)
                if (command.state in setOf("awaiting_confirmation", "failed", "needs_attention")) MediaHubButton(label = if (command.state == "awaiting_confirmation") "确认删除" else "确认重试", onClick = { onConfirm(command) })
            }
        }
    }
}

@Composable
private fun LocalUploadSection(
    state: OperationsUiState,
    onRootChanged: (String) -> Unit,
    onPathChanged: (String) -> Unit,
    onEntrySelected: (com.mediahub.android.core.network.LocalUploadEntry) -> Unit,
    onDestinationChanged: (String) -> Unit,
    onCreate: () -> Unit,
    onRetry: (com.mediahub.android.core.network.LocalUploadJob) -> Unit,
 ) {
    Column(modifier = Modifier.fillMaxWidth().padding(vertical = 12.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            com.mediahub.android.core.designsystem.MediaHubIcon(imageVector = Lucide.Upload, contentDescription = null, tint = MediaHubColors.Accent)
            Spacer(Modifier.padding(horizontal = 4.dp))
            MediaHubText(text = "本地上传", color = MediaHubColors.TextPrimary, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
        }
        Row(modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) { state.localRoots.forEach { root -> ChoiceChip(root.id,root.id==state.localRootId){onRootChanged(root.id)} } }
        MediaHubTextField(value=state.localPath,onValueChange=onPathChanged,placeholder="相对目录",modifier=Modifier.fillMaxWidth())
        if(state.localPath.isNotEmpty()) MediaHubButton(label="返回上级",onClick={onPathChanged(state.localPath.substringBeforeLast('/',""))})
        state.localEntries.take(50).forEach { entry ->
            Row(modifier=Modifier.fillMaxWidth().clickable{onEntrySelected(entry)}.background(if(state.localSelectedFile==entry.path)MediaHubColors.SurfaceSelected else MediaHubColors.Canvas).padding(vertical=9.dp),verticalAlignment=Alignment.CenterVertically){
                com.mediahub.android.core.designsystem.MediaHubIcon(imageVector=Lucide.FolderOpen,contentDescription=null,tint=if(entry.directory)MediaHubColors.Accent else MediaHubColors.TextMuted)
                Spacer(Modifier.padding(horizontal=5.dp));Column(Modifier.weight(1f)){MediaHubText(entry.name,fontSize=12.sp);MediaHubText(if(entry.directory)"目录" else formatBytes(entry.size),color=MediaHubColors.TextMuted,fontSize=9.sp)}
            }
        }
        MediaHubText(text=state.localSelectedFile.ifBlank{"请选择一个文件"},color=MediaHubColors.TextMuted,fontSize=10.sp)
        MediaHubTextField(value=state.localDestinationId,onValueChange=onDestinationChanged,placeholder="115 目标目录 ID",modifier=Modifier.fillMaxWidth(),keyboardType=KeyboardType.Number)
        MediaHubButton(label=if(state.localInvoking)"正在创建" else "创建上传任务",icon=Lucide.Upload,onClick=onCreate,enabled=!state.localInvoking&&state.localSelectedFile.isNotBlank()&&state.localDestinationId.isNotBlank(),modifier=Modifier.fillMaxWidth())
        state.localUploads.take(8).forEach { upload ->
            Row(modifier=Modifier.fillMaxWidth().padding(vertical=7.dp),horizontalArrangement=Arrangement.spacedBy(8.dp),verticalAlignment=Alignment.CenterVertically){
                Column(Modifier.weight(1f)){MediaHubText(upload.path,fontSize=12.sp);MediaHubText(if(upload.bytesTotal>0)"${upload.bytesDone*100/upload.bytesTotal}% · ${formatBytes(upload.bytesTotal)}" else upload.id,color=MediaHubColors.TextMuted,fontSize=9.sp)}
                MediaHubText(localUploadState(upload.state),color=commandStateColor(upload.state),fontSize=10.sp)
                if(upload.state=="failed"||upload.state=="needs_attention")MediaHubButton(label="确认重试",onClick={onRetry(upload)})
            }
        }
    }
}

private fun localUploadState(state:String)=when(state){"queued"->"等待中";"hashing"->"校验中";"submitting_init"->"初始化";"uploading"->"上传中";"completed"->"已完成";"failed"->"失败";"needs_attention"->"待核对";else->state}

private fun formatBytes(value: Long): String = when {
    value < 1024 -> "$value B"
    value < 1024 * 1024 -> "%.1f KB".format(value / 1024.0)
    value < 1024L * 1024 * 1024 -> "%.1f MB".format(value / (1024.0 * 1024))
    else -> "%.1f GB".format(value / (1024.0 * 1024 * 1024))
}

@Composable
private fun ChoiceChip(label: String, selected: Boolean, onClick: () -> Unit) {
    MediaHubText(
        text = label,
        color = if (selected) MediaHubColors.Accent else MediaHubColors.TextSecondary,
        fontSize = 12.sp,
        fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
        modifier = Modifier
            .background(if (selected) MediaHubColors.SurfaceSelected else MediaHubColors.Surface)
            .clickable(onClick = onClick)
            .padding(horizontal = 12.dp, vertical = 9.dp),
    )
}

@Composable
private fun JsonResult(value: String) {
    MediaHubText(
        text = value,
        color = MediaHubColors.TextSecondary,
        fontSize = 11.sp,
        modifier = Modifier.fillMaxWidth().background(MediaHubColors.SurfaceInput).padding(12.dp),
    )
}

private fun commandStateLabel(state: String) = when (state) {
    "awaiting_confirmation" -> "待确认"
    "queued" -> "等待中"
    "submitting" -> "提交中"
    "completed" -> "已完成"
    "failed" -> "失败"
    "needs_attention" -> "待确认"
    else -> state
}

private fun commandStateColor(state: String) = when (state) {
    "awaiting_confirmation" -> MediaHubColors.Warning
    "completed" -> MediaHubColors.Source
    "failed" -> MediaHubColors.Error
    "needs_attention" -> MediaHubColors.Warning
    else -> MediaHubColors.TextSecondary
}

@Composable
private fun ArchiveSection(
    state: OperationsUiState, onParentChanged: (String) -> Unit, onTargetChanged: (String) -> Unit,
    onPreview: () -> Unit, onToggle: (String) -> Unit, onNameChanged: (String, String) -> Unit,
    onCreate: () -> Unit, onConfirm: (com.mediahub.android.core.network.ArchivePlan) -> Unit,
    onRetry: (com.mediahub.android.core.network.ArchivePlan) -> Unit,
 ) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        com.mediahub.android.core.designsystem.MediaHubIcon(
            imageVector = Lucide.FolderOpen,
            contentDescription = null,
            tint = MediaHubColors.Accent,
        )
        Spacer(Modifier.padding(horizontal = 4.dp))
        Column {
            MediaHubText("原生归档整理", fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
            MediaHubText(
                "预览只生成建议；选中的步骤落库并以计划 ID 二次确认后执行",
                color = MediaHubColors.TextMuted,
                fontSize = 10.sp,
            )
        }
    }
    MediaHubTextField(state.archiveParentId, onParentChanged, "来源目录 ID", keyboardType = KeyboardType.Number)
    Spacer(Modifier.height(8.dp))
    MediaHubTextField(state.archiveTargetId, onTargetChanged, "移动目标目录 ID（可选）", keyboardType = KeyboardType.Number)
    Spacer(Modifier.height(8.dp))
    MediaHubButton("预览建议", onClick = onPreview, enabled = state.archiveParentId.isNotBlank())
    state.archiveSuggestions.forEach { item ->
        Spacer(Modifier.height(8.dp))
        Column(Modifier.fillMaxWidth().background(MediaHubColors.Surface).padding(10.dp)) {
            MediaHubText(item.currentName, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
            MediaHubText("${if(item.kind=="directory") "目录" else "文件"} · 需人工复核", color = MediaHubColors.TextSecondary, fontSize = 11.sp)
            Spacer(Modifier.height(6.dp))
            MediaHubButton(if(item.fileId in state.archiveSelected) "取消" else "选择", onClick = { onToggle(item.fileId) })
            Spacer(Modifier.height(6.dp))
            MediaHubTextField(state.archiveNames[item.fileId] ?: item.suggestedName, { onNameChanged(item.fileId,it) }, "新名称")
        }
    }
    if(state.archiveSuggestions.isNotEmpty()){Spacer(Modifier.height(10.dp));MediaHubButton("保存待确认计划", onClick=onCreate, enabled=state.archiveSelected.isNotEmpty())}
    if(state.archivePlans.isNotEmpty()){Spacer(Modifier.height(16.dp));MediaHubText("归档计划",fontSize=15.sp,fontWeight=FontWeight.Bold)}
    state.archivePlans.forEach { plan ->
        Row(Modifier.fillMaxWidth().padding(vertical=8.dp),horizontalArrangement=Arrangement.SpaceBetween,verticalAlignment=Alignment.CenterVertically){
            Column(Modifier.weight(1f)){MediaHubText(plan.state,color=commandStateColor(plan.state),fontSize=12.sp,fontWeight=FontWeight.Bold);MediaHubText("${plan.stepIndex} / ${plan.stepTotal} 步 · ${plan.id}",color=MediaHubColors.TextSecondary,fontSize=10.sp);if(plan.errorMessage.isNotBlank())MediaHubText(plan.errorMessage,color=MediaHubColors.Error,fontSize=11.sp)}
            when(plan.state){"awaiting_confirmation"->MediaHubButton("确认计划 ID",onClick={onConfirm(plan)});"failed","needs_attention"->MediaHubButton("核对后继续",onClick={onRetry(plan)});else->{}}
        }
    }
}
