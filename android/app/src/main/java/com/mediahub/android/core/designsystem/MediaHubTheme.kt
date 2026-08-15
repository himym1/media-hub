package com.mediahub.android.core.designsystem

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.graphics.Color
import top.yukonga.miuix.kmp.theme.ColorSchemeMode
import top.yukonga.miuix.kmp.theme.MiuixTheme
import top.yukonga.miuix.kmp.theme.ThemeController

object MediaHubColors {
    val Canvas = Color(0xFF111416)
    val Surface = Color(0xFF181D20)
    val SurfaceInput = Color(0xFF20262A)
    val SurfaceSelected = Color(0xFF203029)
    val Accent = Color(0xFF8FC7A3)
    val TextPrimary = Color(0xFFEDF1EF)
    val TextStrong = Color(0xFFDDE3E0)
    val TextSecondary = Color(0xFFB7BFBC)
    val TextMuted = Color(0xFF8D9793)
    val TextFaint = Color(0xFF7A8580)
    val Source = Color(0xFF7EB6D8)
    val Success = Color(0xFF75BEA0)
    val Error = Color(0xFFE07B75)
    val Warning = Color(0xFFE0B46E)
    val Border = Color(0xFF30363A)
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
