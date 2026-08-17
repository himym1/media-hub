package com.mediahub.android.feature.library

import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.produceState
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.unit.dp
import com.composables.icons.lucide.Film
import com.composables.icons.lucide.Lucide
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.image.PosterLoader

@Composable
internal fun EmbyPoster(
    itemId: String,
    loader: PosterLoader,
    contentDescription: String,
    modifier: Modifier = Modifier,
) {
    val image by produceState(initialValue = loader.cached(itemId), itemId, loader) {
        if (value == null) value = loader.load(itemId)
    }
    val posterModifier = modifier.aspectRatio(2f / 3f).clip(RoundedCornerShape(6.dp))
    if (image != null) {
        Image(
            bitmap = requireNotNull(image),
            contentDescription = contentDescription,
            modifier = posterModifier,
            contentScale = ContentScale.Crop,
        )
    } else {
        Box(
            modifier = posterModifier.background(MediaHubColors.SurfaceInput),
            contentAlignment = Alignment.Center,
        ) {
            MediaHubIcon(
                imageVector = Lucide.Film,
                contentDescription = null,
                tint = MediaHubColors.TextFaint,
                modifier = Modifier.fillMaxSize(0.32f),
            )
        }
    }
}
