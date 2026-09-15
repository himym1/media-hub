package com.mediahub.android.app

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.BuildConfig
import com.mediahub.android.core.network.AndroidRelease
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.SearchCandidate
import com.mediahub.android.data.MediaHubRepository
import java.io.File
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

enum class MainDestination(val title: String, val subtitle: String) {
    Search("发现", "搜索并转存"),
    Subscriptions("订阅", "自动追剧"),
    Transfers("任务", "转存进度"),
    Library("媒体库", "已入库内容"),
    Services("服务与设置", "账户与接入"),
}

val primaryDestinations = listOf(
    MainDestination.Search,
    MainDestination.Transfers,
    MainDestination.Subscriptions,
    MainDestination.Library,
)

sealed interface WorkspaceDetail {
    data class LibraryItem(val itemId: String) : WorkspaceDetail
    data class Transfer(val transferId: String) : WorkspaceDetail
    data class SubscriptionEditor(
        val subscriptionId: String?,
        val key: Long = System.nanoTime(),
    ) : WorkspaceDetail
}

data class WorkspaceRoute(
    val destination: MainDestination = MainDestination.Search,
    val detail: WorkspaceDetail? = null,
)

internal class MainNavigationHistory(
    initial: MainDestination = MainDestination.Search,
) {
    private var lastPrimary = initial.takeIf { it in primaryDestinations } ?: MainDestination.Search
    var current = WorkspaceRoute(lastPrimary)
        private set

    fun show(destination: MainDestination): WorkspaceRoute {
        if (destination in primaryDestinations) lastPrimary = destination
        return WorkspaceRoute(destination).also { current = it }
    }

    fun openSystem(): WorkspaceRoute = WorkspaceRoute(MainDestination.Services).also { current = it }

    fun showSystem(destination: MainDestination): WorkspaceRoute {
        val target = destination.takeIf { it == MainDestination.Services } ?: MainDestination.Services
        return WorkspaceRoute(target).also { current = it }
    }

    fun closeSystem(): WorkspaceRoute = WorkspaceRoute(lastPrimary).also { current = it }

    fun openDetail(detail: WorkspaceDetail): WorkspaceRoute {
        val expectedDestination = when (detail) {
            is WorkspaceDetail.LibraryItem -> MainDestination.Library
            is WorkspaceDetail.Transfer -> MainDestination.Transfers
            is WorkspaceDetail.SubscriptionEditor -> MainDestination.Subscriptions
        }
        if (current.destination == expectedDestination) current = current.copy(detail = detail)
        return current
    }

    fun closeDetail(): WorkspaceRoute = current.copy(detail = null).also { current = it }

    fun reset(): WorkspaceRoute {
        lastPrimary = MainDestination.Search
        return WorkspaceRoute(lastPrimary).also { current = it }
    }
}

sealed interface AppState {
    data object Loading : AppState
    data object Authenticated : AppState
    data object Unauthenticated : AppState
    data class Error(val message: String) : AppState
}

data class AppUpdatePrompt(
    val release: AndroidRelease,
    val downloading: Boolean = false,
    val downloadedBytes: Long = 0,
    val downloadedPath: String? = null,
    val errorMessage: String? = null,
    val hidden: Boolean = false,
)

class AppViewModel(
    private val repository: MediaHubRepository,
    private val updateNotifier: AndroidUpdateNotifier = AndroidUpdateNotifier.None,
    private val updatePromptStore: AndroidUpdatePromptStore? = null,
) : ViewModel() {
    private val _state = MutableStateFlow<AppState>(AppState.Loading)
    val state: StateFlow<AppState> = _state.asStateFlow()

    private val navigation = MainNavigationHistory()
    private val _route = MutableStateFlow(navigation.current)
    val route: StateFlow<WorkspaceRoute> = _route.asStateFlow()

    private val _subscriptionDraft = MutableStateFlow<SearchCandidate?>(null)
    val subscriptionDraft: StateFlow<SearchCandidate?> = _subscriptionDraft.asStateFlow()

    private val _updatePrompt = MutableStateFlow<AppUpdatePrompt?>(null)
    val updatePrompt: StateFlow<AppUpdatePrompt?> = _updatePrompt.asStateFlow()
    private var dismissedUpdateCode: Int? = updatePromptStore?.dismissedVersionCode()?.takeIf { it > 0 }

    init {
        viewModelScope.launch {
            repository.sessionExpired.collect {
                _route.value = navigation.reset()
                _subscriptionDraft.value = null
                _updatePrompt.value = null
                updateNotifier.cancel()
                _state.value = AppState.Unauthenticated
            }
        }
        restoreSession()
    }

    fun restoreSession() {
        viewModelScope.launch {
            _state.value = AppState.Loading
            _state.value = try {
                if (repository.restoreSession()) {
                    checkForUpdate()
                    AppState.Authenticated
                } else {
                    AppState.Unauthenticated
                }
            } catch (error: ApiException) {
                AppState.Error(error.message ?: "无法连接 Media Hub")
            } catch (_: Exception) {
                AppState.Error("无法连接 Media Hub")
            }
        }
    }

    fun onAuthenticated() {
        _state.value = AppState.Authenticated
        checkForUpdate()
    }

    fun dismissUpdate() {
        val prompt = _updatePrompt.value ?: return
        if (prompt.downloading) {
            _updatePrompt.value = prompt.copy(hidden = true)
            return
        }
        dismissedUpdateCode = prompt.release.versionCode
        updatePromptStore?.rememberDismissed(prompt.release.versionCode)
        _updatePrompt.value = null
        updateNotifier.cancel()
    }

    fun downloadUpdate(destination: File) {
        val prompt = _updatePrompt.value ?: return
        if (prompt.downloading) return
        viewModelScope.launch {
            _updatePrompt.value = prompt.copy(
                downloading = true,
                downloadedBytes = 0,
                hidden = false,
                errorMessage = null,
                downloadedPath = null,
            )
            updateNotifier.showProgress(prompt.release.versionName, 0, prompt.release.sizeBytes)
            try {
                val downloaded = repository.downloadAndroidRelease(prompt.release, destination) { bytes ->
                    val current = _updatePrompt.value ?: return@downloadAndroidRelease
                    _updatePrompt.value = current.copy(downloadedBytes = bytes)
                    updateNotifier.showProgress(current.release.versionName, bytes, current.release.sizeBytes)
                }
                _updatePrompt.value = _updatePrompt.value?.copy(
                    downloading = false,
                    downloadedBytes = downloaded.release.sizeBytes,
                    downloadedPath = downloaded.file.absolutePath,
                    hidden = false,
                )
                updateNotifier.showReady(downloaded.release.versionName, downloaded.file)
            } catch (error: ApiException) {
                val message = error.message ?: "更新下载失败"
                _updatePrompt.value = _updatePrompt.value?.copy(
                    downloading = false,
                    errorMessage = message,
                    hidden = false,
                )
                updateNotifier.showFailed(message)
            } catch (_: Exception) {
                val message = "更新下载或校验失败"
                _updatePrompt.value = _updatePrompt.value?.copy(
                    downloading = false,
                    errorMessage = message,
                    hidden = false,
                )
                updateNotifier.showFailed(message)
            }
        }
    }

    fun consumeDownloadedUpdate() {
        _updatePrompt.value = _updatePrompt.value?.copy(downloadedPath = null)
        updateNotifier.cancel()
    }

    private fun checkForUpdate() {
        viewModelScope.launch {
            val latest = runCatching { repository.latestAndroidRelease() }.getOrNull() ?: return@launch
            val release = newerAndroidRelease(BuildConfig.VERSION_CODE, latest) ?: return@launch
            if (dismissedUpdateCode == release.versionCode) return@launch
            val current = _updatePrompt.value
            if (current?.release?.versionCode == release.versionCode) return@launch
            _updatePrompt.value = AppUpdatePrompt(release)
        }
    }

    fun prepareSubscription(candidate: SearchCandidate) {
        if (candidate.tmdbId == null || candidate.transferState == "identity_required") return
        _subscriptionDraft.value = candidate
        showDestination(MainDestination.Subscriptions)
        openDetail(WorkspaceDetail.SubscriptionEditor(null))
    }

    fun consumeSubscriptionDraft() {
        _subscriptionDraft.value = null
    }

    fun showDestination(destination: MainDestination) {
        _route.value = navigation.show(destination)
    }

    fun openSystem() {
        _route.value = navigation.openSystem()
    }

    fun closeSystem() {
        _route.value = navigation.closeSystem()
    }

    fun openDetail(detail: WorkspaceDetail) {
        _route.value = navigation.openDetail(detail)
    }

    fun closeDetail() {
        _route.value = navigation.closeDetail()
    }

    fun logout() {
        viewModelScope.launch {
            runCatching { repository.logout() }
            _route.value = navigation.reset()
            _subscriptionDraft.value = null
            _updatePrompt.value = null
            updateNotifier.cancel()
            _state.value = AppState.Unauthenticated
        }
    }
}
