package com.mediahub.android.core.network

data class AndroidRelease(
    val versionCode: Int,
    val versionName: String,
    val minimumSupportedVersionCode: Int,
    val sha256: String,
    val sizeBytes: Long,
    val publishedAt: String,
    val notes: String,
    val downloadPath: String,
 )

data class DownloadedAndroidRelease(
    val release: AndroidRelease,
    val file: java.io.File,
 )

data class SecretStatus(val configured: Boolean)
data class SecretUpdate(val value: String = "", val clear: Boolean = false)
data class WorkflowTargetSettings(val destinationId: String, val qMediaSyncTargetPath: String, val embyLibraryId: String)
data class WorkflowSettings(
    val syncMode: String = "builtin",
    val strmBaseUrl: String = "",
    val strmRootMount: String = "",
    val qMediaSyncAccountId: Int,
    val movie: WorkflowTargetSettings,
    val series: WorkflowTargetSettings,
 )
data class ProviderSourceSettings(val id: String, val label: String, val baseUrl: String, val account: String, val authMode: String, val token: SecretStatus)
data class WeComSettings(
    val baseUrl: String,
    val corpId: String,
    val secret: SecretStatus,
    val sendMode: String,
    val agentId: Long,
    val toUser: String,
    val chatId: String,
 )
data class SharedEmbySettings(
    val baseUrl: String = "",
    val username: String = "",
    val password: SecretStatus = SecretStatus(false),
    val proxyUrl: String = "",
)
data class SharedEmbySettingsUpdate(
    val baseUrl: String = "",
    val username: String = "",
    val password: SecretUpdate = SecretUpdate(),
    val proxyUrl: String = "",
)
data class WeComSettingsUpdate(
    val baseUrl: String,
    val corpId: String,
    val secret: SecretUpdate = SecretUpdate(),
    val sendMode: String,
    val agentId: Long,
    val toUser: String,
    val chatId: String,
 )
data class ProviderSettings(
    val qmediaSyncBaseUrl: String,
    val qmediaSyncApiKey: SecretStatus,
    val embyBaseUrl: String,
    val embyApiKey: SecretStatus,
    val embyUserId: String,
    val embyPassword: SecretStatus,
    val sharedEmby: SharedEmbySettings = SharedEmbySettings(),
    val drive115ClientId: String,
    val tmdbBaseUrl: String,
    val tmdbAccessToken: SecretStatus,
    val assrtBaseUrl: String,
    val assrtToken: SecretStatus,
    val wecom: WeComSettings,
    val workflow: WorkflowSettings,
    val checkIn: CheckInSettings,
    val sources: List<ProviderSourceSettings>,
 )

data class CheckInSettings(
    val enabled: Boolean = true,
    val hour: Int = 0,
    val minute: Int = 5,
    val sources: List<String> = listOf("framehdr", "juying"),
)
data class ProviderSourceSettingsUpdate(val id: String, val baseUrl: String, val account: String, val authMode: String, val token: SecretUpdate = SecretUpdate())
data class ProviderSettingsUpdate(
    val qmediaSyncBaseUrl: String,
    val qmediaSyncApiKey: SecretUpdate,
    val embyBaseUrl: String,
    val embyApiKey: SecretUpdate,
    val embyUserId: String,
    val embyPassword: SecretUpdate,
    val sharedEmby: SharedEmbySettingsUpdate = SharedEmbySettingsUpdate(),
    val drive115ClientId: String,
    val tmdbBaseUrl: String,
    val tmdbAccessToken: SecretUpdate,
    val assrtBaseUrl: String,
    val assrtToken: SecretUpdate,
    val wecom: WeComSettingsUpdate,
    val workflow: WorkflowSettings,
    val checkIn: CheckInSettings,
    val sources: List<ProviderSourceSettingsUpdate>,
 )

data class IntegrationHealth(
    val id: String,
    val label: String,
    val status: String,
    val detail: String,
)

data class STRMStatus(
    val mode: String,
    val running: Boolean,
    val mountPath: String,
    val mountWritable: Boolean,
    val sessionOk: Boolean,
    val lastError: String = "",
    val lastSummary: String = "",
    val lastSyncAt: Long = 0,
    val lastMediaType: String = "",
)

data class SourceCheckIn(
    val sourceId: String,
    val label: String,
    val state: String,
    val message: String,
    val retryable: Boolean,
    val updatedAt: String,
)

data class OperationalStatistics(
    val transfersTotal: Int,
    val transfersActive: Int,
    val transfersCompleted: Int,
    val transfersFailed: Int,
    val transfersNeedsAttention: Int,
    val subscriptionsTotal: Int,
    val subscriptionsEnabled: Int,
    val runsTotal: Int,
    val runsFailed: Int,
    val commandsPending: Int,
    val commandsNeedsAttention: Int,
    val notificationsNeedsAttention: Int,
)

data class Drive115DeviceAuthorization(
    val id: String,
    val qrImage: String?,
    val state: String,
    val expiresAt: String,
)

data class Drive115File(
    val id: String,
    val parentId: String,
    val name: String,
    val kind: String,
    val size: Long,
    val updatedAt: Long,
)

data class Drive115Command(
    val id: String,
    val operation: String,
    val state: String,
    val attempts: Int,
    val errorMessage: String?,
    val createdAt: String,
    val updatedAt: String,
)

data class LocalUploadRoot(val id: String)
data class LocalUploadEntry(val name: String, val path: String, val directory: Boolean, val size: Long)
data class LocalUploadJob(
    val id: String,
    val path: String,
    val state: String,
    val bytesDone: Long,
    val bytesTotal: Long,
    val errorMessage: String?,
    val createdAt: String,
    val updatedAt: String,
)

data class ReleaseFacts(
    val resolution: String,
    val videoCodec: String,
    val dynamicRange: String?,
    val audio: String?,
    val sizeBytes: Long,
)

data class SearchIdentity(
    val tmdbId: String,
    val title: String,
    val originalTitle: String? = null,
    val year: Int,
    val mediaType: String,
    val posterUrl: String? = null,
    val rating: Double? = null,
    val overview: String? = null,
)

data class SearchCandidate(
    val id: String,
    val title: String,
    val year: Int,
    val season: Int,
    val episodeStart: Int = 0,
    val episodeEnd: Int = 0,
    val mediaType: String,
    val tmdbId: String?,
    val source: String,
    val sourceId: String = "",
    val provider: String? = null,
    val posterUrl: String?,
    val rating: Double? = null,
    val overview: String? = null,
    val release: ReleaseFacts,
    val transferState: String,
    val transferToken: String?,
)

data class DiscoveryItem(
    val tmdbId: String,
    val title: String,
    val year: Int,
    val mediaType: String,
    val posterUrl: String?,
)

data class DiscoveryGenre(
    val id: Int,
    val name: String,
)

data class SourceError(
    val source: String,
    val code: String,
    val message: String,
    val retryable: Boolean,
)

data class SearchResponse(
    val query: String,
    val partial: Boolean,
    val identities: List<SearchIdentity> = emptyList(),
    val results: List<SearchCandidate>,
    val sourceErrors: List<SourceError>,
)

data class TransferJob(
    val id: String,
    val title: String,
    val year: Int,
    val season: Int,
    val episodeStart: Int = 0,
    val episodeEnd: Int = 0,
    val mediaType: String,
    val tmdbId: String,
    val source: String,
    val state: String,
    val errorCode: String?,
    val errorMessage: String?,
    val retryable: Boolean,
    val archived: Boolean,
    val createdAt: String,
    val updatedAt: String,
    val events: List<TransferEvent> = emptyList(),
)

data class TransferNotification(
    val id: String,
    val jobId: String,
    val eventType: String,
    val jobState: String,
    val title: String,
    val state: String,
    val attempts: Int,
    val createdAt: String,
    val updatedAt: String,
)

data class SubscriptionPreferences(
    val resolutions: List<String> = emptyList(),
    val videoCodecs: List<String> = emptyList(),
    val dynamicRanges: List<String> = emptyList(),
    val audioContains: List<String> = emptyList(),
    val preferredSources: List<String> = emptyList(),
    val minSizeBytes: Long = 0,
    val maxSizeBytes: Long = 0,
    val allowUnknownSize: Boolean = false,
    val preferSmaller: Boolean = false,
)

data class SubscriptionInput(
    val tmdbId: String,
    val title: String,
    val originalTitle: String,
    val year: Int,
    val mediaType: String,
    val season: Int,
    val policy: String,
    val enabled: Boolean,
    val intervalMinutes: Int,
    val sourceIds: List<String>,
    val preferences: SubscriptionPreferences,
)

data class MediaSubscription(
    val id: String,
    val tmdbId: String,
    val title: String,
    val originalTitle: String,
    val year: Int,
    val mediaType: String,
    val season: Int,
    val lastEpisode: Int = 0,
    val policy: String,
    val enabled: Boolean,
    val intervalMinutes: Int,
    val sourceIds: List<String>,
    val preferences: SubscriptionPreferences,
    val nextRunAt: String,
    val lastRunAt: String?,
    val createdAt: String,
    val updatedAt: String,
)

data class SubscriptionRun(
    val id: String,
    val subscriptionId: String,
    val triggerType: String,
    val state: String,
    val sourceId: String?,
    val transferJobId: String?,
    val errorCode: String?,
    val message: String?,
    val retryable: Boolean,
    val startedAt: String,
    val finishedAt: String?,
    val updatedAt: String,
)


data class TransferEvent(
    val id: Long,
    val state: String,
    val message: String,
    val createdAt: String,
)

data class MediaLibrary(
    val id: String,
    val name: String,
    val collectionType: String?,
    val parentId: String? = null,
)

data class EmbyItem(
    val id: String,
    val name: String,
    val type: String,
    val year: Int?,
    val tmdbId: String?,
    val season: Int = 0,
    val episode: Int = 0,
    val playbackPositionMs: Long = 0,
    val played: Boolean = false,
)

data class EmbyItemPage(
    val items: List<EmbyItem>,
    val total: Int,
)

data class EmbyEpisode(
    val item: EmbyItem,
    val externalUrl: String,
    val appUrl: String?,
)

data class EmbyItemDetail(
    val item: EmbyItem,
    val originalTitle: String?,
    val overview: String?,
    val communityRating: Double?,
    val runtimeMinutes: Int?,
    val genres: List<String>,
    val mediaSourceCount: Int,
    val externalUrl: String,
    val appUrl: String? = null,
)

data class EmbyDeletePreview(
    val id: String,
    val name: String,
    val type: String,
    val fileCount: Int,
    val deletesFiles: Boolean,
    val cloudKept: Boolean,
    val versionCount: Int = 1,
)

data class EmbyRemoteSubtitle(
    val id: String,
    val name: String,
    val language: String = "",
    val format: String = "",
    val providerName: String = "",
    val author: String = "",
    val comment: String = "",
    val communityRating: Double? = null,
    val downloadCount: Int = 0,
    val isHashMatch: Boolean = false,
    val hearingImpaired: Boolean = false,
    val forced: Boolean = false,
)


class ApiException(
    val status: Int,
    val code: String,
    message: String,
) : Exception(message)

data class DownloadedFile(
    val bytes: ByteArray,
    val contentType: String,
    val fileName: String,
)

data class ArchiveSuggestion(val fileId: String, val currentName: String, val suggestedName: String, val kind: String, val confidence: String)
data class ArchiveStep(val operation: String, val fileId: String, val name: String = "", val targetParentId: String = "")
data class ArchivePlan(val id: String, val state: String, val stepIndex: Int, val stepTotal: Int, val errorMessage: String, val createdAt: String)
