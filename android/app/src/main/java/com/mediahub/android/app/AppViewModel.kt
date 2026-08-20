package com.mediahub.android.app

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.SearchCandidate
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

enum class MainDestination(val title: String, val subtitle: String) {
    Search("发现", "搜索并转存"),
    Subscriptions("订阅", "自动追剧"),
    Transfers("任务", "转存进度"),
    Library("媒体库", "已入库内容"),
    Operations("运维", "115 与归档"),
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
        val target = destination.takeIf { it == MainDestination.Services || it == MainDestination.Operations }
            ?: MainDestination.Services
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

class AppViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _state = MutableStateFlow<AppState>(AppState.Loading)
    val state: StateFlow<AppState> = _state.asStateFlow()

    private val navigation = MainNavigationHistory()
    private val _route = MutableStateFlow(navigation.current)
    val route: StateFlow<WorkspaceRoute> = _route.asStateFlow()

    private val _subscriptionDraft = MutableStateFlow<SearchCandidate?>(null)
    val subscriptionDraft: StateFlow<SearchCandidate?> = _subscriptionDraft.asStateFlow()

    init {
        viewModelScope.launch {
            repository.sessionExpired.collect {
                _route.value = navigation.reset()
                _subscriptionDraft.value = null
                _state.value = AppState.Unauthenticated
            }
        }
        restoreSession()
    }

    fun restoreSession() {
        viewModelScope.launch {
            _state.value = AppState.Loading
            _state.value = try {
                if (repository.restoreSession()) AppState.Authenticated else AppState.Unauthenticated
            } catch (error: ApiException) {
                AppState.Error(error.message ?: "无法连接 Media Hub")
            } catch (_: Exception) {
                AppState.Error("无法连接 Media Hub")
            }
        }
    }

    fun onAuthenticated() {
        _state.value = AppState.Authenticated
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

    fun showSystemDestination(destination: MainDestination) {
        _route.value = navigation.showSystem(destination)
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
            _state.value = AppState.Unauthenticated
        }
    }
}
