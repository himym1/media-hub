package com.mediahub.android

import androidx.activity.compose.BackHandler
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
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.ViewModelStoreOwner
import androidx.lifecycle.viewmodel.compose.LocalViewModelStoreOwner
import androidx.lifecycle.viewmodel.compose.viewModel
import com.composables.icons.lucide.ArrowLeft
import com.composables.icons.lucide.Film
import com.composables.icons.lucide.LibraryBig
import com.composables.icons.lucide.ListPlus
import com.composables.icons.lucide.ListTodo
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.Search
import com.composables.icons.lucide.Server
import com.composables.icons.lucide.Settings2
import com.composables.icons.lucide.SquareTerminal
import com.mediahub.android.app.AppState
import com.mediahub.android.app.AppViewModel
import com.mediahub.android.app.MainDestination
import com.mediahub.android.app.WorkspaceDetail
import com.mediahub.android.app.primaryDestinations
import com.mediahub.android.app.MediaHubViewModelFactory
import com.mediahub.android.app.ServerViewModelStoreHolder
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubSecondaryButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubSegmentedControl
import com.mediahub.android.core.image.PosterLoader
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTheme
import com.mediahub.android.feature.auth.AuthRoute
import com.mediahub.android.feature.auth.AuthViewModel
import com.mediahub.android.feature.config.ServerConfigScreen
import com.mediahub.android.feature.config.ServerConfigViewModel
import com.mediahub.android.feature.library.LibraryRoute
import com.mediahub.android.feature.library.LibraryDetailViewModel
import com.mediahub.android.feature.library.LibraryViewModel
import com.mediahub.android.feature.operations.ArchiveViewModel
import com.mediahub.android.feature.operations.Drive115ViewModel
import com.mediahub.android.feature.operations.LocalUploadViewModel
import com.mediahub.android.feature.operations.OperationsRoute
import com.mediahub.android.feature.player.PlayerActivity
import com.mediahub.android.playback.Drive115Target
import com.mediahub.android.playback.EmbyItemTarget
import com.mediahub.android.playback.PlaybackRequest
import com.mediahub.android.feature.search.SearchRoute
import com.mediahub.android.feature.search.SearchViewModel
import com.mediahub.android.playback.MediaHubPlaybackService
import com.mediahub.android.feature.services.ServicesRoute
import com.mediahub.android.feature.services.ServicesViewModel
import com.mediahub.android.feature.subscriptions.SubscriptionRoute
import com.mediahub.android.feature.subscriptions.SubscriptionViewModel
import com.mediahub.android.feature.subscriptions.SubscriptionNavigator
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
        val changeServer = {
            applicationContext.startService(MediaHubPlaybackService.invalidateIntent(applicationContext))
            serverStoreHolder.clearServerScope()
            container.clearConfiguration()
            configuredServerUrl.value = ""
        }
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
                    AuthRoute(
                        viewModel = authViewModel,
                        serverUrl = dependencies.serverUrl,
                        onAuthenticated = appViewModel::onAuthenticated,
                        onChangeServer = changeServer,
                    )
                }
                AppState.Authenticated -> AuthenticatedWorkspace(
                    appViewModel = appViewModel,
                    factory = factory,
                    serverGeneration = serverGeneration,
                    serverIdentity = serverIdentity,
                    posterLoader = dependencies.posterLoader,
                    onChangeServer = changeServer,
                )
                is AppState.Error -> AppMessageScreen(
                    title = "连接失败",
                    message = currentState.message,
                    actionLabel = "重试",
                    onAction = appViewModel::restoreSession,
                    secondaryLabel = "更换服务器",
                    onSecondary = changeServer,
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
    posterLoader: PosterLoader,
) {
    val context = LocalContext.current
    val route by appViewModel.route.collectAsState()
    val destination = route.destination
    val detail = route.detail
    val subscriptionDraft by appViewModel.subscriptionDraft.collectAsState()
    val inSystem = destination == MainDestination.Services || destination == MainDestination.Operations
    val detailOpen = detail != null
    BackHandler(enabled = inSystem, onBack = appViewModel::closeSystem)
    WorkspaceShell(
        destination = destination,
        detailOpen = detailOpen,
        onSystemBack = appViewModel::closeSystem,
        onOpenSystem = appViewModel::openSystem,
        onSystemSelected = appViewModel::showSystemDestination,
        onPrimarySelected = appViewModel::showDestination,
    ) {
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
                    TransferRoute(
                        viewModel = transferViewModel,
                        detailId = (detail as? WorkspaceDetail.Transfer)?.transferId,
                        onDetailChanged = { id ->
                            if (id == null) appViewModel.closeDetail()
                            else appViewModel.openDetail(WorkspaceDetail.Transfer(id))
                        },
                    )
                }
                MainDestination.Subscriptions -> {
                    val subscriptionViewModel = viewModel<SubscriptionViewModel>(key = "subscriptions-$serverGeneration", factory = factory)
                    SubscriptionRoute(
                        viewModel = subscriptionViewModel,
                        draft = subscriptionDraft,
                        navigator = SubscriptionNavigator(
                            editorItemId = (detail as? WorkspaceDetail.SubscriptionEditor)?.subscriptionId,
                            editorKey = (detail as? WorkspaceDetail.SubscriptionEditor)?.key,
                            replaceEditor = { id, key ->
                                appViewModel.openDetail(WorkspaceDetail.SubscriptionEditor(id, key))
                            },
                            openEditor = { id -> appViewModel.openDetail(WorkspaceDetail.SubscriptionEditor(id)) },
                            closeEditor = appViewModel::closeDetail,
                        ),
                        onDraftConsumed = appViewModel::consumeSubscriptionDraft,
                    )
                }
                MainDestination.Library -> {
                    val libraryViewModel = viewModel<LibraryViewModel>(key = "library-$serverGeneration", factory = factory)
                    val detailViewModel = viewModel<LibraryDetailViewModel>(key = "library-detail-$serverGeneration", factory = factory)
                    LibraryRoute(
                        browseViewModel = libraryViewModel,
                        detailViewModel = detailViewModel,
                        posterLoader = posterLoader,
                        selectedItemId = (detail as? WorkspaceDetail.LibraryItem)?.itemId,
                        onSelectedItemChanged = { id ->
                            if (id == null) appViewModel.closeDetail()
                            else appViewModel.openDetail(WorkspaceDetail.LibraryItem(id))
                        },
                        onPlayItem = { item, fallback ->
                            context.startActivity(
                                PlayerActivity.intent(
                                    context,
                                    PlaybackRequest(EmbyItemTarget(item.id), item.name, serverIdentity, fallback),
                                ),
                            )
                        },
                    )
                }
                MainDestination.Operations -> {
                    val driveViewModel = viewModel<Drive115ViewModel>(key = "operations-drive-$serverGeneration", factory = factory)
                    val localUploadViewModel = viewModel<LocalUploadViewModel>(key = "operations-upload-$serverGeneration", factory = factory)
                    val archiveViewModel = viewModel<ArchiveViewModel>(key = "operations-archive-$serverGeneration", factory = factory)
                    OperationsRoute(
                        driveViewModel = driveViewModel,
                        localUploadViewModel = localUploadViewModel,
                        archiveViewModel = archiveViewModel,
                        onPlayDriveFile = { file, parentId ->
                            context.startActivity(
                                PlayerActivity.intent(
                                    context,
                                    PlaybackRequest(Drive115Target(parentId, file.id), file.name, serverIdentity),
                                ),
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
}

@Composable
internal fun WorkspaceShell(
    destination: MainDestination,
    detailOpen: Boolean,
    onSystemBack: () -> Unit,
    onOpenSystem: () -> Unit,
    onSystemSelected: (MainDestination) -> Unit,
    onPrimarySelected: (MainDestination) -> Unit,
    content: @Composable () -> Unit,
) {
    val inSystem = destination == MainDestination.Services || destination == MainDestination.Operations
    Column(
        modifier = Modifier.fillMaxSize().background(MediaHubColors.Canvas).statusBarsPadding(),
    ) {
        if (!detailOpen) {
            Box(Modifier.testTag("workspace-top-bar")) {
                WorkspaceTopBar(
                    destination = destination,
                    inSystem = inSystem,
                    onBack = onSystemBack,
                    onOpenSystem = onOpenSystem,
                )
            }
        }
        if (inSystem) {
            SystemSectionSwitcher(destination = destination, onSelected = onSystemSelected)
        }
        Box(Modifier.weight(1f)) { content() }
        if (!inSystem && !detailOpen) {
            Box(Modifier.testTag("workspace-bottom-nav")) {
                MainNavigationBar(selected = destination, onSelected = onPrimarySelected)
            }
        }
    }
}

@Composable
internal fun WorkspaceTopBar(
    destination: MainDestination,
    inSystem: Boolean,
    onBack: () -> Unit,
    onOpenSystem: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .height(64.dp)
            .padding(horizontal = 16.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (inSystem) {
            MediaHubIconButton(
                imageVector = Lucide.ArrowLeft,
                contentDescription = "返回主页面",
                onClick = onBack,
            )
        } else {
            Box(
                modifier = Modifier
                    .size(34.dp)
                    .background(MediaHubColors.SurfaceSelected, androidx.compose.foundation.shape.RoundedCornerShape(8.dp))
                    .border(width = 1.dp, color = MediaHubColors.Accent, shape = androidx.compose.foundation.shape.RoundedCornerShape(8.dp)),
                contentAlignment = Alignment.Center,
            ) {
                MediaHubIcon(
                    imageVector = Lucide.Film,
                    contentDescription = null,
                    tint = MediaHubColors.Accent,
                    modifier = Modifier.size(18.dp),
                )
            }
        }
        Column(modifier = Modifier.weight(1f).padding(start = 12.dp)) {
            MediaHubText(
                text = if (inSystem) "系统" else destination.title,
                fontSize = 18.sp,
                fontWeight = FontWeight.SemiBold,
            )
            MediaHubText(
                text = if (inSystem) destination.title else "Media Hub",
                color = MediaHubColors.TextMuted,
                fontSize = 12.sp,
            )
        }
        if (!inSystem) {
            MediaHubIconButton(
                imageVector = Lucide.Settings2,
                contentDescription = "打开系统",
                onClick = onOpenSystem,
            )
        }
    }
}

@Composable
internal fun SystemSectionSwitcher(
    destination: MainDestination,
    onSelected: (MainDestination) -> Unit,
) {
    MediaHubSegmentedControl(
        options = listOf(
            MainDestination.Services.name to "服务接入",
            MainDestination.Operations.name to "运维管理",
        ),
        selected = destination.name,
        onSelected = { value ->
            MainDestination.entries.firstOrNull { it.name == value }?.let(onSelected)
        },
        modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
    )
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
            .height(68.dp),
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
    Box(
        modifier = modifier
            .fillMaxHeight()
            .selectable(selected = selected, role = Role.Tab, onClick = onClick),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            modifier = Modifier
                .padding(horizontal = 6.dp, vertical = 4.dp)
                .background(
                    if (selected) MediaHubColors.SurfaceSelected else androidx.compose.ui.graphics.Color.Transparent,
                    androidx.compose.foundation.shape.RoundedCornerShape(10.dp),
                )
                .padding(horizontal = 12.dp, vertical = 6.dp),
            verticalArrangement = Arrangement.Center,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            MediaHubIcon(
                imageVector = icon,
                contentDescription = null,
                tint = if (selected) MediaHubColors.Accent else MediaHubColors.TextMuted,
                modifier = Modifier.size(20.dp),
            )
            MediaHubText(
                text = label,
                modifier = Modifier.padding(top = 3.dp),
                color = if (selected) MediaHubColors.Accent else MediaHubColors.TextMuted,
                fontSize = 12.sp,
                fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
            )
        }
    }
}

@Composable
private fun AppMessageScreen(
    title: String,
    message: String,
    actionLabel: String? = null,
    onAction: () -> Unit = {},
    secondaryLabel: String? = null,
    onSecondary: () -> Unit = {},
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
        if (secondaryLabel != null) {
            MediaHubSecondaryButton(
                label = secondaryLabel,
                icon = Lucide.Server,
                onClick = onSecondary,
                modifier = Modifier.padding(top = 12.dp),
            )
        }
    }
}
