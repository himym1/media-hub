package com.mediahub.android.playback

import android.content.Context
import android.content.Intent
import androidx.core.net.toUri
import com.mediahub.android.feature.library.validatedEmbyAppUrl

internal fun openPlaybackFallback(context: Context, fallback: PlaybackFallback?) {
    val appUrl = validatedEmbyAppUrl(fallback?.appUrl)
    if (appUrl != null) {
        context.startActivity(Intent(Intent.ACTION_VIEW, appUrl.toUri()))
        return
    }
    val webUrl = fallback?.webUrl?.trim()?.takeIf { value ->
        value.startsWith("https://", ignoreCase = true)
    }
    if (webUrl != null) {
        context.startActivity(Intent(Intent.ACTION_VIEW, webUrl.toUri()))
    }
}
