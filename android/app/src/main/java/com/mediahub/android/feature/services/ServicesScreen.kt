package com.mediahub.android.feature.services

import android.content.Intent
import android.provider.Settings
import androidx.core.content.FileProvider
import android.graphics.BitmapFactory
import android.util.Base64
import androidx.compose.foundation.background
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
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
import com.composables.icons.lucide.Settings2
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTextField
import com.mediahub.android.core.network.IntegrationHealth
import com.mediahub.android.core.network.OperationalStatistics

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
private fun ServicesScreen(
    uiState: ServicesUiState,
    onRefresh: () -> Unit,
    onToggleSettings: () -> Unit,
    onSettingsDraftChange: (com.mediahub.android.core.network.ProviderSettingsUpdate) -> Unit,
    onSaveSettings: () -> Unit,
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
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MediaHubColors.Canvas)
            .statusBarsPadding()
            .navigationBarsPadding()
            .padding(horizontal = 20.dp)
            .padding(bottom = 64.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(top = 10.dp, bottom = 22.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                MediaHubIcon(
                    imageVector = Lucide.Settings2,
                    contentDescription = null,
                    tint = MediaHubColors.Canvas,
                    modifier = Modifier
                        .size(28.dp)
                        .background(MediaHubColors.Accent, RoundedCornerShape(8.dp))
                        .padding(6.dp),
                )
                Spacer(Modifier.width(10.dp))
                MediaHubText(text = "服务状态", fontSize = 20.sp, fontWeight = FontWeight.SemiBold)
            }
            Row {
                MediaHubIconButton(
                    imageVector = Lucide.RefreshCw,
                    contentDescription = "刷新服务状态",
                    enabled = !uiState.loading,
                    onClick = onRefresh,
                )
                MediaHubIconButton(
                    imageVector = Lucide.LogOut,
                    contentDescription = "退出登录",
                    onClick = onLogout,
                )
            }
        }
        uiState.errorMessage?.let { message ->
            Row(
                modifier = Modifier.fillMaxWidth().padding(bottom = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubIcon(
                    imageVector = Lucide.CircleAlert,
                    contentDescription = null,
                    tint = MediaHubColors.Error,
                    modifier = Modifier.size(17.dp),
                )
                Spacer(Modifier.width(8.dp))
                MediaHubText(text = message, color = MediaHubColors.Error, fontSize = 12.sp)
            }
        }
        LazyColumn(
            modifier = Modifier.weight(1f),
            contentPadding = PaddingValues(bottom = 16.dp),
            verticalArrangement = Arrangement.spacedBy(9.dp),
        ) {
            item(key = "summary") {
                MediaHubText(
                    text = serviceSummary(uiState.loading, uiState.integrations),
                    modifier = Modifier.padding(bottom = 6.dp),
                    color = MediaHubColors.TextMuted,
                    fontSize = 11.sp,
                )
            }
            items(uiState.integrations, key = { it.id }) { integration -> ServiceRow(integration) }
            item(key = "password") {
                Column(
                    modifier = Modifier.fillMaxWidth().padding(top = 14.dp, bottom = 8.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        MediaHubIcon(imageVector = Lucide.KeyRound, contentDescription = null, tint = MediaHubColors.Accent, modifier = Modifier.size(18.dp))
                        Spacer(Modifier.width(9.dp))
                        MediaHubText(text = "管理员密码", fontWeight = FontWeight.SemiBold)
                    }
                    MediaHubText(text = "修改当前账户密码，并撤销其他设备会话", color = MediaHubColors.TextMuted, fontSize = 11.sp)
                    MediaHubTextField(
                        value = uiState.currentPassword,
                        onValueChange = onCurrentPasswordChange,
                        placeholder = "当前密码",
                        keyboardType = KeyboardType.Password,
                        password = true,
                    )
                    MediaHubTextField(
                        value = uiState.newPassword,
                        onValueChange = onNewPasswordChange,
                        placeholder = "新密码（至少 12 位）",
                        keyboardType = KeyboardType.Password,
                        password = true,
                    )
                    MediaHubTextField(
                        value = uiState.confirmation,
                        onValueChange = onConfirmationChange,
                        placeholder = "确认新密码",
                        keyboardType = KeyboardType.Password,
                        password = true,
                    )
                    if (uiState.passwordChanged) {
                        MediaHubText(text = "密码已修改，其他设备的会话已撤销", color = MediaHubColors.Source, fontSize = 11.sp)
                    }
                    MediaHubButton(
                        label = if (uiState.changingPassword) "正在修改" else "修改密码",
                        icon = Lucide.KeyRound,
                        enabled = !uiState.changingPassword && uiState.newPassword.length >= 12 && uiState.newPassword == uiState.confirmation && uiState.currentPassword != uiState.newPassword,
                        onClick = onChangePassword,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
            uiState.statistics?.let { statistics ->
                item(key = "statistics") { OperationalSummary(statistics) }
            }
            item(key = "provider-settings") {
                ProviderSettingsPanel(
                    settings = uiState.providerSettings,
                    draft = uiState.settingsDraft,
                    expanded = uiState.settingsExpanded,
                    saving = uiState.savingSettings,
                    saved = uiState.settingsSaved,
                    onToggle = onToggleSettings,
                    onDraftChange = onSettingsDraftChange,
                    onSave = onSaveSettings,
                )
            }
            item(key = "drive-authorization") {
                Column(
                    modifier = Modifier.fillMaxWidth().padding(top = 18.dp, bottom = 8.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                ) {
                    Row(modifier = Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                        MediaHubIcon(imageVector = Lucide.QrCode, contentDescription = null, tint = MediaHubColors.Accent, modifier = Modifier.size(18.dp))
                        Spacer(Modifier.width(9.dp))
                        MediaHubText(text = "115 扫码授权", fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
                    }
                    uiState.driveAuthorization?.qrImage?.let { dataURL ->
                        remember(dataURL) { decodeQRImage(dataURL) }?.let { image ->
                            Image(bitmap = image, contentDescription = "115 扫码授权二维码", modifier = Modifier.size(180.dp))
                        }
                    }
                    uiState.driveAuthorization?.let { authorization ->
                        MediaHubText(
                            text = when (authorization.state) { "confirmed" -> "授权已完成"; "expired" -> "二维码已过期"; else -> "等待 115 客户端扫码确认" },
                            color = if (authorization.state == "confirmed") MediaHubColors.Source else MediaHubColors.TextMuted,
                            fontSize = 11.sp,
                        )
                    }
                    MediaHubButton(
                        label = if (uiState.authorizingDrive) "等待扫码确认" else if (uiState.driveAuthorization?.state == "confirmed") "重新授权" else "开始扫码授权",
                        icon = Lucide.QrCode,
                        enabled = !uiState.authorizingDrive,
                        onClick = onStartDriveAuthorization,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
            item(key = "android-update") {
                val release = uiState.androidRelease
                Column(
                    modifier = Modifier.fillMaxWidth().padding(top = 18.dp, bottom = 8.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        MediaHubIcon(imageVector = Lucide.Download, contentDescription = null, tint = MediaHubColors.Accent, modifier = Modifier.size(18.dp))
                        Spacer(Modifier.width(9.dp))
                        MediaHubText(text = "应用更新", fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
                    }
                    MediaHubText(
                        text = release?.let { "发现 ${it.versionName}，${it.notes}" } ?: "当前已是最新版本",
                        color = MediaHubColors.TextMuted,
                        fontSize = 11.sp,
                    )
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
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
            item(key = "server") {
                Column(
                    modifier = Modifier.fillMaxWidth().padding(top = 18.dp, bottom = 8.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    MediaHubText(text = "私有服务器", fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
                    MediaHubButton(
                        label = "更换服务器",
                        icon = Lucide.Server,
                        onClick = onChangeServer,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
        }
    }
}

@Composable
private fun OperationalSummary(statistics: OperationalStatistics) {
    Column(
        modifier = Modifier.fillMaxWidth().padding(top = 16.dp, bottom = 8.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        MediaHubText(text = "运营摘要", color = MediaHubColors.TextStrong, fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
        Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Metric("进行中", statistics.transfersActive, Modifier.weight(1f))
            Metric("需处理", statistics.transfersNeedsAttention + statistics.commandsNeedsAttention + statistics.notificationsNeedsAttention, Modifier.weight(1f))
            Metric("启用订阅", statistics.subscriptionsEnabled, Modifier.weight(1f))
            Metric("失败运行", statistics.runsFailed, Modifier.weight(1f))
        }
    }
}

@Composable
private fun Metric(label: String, value: Int, modifier: Modifier) {
    Column(modifier = modifier.background(MediaHubColors.Surface, RoundedCornerShape(7.dp)).padding(10.dp)) {
        MediaHubText(text = value.toString(), color = MediaHubColors.TextStrong, fontSize = 17.sp, fontWeight = FontWeight.SemiBold)
        MediaHubText(text = label, modifier = Modifier.padding(top = 3.dp), color = MediaHubColors.TextMuted, fontSize = 9.sp)
    }
}

@Composable
private fun ServiceRow(integration: IntegrationHealth) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(MediaHubColors.Surface, RoundedCornerShape(8.dp))
            .padding(15.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Spacer(
            Modifier
                .size(9.dp)
                .background(statusColor(integration.status), CircleShape),
        )
        Spacer(Modifier.width(11.dp))
        Column(Modifier.weight(1f)) {
            MediaHubText(text = integration.label, fontSize = 13.sp, fontWeight = FontWeight.Medium)
            MediaHubText(
                text = integration.detail,
                modifier = Modifier.padding(top = 4.dp),
                color = MediaHubColors.TextMuted,
                fontSize = 10.sp,
            )
        }
        MediaHubText(text = statusLabel(integration.status), color = statusColor(integration.status), fontSize = 10.sp)
    }
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
