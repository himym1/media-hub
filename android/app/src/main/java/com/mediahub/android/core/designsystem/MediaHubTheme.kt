package com.mediahub.android.core.designsystem

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.graphics.Color
import top.yukonga.miuix.kmp.theme.ColorSchemeMode
import top.yukonga.miuix.kmp.theme.MiuixTheme
import top.yukonga.miuix.kmp.theme.ThemeController

object MediaHubColors {
    val Canvas = Color(0xFF101214)
    val Surface = Color(0xFF171A1B)
    val SurfaceInput = Color(0xFF191C1D)
    val SurfaceSelected = Color(0xFF242A22)
    val Accent = Color(0xFFD7F36A)
    val TextPrimary = Color(0xFFF2F1EB)
    val TextStrong = Color(0xFFE6E6DF)
    val TextSecondary = Color(0xFF858B88)
    val TextMuted = Color(0xFF777D79)
    val TextFaint = Color(0xFF666D69)
    val Source = Color(0xFFC4D17F)
    val Error = Color(0xFFFF8A7A)
    val Warning = Color(0xFFF2BF67)
    val Border = Color(0xFF2A2D30)
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
