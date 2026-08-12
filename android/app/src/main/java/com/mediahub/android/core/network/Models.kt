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

data class IntegrationHealth(
    val id: String,
    val label: String,
    val status: String,
    val detail: String,
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

data class SourceError(
    val source: String,
    val code: String,
    val message: String,
    val retryable: Boolean,
)

data class SearchResponse(
    val query: String,
    val partial: Boolean,
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
)

data class EmbyItem(
    val id: String,
    val name: String,
    val type: String,
    val year: Int?,
    val tmdbId: String?,
)


class ApiException(
    val status: Int,
    val code: String,
    message: String,
) : Exception(message)


data class SubXMigrationReadiness(
    val canStopSubX: Boolean,
    val subXConfigured: Boolean,
    val fallbackSourceEnabled: Boolean,
    val coreConfigurationReady: Boolean,
    val nativeSourceCount: Int,
    val parallelValidationCompleted: Boolean,
    val nativeSubscriptions: Int,
    val delegatedOperations: Int,
    val delegatedGroups: List<String>,
    val blockers: List<String>,
)

data class SubXMigrationResult(
    val detected: Int,
    val importable: Int,
    val rejected: Int,
    val created: Int,
    val skipped: Int,
)

data class MigrationSourceCommand(val id: String, val operationId: String, val state: String, val attempts: Int, val errorMessage: String, val createdAt: String)

data class ArchiveSuggestion(val fileId: String, val currentName: String, val suggestedName: String, val kind: String, val confidence: String)
data class ArchiveStep(val operation: String, val fileId: String, val name: String = "", val targetParentId: String = "")
data class ArchivePlan(val id: String, val state: String, val stepIndex: Int, val stepTotal: Int, val errorMessage: String, val createdAt: String)
