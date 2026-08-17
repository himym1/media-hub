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
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.selection.selectable
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
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
import androidx.lifecycle.ViewModelStoreOwner
import androidx.lifecycle.viewmodel.compose.LocalViewModelStoreOwner
import androidx.lifecycle.viewmodel.compose.viewModel
import com.composables.icons.lucide.Film
import com.composables.icons.lucide.LibraryBig
import com.composables.icons.lucide.ListPlus
import com.composables.icons.lucide.ListTodo
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.Search
import com.composables.icons.lucide.Settings2
import com.composables.icons.lucide.SquareTerminal
import com.mediahub.android.app.AppState
import com.mediahub.android.app.AppViewModel
import com.mediahub.android.app.MainDestination
import com.mediahub.android.app.primaryDestinations
import com.mediahub.android.app.MediaHubViewModelFactory
import com.mediahub.android.app.ServerViewModelStoreHolder
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTheme
import com.mediahub.android.feature.auth.AuthRoute
import com.mediahub.android.feature.auth.AuthViewModel
import com.mediahub.android.feature.config.ServerConfigScreen
import com.mediahub.android.feature.config.ServerConfigViewModel
import com.mediahub.android.feature.library.LibraryRoute
import com.mediahub.android.feature.library.LibraryViewModel
import com.mediahub.android.feature.operations.OperationsRoute
import com.mediahub.android.feature.operations.OperationsViewModel
import com.mediahub.android.feature.player.PlayerActivity
import com.mediahub.android.playback.PlaybackRequest
import com.mediahub.android.feature.search.SearchRoute
import com.mediahub.android.feature.search.SearchViewModel
import com.mediahub.android.playback.MediaHubPlaybackService
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
        val container = remember { (applicationContext as MediaHubApplication).container }
        val serverStoreHolder = viewModel<ServerViewModelStoreHolder>()
        val serverUrlStore = container.serverUrlStore
        val initialServerUrl = remember { container.initialServerUrl() }
        val configuredServerUrl = remember { androidx.compose.runtime.mutableStateOf(initialServerUrl) }
        if (configuredServerUrl.value.isBlank()) {
            val configViewModel = remember { ServerConfigViewModel(serverUrlStore, "") }
            ServerConfigScreen(
                viewModel = configViewModel,
                onConfigured = { value ->
                    container.configureServer(value)
                    configuredServerUrl.value = value
                },
            )
            return@MediaHubTheme
        }
        val dependenciesResult = remember(configuredServerUrl.value) {
            runCatching { container.configureServer(configuredServerUrl.value) }
        }
        val dependencies = dependenciesResult.getOrNull()
        if (dependencies == null) {
            val configViewModel = remember { ServerConfigViewModel(serverUrlStore, configuredServerUrl.value) }
            ServerConfigScreen(
                viewModel = configViewModel,
                onConfigured = { value ->
                    container.configureServer(value)
                    configuredServerUrl.value = value
                },
            )
            return@MediaHubTheme
        }
        val repository = dependencies.repository
        val serverGeneration = dependencies.generation
        val serverIdentity = dependencies.serverIdentity
        val serverViewModelStoreOwner = remember(serverStoreHolder, serverGeneration) {
            serverStoreHolder.ownerFor(serverGeneration)
        }
        ServerViewModelScope(serverViewModelStoreOwner) {
            val factory = remember(repository) { MediaHubViewModelFactory(repository) }
            val appViewModel = viewModel<AppViewModel>(key = "app-$serverGeneration", factory = factory)
            val state by appViewModel.state.collectAsState()
            when (val currentState = state) {
                AppState.Loading -> AppMessageScreen(title = "MEDIA HUB", message = "正在连接…")
                AppState.Unauthenticated -> {
                    val authViewModel = viewModel<AuthViewModel>(key = "auth-$serverGeneration", factory = factory)
                    AuthRoute(viewModel = authViewModel, onAuthenticated = appViewModel::onAuthenticated)
                }
                AppState.Authenticated -> AuthenticatedWorkspace(
                    appViewModel = appViewModel,
                    factory = factory,
                    serverGeneration = serverGeneration,
                    serverIdentity = serverIdentity,
                    onChangeServer = {
                        applicationContext.startService(MediaHubPlaybackService.invalidateIntent(applicationContext))
                        serverStoreHolder.clearServerScope()
                        container.clearConfiguration()
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
}

@Composable
private fun ServerViewModelScope(owner: ViewModelStoreOwner, content: @Composable () -> Unit) {
    CompositionLocalProvider(LocalViewModelStoreOwner provides owner, content = content)
}

@Composable
private fun AuthenticatedWorkspace(
    appViewModel: AppViewModel,
    factory: MediaHubViewModelFactory,
    onChangeServer: () -> Unit,
    serverGeneration: Long,
    serverIdentity: String,
) {
    val context = LocalContext.current
    val destination by appViewModel.destination.collectAsState()
    val subscriptionDraft by appViewModel.subscriptionDraft.collectAsState()
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MediaHubColors.Canvas)
            .statusBarsPadding(),
    ) {
        WorkspaceTopBar(
            destination = destination,
            onSystemSelected = {
                appViewModel.showDestination(
                    if (destination == MainDestination.Services) MainDestination.Operations else MainDestination.Services,
                )
            },
        )
        Box(Modifier.weight(1f)) {
            when (destination) {
                MainDestination.Search -> {
                    val searchViewModel = viewModel<SearchViewModel>(key = "search-$serverGeneration", factory = factory)
                    SearchRoute(
                        viewModel = searchViewModel,
                        onTransferCreated = { appViewModel.showDestination(MainDestination.Transfers) },
                        onSubscriptionRequested = appViewModel::prepareSubscription,
                    )
                }
                MainDestination.Transfers -> {
                    val transferViewModel = viewModel<TransferViewModel>(key = "transfers-$serverGeneration", factory = factory)
                    TransferRoute(viewModel = transferViewModel)
                }
                MainDestination.Subscriptions -> {
                    val subscriptionViewModel = viewModel<SubscriptionViewModel>(key = "subscriptions-$serverGeneration", factory = factory)
                    SubscriptionRoute(
                        viewModel = subscriptionViewModel,
                        draft = subscriptionDraft,
                        onDraftConsumed = appViewModel::consumeSubscriptionDraft,
                    )
                }
                MainDestination.Library -> {
                    val libraryViewModel = viewModel<LibraryViewModel>(key = "library-$serverGeneration", factory = factory)
                    LibraryRoute(viewModel = libraryViewModel)
                }
                MainDestination.Operations -> {
                    val operationsViewModel = viewModel<OperationsViewModel>(key = "operations-$serverGeneration", factory = factory)
                    OperationsRoute(
                        viewModel = operationsViewModel,
                        onPlayDriveFile = { file, parentId ->
                            context.startActivity(
                                PlayerActivity.intent(context, PlaybackRequest(parentId, file.id, file.name, serverIdentity)),
                            )
                        },
                    )
                }
                MainDestination.Services -> {
                    val servicesViewModel = viewModel<ServicesViewModel>(key = "services-$serverGeneration", factory = factory)
                    ServicesRoute(
                        viewModel = servicesViewModel,
                        onLogout = {
                            context.startService(MediaHubPlaybackService.invalidateIntent(context))
                            appViewModel.logout()
                        },
                        onChangeServer = onChangeServer,
                    )
                }
            }
        }
        MainNavigationBar(selected = destination, onSelected = appViewModel::showDestination)
    }
}

@Composable
private fun WorkspaceTopBar(destination: MainDestination, onSystemSelected: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(60.dp)
            .border(width = 1.dp, color = MediaHubColors.Border)
            .padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MediaHubIcon(
            imageVector = Lucide.Film,
            contentDescription = null,
            tint = MediaHubColors.Accent,
            modifier = Modifier.size(22.dp),
        )
        Column(modifier = Modifier.weight(1f).padding(start = 11.dp)) {
            MediaHubText(text = destination.title, fontSize = 19.sp, fontWeight = FontWeight.SemiBold)
            MediaHubText(text = if (destination == MainDestination.Operations || destination == MainDestination.Services) "系统管理" else "Media Hub", color = MediaHubColors.TextMuted, fontSize = 12.sp)
        }
        MediaHubIconButton(
            imageVector = if (destination == MainDestination.Services) Lucide.SquareTerminal else Lucide.Settings2,
            contentDescription = if (destination == MainDestination.Services) "打开运维工具" else "打开系统设置",
            onClick = onSystemSelected,
        )
    }
}

private fun destinationIcon(destination: MainDestination) = when (destination) {
    MainDestination.Search -> Lucide.Search
    MainDestination.Transfers -> Lucide.ListTodo
    MainDestination.Subscriptions -> Lucide.ListPlus
    MainDestination.Library -> Lucide.LibraryBig
    MainDestination.Operations -> Lucide.SquareTerminal
    MainDestination.Services -> Lucide.Settings2
}

@Composable
private fun MainNavigationBar(selected: MainDestination, onSelected: (MainDestination) -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .background(MediaHubColors.Surface)
            .border(width = 1.dp, color = MediaHubColors.Border)
            .navigationBarsPadding()
            .height(66.dp),
    ) {
        primaryDestinations.forEach { destination ->
            NavigationItem(
                label = destination.title,
                icon = destinationIcon(destination),
                selected = selected == destination,
                onClick = { onSelected(destination) },
                modifier = Modifier.weight(1f),
            )
        }
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
        modifier = modifier.fillMaxHeight().selectable(selected = selected, role = Role.Tab, onClick = onClick),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        MediaHubIcon(imageVector = icon, contentDescription = null, tint = if (selected) MediaHubColors.Accent else MediaHubColors.TextMuted)
        MediaHubText(text = label, modifier = Modifier.padding(top = 4.dp), color = if (selected) MediaHubColors.Accent else MediaHubColors.TextMuted, fontSize = 12.sp)
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
        MediaHubText(text = message, modifier = Modifier.padding(top = 10.dp), color = MediaHubColors.TextSecondary, fontSize = 13.sp)
        if (actionLabel != null) {
            MediaHubButton(label = actionLabel, icon = Lucide.RefreshCw, onClick = onAction, modifier = Modifier.padding(top = 22.dp))
        }
    }
}
