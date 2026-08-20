package com.mediahub.android.core.designsystem

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.graphics.Color
import top.yukonga.miuix.kmp.theme.ColorSchemeMode
import top.yukonga.miuix.kmp.theme.MiuixTheme
import top.yukonga.miuix.kmp.theme.ThemeController

object MediaHubColors {
    val Canvas = Color(0xFFF7F7F7)
    val Surface = Color(0xFFFFFFFF)
    val SurfaceHigh = Color(0xFFF0F0F0)
    val SurfaceInput = Color(0xFFF0F0F0)
    val SurfaceSelected = Color(0xFFEAF2FF)
    val Accent = Color(0xFF3482FF)
    val OnAccent = Color(0xFFFFFFFF)
    val TextPrimary = Color(0xFF1A1A1A)
    val TextStrong = Color(0xFF111111)
    val TextSecondary = Color(0xFF8C8C8C)
    val TextMuted = Color(0xFF8C8C8C)
    val TextFaint = Color(0xFFB2B2B2)
    val Source = Color(0xFF3482FF)
    val Success = Color(0xFF34C759)
    val Error = Color(0xFFE94634)
    val Warning = Color(0xFFE6A817)
    val Border = Color(0x14000000)
}

@Composable
fun MediaHubTheme(content: @Composable () -> Unit) {
    val controller = remember {
        ThemeController(colorSchemeMode = ColorSchemeMode.Light)
    }

    MiuixTheme(controller = controller) {
        content()
    }
}
