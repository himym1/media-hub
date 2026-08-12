package com.mediahub.android.feature.services

import com.mediahub.android.BuildConfig
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.AndroidRelease
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.Drive115DeviceAuthorization
import com.mediahub.android.core.network.IntegrationHealth
import com.mediahub.android.core.network.OperationalStatistics
import com.mediahub.android.data.MediaHubRepository
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class ServicesUiState(
    val integrations: List<IntegrationHealth> = emptyList(),
    val statistics: OperationalStatistics? = null,
    val driveAuthorization: Drive115DeviceAuthorization? = null,
    val authorizingDrive: Boolean = false,
    val androidRelease: AndroidRelease? = null,
    val checkingUpdate: Boolean = false,
    val downloadingUpdate: Boolean = false,
    val downloadedUpdatePath: String? = null,
    val loading: Boolean = false,
    val currentPassword: String = "",
    val newPassword: String = "",
    val confirmation: String = "",
    val changingPassword: Boolean = false,
    val passwordChanged: Boolean = false,
    val errorMessage: String? = null,
)

class ServicesViewModel(
    private val repository: MediaHubRepository,
) : ViewModel() {
    private val _uiState = MutableStateFlow(ServicesUiState())
    private var authorizationJob: Job? = null
    val uiState: StateFlow<ServicesUiState> = _uiState.asStateFlow()

    fun checkForUpdate() {
        if (_uiState.value.checkingUpdate || _uiState.value.downloadingUpdate) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(checkingUpdate = true, errorMessage = null, downloadedUpdatePath = null)
            try {
                val release = repository.latestAndroidRelease()
                _uiState.value = _uiState.value.copy(
                    checkingUpdate = false,
                    androidRelease = release.takeIf { it.versionCode > BuildConfig.VERSION_CODE },
                )
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(
                    checkingUpdate = false,
                    androidRelease = null,
                    errorMessage = if (error.status == 404) null else error.message ?: "无法检查更新",
                )
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(checkingUpdate = false, errorMessage = "无法检查更新")
            }
        }
    }

    fun downloadUpdate(destination: java.io.File) {
        val release = _uiState.value.androidRelease ?: return
        if (_uiState.value.downloadingUpdate) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(downloadingUpdate = true, errorMessage = null, downloadedUpdatePath = null)
            try {
                val downloaded = repository.downloadAndroidRelease(release, destination)
                _uiState.value = _uiState.value.copy(
                    downloadingUpdate = false,
                    downloadedUpdatePath = downloaded.file.absolutePath,
                )
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(downloadingUpdate = false, errorMessage = error.message ?: "更新下载失败")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(downloadingUpdate = false, errorMessage = "更新下载或校验失败")
            }
        }
    }

    fun consumeDownloadedUpdate() {
        _uiState.value = _uiState.value.copy(downloadedUpdatePath = null)
    }

    fun setCurrentPassword(value: String) {
        _uiState.value = _uiState.value.copy(currentPassword = value.take(1024), passwordChanged = false)
    }

    fun setNewPassword(value: String) {
        _uiState.value = _uiState.value.copy(newPassword = value.take(1024), passwordChanged = false)
    }

    fun setConfirmation(value: String) {
        _uiState.value = _uiState.value.copy(confirmation = value.take(1024), passwordChanged = false)
    }

    fun changePassword() {
        val state = _uiState.value
        if (state.changingPassword || state.newPassword.length < 12 ||
            state.newPassword != state.confirmation || state.currentPassword == state.newPassword
        ) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(
                changingPassword = true,
                passwordChanged = false,
                errorMessage = null,
            )
            try {
                repository.changePassword(state.currentPassword, state.newPassword)
                _uiState.value = _uiState.value.copy(
                    currentPassword = "",
                    newPassword = "",
                    confirmation = "",
                    changingPassword = false,
                    passwordChanged = true,
                )
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(
                    changingPassword = false,
                    errorMessage = error.message ?: "密码修改失败",
                )
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(
                    changingPassword = false,
                    errorMessage = "无法修改密码",
                )
            }
        }
    }

    fun startDriveAuthorization() {
        if (_uiState.value.authorizingDrive) return
        authorizationJob?.cancel()
        authorizationJob = viewModelScope.launch {
            _uiState.value = _uiState.value.copy(authorizingDrive = true, errorMessage = null)
            try {
                var authorization = repository.startDrive115Authorization()
                _uiState.value = _uiState.value.copy(driveAuthorization = authorization)
                while (authorization.state == "pending") {
                    delay(2_000)
                    authorization = repository.pollDrive115Authorization(authorization.id)
                    _uiState.value = _uiState.value.copy(driveAuthorization = authorization)
                }
                _uiState.value = _uiState.value.copy(authorizingDrive = false)
                if (authorization.state == "confirmed") refresh()
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(authorizingDrive = false, errorMessage = error.message ?: "115 授权失败")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(authorizingDrive = false, errorMessage = "无法完成 115 授权")
            }
        }
    }

    fun stopDriveAuthorization() {
        authorizationJob?.cancel()
        authorizationJob = null
        _uiState.value = _uiState.value.copy(authorizingDrive = false)
    }

    fun refresh() {
        if (_uiState.value.loading) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true, errorMessage = null)
            try {
                val integrations = repository.overview()
                val statistics = try { repository.operationalStatistics() } catch (_: Exception) { null }
                _uiState.value = _uiState.value.copy(
                    integrations = integrations,
                    statistics = statistics,
                    loading = false,
                )
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(
                    loading = false,
                    errorMessage = error.message ?: "服务状态读取失败",
                )
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(
                    loading = false,
                    errorMessage = "无法读取服务状态",
                )
            }
        }
    }
}
