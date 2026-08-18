package com.mediahub.android.feature.operations

import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import com.mediahub.android.core.designsystem.MediaHubColors

internal fun formatBytes(value: Long): String = when {
    value < 1024 -> "$value B"
    value < 1024 * 1024 -> "%.1f KB".format(value / 1024.0)
    value < 1024L * 1024 * 1024 -> "%.1f MB".format(value / (1024.0 * 1024))
    else -> "%.1f GB".format(value / (1024.0 * 1024 * 1024))
}

internal fun commandStateLabel(state: String): String = when (state) {
    "awaiting_confirmation" -> "待确认"
    "queued" -> "等待中"
    "submitting" -> "提交中"
    "completed" -> "已完成"
    "failed" -> "失败"
    "needs_attention" -> "待确认"
    else -> state
}

@Composable
internal fun commandStateColor(state: String): Color = when (state) {
    "awaiting_confirmation" -> MediaHubColors.Warning
    "completed" -> MediaHubColors.Source
    "failed" -> MediaHubColors.Error
    "needs_attention" -> MediaHubColors.Warning
    else -> MediaHubColors.TextSecondary
}

internal fun localUploadState(state: String): String = when (state) {
    "queued" -> "等待中"
    "hashing" -> "校验中"
    "submitting_init" -> "初始化"
    "uploading" -> "上传中"
    "completed" -> "已完成"
    "failed" -> "失败"
    "needs_attention" -> "待核对"
    else -> state
}
