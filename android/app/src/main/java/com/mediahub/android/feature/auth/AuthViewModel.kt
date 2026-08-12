package com.mediahub.android.feature.auth

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class AuthUiState(
    val password: String = "",
    val passwordVisible: Boolean = false,
    val submitting: Boolean = false,
    val errorMessage: String? = null,
)

sealed interface AuthEvent {
    data object Authenticated : AuthEvent
}

class AuthViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(AuthUiState())
    val uiState: StateFlow<AuthUiState> = _uiState.asStateFlow()

    private val _events = MutableSharedFlow<AuthEvent>()
    val events: SharedFlow<AuthEvent> = _events.asSharedFlow()

    fun onPasswordChanged(password: String) {
        if (password.length > 1024) return
        _uiState.value = _uiState.value.copy(password = password, errorMessage = null)
    }

    fun togglePasswordVisibility() {
        _uiState.value = _uiState.value.copy(passwordVisible = !_uiState.value.passwordVisible)
    }

    fun login() {
        val password = _uiState.value.password
        if (password.isEmpty() || _uiState.value.submitting) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(submitting = true, errorMessage = null)
            try {
                repository.login(password)
                _uiState.value = AuthUiState()
                _events.emit(AuthEvent.Authenticated)
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(
                    submitting = false,
                    errorMessage = error.message ?: "登录失败",
                )
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(
                    submitting = false,
                    errorMessage = "无法连接 Media Hub",
                )
            }
        }
    }
}
