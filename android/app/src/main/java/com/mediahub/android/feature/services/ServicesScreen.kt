package com.mediahub.android.feature.services

import android.content.Intent
import android.provider.Settings
import androidx.core.content.FileProvider
import android.graphics.BitmapFactory
import android.util.Base64
import androidx.compose.foundation.background
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyListScope
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Download
import com.composables.icons.lucide.KeyRound
import com.composables.icons.lucide.LogOut
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.QrCode
import com.composables.icons.lucide.Server
import com.mediahub.android.app.LocalTwoPane
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubListDetail
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubPreferenceRow
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTextField
import com.mediahub.android.core.designsystem.MediaHubSegmentedControl
import com.mediahub.android.core.network.IntegrationHealth
import com.mediahub.android.core.network.OperationalStatistics

internal val serviceSectionOptions = listOf(
    "overview" to "状态概览",
    "providers" to "服务配置",
    "account" to "账户与更新",
)

@Composable
internal fun ServicesRoute(
    viewModel: ServicesViewModel,
    onLogout: () -> Unit,
    onChangeServer: () -> Unit,
) {
    val uiState by viewModel.uiState.collectAsState()
    val context = androidx.compose.ui.platform.LocalContext.current
    LaunchedEffect(viewModel) { viewModel.refresh() }
    LaunchedEffect(viewModel) { viewModel.checkForUpdate() }
    DisposableEffect(viewModel) { onDispose(viewModel::stopDriveAuthorization) }
    ServicesScreen(
        uiState = uiState,
        onRefresh = viewModel::refresh,
        onToggleSettings = viewModel::toggleSettings,
        onSettingsDraftChange = viewModel::setSettingsDraft,
        onSaveSettings = viewModel::saveSettings,
        onTestWeCom = viewModel::testWeComNotification,
        onStartDriveAuthorization = viewModel::startDriveAuthorization,
        onCurrentPasswordChange = viewModel::setCurrentPassword,
        onNewPasswordChange = viewModel::setNewPassword,
        onConfirmationChange = viewModel::setConfirmation,
        onChangePassword = viewModel::changePassword,
        onChangeServer = onChangeServer,
        onCheckForUpdate = viewModel::checkForUpdate,
        onDownloadUpdate = { release ->
            val directory = java.io.File(context.cacheDir, "updates").apply { mkdirs() }
            viewModel.downloadUpdate(java.io.File(directory, "media-hub-${release.versionCode}.apk"))
        },
        onInstallUpdate = { path -> if (installUpdate(context, java.io.File(path))) viewModel.consumeDownloadedUpdate() },
        onLogout = onLogout,
    )
}

@Composable
internal fun ServicesScreen(
    uiState: ServicesUiState,
    onRefresh: () -> Unit,
    onToggleSettings: () -> Unit,
    onSettingsDraftChange: (com.mediahub.android.core.network.ProviderSettingsUpdate) -> Unit,
    onSaveSettings: () -> Unit,
    onTestWeCom: () -> Unit,
    onStartDriveAuthorization: () -> Unit,
    onCurrentPasswordChange: (String) -> Unit,
    onNewPasswordChange: (String) -> Unit,
    onConfirmationChange: (String) -> Unit,
    onChangePassword: () -> Unit,
    onChangeServer: () -> Unit,
    onCheckForUpdate: () -> Unit,
    onDownloadUpdate: (com.mediahub.android.core.network.AndroidRelease) -> Unit,
    onInstallUpdate: (String) -> Unit,
    onLogout: () -> Unit,
) {
    val section = rememberSaveable { androidx.compose.runtime.mutableStateOf("overview") }
    if (LocalTwoPane.current) {
        ServicesTwoPane(
            section = section.value,
            onSectionChanged = { section.value = it },
            uiState = uiState,
            onRefresh = onRefresh,
            onToggleSettings = onToggleSettings,
            onSettingsDraftChange = onSettingsDraftChange,
            onSaveSettings = onSaveSettings,
            onTestWeCom = onTestWeCom,
            onStartDriveAuthorization = onStartDriveAuthorization,
            onCurrentPasswordChange = onCurrentPasswordChange,
            onNewPasswordChange = onNewPasswordChange,
            onConfirmationChange = onConfirmationChange,
            onChangePassword = onChangePassword,
            onChangeServer = onChangeServer,
            onCheckForUpdate = onCheckForUpdate,
            onDownloadUpdate = onDownloadUpdate,
            onInstallUpdate = onInstallUpdate,
            onLogout = onLogout,
        )
        return
    }
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 12.dp),
    ) {
        MediaHubSegmentedControl(
            options = serviceSectionOptions,
            selected = section.value,
            onSelected = { section.value = it },
            modifier = Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 10.dp),
        )
        uiState.errorMessage?.let { message ->
            Row(
                modifier = Modifier.fillMaxWidth().padding(bottom = 10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubIcon(imageVector = Lucide.CircleAlert, contentDescription = null, tint = MediaHubColors.Error, modifier = Modifier.size(17.dp))
                Spacer(Modifier.width(8.dp))
                MediaHubText(text = message, color = MediaHubColors.Error, fontSize = 12.sp)
            }
        }
        LazyColumn(
            modifier = Modifier.weight(1f),
            contentPadding = PaddingValues(bottom = 18.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            servicesSectionItems(
                section = section.value,
                uiState = uiState,
                onRefresh = onRefresh,
                onToggleSettings = onToggleSettings,
                onSettingsDraftChange = onSettingsDraftChange,
                onSaveSettings = onSaveSettings,
                onTestWeCom = onTestWeCom,
                onStartDriveAuthorization = onStartDriveAuthorization,
                onCurrentPasswordChange = onCurrentPasswordChange,
                onNewPasswordChange = onNewPasswordChange,
                onConfirmationChange = onConfirmationChange,
                onChangePassword = onChangePassword,
                onChangeServer = onChangeServer,
                onCheckForUpdate = onCheckForUpdate,
                onDownloadUpdate = onDownloadUpdate,
                onInstallUpdate = onInstallUpdate,
                onLogout = onLogout,
            )
        }
    }
}

@Composable
private fun ServicesTwoPane(
    section: String,
    onSectionChanged: (String) -> Unit,
    uiState: ServicesUiState,
    onRefresh: () -> Unit,
    onToggleSettings: () -> Unit,
    onSettingsDraftChange: (com.mediahub.android.core.network.ProviderSettingsUpdate) -> Unit,
    onSaveSettings: () -> Unit,
    onTestWeCom: () -> Unit,
    onStartDriveAuthorization: () -> Unit,
    onCurrentPasswordChange: (String) -> Unit,
    onNewPasswordChange: (String) -> Unit,
    onConfirmationChange: (String) -> Unit,
    onChangePassword: () -> Unit,
    onChangeServer: () -> Unit,
    onCheckForUpdate: () -> Unit,
    onDownloadUpdate: (com.mediahub.android.core.network.AndroidRelease) -> Unit,
    onInstallUpdate: (String) -> Unit,
    onLogout: () -> Unit,
) {
    MediaHubListDetail(
        detailOpen = true,
        emptyTitle = "选择一项设置",
        emptyMessage = "从左侧打开系统分区",
        emptyIcon = Lucide.Server,
        list = {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = 12.dp, vertical = 8.dp)
                    .testTag("services-section-list"),
            ) {
                MediaHubCard {
                    serviceSectionOptions.forEachIndexed { index, (key, label) ->
                        if (index > 0) MediaHubListDivider()
                        MediaHubPreferenceRow(
                            title = label,
                            summary = if (section == key) "当前分区" else null,
                            onClick = { onSectionChanged(key) },
                        )
                    }
                }
            }
        },
        detail = {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = 12.dp)
                    .testTag("services-section-detail"),
            ) {
                uiState.errorMessage?.let { message ->
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(bottom = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        MediaHubIcon(imageVector = Lucide.CircleAlert, contentDescription = null, tint = MediaHubColors.Error, modifier = Modifier.size(17.dp))
                        Spacer(Modifier.width(8.dp))
                        MediaHubText(text = message, color = MediaHubColors.Error, fontSize = 12.sp)
                    }
                }
                LazyColumn(
                    modifier = Modifier.weight(1f),
                    contentPadding = PaddingValues(bottom = 18.dp),
                    verticalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    servicesSectionItems(
                        section = section,
                        uiState = uiState,
                        onRefresh = onRefresh,
                        onToggleSettings = onToggleSettings,
                        onSettingsDraftChange = onSettingsDraftChange,
                        onSaveSettings = onSaveSettings,
                        onTestWeCom = onTestWeCom,
                        onStartDriveAuthorization = onStartDriveAuthorization,
                        onCurrentPasswordChange = onCurrentPasswordChange,
                        onNewPasswordChange = onNewPasswordChange,
                        onConfirmationChange = onConfirmationChange,
                        onChangePassword = onChangePassword,
                        onChangeServer = onChangeServer,
                        onCheckForUpdate = onCheckForUpdate,
                        onDownloadUpdate = onDownloadUpdate,
                        onInstallUpdate = onInstallUpdate,
                        onLogout = onLogout,
                    )
                }
            }
        },
    )
}

private fun LazyListScope.servicesSectionItems(
    section: String,
    uiState: ServicesUiState,
    onRefresh: () -> Unit,
    onToggleSettings: () -> Unit,
    onSettingsDraftChange: (com.mediahub.android.core.network.ProviderSettingsUpdate) -> Unit,
    onSaveSettings: () -> Unit,
    onTestWeCom: () -> Unit,
    onStartDriveAuthorization: () -> Unit,
    onCurrentPasswordChange: (String) -> Unit,
    onNewPasswordChange: (String) -> Unit,
    onConfirmationChange: (String) -> Unit,
    onChangePassword: () -> Unit,
    onChangeServer: () -> Unit,
    onCheckForUpdate: () -> Unit,
    onDownloadUpdate: (com.mediahub.android.core.network.AndroidRelease) -> Unit,
    onInstallUpdate: (String) -> Unit,
    onLogout: () -> Unit,
) {
    when (section) {
                "overview" -> {
                    item(key = "summary") {
                        MediaHubSmallTitle(text = "服务状态")
                        MediaHubCard {
                            MediaHubPreferenceRow(
                                title = serviceSummary(uiState.loading, uiState.integrations),
                                summary = "点按刷新各外部服务健康状态",
                                onClick = onRefresh,
                                end = {
                                    MediaHubIconButton(
                                        imageVector = Lucide.RefreshCw,
                                        contentDescription = "刷新服务状态",
                                        enabled = !uiState.loading,
                                        onClick = onRefresh,
                                    )
                                },
                            )
                            uiState.integrations.forEach { integration ->
                                MediaHubListDivider()
                                ServiceRow(integration)
                            }
                        }
                    }
                    uiState.statistics?.let { statistics ->
                        item(key = "statistics") { OperationalSummary(statistics) }
                    }
                    item(key = "drive-authorization") {
                        MediaHubSmallTitle(text = "115 扫码授权")
                        MediaHubCard(insideMargin = PaddingValues(16.dp)) {
                            val qrDataURL = uiState.driveAuthorization?.qrImage
                            val qrBitmap = remember(qrDataURL) { qrDataURL?.let(::decodeQRImage) }
                            if (qrBitmap != null) {
                                Box(
                                    modifier = Modifier
                                        .align(Alignment.CenterHorizontally)
                                        .background(androidx.compose.ui.graphics.Color.White, RoundedCornerShape(16.dp))
                                        .padding(14.dp),
                                    contentAlignment = Alignment.Center,
                                ) {
                                    Image(bitmap = qrBitmap, contentDescription = "115 扫码授权二维码", modifier = Modifier.size(180.dp))
                                }
                            }
                            uiState.driveAuthorization?.let { authorization ->
                                MediaHubText(
                                    text = when (authorization.state) {
                                        "confirmed" -> "授权已完成"
                                        "expired" -> "二维码已过期"
                                        else -> "等待 115 客户端扫码确认"
                                    },
                                    modifier = Modifier.padding(top = 12.dp),
                                    color = if (authorization.state == "confirmed") MediaHubColors.Source else MediaHubColors.TextMuted,
                                    fontSize = 12.sp,
                                )
                            }
                            MediaHubButton(
                                label = if (uiState.authorizingDrive) "等待扫码确认" else if (uiState.driveAuthorization?.state == "confirmed") "重新授权" else "开始扫码授权",
                                icon = Lucide.QrCode,
                                enabled = !uiState.authorizingDrive,
                                onClick = onStartDriveAuthorization,
                                modifier = Modifier.fillMaxWidth().padding(top = 12.dp),
                            )
                        }
                    }
                }
                "providers" -> {
                    item(key = "provider-settings") {
                        ProviderSettingsPanel(
                            settings = uiState.providerSettings,
                            draft = uiState.settingsDraft,
                            expanded = uiState.settingsExpanded,
                            saving = uiState.savingSettings,
                            saved = uiState.settingsSaved,
                            testing = uiState.testingWeCom,
                            tested = uiState.weComTested,
                            onToggle = onToggleSettings,
                            onDraftChange = onSettingsDraftChange,
                            onSave = onSaveSettings,
                            onTest = onTestWeCom,
                        )
                    }
                }
                else -> {
                    item(key = "password") {
                        MediaHubSmallTitle(text = "管理员密码")
                        MediaHubCard(insideMargin = PaddingValues(16.dp)) {
                            MediaHubText(text = "修改密码后会撤销其他设备会话", color = MediaHubColors.TextMuted, fontSize = 12.sp)
                            MediaHubTextField(value = uiState.currentPassword, onValueChange = onCurrentPasswordChange, placeholder = "当前密码", keyboardType = KeyboardType.Password, password = true)
                            MediaHubTextField(value = uiState.newPassword, onValueChange = onNewPasswordChange, placeholder = "新密码（至少 12 位）", keyboardType = KeyboardType.Password, password = true)
                            MediaHubTextField(value = uiState.confirmation, onValueChange = onConfirmationChange, placeholder = "确认新密码", keyboardType = KeyboardType.Password, password = true)
                            if (uiState.passwordChanged) {
                                MediaHubText(text = "密码已修改，其他设备的会话已撤销", color = MediaHubColors.Source, fontSize = 12.sp)
                            }
                            MediaHubButton(
                                label = if (uiState.changingPassword) "正在修改" else "修改密码",
                                icon = Lucide.KeyRound,
                                enabled = !uiState.changingPassword && uiState.newPassword.length >= 12 && uiState.newPassword == uiState.confirmation && uiState.currentPassword != uiState.newPassword,
                                onClick = onChangePassword,
                                modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
                            )
                        }
                    }
                    item(key = "android-update") {
                        val release = uiState.androidRelease
                        MediaHubSmallTitle(text = "应用更新")
                        MediaHubCard(insideMargin = PaddingValues(16.dp)) {
                            MediaHubText(text = release?.let { "发现 ${it.versionName}，${it.notes}" } ?: "当前已是最新版本", color = MediaHubColors.TextMuted, fontSize = 12.sp)
                            MediaHubButton(
                                label = when {
                                    uiState.downloadedUpdatePath != null -> "安装 ${release?.versionName.orEmpty()}"
                                    uiState.downloadingUpdate -> "正在下载并校验"
                                    release != null -> "下载 ${release.versionName}"
                                    uiState.checkingUpdate -> "正在检查"
                                    else -> "检查更新"
                                },
                                icon = if (release != null) Lucide.Download else Lucide.RefreshCw,
                                enabled = !uiState.checkingUpdate && !uiState.downloadingUpdate,
                                onClick = {
                                    val path = uiState.downloadedUpdatePath
                                    if (path != null) onInstallUpdate(path)
                                    else if (release != null) onDownloadUpdate(release)
                                    else onCheckForUpdate()
                                },
                                modifier = Modifier.fillMaxWidth().padding(top = 10.dp),
                            )
                        }
                    }
                    item(key = "server") {
                        MediaHubSmallTitle(text = "连接与会话")
                        MediaHubCard {
                            MediaHubPreferenceRow(
                                title = "更换服务器",
                                summary = "切换到另一套 Media Hub 部署",
                                onClick = onChangeServer,
                                start = {
                                    MediaHubIcon(Lucide.Server, null, Modifier.size(18.dp).padding(end = 12.dp), MediaHubColors.TextSecondary)
                                },
                            )
                            MediaHubListDivider()
                            MediaHubPreferenceRow(
                                title = "退出登录",
                                summary = "撤销当前设备会话",
                                onClick = onLogout,
                                start = {
                                    MediaHubIcon(Lucide.LogOut, null, Modifier.size(18.dp).padding(end = 12.dp), MediaHubColors.Error)
                                },
                            )
                        }
                    }
                }
    }
}

@Composable
private fun OperationalSummary(statistics: OperationalStatistics) {
    MediaHubSmallTitle(text = "运营摘要")
    MediaHubCard {
        MediaHubPreferenceRow(title = "进行中", summary = "${statistics.transfersActive}")
        MediaHubListDivider()
        MediaHubPreferenceRow(
            title = "需处理",
            summary = "${statistics.transfersNeedsAttention + statistics.commandsNeedsAttention + statistics.notificationsNeedsAttention}",
        )
        MediaHubListDivider()
        MediaHubPreferenceRow(title = "启用订阅", summary = "${statistics.subscriptionsEnabled}")
        MediaHubListDivider()
        MediaHubPreferenceRow(title = "失败运行", summary = "${statistics.runsFailed}")
    }
}

@Composable
private fun ServiceRow(integration: IntegrationHealth) {
    MediaHubPreferenceRow(
        title = integration.label,
        summary = integration.detail,
        start = {
            Spacer(
                Modifier
                    .padding(end = 12.dp)
                    .size(8.dp)
                    .background(statusColor(integration.status), CircleShape),
            )
        },
        end = {
            MediaHubText(text = statusLabel(integration.status), color = statusColor(integration.status), fontSize = 12.sp, fontWeight = FontWeight.Medium)
        },
    )
}

private fun installUpdate(context: android.content.Context, file: java.io.File): Boolean {
    if (!context.packageManager.canRequestPackageInstalls()) {
        context.startActivity(Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES).apply {
            data = android.net.Uri.parse("package:${context.packageName}")
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        })
        return false
    }
    val uri = FileProvider.getUriForFile(context, "${context.packageName}.files", file)
    context.startActivity(Intent(Intent.ACTION_VIEW).apply {
        setDataAndType(uri, "application/vnd.android.package-archive")
        addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
    })
    return true
}

private fun decodeQRImage(dataURL: String): androidx.compose.ui.graphics.ImageBitmap? {
    val encoded = dataURL.substringAfter("base64,", "")
    if (encoded.isEmpty()) return null
    return runCatching {
        val bytes = Base64.decode(encoded, Base64.DEFAULT)
        BitmapFactory.decodeByteArray(bytes, 0, bytes.size)?.asImageBitmap()
    }.getOrNull()
}

private fun serviceSummary(loading: Boolean, integrations: List<IntegrationHealth>): String {
    if (loading) return "正在检查服务"
    val healthy = integrations.count { it.status == "healthy" }
    if (healthy > 0) return "$healthy / ${integrations.size} 在线"
    return if (integrations.any { it.status != "unconfigured" }) "已配置服务当前不可用" else "尚未配置服务"
}

private fun statusLabel(status: String): String = when (status) {
    "healthy" -> "在线"
    "degraded" -> "受限"
    "unavailable" -> "离线"
    else -> "未配置"
}

private fun statusColor(status: String) = when (status) {
    "healthy" -> MediaHubColors.Source
    "degraded" -> MediaHubColors.Warning
    "unavailable" -> MediaHubColors.Error
    else -> MediaHubColors.TextMuted
}
