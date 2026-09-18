package com.mediahub.android.feature.services

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.ChevronDown
import com.composables.icons.lucide.ChevronUp
import com.composables.icons.lucide.Film
import com.composables.icons.lucide.KeyRound
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Save
import com.composables.icons.lucide.ServerCog
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubCheckboxRow
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubPreferenceRow
import com.mediahub.android.core.designsystem.MediaHubSegmentedControl
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTextField
import com.mediahub.android.core.network.CheckInSettings
import com.mediahub.android.core.network.ProviderSettings
import com.mediahub.android.core.network.ProviderSettingsUpdate
import com.mediahub.android.core.network.SecretStatus
import com.mediahub.android.core.network.SecretUpdate
import com.mediahub.android.core.network.WorkflowTargetSettings

@Composable
internal fun ProviderSettingsPanel(
    settings: ProviderSettings?,
    draft: ProviderSettingsUpdate?,
    saving: Boolean,
    saved: Boolean,
    testing: Boolean,
    tested: Boolean,
    onDraftChange: (ProviderSettingsUpdate) -> Unit,
    onSave: () -> Unit,
    onTest: () -> Unit,
) {
    Column(
        modifier = Modifier.fillMaxWidth().padding(bottom = 8.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        MediaHubSmallTitle(text = "服务设置")
        if (settings == null || draft == null) {
            MediaHubText("正在读取加密设置", color = MediaHubColors.TextMuted, fontSize = 12.sp)
            return@Column
        }

        SettingsSection("核心媒体服务", "TMDB、网盘同步与 Emby 连接配置", Lucide.Film, defaultExpanded = true) {
            LabeledField("TMDB API 地址", draft.tmdbBaseUrl) { onDraftChange(draft.copy(tmdbBaseUrl = it)) }
            SecretField("TMDB Read Access Token", settings.tmdbAccessToken, draft.tmdbAccessToken) {
                onDraftChange(draft.copy(tmdbAccessToken = it))
            }
            LabeledField("Assrt API 地址", draft.assrtBaseUrl) { onDraftChange(draft.copy(assrtBaseUrl = it)) }
            SecretField("Assrt Token", settings.assrtToken, draft.assrtToken) {
                onDraftChange(draft.copy(assrtToken = it))
            }
            MediaHubText("在 assrt.net 用户面板申请 32 位 Token。中文搜索优先走 Assrt。", color = MediaHubColors.TextMuted, fontSize = 12.sp)
            MediaHubText("字幕服务由 assrt.net 提供", color = MediaHubColors.TextMuted, fontSize = 12.sp)
            LabeledField("Emby 地址", draft.embyBaseUrl) { onDraftChange(draft.copy(embyBaseUrl = it)) }
            SecretField("Emby API Key", settings.embyApiKey, draft.embyApiKey) {
                onDraftChange(draft.copy(embyApiKey = it))
            }
            LabeledField("Emby 用户 ID", draft.embyUserId) { onDraftChange(draft.copy(embyUserId = it)) }
            SecretField("Emby 用户密码", settings.embyPassword, draft.embyPassword) {
                onDraftChange(draft.copy(embyPassword = it))
            }
            MediaHubText("Media Hub 从 NAS 访问 Emby，请填局域网地址。公网域名会在容器里回环，容易变成 502。", color = MediaHubColors.TextMuted, fontSize = 12.sp)
            MediaHubText("从 Emby 删除媒体需要该用户的登录密码；仅 API Key 无法删除。", color = MediaHubColors.TextMuted, fontSize = 12.sp)
            LabeledField("共享 Emby 地址", draft.sharedEmby.baseUrl) {
                onDraftChange(draft.copy(sharedEmby = draft.sharedEmby.copy(baseUrl = it)))
            }
            LabeledField("共享 Emby 用户名", draft.sharedEmby.username) {
                onDraftChange(draft.copy(sharedEmby = draft.sharedEmby.copy(username = it)))
            }
            SecretField("共享 Emby 密码", settings.sharedEmby.password, draft.sharedEmby.password) {
                onDraftChange(draft.copy(sharedEmby = draft.sharedEmby.copy(password = it)))
            }
            LabeledField("共享 Emby 代理（可选）", draft.sharedEmby.proxyUrl) {
                onDraftChange(draft.copy(sharedEmby = draft.sharedEmby.copy(proxyUrl = it)))
            }
            MediaHubText("只读第二路目录，不替换 NAS 上的 Emby。播放由播放器直连远程 Emby。", color = MediaHubColors.TextMuted, fontSize = 12.sp)
            MediaHubText("115 网盘请在「概览」页扫码授权，无需开放平台开发者账号。", color = MediaHubColors.TextMuted, fontSize = 12.sp)
        }

        SettingsSection("工作流目录映射", "转存目录、STRM 写入路径与 Emby 库。播放地址写到 Media Hub /115/url/", Lucide.ServerCog) {
            LabeledField("STRM 基址", draft.workflow.strmBaseUrl) {
                onDraftChange(draft.copy(workflow = draft.workflow.copy(strmBaseUrl = it, syncMode = "builtin")))
            }
            LabeledField("STRM 根挂载", draft.workflow.strmRootMount) {
                onDraftChange(draft.copy(workflow = draft.workflow.copy(strmRootMount = it, syncMode = "builtin")))
            }
            WorkflowTargetEditor("电影", draft.workflow.movie) {
                onDraftChange(draft.copy(workflow = draft.workflow.copy(movie = it, syncMode = "builtin")))
            }
            WorkflowTargetEditor("剧集", draft.workflow.series) {
                onDraftChange(draft.copy(workflow = draft.workflow.copy(series = it, syncMode = "builtin")))
            }
            WorkflowTargetEditor("成人影视", draft.workflow.adult) {
                onDraftChange(draft.copy(workflow = draft.workflow.copy(adult = it, syncMode = "builtin")))
            }
            MediaHubText("成人导入只进这一组目录。Emby 库 ID 可填 adult。", color = MediaHubColors.TextMuted, fontSize = 12.sp)
        }

        SettingsSection("消息通知", "企业微信推送通知与消息卡片", Lucide.ServerCog) {
            WeComModePicker(draft.wecom.sendMode) { nextMode ->
                onDraftChange(draft.copy(wecom = draft.wecom.copy(
                    sendMode = nextMode, agentId = 0, toUser = if (nextMode == "app") "@all" else "", chatId = "",
                )))
            }
            LabeledField("企业微信 API 地址", draft.wecom.baseUrl) { onDraftChange(draft.copy(wecom = draft.wecom.copy(baseUrl = it))) }
            LabeledField("企业微信 Corp ID", draft.wecom.corpId) { onDraftChange(draft.copy(wecom = draft.wecom.copy(corpId = it))) }
            SecretField("企业微信 Secret", settings.wecom.secret, draft.wecom.secret) {
                onDraftChange(draft.copy(wecom = draft.wecom.copy(secret = it)))
            }
            if (draft.wecom.sendMode == "app") {
                LabeledField("企业微信 Agent ID", draft.wecom.agentId.takeIf { it > 0 }?.toString().orEmpty(), KeyboardType.Number) {
                    onDraftChange(draft.copy(wecom = draft.wecom.copy(agentId = it.toLongOrNull() ?: 0)))
                }
                LabeledField("企业微信接收人", draft.wecom.toUser) { onDraftChange(draft.copy(wecom = draft.wecom.copy(toUser = it))) }
            } else {
                LabeledField("企业微信 Chat ID", draft.wecom.chatId) { onDraftChange(draft.copy(wecom = draft.wecom.copy(chatId = it))) }
            }
            MediaHubButton(
                label = if (testing) "正在发送…" else "发送测试通知",
                icon = Lucide.ServerCog,
                enabled = settings.wecom.secret.configured && !saving && !testing,
                onClick = onTest,
                modifier = Modifier.fillMaxWidth(),
            )
            if (tested) MediaHubText("测试通知已提交", color = MediaHubColors.Source, fontSize = 12.sp)
        }

        SettingsSection("资源搜索源", "内置媒体搜索适配器配置", Lucide.ServerCog) {
            MediaHubText("蜜柑和 Sidhub 使用内置匿名适配器；帧影使用站点账号；聚影可选择网页登录或开发者 API。盘搜只收 115 分享，TG 频道配在盘搜服务里。", color = MediaHubColors.TextMuted, fontSize = 12.sp)
            settings.sources.forEachIndexed { index, source ->
                val item = draft.sources[index]
                val authMode = if (source.id == "juying") item.authMode.ifBlank { "web" } else ""
                val modeChanged = source.id == "juying" && authMode != source.authMode
                val accountLabel = when (source.id) { "framehdr" -> "用户名"; "juying" -> if (authMode == "web") "用户名" else "App ID"; else -> null }
                val secretLabel = when (source.id) { "framehdr" -> "密码"; "dian" -> "OpenAPI Key"; "juying" -> if (authMode == "web") "密码" else "API Key"; "mikan", "sidhub" -> null; "pansou" -> "Bearer Token（可选）"; else -> "Bearer Token" }
                MediaHubText(source.label, color = MediaHubColors.TextStrong, fontSize = 12.sp, fontWeight = FontWeight.Medium)
                if (source.id == "juying") AuthModePicker(authMode) { nextMode ->
                    onDraftChange(draft.copy(sources = draft.sources.mapIndexed { itemIndex, current ->
                        if (itemIndex == index) current.copy(authMode = nextMode, account = "", token = SecretUpdate(clear = source.token.configured)) else current
                    }))
                }
                LabeledField("${source.label} 适配器地址", item.baseUrl) { value ->
                    onDraftChange(draft.copy(sources = draft.sources.mapIndexed { itemIndex, current -> if (itemIndex == index) current.copy(baseUrl = value) else current }))
                }
                if (accountLabel != null) LabeledField("${source.label} $accountLabel", item.account) { value ->
                    onDraftChange(draft.copy(sources = draft.sources.mapIndexed { itemIndex, current -> if (itemIndex == index) current.copy(account = value.take(200)) else current }))
                }
                if (secretLabel != null) SecretField("${source.label} $secretLabel", source.token, item.token, resetRequired = modeChanged) { value ->
                    onDraftChange(draft.copy(sources = draft.sources.mapIndexed { itemIndex, current -> if (itemIndex == index) current.copy(token = value) else current }))
                }
            }
        }

        SettingsSection("每日签到", "按北京时间每天签一次；关闭后仍可在概览手动签到", Lucide.ServerCog) {
            MediaHubCheckboxRow(
                title = "自动签到",
                checked = draft.checkIn.enabled,
                onCheckedChange = { onDraftChange(draft.copy(checkIn = draft.checkIn.copy(enabled = it))) },
            )
            LabeledField("签到时间（北京时间 HH:mm）", formatCheckInTime(draft.checkIn.hour, draft.checkIn.minute)) { value ->
                val (hour, minute) = parseCheckInTime(value)
                onDraftChange(draft.copy(checkIn = draft.checkIn.copy(hour = hour, minute = minute)))
            }
            MediaHubCheckboxRow(
                title = "帧影",
                checked = draft.checkIn.sources.contains("framehdr"),
                enabled = draft.checkIn.enabled,
                onCheckedChange = { onDraftChange(draft.copy(checkIn = draft.checkIn.toggleSource("framehdr", it))) },
            )
            MediaHubCheckboxRow(
                title = "聚影",
                checked = draft.checkIn.sources.contains("juying"),
                enabled = draft.checkIn.enabled,
                onCheckedChange = { onDraftChange(draft.copy(checkIn = draft.checkIn.toggleSource("juying", it))) },
            )
        }

        if (saved) MediaHubText("设置已加密保存并立即应用", color = MediaHubColors.Source, fontSize = 12.sp)
        MediaHubButton(
            label = if (saving) "正在保存" else "保存服务设置",
            icon = Lucide.Save,
            enabled = !saving,
            onClick = onSave,
            modifier = Modifier.fillMaxWidth(),
        )
        Row(verticalAlignment = Alignment.CenterVertically) {
            MediaHubIcon(imageVector = Lucide.KeyRound, contentDescription = null, tint = MediaHubColors.TextMuted)
            Spacer(Modifier.width(7.dp))
            MediaHubText("密钥不回显；留空保持，显式选择后才清除。", color = MediaHubColors.TextMuted, fontSize = 12.sp)
        }
    }
}

@Composable
private fun SettingsSection(
    title: String,
    subtitle: String,
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    defaultExpanded: Boolean = false,
    content: @Composable () -> Unit,
) {
    var expanded by remember { mutableStateOf(defaultExpanded) }
    MediaHubCard(elevated = false) {
        MediaHubPreferenceRow(
            title = title,
            summary = subtitle,
            onClick = { expanded = !expanded },
            start = {
                MediaHubIcon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = MediaHubColors.Accent,
                    modifier = Modifier.size(18.dp).padding(end = 12.dp),
                )
            },
            end = {
                MediaHubIcon(
                    imageVector = if (expanded) Lucide.ChevronUp else Lucide.ChevronDown,
                    contentDescription = if (expanded) "收起" else "展开",
                    tint = MediaHubColors.TextMuted,
                )
            },
        )
        if (expanded) {
            MediaHubListDivider()
            Column(
                modifier = Modifier.padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(9.dp),
            ) {
                content()
            }
        }
    }
}

@Composable
private fun WeComModePicker(value: String, onChange: (String) -> Unit) {
    MediaHubSegmentedControl(
        options = listOf("app" to "自建应用", "appchat" to "AppChat"),
        selected = value,
        onSelected = onChange,
        role = Role.RadioButton,
        raised = false,
    )
}

@Composable
private fun AuthModePicker(value: String, onChange: (String) -> Unit) {
    MediaHubSegmentedControl(
        options = listOf("web" to "网页登录", "developer" to "开发者 API"),
        selected = value,
        onSelected = onChange,
        role = Role.RadioButton,
        raised = false,
    )
}

@Composable
private fun LabeledField(
    label: String,
    value: String,
    keyboardType: KeyboardType = KeyboardType.Text,
    password: Boolean = false,
    onChange: (String) -> Unit,
) {
    Column(verticalArrangement = Arrangement.spacedBy(5.dp)) {
        MediaHubText(label, color = MediaHubColors.TextMuted, fontSize = 12.sp)
        MediaHubTextField(value, onChange, label, Modifier.fillMaxWidth(), keyboardType = keyboardType, password = password)
    }
}

@Composable
private fun SecretField(label: String, status: SecretStatus, value: SecretUpdate, resetRequired: Boolean = false, onChange: (SecretUpdate) -> Unit) {
    LabeledField(
        label = "$label · ${if (resetRequired) "切换模式后需重新填写" else if (status.configured) "已保存，留空保持" else "尚未保存"}",
        value = value.value,
        keyboardType = KeyboardType.Password,
        password = true,
    ) { onChange(value.copy(value = it.take(4096), clear = false)) }
    if (status.configured) {
        MediaHubCheckboxRow(
            title = if (value.clear) "将清除已保存密钥" else "保留已保存密钥",
            checked = value.clear,
            onCheckedChange = { onChange(SecretUpdate(clear = it)) },
        )
    }
}

@Composable
private fun WorkflowTargetEditor(label: String, target: WorkflowTargetSettings, onChange: (WorkflowTargetSettings) -> Unit) {
    MediaHubText(label, color = MediaHubColors.TextStrong, fontSize = 12.sp, fontWeight = FontWeight.Medium)
    LabeledField("$label 115 目标目录 ID", target.destinationId) { onChange(target.copy(destinationId = it)) }
    LabeledField("$label STRM 目标路径", target.qMediaSyncTargetPath) {
        onChange(target.copy(qMediaSyncTargetPath = it))
    }
    LabeledField("$label Emby 媒体库 ID", target.embyLibraryId) { onChange(target.copy(embyLibraryId = it)) }
}

private fun formatCheckInTime(hour: Int, minute: Int) = "%02d:%02d".format(hour, minute)

private fun parseCheckInTime(value: String): Pair<Int, Int> {
    val parts = value.split(":")
    val hour = parts.getOrNull(0)?.toIntOrNull()?.coerceIn(0, 23) ?: 0
    val minute = parts.getOrNull(1)?.toIntOrNull()?.coerceIn(0, 59) ?: 0
    return hour to minute
}

private fun CheckInSettings.toggleSource(id: String, enabled: Boolean): CheckInSettings {
    val next = if (enabled) sources.filterNot { it == id } + id else sources.filterNot { it == id }
    return copy(sources = next)
}
