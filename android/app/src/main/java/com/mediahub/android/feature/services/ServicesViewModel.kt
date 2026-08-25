package com.mediahub.android.feature.services

import com.mediahub.android.BuildConfig
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.mediahub.android.core.network.AndroidRelease
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.Drive115DeviceAuthorization
import com.mediahub.android.core.network.IntegrationHealth
import com.mediahub.android.core.network.OperationalStatistics
import com.mediahub.android.core.network.ProviderSettings
import com.mediahub.android.core.network.ProviderSettingsUpdate
import com.mediahub.android.core.network.ProviderSourceSettingsUpdate
import com.mediahub.android.core.network.SecretUpdate
import com.mediahub.android.core.network.SourceCheckIn
import com.mediahub.android.core.network.STRMStatus
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
    val sourceCheckIns: List<SourceCheckIn> = emptyList(),
    val strmStatus: STRMStatus? = null,
    val syncingSTRM: Boolean = false,
    val retryingCheckInId: String? = null,
    val providerSettings: ProviderSettings? = null,
    val settingsDraft: ProviderSettingsUpdate? = null,
    val settingsDirty: Boolean = false,
    val savingSettings: Boolean = false,
    val settingsSaved: Boolean = false,
    val testingWeCom: Boolean = false,
    val weComTested: Boolean = false,
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

    fun setSettingsDraft(value: ProviderSettingsUpdate) {
        _uiState.value = _uiState.value.copy(
            settingsDraft = value,
            settingsDirty = true,
            settingsSaved = false,
            weComTested = false,
        )
    }

    fun saveSettings() {
        val draft = _uiState.value.settingsDraft ?: return
        if (_uiState.value.savingSettings) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(savingSettings = true, settingsSaved = false, errorMessage = null)
            try {
                val settings = repository.updateProviderSettings(draft)
                _uiState.value = _uiState.value.copy(
                    providerSettings = settings,
                    settingsDraft = settings.toUpdate(),
                    settingsDirty = false,
                    savingSettings = false,
                    settingsSaved = true,
                )
                refresh()
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(savingSettings = false, errorMessage = error.message ?: "服务设置保存失败")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(savingSettings = false, errorMessage = "无法保存服务设置")
            }
        }
    }

    fun testWeComNotification() {
        if (_uiState.value.testingWeCom) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(testingWeCom = true, weComTested = false, errorMessage = null)
            try {
                repository.testWeComNotification()
                _uiState.value = _uiState.value.copy(testingWeCom = false, weComTested = true)
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(testingWeCom = false, errorMessage = error.message ?: "企业微信测试通知失败")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(testingWeCom = false, errorMessage = "无法发送企业微信测试通知")
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

    fun ensureLoaded() {
        if (_uiState.value.integrations.isNotEmpty() || _uiState.value.providerSettings != null) {
            refresh(showLoading = false)
        } else {
            refresh(showLoading = true)
        }
    }

    fun refresh(showLoading: Boolean = _uiState.value.integrations.isEmpty()) {
        if (_uiState.value.loading) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(
                loading = showLoading,
                errorMessage = null,
            )
            try {
                val integrations = repository.overview().filter { it.id != "qmediasync" }
                val statistics = try { repository.operationalStatistics() } catch (_: Exception) { null }
                val sourceCheckIns = try { repository.sourceCheckIns() } catch (_: Exception) { emptyList() }
                val strmStatus = try { repository.strmStatus() } catch (_: Exception) { null }
                val providerSettings = try { repository.providerSettings() } catch (_: Exception) { null }
                val current = _uiState.value
                _uiState.value = current.copy(
                    integrations = integrations,
                    statistics = statistics,
                    sourceCheckIns = sourceCheckIns,
                    strmStatus = strmStatus,
                    providerSettings = providerSettings ?: current.providerSettings,
                    settingsDraft = if (!current.settingsDirty && providerSettings != null) providerSettings.toUpdate() else current.settingsDraft,
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

    fun syncSTRMLibrary() {
        if (_uiState.value.syncingSTRM || _uiState.value.strmStatus?.running == true) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(syncingSTRM = true, errorMessage = null)
            try {
                repository.syncSTRMLibrary()
                val status = try { repository.strmStatus() } catch (_: Exception) { _uiState.value.strmStatus }
                _uiState.value = _uiState.value.copy(strmStatus = status, syncingSTRM = false)
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(syncingSTRM = false, errorMessage = error.message ?: "STRM 同步失败")
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(syncingSTRM = false, errorMessage = "无法启动 STRM 同步")
            }
        }
    }

    fun retryCheckIn(sourceId: String) {
        if (_uiState.value.retryingCheckInId != null) return
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(retryingCheckInId = sourceId, errorMessage = null)
            try {
                repository.retrySourceCheckIn(sourceId)
                val items = repository.sourceCheckIns()
                _uiState.value = _uiState.value.copy(sourceCheckIns = items, retryingCheckInId = null)
            } catch (error: ApiException) {
                _uiState.value = _uiState.value.copy(
                    retryingCheckInId = null,
                    errorMessage = error.message ?: "资源源签到失败",
                )
            } catch (_: Exception) {
                _uiState.value = _uiState.value.copy(retryingCheckInId = null, errorMessage = "无法重新签到")
            }
        }
    }
}

private fun ProviderSettings.toUpdate() = ProviderSettingsUpdate(
    qmediaSyncBaseUrl = qmediaSyncBaseUrl,
    qmediaSyncApiKey = SecretUpdate(),
    embyBaseUrl = embyBaseUrl,
    embyApiKey = SecretUpdate(),
    embyUserId = embyUserId,
    embyPassword = SecretUpdate(),
    drive115ClientId = drive115ClientId,
    tmdbBaseUrl = tmdbBaseUrl,
    tmdbAccessToken = SecretUpdate(),
    assrtBaseUrl = assrtBaseUrl,
    assrtToken = SecretUpdate(),
    wecom = wecom.toUpdate(),
    workflow = workflow.copy(syncMode = "builtin"),
    checkIn = checkIn,
    sources = sources.map { ProviderSourceSettingsUpdate(id = it.id, baseUrl = it.baseUrl, account = it.account, authMode = it.authMode) },
)

private fun com.mediahub.android.core.network.WeComSettings.toUpdate() = com.mediahub.android.core.network.WeComSettingsUpdate(
    baseUrl = baseUrl,
    corpId = corpId,
    sendMode = sendMode,
    agentId = agentId,
    toUser = if (sendMode == "appchat") toUser else toUser.ifBlank { "@all" },
    chatId = chatId,
)
