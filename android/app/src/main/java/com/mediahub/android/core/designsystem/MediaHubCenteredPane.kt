package com.mediahub.android.core.designsystem

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import com.mediahub.android.app.LocalTwoPane

@Composable
fun MediaHubCenteredPane(
    modifier: Modifier = Modifier,
    maxWidth: Dp = 480.dp,
    contentPadding: PaddingValues = PaddingValues(0.dp),
    verticalArrangement: Arrangement.Vertical = Arrangement.Top,
    horizontalAlignment: Alignment.Horizontal = Alignment.Start,
    content: @Composable ColumnScope.() -> Unit,
) {
    val twoPane = LocalTwoPane.current
    Box(
        modifier = modifier.fillMaxSize(),
        contentAlignment = if (twoPane) Alignment.Center else Alignment.TopCenter,
    ) {
        Column(
            modifier = Modifier
                .widthIn(max = if (twoPane) maxWidth else Dp.Unspecified)
                .fillMaxWidth()
                .then(if (twoPane) Modifier else Modifier.fillMaxSize())
                .padding(contentPadding),
            verticalArrangement = verticalArrangement,
            horizontalAlignment = horizontalAlignment,
            content = content,
        )
    }
}
