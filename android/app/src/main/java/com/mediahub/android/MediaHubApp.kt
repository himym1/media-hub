package com.mediahub.android

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.selection.selectable
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.viewmodel.compose.viewModel
import com.composables.icons.lucide.LibraryBig
import com.composables.icons.lucide.ListPlus
import com.composables.icons.lucide.ListTodo
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.Search
import com.composables.icons.lucide.SquareTerminal
import com.composables.icons.lucide.Settings2
import com.mediahub.android.app.AppState
import com.mediahub.android.app.AppViewModel
import com.mediahub.android.app.MainDestination
import com.mediahub.android.app.MediaHubViewModelFactory
import com.mediahub.android.core.auth.SecureSessionStore
import com.mediahub.android.core.config.ServerUrlStore
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTheme
import com.mediahub.android.core.network.MediaHubApi
import com.mediahub.android.data.MediaHubRepository
import com.mediahub.android.feature.auth.AuthRoute
import com.mediahub.android.feature.auth.AuthViewModel
import com.mediahub.android.feature.config.ServerConfigScreen
import com.mediahub.android.feature.config.ServerConfigViewModel
import com.mediahub.android.feature.library.LibraryRoute
import com.mediahub.android.feature.library.LibraryViewModel
import com.mediahub.android.feature.operations.OperationsRoute
import com.mediahub.android.feature.operations.OperationsViewModel
import com.mediahub.android.feature.search.SearchRoute
import com.mediahub.android.feature.search.SearchViewModel
import com.mediahub.android.feature.services.ServicesRoute
import com.mediahub.android.feature.services.ServicesViewModel
import com.mediahub.android.feature.subscriptions.SubscriptionRoute
import com.mediahub.android.feature.subscriptions.SubscriptionViewModel
import com.mediahub.android.feature.transfers.TransferRoute
import com.mediahub.android.feature.transfers.TransferViewModel

@Composable
fun MediaHubApp() {
    MediaHubTheme {
        val applicationContext = LocalContext.current.applicationContext
        val serverUrlStore = remember { ServerUrlStore(applicationContext) }
        val initialServerUrl = remember { serverUrlStore.load(BuildConfig.API_BASE_URL) }
        val configuredServerUrl = remember { androidx.compose.runtime.mutableStateOf(initialServerUrl) }
        if (configuredServerUrl.value.isBlank()) {
            val configViewModel = remember { ServerConfigViewModel(serverUrlStore, "") }
            ServerConfigScreen(viewModel = configViewModel, onConfigured = { configuredServerUrl.value = it })
            return@MediaHubTheme
        }
        val repositoryResult = remember(configuredServerUrl.value) {
            runCatching {
                MediaHubRepository(
                    api = MediaHubApi(configuredServerUrl.value),
                    sessionStore = SecureSessionStore(applicationContext),
                )
            }
        }
        val repository = repositoryResult.getOrNull()
        if (repository == null) {
            val configViewModel = remember { ServerConfigViewModel(serverUrlStore, configuredServerUrl.value) }
            ServerConfigScreen(viewModel = configViewModel, onConfigured = { configuredServerUrl.value = it })
            return@MediaHubTheme
        }

        val factory = remember(repository) { MediaHubViewModelFactory(repository) }
        val appViewModel = viewModel<AppViewModel>(factory = factory)
        val state by appViewModel.state.collectAsState()

        when (val currentState = state) {
            AppState.Loading -> AppMessageScreen(title = "MEDIA HUB", message = "正在连接")
            AppState.Unauthenticated -> {
                val authViewModel = viewModel<AuthViewModel>(factory = factory)
                AuthRoute(
                    viewModel = authViewModel,
                    onAuthenticated = appViewModel::onAuthenticated,
                )
            }
            AppState.Authenticated -> AuthenticatedWorkspace(
                appViewModel = appViewModel,
                factory = factory,
                onChangeServer = {
                    serverUrlStore.clear()
                    SecureSessionStore(applicationContext).clear()
                    configuredServerUrl.value = ""
                },
            )
            is AppState.Error -> AppMessageScreen(
                title = "连接失败",
                message = currentState.message,
                actionLabel = "重试",
                onAction = appViewModel::restoreSession,
            )
        }
    }
}

@Composable
private fun AuthenticatedWorkspace(
    appViewModel: AppViewModel,
    factory: MediaHubViewModelFactory,
    onChangeServer: () -> Unit,
) {
    val destination by appViewModel.destination.collectAsState()
    val subscriptionDraft by appViewModel.subscriptionDraft.collectAsState()
    Box(Modifier.fillMaxSize().background(MediaHubColors.Canvas)) {
        when (destination) {
            MainDestination.Search -> {
                val searchViewModel = viewModel<SearchViewModel>(factory = factory)
                SearchRoute(
                    viewModel = searchViewModel,
                    onLogout = appViewModel::logout,
                    onTransferCreated = { appViewModel.showDestination(MainDestination.Transfers) },
                    onSubscriptionRequested = appViewModel::prepareSubscription,
                )
            }
            MainDestination.Subscriptions -> {
                val subscriptionViewModel = viewModel<SubscriptionViewModel>(factory = factory)
                SubscriptionRoute(
                    viewModel = subscriptionViewModel,
                    draft = subscriptionDraft,
                    onDraftConsumed = appViewModel::consumeSubscriptionDraft,
                    onLogout = appViewModel::logout,
                )
            }
            MainDestination.Transfers -> {
                val transferViewModel = viewModel<TransferViewModel>(factory = factory)
                TransferRoute(
                    viewModel = transferViewModel,
                    onLogout = appViewModel::logout,
                )
            }
            MainDestination.Library -> {
                val libraryViewModel = viewModel<LibraryViewModel>(factory = factory)
                LibraryRoute(
                    viewModel = libraryViewModel,
                    onLogout = appViewModel::logout,
                )
            }
            MainDestination.Operations -> {
                val operationsViewModel = viewModel<OperationsViewModel>(factory = factory)
                OperationsRoute(
                    viewModel = operationsViewModel,
                    onLogout = appViewModel::logout,
                )
            }
            MainDestination.Services -> {
                val servicesViewModel = viewModel<ServicesViewModel>(factory = factory)
                ServicesRoute(
                    viewModel = servicesViewModel,
                    onLogout = appViewModel::logout,
                onChangeServer = onChangeServer,
                )
            }
        }
        MainNavigationBar(
            selected = destination,
            onSelected = appViewModel::showDestination,
            modifier = Modifier.align(Alignment.BottomCenter),
        )
    }
}

@Composable
private fun MainNavigationBar(
    selected: MainDestination,
    onSelected: (MainDestination) -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .background(MediaHubColors.Surface)
            .border(width = 1.dp, color = MediaHubColors.Border)
            .navigationBarsPadding()
            .height(64.dp),
    ) {
        NavigationItem(
            label = "发现",
            icon = Lucide.Search,
            selected = selected == MainDestination.Search,
            onClick = { onSelected(MainDestination.Search) },
            modifier = Modifier.weight(1f),
        )
        NavigationItem(
            label = "订阅",
            icon = Lucide.ListPlus,
            selected = selected == MainDestination.Subscriptions,
            onClick = { onSelected(MainDestination.Subscriptions) },
            modifier = Modifier.weight(1f),
        )
        NavigationItem(
            label = "任务",
            icon = Lucide.ListTodo,
            selected = selected == MainDestination.Transfers,
            onClick = { onSelected(MainDestination.Transfers) },
            modifier = Modifier.weight(1f),
        )
        NavigationItem(
            label = "媒体库",
            icon = Lucide.LibraryBig,
            selected = selected == MainDestination.Library,
            onClick = { onSelected(MainDestination.Library) },
            modifier = Modifier.weight(1f),
        )
        NavigationItem(
            label = "运维",
            icon = Lucide.SquareTerminal,
            selected = selected == MainDestination.Operations,
            onClick = { onSelected(MainDestination.Operations) },
            modifier = Modifier.weight(1f),
        )
        NavigationItem(
            label = "服务",
            icon = Lucide.Settings2,
            selected = selected == MainDestination.Services,
            onClick = { onSelected(MainDestination.Services) },
            modifier = Modifier.weight(1f),
        )
    }
}

@Composable
private fun NavigationItem(
    label: String,
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    selected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .fillMaxHeight()
            .selectable(selected = selected, role = Role.Tab, onClick = onClick),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        MediaHubIcon(
            imageVector = icon,
            contentDescription = null,
            tint = if (selected) MediaHubColors.Accent else MediaHubColors.TextMuted,
        )
        MediaHubText(
            text = label,
            modifier = Modifier.padding(top = 4.dp),
            color = if (selected) MediaHubColors.Accent else MediaHubColors.TextMuted,
            fontSize = 10.sp,
        )
    }
}

@Composable
private fun AppMessageScreen(
    title: String,
    message: String,
    actionLabel: String? = null,
    onAction: () -> Unit = {},
) {
    Column(
        modifier = Modifier.fillMaxSize().background(MediaHubColors.Canvas).padding(28.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        MediaHubText(text = title, fontSize = 24.sp, fontWeight = FontWeight.SemiBold)
        MediaHubText(
            text = message,
            modifier = Modifier.padding(top = 10.dp),
            color = MediaHubColors.TextSecondary,
            fontSize = 13.sp,
        )
        if (actionLabel != null) {
            MediaHubButton(
                label = actionLabel,
                icon = Lucide.RefreshCw,
                onClick = onAction,
                modifier = Modifier.padding(top = 22.dp),
            )
        }
    }
}
