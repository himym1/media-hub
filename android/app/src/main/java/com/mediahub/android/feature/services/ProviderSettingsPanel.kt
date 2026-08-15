package com.mediahub.android.feature.services

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.ChevronDown
import com.composables.icons.lucide.ChevronUp
import com.composables.icons.lucide.KeyRound
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Save
import com.composables.icons.lucide.ServerCog
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTextField
import com.mediahub.android.core.network.ProviderSettings
import com.mediahub.android.core.network.ProviderSettingsUpdate
import com.mediahub.android.core.network.SecretStatus
import com.mediahub.android.core.network.SecretUpdate
import com.mediahub.android.core.network.WorkflowTargetSettings

@Composable
internal fun ProviderSettingsPanel(
    settings: ProviderSettings?,
    draft: ProviderSettingsUpdate?,
    expanded: Boolean,
    saving: Boolean,
    saved: Boolean,
    testing: Boolean,
    tested: Boolean,
    onToggle: () -> Unit,
    onDraftChange: (ProviderSettingsUpdate) -> Unit,
    onSave: () -> Unit,
    onTest: () -> Unit,
) {
    Column(
        modifier = Modifier.fillMaxWidth().padding(top = 18.dp, bottom = 8.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            MediaHubIcon(imageVector = Lucide.ServerCog, contentDescription = null, tint = MediaHubColors.Accent)
            Spacer(Modifier.width(9.dp))
            MediaHubText("服务设置", fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
        }
        MediaHubButton(
            label = if (expanded) "收起设置" else "编辑服务设置",
            icon = if (expanded) Lucide.ChevronUp else Lucide.ChevronDown,
            onClick = onToggle,
            modifier = Modifier.fillMaxWidth(),
        )
        if (!expanded) return@Column
        if (settings == null || draft == null) {
            MediaHubText("正在读取加密设置", color = MediaHubColors.TextMuted, fontSize = 12.sp)
            return@Column
        }

        SettingsSection("核心服务") {
            LabeledField("TMDB API 地址", draft.tmdbBaseUrl) { onDraftChange(draft.copy(tmdbBaseUrl = it)) }
            SecretField("TMDB Read Access Token", settings.tmdbAccessToken, draft.tmdbAccessToken) {
                onDraftChange(draft.copy(tmdbAccessToken = it))
            }
            LabeledField("QMediaSync 地址", draft.qmediaSyncBaseUrl) { onDraftChange(draft.copy(qmediaSyncBaseUrl = it)) }
            SecretField("QMediaSync API Key", settings.qmediaSyncApiKey, draft.qmediaSyncApiKey) {
                onDraftChange(draft.copy(qmediaSyncApiKey = it))
            }
            LabeledField("Emby 地址", draft.embyBaseUrl) { onDraftChange(draft.copy(embyBaseUrl = it)) }
            SecretField("Emby API Key", settings.embyApiKey, draft.embyApiKey) {
                onDraftChange(draft.copy(embyApiKey = it))
            }
            LabeledField("Emby 用户 ID", draft.embyUserId) { onDraftChange(draft.copy(embyUserId = it)) }
            MediaHubText("用 115 App 在本页上方扫码授权。不需要开放平台开发者账号。", color = MediaHubColors.TextMuted, fontSize = 12.sp)
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
                label = if (testing) "正在发送" else "测试已保存配置",
                icon = Lucide.ServerCog,
                enabled = settings.wecom.secret.configured && !saving && !testing,
                onClick = onTest,
                modifier = Modifier.fillMaxWidth(),
            )
            if (tested) MediaHubText("测试通知已提交", color = MediaHubColors.Source, fontSize = 12.sp)
        }

        SettingsSection("工作流目标") {
            LabeledField("QMediaSync Account ID", draft.workflow.qMediaSyncAccountId.toString(), KeyboardType.Number) {
                onDraftChange(draft.copy(workflow = draft.workflow.copy(qMediaSyncAccountId = it.toIntOrNull() ?: 0)))
            }
            WorkflowTargetEditor("电影", draft.workflow.movie) {
                onDraftChange(draft.copy(workflow = draft.workflow.copy(movie = it)))
            }
            WorkflowTargetEditor("剧集", draft.workflow.series) {
                onDraftChange(draft.copy(workflow = draft.workflow.copy(series = it)))
            }
        }

        SettingsSection("原生资源源") {
            MediaHubText("蜜柑和 Sidhub 使用内置匿名适配器；帧影使用站点账号；聚影可选择网页登录或开发者 API。癫影当前仍使用合同适配器。", color = MediaHubColors.TextMuted, fontSize = 12.sp)
            settings.sources.forEachIndexed { index, source ->
                val item = draft.sources[index]
                val authMode = if (source.id == "juying") item.authMode.ifBlank { "web" } else ""
                val modeChanged = source.id == "juying" && authMode != source.authMode
                val accountLabel = when (source.id) { "framehdr" -> "用户名"; "juying" -> if (authMode == "web") "用户名" else "App ID"; else -> null }
                val secretLabel = when (source.id) { "framehdr" -> "密码"; "dian" -> "OpenAPI Key"; "juying" -> if (authMode == "web") "密码" else "API Key"; "mikan", "sidhub" -> null; else -> "Bearer Token" }
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
private fun SettingsSection(title: String, content: @Composable () -> Unit) {
    var expanded by remember { mutableStateOf(title == "核心服务") }
    Column(
        modifier = Modifier.fillMaxWidth().background(MediaHubColors.Surface, RoundedCornerShape(8.dp)).padding(12.dp),
        verticalArrangement = Arrangement.spacedBy(9.dp),
    ) {
        MediaHubButton(
            label = if (expanded) "$title · 收起" else title,
            icon = if (expanded) Lucide.ChevronUp else Lucide.ChevronDown,
            onClick = { expanded = !expanded },
            modifier = Modifier.fillMaxWidth(),
        )
        if (expanded) content()
    }
}

@Composable
private fun WeComModePicker(value: String, onChange: (String) -> Unit) {
    ModePicker(value, listOf("app" to "自建应用", "appchat" to "AppChat"), onChange)
}

@Composable
private fun AuthModePicker(value: String, onChange: (String) -> Unit) {
    ModePicker(value, listOf("web" to "网页登录", "developer" to "开发者 API"), onChange)
}

@Composable
private fun ModePicker(value: String, options: List<Pair<String, String>>, onChange: (String) -> Unit) {
    Row(
        modifier = Modifier.fillMaxWidth().height(38.dp).clip(RoundedCornerShape(7.dp)).background(MediaHubColors.SurfaceInput),
    ) {
        options.forEach { (mode, label) ->
            val selected = value == mode
            Box(
                modifier = Modifier.weight(1f).selectable(selected = selected, role = Role.RadioButton) { onChange(mode) }
                    .background(if (selected) MediaHubColors.SurfaceSelected else MediaHubColors.SurfaceInput).padding(vertical = 10.dp),
                contentAlignment = Alignment.Center,
            ) {
                MediaHubText(label, color = if (selected) MediaHubColors.Accent else MediaHubColors.TextMuted, fontSize = 12.sp)
            }
        }
    }
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
        Row(
            modifier = Modifier.fillMaxWidth().toggleable(value.clear, role = Role.Checkbox) { onChange(SecretUpdate(clear = it)) }
                .background(MediaHubColors.SurfaceInput, RoundedCornerShape(7.dp)).padding(11.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(if (value.clear) "将清除已保存密钥" else "保留已保存密钥", color = if (value.clear) MediaHubColors.Error else MediaHubColors.TextMuted, fontSize = 12.sp)
        }
    }
}

@Composable
private fun WorkflowTargetEditor(label: String, target: WorkflowTargetSettings, onChange: (WorkflowTargetSettings) -> Unit) {
    MediaHubText(label, color = MediaHubColors.TextStrong, fontSize = 12.sp, fontWeight = FontWeight.Medium)
    LabeledField("$label 115 目标目录 ID", target.destinationId) { onChange(target.copy(destinationId = it)) }
    LabeledField("$label QMediaSync 目标路径", target.qMediaSyncTargetPath) { onChange(target.copy(qMediaSyncTargetPath = it)) }
    LabeledField("$label Emby 媒体库 ID", target.embyLibraryId) { onChange(target.copy(embyLibraryId = it)) }
}
