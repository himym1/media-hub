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

enum class MainDestination(val title: String) {
    Search("发现"),
    Subscriptions("订阅"),
    Transfers("任务"),
    Library("媒体库"),
    Operations("运维"),
    Services("服务与设置"),
}

val primaryDestinations = listOf(
    MainDestination.Search,
    MainDestination.Transfers,
    MainDestination.Subscriptions,
    MainDestination.Library,
)

internal class MainNavigationHistory(
    initial: MainDestination = MainDestination.Search,
) {
    private var lastPrimary = initial.takeIf { it in primaryDestinations } ?: MainDestination.Search

    fun show(destination: MainDestination): MainDestination {
        if (destination in primaryDestinations) lastPrimary = destination
        return destination
    }

    fun openSystem(): MainDestination = MainDestination.Services

    fun showSystem(destination: MainDestination): MainDestination =
        destination.takeIf { it == MainDestination.Services || it == MainDestination.Operations } ?: MainDestination.Services

    fun closeSystem(): MainDestination = lastPrimary

    fun reset(): MainDestination {
        lastPrimary = MainDestination.Search
        return lastPrimary
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

    private val _destination = MutableStateFlow(MainDestination.Search)
    val destination: StateFlow<MainDestination> = _destination.asStateFlow()
    private val navigation = MainNavigationHistory()

    private val _subscriptionDraft = MutableStateFlow<SearchCandidate?>(null)
    val subscriptionDraft: StateFlow<SearchCandidate?> = _subscriptionDraft.asStateFlow()

    init {
        viewModelScope.launch {
            repository.sessionExpired.collect {
                _destination.value = navigation.reset()
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
    }

    fun consumeSubscriptionDraft() {
        _subscriptionDraft.value = null
    }

    fun showDestination(destination: MainDestination) {
        _destination.value = navigation.show(destination)
    }

    fun openSystem() {
        _destination.value = navigation.openSystem()
    }

    fun showSystemDestination(destination: MainDestination) {
        _destination.value = navigation.showSystem(destination)
    }

    fun closeSystem() {
        _destination.value = navigation.closeSystem()
    }

    fun logout() {
        viewModelScope.launch {
            runCatching { repository.logout() }
            _destination.value = navigation.reset()
            _subscriptionDraft.value = null
            _state.value = AppState.Unauthenticated
        }
    }
}
