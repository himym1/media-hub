package com.mediahub.android.core.designsystem

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.graphics.Color
import top.yukonga.miuix.kmp.theme.ColorSchemeMode
import top.yukonga.miuix.kmp.theme.MiuixTheme
import top.yukonga.miuix.kmp.theme.ThemeController

object MediaHubColors {
    val Canvas = Color(0xFF0E1114)
    val Surface = Color(0xFF1B2228)
    val SurfaceInput = Color(0xFF252E35)
    val SurfaceSelected = Color(0xFF183B2B)
    val Accent = Color(0xFF34D399)
    val TextPrimary = Color(0xFFF8FAFC)
    val TextStrong = Color(0xFFF1F5F9)
    val TextSecondary = Color(0xFFCBD5E1)
    val TextMuted = Color(0xFF94A3B8)
    val TextFaint = Color(0xFF64748B)
    val Source = Color(0xFF60A5FA)
    val Success = Color(0xFF34D399)
    val Error = Color(0xFFF87171)
    val Warning = Color(0xFFFBBF24)
    val Border = Color(0xFF2E3B46)
}

@Composable
fun MediaHubTheme(content: @Composable () -> Unit) {
    val controller = remember {
        ThemeController(
            colorSchemeMode = ColorSchemeMode.Dark,
            keyColor = MediaHubColors.Accent,
        )
    }

    MiuixTheme(controller = controller) {
        content()
    }
}
