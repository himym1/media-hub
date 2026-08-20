package com.mediahub.android.data

import com.mediahub.android.core.auth.SecureSessionStore
import com.mediahub.android.core.network.AndroidRelease
import com.mediahub.android.core.network.DownloadedAndroidRelease
import com.mediahub.android.core.network.ArchivePlan
import com.mediahub.android.core.network.ArchiveStep
import com.mediahub.android.core.network.ArchiveSuggestion
import com.mediahub.android.core.network.ApiException
import com.mediahub.android.core.network.Drive115DeviceAuthorization
import com.mediahub.android.core.network.Drive115Command
import com.mediahub.android.core.network.Drive115File
import com.mediahub.android.core.network.DiscoveryItem
import com.mediahub.android.core.network.EmbyEpisode
import com.mediahub.android.core.network.EmbyItem
import com.mediahub.android.core.network.EmbyItemDetail
import com.mediahub.android.core.network.EmbyItemPage
import com.mediahub.android.core.network.IntegrationHealth
import com.mediahub.android.core.network.MediaHubApi
import com.mediahub.android.core.network.LocalUploadEntry
import com.mediahub.android.core.network.LocalUploadJob
import com.mediahub.android.core.network.LocalUploadRoot
import com.mediahub.android.core.network.MediaLibrary
import com.mediahub.android.core.network.MediaSubscription
import com.mediahub.android.core.network.OperationalStatistics
import com.mediahub.android.core.network.ProviderSettings
import com.mediahub.android.core.network.ProviderSettingsUpdate
import com.mediahub.android.core.network.SearchResponse
import com.mediahub.android.core.network.SubscriptionInput
import com.mediahub.android.core.network.SubscriptionRun
import com.mediahub.android.core.network.TransferJob
import com.mediahub.android.core.network.TransferNotification
import java.util.UUID
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow

class MediaHubRepository(
    private val api: MediaHubApi,
    private val sessionStore: SecureSessionStore,
) {
    private val _sessionExpired = MutableSharedFlow<Unit>(extraBufferCapacity = 1)
    val sessionExpired: SharedFlow<Unit> = _sessionExpired

    suspend fun restoreSession(): Boolean {
        val token = sessionStore.load() ?: return false
        return try {
            api.session(token)
            true
        } catch (error: ApiException) {
            if (error.status == 401) {
                sessionStore.clear()
                false
            } else {
                throw error
            }
        }
    }

    suspend fun login(password: String) {
        sessionStore.save(api.login(password))
    }

    suspend fun logout() {
        val token = sessionStore.load()
        try {
            if (token != null) api.logout(token)
        } finally {
            sessionStore.clear()
        }
    }

    suspend fun changePassword(currentPassword: String, newPassword: String) =
        authenticated { api.changePassword(it, currentPassword, newPassword) }

    suspend fun latestAndroidRelease(): AndroidRelease = authenticated(api::latestAndroidRelease)
    suspend fun downloadAndroidRelease(release: AndroidRelease, destination: java.io.File): DownloadedAndroidRelease =
        authenticated { api.downloadAndroidRelease(it, release, destination) }

    suspend fun overview(): List<IntegrationHealth> = authenticated { token -> api.overview(token) }

    suspend fun operationalStatistics(): OperationalStatistics = authenticated(api::operationalStatistics)

    suspend fun startDrive115Authorization(): Drive115DeviceAuthorization = authenticated(api::startDrive115Authorization)

    suspend fun pollDrive115Authorization(id: String): Drive115DeviceAuthorization =
        authenticated { api.pollDrive115Authorization(it, id) }

    suspend fun drive115Files(parentId: String): List<Drive115File> = authenticated { api.drive115Files(it, parentId) }
    suspend fun drive115Commands(): List<Drive115Command> = authenticated(api::drive115Commands)
    suspend fun createDrive115Command(operation: String, params: Map<String, Any>): Drive115Command =
        authenticated { api.createDrive115Command(it, operation, params) }
    suspend fun confirmDrive115Command(command: Drive115Command): Drive115Command =
        authenticated { api.confirmDrive115Command(it, command) }
    suspend fun retryDrive115Command(command: Drive115Command): Drive115Command =
        authenticated { api.retryDrive115Command(it, command) }

    suspend fun localUploadRoots(): List<LocalUploadRoot> = authenticated(api::localUploadRoots)
    suspend fun localUploadFiles(rootId: String, path: String): List<LocalUploadEntry> = authenticated { api.localUploadFiles(it, rootId, path) }
    suspend fun localUploads(): List<LocalUploadJob> = authenticated(api::localUploads)
    suspend fun createLocalUpload(rootId: String, path: String, destinationId: String): LocalUploadJob = authenticated { api.createLocalUpload(it, rootId, path, destinationId) }
    suspend fun retryLocalUpload(upload: LocalUploadJob): LocalUploadJob = authenticated { api.retryLocalUpload(it, upload) }
    suspend fun previewArchive(parentId: String): List<ArchiveSuggestion> = authenticated { api.previewArchive(it, parentId) }
    suspend fun archivePlans(): List<ArchivePlan> = authenticated(api::listArchivePlans)
    suspend fun createArchivePlan(steps: List<ArchiveStep>): ArchivePlan = authenticated { api.createArchivePlan(it, steps) }
    suspend fun confirmArchivePlan(plan: ArchivePlan): ArchivePlan = authenticated { api.confirmArchivePlan(it, plan) }
    suspend fun retryArchivePlan(plan: ArchivePlan): ArchivePlan = authenticated { api.retryArchivePlan(it, plan) }

    suspend fun search(query: String): SearchResponse = authenticated { token -> api.search(token, query) }

    suspend fun trending(): List<DiscoveryItem> = authenticated { api.trending(it) }

    suspend fun recommendations(mediaType: String, tmdbId: String): List<DiscoveryItem> =
        authenticated { api.recommendations(it, mediaType, tmdbId) }

    suspend fun createTransfer(transferToken: String): TransferJob = authenticated { token ->
        api.createTransfer(token, transferToken, UUID.randomUUID().toString())
    }

    suspend fun subscriptions(): List<MediaSubscription> = authenticated { token -> api.subscriptions(token) }

    suspend fun exportSubscriptions(): String = authenticated(api::exportSubscriptions)

    suspend fun importSubscriptions(backup: String): Pair<Int, Int> =
        authenticated { api.importSubscriptions(it, backup) }

    suspend fun setSubscriptionsEnabled(ids: List<String>, enabled: Boolean): List<MediaSubscription> =
        authenticated { api.setSubscriptionsEnabled(it, ids, enabled) }

    suspend fun createSubscription(input: SubscriptionInput): MediaSubscription = authenticated { token ->
        api.createSubscription(token, input)
    }

    suspend fun updateSubscription(id: String, input: SubscriptionInput): MediaSubscription = authenticated { token ->
        api.updateSubscription(token, id, input)
    }

    suspend fun setSubscriptionEnabled(id: String, enabled: Boolean): MediaSubscription = authenticated { token ->
        api.setSubscriptionEnabled(token, id, enabled)
    }

    suspend fun runSubscription(id: String): SubscriptionRun = authenticated { token -> api.runSubscription(token, id) }

    suspend fun subscriptionRuns(id: String): List<SubscriptionRun> = authenticated { token ->
        api.subscriptionRuns(token, id)
    }

    suspend fun deleteSubscription(id: String) = authenticated { token -> api.deleteSubscription(token, id) }

    suspend fun transfers(archived: Boolean = false): List<TransferJob> = authenticated { token -> api.transfers(token, archived = archived) }

    suspend fun transfer(id: String): TransferJob = authenticated { token -> api.transfer(token, id) }

    suspend fun retryTransfer(id: String): TransferJob = authenticated { token -> api.retryTransfer(token, id) }

    suspend fun setTransferArchived(id: String, archived: Boolean): TransferJob =
        authenticated { token -> api.setTransferArchived(token, id, archived) }

    suspend fun transferNotifications(): List<TransferNotification> = authenticated(api::transferNotifications)

    suspend fun retryTransferNotification(notification: TransferNotification): TransferNotification =
        authenticated { api.retryTransferNotification(it, notification) }

    suspend fun libraries(): List<MediaLibrary> = authenticated { token -> api.libraries(token) }

    suspend fun items(query: String): List<EmbyItem> = authenticated { token -> api.items(token, query) }

    suspend fun libraryItems(libraryId: String, offset: Int, limit: Int): EmbyItemPage =
        authenticated { token -> api.libraryItems(token, libraryId, offset, limit) }

    suspend fun itemDetails(itemId: String): EmbyItemDetail = authenticated { token -> api.itemDetails(token, itemId) }

    suspend fun episodes(seriesId: String): List<EmbyEpisode> = authenticated { token -> api.episodes(token, seriesId) }

    suspend fun refreshLibrary(libraryId: String) = authenticated { token -> api.refreshLibrary(token, libraryId) }

    suspend fun refreshItem(itemId: String) = authenticated { token -> api.refreshItem(token, itemId) }

    suspend fun previewItemDelete(itemId: String) = authenticated { token -> api.previewItemDelete(token, itemId) }

    suspend fun deleteItem(itemId: String) = authenticated { token -> api.deleteItem(token, itemId) }

    suspend fun providerSettings(): ProviderSettings = authenticated(api::providerSettings)
    suspend fun updateProviderSettings(input: ProviderSettingsUpdate): ProviderSettings = authenticated { api.updateProviderSettings(it, input) }
    suspend fun testWeComNotification() = authenticated(api::testWeComNotification)

    private suspend fun <T> authenticated(block: suspend (String) -> T): T {
        val token = sessionStore.load() ?: throw ApiException(401, "authentication_required", "需要登录")
        return try {
            block(token)
        } catch (error: ApiException) {
            if (error.status == 401) {
                sessionStore.clear()
                _sessionExpired.tryEmit(Unit)
            }
            throw error
        }
    }
}
