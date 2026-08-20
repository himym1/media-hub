package com.mediahub.android.core.network

import com.mediahub.android.BuildConfig
import java.io.ByteArrayOutputStream
import java.io.IOException
import java.io.File
import java.io.InputStream
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder
import java.util.UUID
import java.security.MessageDigest
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject

class MediaHubApi(private val http: MediaHubHttpClient) {
    constructor(baseUrl: String) : this(MediaHubHttpClient(baseUrl))

    private val baseUrl: String
		get() = http.baseUrl

    suspend fun login(password: String): String {
        val body = JSONObject()
            .put("password", password)
            .put("client", "android")
            .toString()
        val response = request("/api/v1/auth/login", method = "POST", body = body)
        return JSONObject(response).getString("token")
    }

    suspend fun session(token: String) {
        request("/api/v1/auth/session", token = token)
    }
    suspend fun latestAndroidRelease(token: String): AndroidRelease {
        val value = JSONObject(request("/api/v1/client/android/releases/latest", token = token))
        return parseAndroidRelease(value)
    }

    suspend fun downloadAndroidRelease(token: String, release: AndroidRelease, destination: File): DownloadedAndroidRelease =
        withContext(Dispatchers.IO) {
            val path = release.downloadPath
            require(path == "/api/v1/client/android/releases/${release.versionCode}/apk") { "Invalid Android release download path" }
            val connection = URL(baseUrl + path).openConnection() as HttpURLConnection
            try {
                connection.requestMethod = "GET"
                connection.connectTimeout = DOWNLOAD_CONNECT_TIMEOUT_MS
                connection.readTimeout = DOWNLOAD_TIMEOUT_MS
                connection.instanceFollowRedirects = false
                connection.setRequestProperty("Accept", "application/vnd.android.package-archive")
                connection.setRequestProperty("User-Agent", "Media-Hub-Android/${BuildConfig.VERSION_NAME}")
                connection.setRequestProperty("Authorization", "Bearer $token")
                val status = connection.responseCode
                if (status !in 200..299) {
                    throw parseError(status, connection.errorStream.readLimited(MAX_RESPONSE_BYTES))
                }
                destination.parentFile?.mkdirs()
                val digest = MessageDigest.getInstance("SHA-256")
                var total = 0L
                destination.outputStream().buffered().use { output ->
                    connection.inputStream.use { input ->
                        val buffer = ByteArray(64 * 1024)
                        while (true) {
                            val count = input.read(buffer)
                            if (count < 0) break
                            total += count
                            if (total > release.sizeBytes) throw IOException("Android update exceeded declared size")
                            digest.update(buffer, 0, count)
                            output.write(buffer, 0, count)
                        }
                    }
                }
                val checksum = digest.digest().joinToString("") { "%02x".format(it) }
                if (total != release.sizeBytes || !checksum.equals(release.sha256, ignoreCase = true)) {
                    destination.delete()
                    throw IOException("Android update verification failed")
                }
                DownloadedAndroidRelease(release, destination)
            } catch (error: Exception) {
                destination.delete()
                throw error
            } finally {
                connection.disconnect()
            }
        }

    private fun parseAndroidRelease(value: JSONObject) = AndroidRelease(
        versionCode = value.getInt("versionCode"),
        versionName = value.getString("versionName"),
        minimumSupportedVersionCode = value.getInt("minimumSupportedVersionCode"),
        sha256 = value.getString("sha256"),
        sizeBytes = value.getLong("sizeBytes"),
        publishedAt = value.getString("publishedAt"),
        notes = value.getString("notes"),
        downloadPath = value.getString("downloadPath"),
    )


    suspend fun logout(token: String) {
        request("/api/v1/auth/logout", method = "POST", token = token)
    }

    suspend fun changePassword(token: String, currentPassword: String, newPassword: String) {
        val body = JSONObject()
            .put("currentPassword", currentPassword)
            .put("newPassword", newPassword)
            .toString()
        request("/api/v1/auth/password", method = "PUT", body = body, token = token)
    }

    suspend fun overview(token: String): List<IntegrationHealth> {
        val payload = JSONObject(request("/api/v1/system/overview", token = token))
        val items = payload.getJSONArray("integrations")
        return buildList(items.length()) {
            for (index in 0 until items.length()) {
                val item = items.getJSONObject(index)
                add(
                    IntegrationHealth(
                        id = item.getString("id"),
                        label = item.getString("label"),
                        status = item.getString("status"),
                        detail = item.getString("detail"),
                    ),
                )
            }
        }
    }
    suspend fun providerSettings(token: String): ProviderSettings = parseProviderSettings(
        JSONObject(request("/api/v1/settings/providers", token = token)),
    )

    suspend fun updateProviderSettings(token: String, input: ProviderSettingsUpdate): ProviderSettings = parseProviderSettings(
        JSONObject(request(
            "/api/v1/settings/providers",
            method = "PUT",
            token = token,
            body = providerSettingsBody(input).toString(),
        )),
    )

    suspend fun testWeComNotification(token: String) {
        request("/api/v1/integrations/wecom/test", method = "POST", token = token)
    }


    suspend fun operationalStatistics(token: String): OperationalStatistics {
        val item = JSONObject(request("/api/v1/statistics/summary", token = token))
        return OperationalStatistics(
            transfersTotal = item.getInt("transfersTotal"),
            transfersActive = item.getInt("transfersActive"),
            transfersCompleted = item.getInt("transfersCompleted"),
            transfersFailed = item.getInt("transfersFailed"),
            transfersNeedsAttention = item.getInt("transfersNeedsAttention"),
            subscriptionsTotal = item.getInt("subscriptionsTotal"),
            subscriptionsEnabled = item.getInt("subscriptionsEnabled"),
            runsTotal = item.getInt("runsTotal"),
            runsFailed = item.getInt("runsFailed"),
            commandsPending = item.getInt("commandsPending"),
            commandsNeedsAttention = item.getInt("commandsNeedsAttention"),
            notificationsNeedsAttention = item.getInt("notificationsNeedsAttention"),
        )
    }

    suspend fun startDrive115Authorization(token: String): Drive115DeviceAuthorization =
        parseDrive115Authorization(JSONObject(request(
            "/api/v1/integrations/115/auth/device", method = "POST", token = token,
        )))

    suspend fun pollDrive115Authorization(token: String, id: String): Drive115DeviceAuthorization =
        parseDrive115Authorization(JSONObject(request(
            "/api/v1/integrations/115/auth/device/${encode(id)}", token = token,
        )))

    suspend fun drive115Files(token: String, parentId: String): List<Drive115File> {
        val root = JSONObject(request("/api/v1/integrations/115/files?parentId=${encode(parentId)}&limit=100", token = token))
        return root.getJSONArray("items").objects(::parseDrive115File)
    }

    suspend fun drive115Commands(token: String): List<Drive115Command> {
        val root = JSONObject(request("/api/v1/integrations/115/commands?limit=100", token = token))
        return root.getJSONArray("commands").objects(::parseDrive115Command)
    }

    suspend fun createDrive115Command(token: String, operation: String, params: Map<String, Any>): Drive115Command {
        val body = JSONObject().put("operation", operation).put("params", JSONObject(params)).toString()
        return parseDrive115Command(JSONObject(request(
            "/api/v1/integrations/115/commands", method = "POST", body = body, token = token,
            headers = mapOf("Idempotency-Key" to UUID.randomUUID().toString()),
        )))
    }

    suspend fun confirmDrive115Command(token: String, command: Drive115Command): Drive115Command =
        parseDrive115Command(JSONObject(request(
            "/api/v1/integrations/115/commands/${encode(command.id)}/confirm", method = "POST", token = token,
            body = JSONObject().put("confirmation", command.id).toString(),
        )))

    suspend fun retryDrive115Command(token: String, command: Drive115Command): Drive115Command =
        parseDrive115Command(JSONObject(request(
            "/api/v1/integrations/115/commands/${encode(command.id)}/retry", method = "POST", token = token,
            body = JSONObject().put("confirmation", command.id).toString(),
        )))

    suspend fun localUploadRoots(token: String): List<LocalUploadRoot> {
        val root = JSONObject(request("/api/v1/local-uploads/roots", token = token))
        return root.getJSONArray("roots").objects { LocalUploadRoot(it.getString("id")) }
    }
    suspend fun localUploadFiles(token: String, rootId: String, path: String): List<LocalUploadEntry> {
        val root = JSONObject(request("/api/v1/local-uploads/files?rootId=${encode(rootId)}&path=${encode(path)}", token = token))
        return root.getJSONArray("items").objects(::parseLocalUploadEntry)
    }
    suspend fun localUploads(token: String): List<LocalUploadJob> {
        val root = JSONObject(request("/api/v1/local-uploads?limit=100", token = token))
        return root.getJSONArray("uploads").objects(::parseLocalUploadJob)
    }
    suspend fun createLocalUpload(token: String, rootId: String, path: String, destinationId: String): LocalUploadJob =
        parseLocalUploadJob(JSONObject(request(
            "/api/v1/local-uploads", method = "POST", token = token,
            body = JSONObject().put("rootId", rootId).put("path", path).put("destinationId", destinationId).toString(),
            headers = mapOf("Idempotency-Key" to UUID.randomUUID().toString()),
        )))
    suspend fun retryLocalUpload(token: String, upload: LocalUploadJob): LocalUploadJob =
        parseLocalUploadJob(JSONObject(request(
            "/api/v1/local-uploads/${encode(upload.id)}/retry", method = "POST", token = token,
            body = JSONObject().put("confirmation", upload.id).toString(),
        )))

    suspend fun previewArchive(token: String, parentId: String): List<ArchiveSuggestion> {
        val root = JSONObject(request("/api/v1/archive/preview", method = "POST", token = token, body = JSONObject().put("parentId", parentId).toString()))
        return root.getJSONArray("suggestions").objects(::parseArchiveSuggestion)
    }
    suspend fun listArchivePlans(token: String): List<ArchivePlan> {
        val root = JSONObject(request("/api/v1/archive/plans", token = token))
        return root.getJSONArray("plans").objects(::parseArchivePlan)
    }
    suspend fun createArchivePlan(token: String, steps: List<ArchiveStep>): ArchivePlan {
        val values = JSONArray(); steps.forEach { values.put(JSONObject().put("operation", it.operation).put("fileId", it.fileId).apply { if (it.name.isNotBlank()) put("name", it.name); if (it.targetParentId.isNotBlank()) put("targetParentId", it.targetParentId) }) }
        return parseArchivePlan(JSONObject(request("/api/v1/archive/plans", method = "POST", token = token, body = JSONObject().put("steps", values).toString())))
    }
    suspend fun confirmArchivePlan(token: String, plan: ArchivePlan): ArchivePlan = changeArchivePlan(token, plan, "confirm")
    suspend fun retryArchivePlan(token: String, plan: ArchivePlan): ArchivePlan = changeArchivePlan(token, plan, "retry")
    private suspend fun changeArchivePlan(token: String, plan: ArchivePlan, action: String) = parseArchivePlan(JSONObject(request("/api/v1/archive/plans/${encode(plan.id)}/$action", method = "POST", token = token, body = JSONObject().put("confirmation", plan.id).toString())))

    suspend fun libraries(token: String): List<MediaLibrary> {
        val payload = JSONObject(request("/api/v1/integrations/emby/libraries", token = token))
        val items = payload.getJSONArray("libraries")
        return buildList(items.length()) {
            for (index in 0 until items.length()) {
                val item = items.getJSONObject(index)
                add(
                    MediaLibrary(
                        id = item.getString("id"),
                        name = item.getString("name"),
                        collectionType = item.optionalString("collectionType"),
                    ),
                )
            }
        }
    }

    suspend fun items(token: String, query: String): List<EmbyItem> {
        val encodedQuery = URLEncoder.encode(query, Charsets.UTF_8.name())
        return parseEmbyPage(JSONObject(request("/api/v1/integrations/emby/items?query=$encodedQuery&limit=50", token = token))).items
    }

    suspend fun libraryItems(token: String, libraryId: String, offset: Int, limit: Int): EmbyItemPage =
        parseEmbyPage(JSONObject(request(
            "/api/v1/integrations/emby/libraries/${encode(libraryId)}/items?offset=$offset&limit=$limit",
            token = token,
        )))

    suspend fun itemDetails(token: String, itemId: String): EmbyItemDetail {
        val payload = JSONObject(request("/api/v1/integrations/emby/items/${encode(itemId)}", token = token))
        return EmbyItemDetail(
            item = parseEmbyItem(payload),
            originalTitle = payload.optionalString("originalTitle"),
            overview = payload.optionalString("overview"),
            communityRating = if (payload.has("communityRating")) payload.getDouble("communityRating") else null,
            runtimeMinutes = if (payload.has("runtimeMinutes")) payload.getInt("runtimeMinutes") else null,
            genres = payload.optJSONArray("genres")?.strings().orEmpty(),
            mediaSourceCount = payload.getInt("mediaSourceCount"),
            externalUrl = payload.getString("externalUrl"),
            appUrl = payload.optionalString("appUrl"),
        )
    }

    suspend fun episodes(token: String, seriesId: String): List<EmbyEpisode> {
        val payload = JSONObject(request(
            "/api/v1/integrations/emby/items/${encode(seriesId)}/episodes",
            token = token,
        ))
        return payload.getJSONArray("items").objects { value ->
            EmbyEpisode(
                item = parseEmbyItem(value),
                externalUrl = value.getString("externalUrl"),
                appUrl = value.optionalString("appUrl"),
            )
        }
    }

    suspend fun refreshLibrary(token: String, libraryId: String) {
        request("/api/v1/integrations/emby/libraries/${encode(libraryId)}/refresh", method = "POST", token = token)
    }

    suspend fun refreshItem(token: String, itemId: String) {
        request("/api/v1/integrations/emby/items/${encode(itemId)}/refresh", method = "POST", token = token)
    }

    suspend fun previewItemDelete(token: String, itemId: String): EmbyDeletePreview {
        val payload = JSONObject(request("/api/v1/integrations/emby/items/${encode(itemId)}/delete-preview", token = token))
        return EmbyDeletePreview(
            id = payload.getString("id"),
            name = payload.getString("name"),
            type = payload.getString("type"),
            fileCount = payload.getInt("fileCount"),
            deletesFiles = payload.getBoolean("deletesFiles"),
            cloudKept = payload.getBoolean("cloudKept"),
        )
    }

    suspend fun deleteItem(token: String, itemId: String) {
        request(
            "/api/v1/integrations/emby/items/${encode(itemId)}/delete",
            method = "POST",
            token = token,
            body = JSONObject().put("confirmation", itemId).toString(),
        )
    }


    suspend fun search(token: String, query: String): SearchResponse {
        val encodedQuery = URLEncoder.encode(query, Charsets.UTF_8.name())
        val payload = JSONObject(request("/api/v1/search?query=$encodedQuery", token = token))
        val results = payload.getJSONArray("results")
        val sourceErrors = payload.getJSONArray("sourceErrors")
        return SearchResponse(
            query = payload.getString("query"),
            partial = payload.getBoolean("partial"),
            results = buildList(results.length()) {
                for (index in 0 until results.length()) add(parseCandidate(results.getJSONObject(index)))
            },
            sourceErrors = buildList(sourceErrors.length()) {
                for (index in 0 until sourceErrors.length()) {
                    val item = sourceErrors.getJSONObject(index)
                    add(
                        SourceError(
                            source = item.getString("source"),
                            code = item.getString("code"),
                            message = item.getString("message"),
                            retryable = item.getBoolean("retryable"),
                        ),
                    )
                }
            },
        )
    }

    suspend fun trending(token: String, limit: Int = 12): List<DiscoveryItem> {
        val payload = JSONObject(request("/api/v1/discovery/trending?mediaType=all&limit=$limit", token = token))
        return payload.getJSONArray("items").objects(::parseDiscoveryItem)
    }

    suspend fun recommendations(token: String, mediaType: String, tmdbId: String, limit: Int = 12): List<DiscoveryItem> {
        val payload = JSONObject(request(
            "/api/v1/discovery/${encode(mediaType)}/${encode(tmdbId)}/recommendations?limit=$limit",
            token = token,
        ))
        return payload.getJSONArray("items").objects(::parseDiscoveryItem)
    }

    suspend fun createTransfer(token: String, transferToken: String, idempotencyKey: String): TransferJob {
        val body = JSONObject().put("transferToken", transferToken).toString()
        val response = request(
            path = "/api/v1/transfers",
            method = "POST",
            body = body,
            token = token,
            headers = mapOf("Idempotency-Key" to idempotencyKey),
        )
        return parseTransferJob(JSONObject(response))
    }

    suspend fun transfers(token: String, limit: Int = 50, archived: Boolean = false): List<TransferJob> {
        val payload = JSONObject(request("/api/v1/transfers?limit=$limit&archived=$archived", token = token))
        val items = payload.getJSONArray("transfers")
        return buildList(items.length()) {
            for (index in 0 until items.length()) add(parseTransferJob(items.getJSONObject(index)))
        }
    }

    suspend fun transfer(token: String, id: String): TransferJob {
        return parseTransferJob(JSONObject(request("/api/v1/transfers/$id", token = token)))
    }

    suspend fun retryTransfer(token: String, id: String): TransferJob {
        return parseTransferJob(JSONObject(request("/api/v1/transfers/$id/retry", method = "POST", token = token)))
    }

    suspend fun setTransferArchived(token: String, id: String, archived: Boolean): TransferJob {
        val body = JSONObject().put("archived", archived).toString()
        return parseTransferJob(JSONObject(request(
            "/api/v1/transfers/${encode(id)}/archived", method = "PATCH", body = body, token = token,
        )))
    }

    suspend fun transferNotifications(token: String): List<TransferNotification> {
        val root = JSONObject(request("/api/v1/notifications?limit=100", token = token))
        return root.getJSONArray("notifications").objects(::parseTransferNotification)
    }

    suspend fun retryTransferNotification(token: String, notification: TransferNotification): TransferNotification {
        val body = JSONObject().put("confirm", notification.id).toString()
        return parseTransferNotification(JSONObject(request(
            "/api/v1/notifications/${encode(notification.jobId)}/${encode(notification.eventType)}/retry",
            method = "POST",
            body = body,
            token = token,
        )))
    }

    suspend fun subscriptions(token: String): List<MediaSubscription> {
        val payload = JSONObject(request("/api/v1/subscriptions", token = token))
        val items = payload.getJSONArray("subscriptions")
        return buildList(items.length()) {
            for (index in 0 until items.length()) add(parseSubscription(items.getJSONObject(index)))
        }
    }

    suspend fun exportSubscriptions(token: String): String =
        request("/api/v1/subscriptions/backup", token = token)

    suspend fun importSubscriptions(token: String, backup: String): Pair<Int, Int> {
        val result = JSONObject(request("/api/v1/subscriptions/backup", method = "POST", body = backup, token = token))
        return result.getInt("created") to result.getInt("skipped")
    }

    suspend fun setSubscriptionsEnabled(token: String, ids: List<String>, enabled: Boolean): List<MediaSubscription> {
        val body = JSONObject().put("ids", JSONArray(ids)).put("enabled", enabled).toString()
        val payload = JSONObject(request("/api/v1/subscriptions/batch/enabled", method = "POST", body = body, token = token))
        return payload.getJSONArray("subscriptions").objects(::parseSubscription)
    }

    suspend fun createSubscription(token: String, input: SubscriptionInput): MediaSubscription {
        val response = request("/api/v1/subscriptions", method = "POST", body = subscriptionBody(input, true), token = token)
        return parseSubscription(JSONObject(response))
    }

    suspend fun updateSubscription(token: String, id: String, input: SubscriptionInput): MediaSubscription {
        val response = request("/api/v1/subscriptions/${encode(id)}", method = "PUT", body = subscriptionBody(input, false), token = token)
        return parseSubscription(JSONObject(response))
    }

    suspend fun setSubscriptionEnabled(token: String, id: String, enabled: Boolean): MediaSubscription {
        val body = JSONObject().put("enabled", enabled).toString()
        return parseSubscription(JSONObject(request("/api/v1/subscriptions/${encode(id)}/enabled", method = "PATCH", body = body, token = token)))
    }

    suspend fun runSubscription(token: String, id: String): SubscriptionRun {
        return parseSubscriptionRun(JSONObject(request("/api/v1/subscriptions/${encode(id)}/runs", method = "POST", token = token)))
    }

    suspend fun subscriptionRuns(token: String, id: String): List<SubscriptionRun> {
        val payload = JSONObject(request("/api/v1/subscriptions/${encode(id)}/runs?limit=50", token = token))
        val items = payload.getJSONArray("runs")
        return buildList(items.length()) {
            for (index in 0 until items.length()) add(parseSubscriptionRun(items.getJSONObject(index)))
        }
    }

    suspend fun deleteSubscription(token: String, id: String) {
        request("/api/v1/subscriptions/${encode(id)}", method = "DELETE", token = token)
    }

    private fun parseArchiveSuggestion(item: JSONObject) = ArchiveSuggestion(item.getString("fileId"), item.getString("currentName"), item.getString("suggestedName"), item.getString("kind"), item.getString("confidence"))
    private fun parseArchivePlan(item: JSONObject) = ArchivePlan(item.getString("id"), item.getString("state"), item.getInt("stepIndex"), item.getInt("stepTotal"), item.optString("errorMessage"), item.getString("createdAt"))

    private fun parseLocalUploadEntry(item: JSONObject) = LocalUploadEntry(
        name = item.getString("name"), path = item.getString("path"),
        directory = item.getBoolean("directory"), size = item.getLong("size"),
    )
    private fun parseLocalUploadJob(item: JSONObject) = LocalUploadJob(
        id = item.getString("id"), path = item.optString("path"), state = item.getString("state"),
        bytesDone = item.getLong("bytesDone"), bytesTotal = item.getLong("bytesTotal"),
        errorMessage = item.optionalString("errorMessage"), createdAt = item.getString("createdAt"), updatedAt = item.getString("updatedAt"),
    )

    private fun parseDrive115File(item: JSONObject) = Drive115File(
        id = item.getString("id"),
        parentId = item.getString("parentId"),
        name = item.getString("name"),
        kind = item.getString("kind"),
        size = item.getLong("size"),
        updatedAt = item.getLong("updatedAt"),
    )

    private fun parseDrive115Command(item: JSONObject) = Drive115Command(
        id = item.getString("id"),
        operation = item.getString("operation"),
        state = item.getString("state"),
        attempts = item.getInt("attempts"),
        errorMessage = item.optionalString("errorMessage"),
        createdAt = item.getString("createdAt"),
        updatedAt = item.getString("updatedAt"),
    )

    private fun parseDrive115Authorization(item: JSONObject) = Drive115DeviceAuthorization(
        id = item.getString("id"),
        qrImage = item.optionalString("qrImage"),
        state = item.getString("state"),
        expiresAt = item.getString("expiresAt"),
    )

    private fun parseDiscoveryItem(item: JSONObject) = DiscoveryItem(
        tmdbId = item.getString("tmdbId"),
        title = item.getString("title"),
        year = item.getInt("year"),
        mediaType = item.getString("mediaType"),
        posterUrl = item.optionalString("posterUrl"),
    )

    private fun parseTransferNotification(item: JSONObject) = TransferNotification(
        id = item.getString("id"),
        jobId = item.getString("jobId"),
        eventType = item.getString("eventType"),
        jobState = item.getString("jobState"),
        title = item.getString("title"),
        state = item.getString("state"),
        attempts = item.getInt("attempts"),
        createdAt = item.getString("createdAt"),
        updatedAt = item.getString("updatedAt"),
    )

    private fun parseCandidate(item: JSONObject): SearchCandidate {
        val release = item.getJSONObject("release")
        return SearchCandidate(
            id = item.getString("id"),
            title = item.getString("title"),
            year = item.getInt("year"),
            season = item.optInt("season", 0),
            episodeStart = item.optInt("episodeStart", 0),
            episodeEnd = item.optInt("episodeEnd", 0),
            mediaType = item.getString("mediaType"),
            tmdbId = item.optionalString("tmdbId"),
            source = item.getString("source"),
            sourceId = item.getString("sourceId"),
            provider = item.optionalString("provider"),
            posterUrl = item.optionalString("posterUrl"),
            release = ReleaseFacts(
                resolution = release.getString("resolution"),
                videoCodec = release.getString("videoCodec"),
                dynamicRange = release.optionalString("dynamicRange"),
                audio = release.optionalString("audio"),
                sizeBytes = release.getLong("sizeBytes"),
            ),
            transferState = item.getString("transferState"),
            transferToken = item.optionalString("transferToken"),
        )
    }

    private fun parseEmbyPage(payload: JSONObject): EmbyItemPage {
        val items = payload.getJSONArray("items")
        return EmbyItemPage(
            items = buildList(items.length()) {
                for (index in 0 until items.length()) add(parseEmbyItem(items.getJSONObject(index)))
            },
            total = payload.getInt("total"),
        )
    }

    private fun parseEmbyItem(item: JSONObject) = EmbyItem(
        id = item.getString("id"),
        name = item.getString("name"),
        type = item.getString("type"),
        year = if (item.has("year")) item.getInt("year") else null,
        tmdbId = item.optJSONObject("providerIds")?.optionalString("Tmdb"),
        season = item.optInt("season", 0),
        episode = item.optInt("episode", 0),
        playbackPositionMs = item.optLong("playbackPositionMs", 0L).coerceAtLeast(0L),
        played = item.optBoolean("played", false),
    )


    private fun parseTransferJob(item: JSONObject): TransferJob {
        val events = item.optJSONArray("events")
        return TransferJob(
            id = item.getString("id"),
            title = item.getString("title"),
            year = item.getInt("year"),
            season = item.optInt("season", 0),
            episodeStart = item.optInt("episodeStart", 0),
            episodeEnd = item.optInt("episodeEnd", 0),
            mediaType = item.getString("mediaType"),
            tmdbId = item.getString("tmdbId"),
            source = item.getString("source"),
            state = item.getString("state"),
            errorCode = item.optionalString("errorCode"),
            errorMessage = item.optionalString("errorMessage"),
            retryable = item.getBoolean("retryable"),
            archived = item.optBoolean("archived", false),
            createdAt = item.getString("createdAt"),
            updatedAt = item.getString("updatedAt"),
            events = if (events == null) emptyList() else buildList(events.length()) {
                for (index in 0 until events.length()) {
                    val event = events.getJSONObject(index)
                    add(
                        TransferEvent(
                            id = event.getLong("id"),
                            state = event.getString("state"),
                            message = event.getString("message"),
                            createdAt = event.getString("createdAt"),
                        ),
                    )
                }
            },
        )
    }


    private fun subscriptionBody(input: SubscriptionInput, includeIdentity: Boolean): String {
        val preferences = JSONObject()
            .put("resolutions", JSONArray(input.preferences.resolutions))
            .put("videoCodecs", JSONArray(input.preferences.videoCodecs))
            .put("dynamicRanges", JSONArray(input.preferences.dynamicRanges))
            .put("audioContains", JSONArray(input.preferences.audioContains))
            .put("preferredSources", JSONArray(input.preferences.preferredSources))
            .put("minSizeBytes", input.preferences.minSizeBytes)
            .put("maxSizeBytes", input.preferences.maxSizeBytes)
            .put("allowUnknownSize", input.preferences.allowUnknownSize)
            .put("preferSmaller", input.preferences.preferSmaller)
        val body = JSONObject()
            .put("title", input.title)
            .put("originalTitle", input.originalTitle)
            .put("year", input.year)
            .put("policy", input.policy)
            .put("enabled", input.enabled)
            .put("intervalMinutes", input.intervalMinutes)
            .put("sourceIds", JSONArray(input.sourceIds))
            .put("preferences", preferences)
        if (includeIdentity) {
            body.put("tmdbId", input.tmdbId)
                .put("mediaType", input.mediaType)
                .put("season", input.season)
        }
        return body.toString()
    }

    private fun parseSubscription(item: JSONObject): MediaSubscription {
        val preferences = item.getJSONObject("preferences")
        return MediaSubscription(
            id = item.getString("id"),
            tmdbId = item.getString("tmdbId"),
            title = item.getString("title"),
            originalTitle = item.optionalString("originalTitle") ?: "",
            year = item.getInt("year"),
            mediaType = item.getString("mediaType"),
            season = item.optInt("season", 0),
            lastEpisode = item.optInt("lastEpisode", 0),
            policy = item.getString("policy"),
            enabled = item.getBoolean("enabled"),
            intervalMinutes = item.getInt("intervalMinutes"),
            sourceIds = item.getJSONArray("sourceIds").strings(),
            preferences = SubscriptionPreferences(
                resolutions = preferences.getJSONArray("resolutions").strings(),
                videoCodecs = preferences.getJSONArray("videoCodecs").strings(),
                dynamicRanges = preferences.getJSONArray("dynamicRanges").strings(),
                audioContains = preferences.getJSONArray("audioContains").strings(),
                preferredSources = preferences.getJSONArray("preferredSources").strings(),
                minSizeBytes = preferences.getLong("minSizeBytes"),
                maxSizeBytes = preferences.getLong("maxSizeBytes"),
                allowUnknownSize = preferences.getBoolean("allowUnknownSize"),
                preferSmaller = preferences.getBoolean("preferSmaller"),
            ),
            nextRunAt = item.getString("nextRunAt"),
            lastRunAt = item.optionalString("lastRunAt"),
            createdAt = item.getString("createdAt"),
            updatedAt = item.getString("updatedAt"),
        )
    }

    private fun parseSubscriptionRun(item: JSONObject) = SubscriptionRun(
        id = item.getString("id"),
        subscriptionId = item.getString("subscriptionId"),
        triggerType = item.getString("triggerType"),
        state = item.getString("state"),
        sourceId = item.optionalString("sourceId"),
        transferJobId = item.optionalString("transferJobId"),
        errorCode = item.optionalString("errorCode"),
        message = item.optionalString("message"),
        retryable = item.getBoolean("retryable"),
        startedAt = item.getString("startedAt"),
        finishedAt = item.optionalString("finishedAt"),
        updatedAt = item.getString("updatedAt"),
)

    private fun parseProviderSettings(item: JSONObject): ProviderSettings {
        val qms = item.getJSONObject("qmediaSync")
        val emby = item.getJSONObject("emby")
        val drive = item.getJSONObject("drive115")
        val tmdb = item.getJSONObject("tmdb")
        val wecom = item.getJSONObject("wecom")
        val workflow = item.getJSONObject("workflow")
        return ProviderSettings(
            qmediaSyncBaseUrl = qms.getString("baseUrl"),
            qmediaSyncApiKey = SecretStatus(qms.getJSONObject("apiKey").getBoolean("configured")),
            embyBaseUrl = emby.getString("baseUrl"),
            embyApiKey = SecretStatus(emby.getJSONObject("apiKey").getBoolean("configured")),
            embyUserId = emby.getString("userId"),
            drive115ClientId = drive.getString("clientId"),
            tmdbBaseUrl = tmdb.getString("baseUrl"),
            tmdbAccessToken = SecretStatus(tmdb.getJSONObject("accessToken").getBoolean("configured")),
            wecom = WeComSettings(
                baseUrl = wecom.getString("baseUrl"),
                corpId = wecom.getString("corpId"),
                secret = SecretStatus(wecom.getJSONObject("secret").getBoolean("configured")),
                sendMode = wecom.optString("sendMode", if (wecom.optString("chatId").isNotBlank()) "appchat" else "app"),
                agentId = wecom.optLong("agentId"),
                toUser = wecom.optString("toUser", "@all"),
                chatId = wecom.getString("chatId"),
            ),
            workflow = parseWorkflowSettings(workflow),
            sources = item.getJSONArray("sources").objects { source ->
                val id = source.getString("id")
                val account = source.optString("account")
                val tokenConfigured = source.getJSONObject("token").getBoolean("configured")
                val fallbackMode = if (id == "juying") {
					if (account.isNotBlank() || tokenConfigured) "developer" else "web"
				} else ""
                ProviderSourceSettings(
                    id = id, label = source.getString("label"), baseUrl = source.getString("baseUrl"),
					account = account, authMode = source.optString("authMode", fallbackMode),
					token = SecretStatus(tokenConfigured),
				)
			},
        )
    }

    private fun parseWorkflowSettings(item: JSONObject) = WorkflowSettings(
        qMediaSyncAccountId = item.getInt("qMediaSyncAccountId"),
        movie = parseWorkflowTarget(item.getJSONObject("movie")),
        series = parseWorkflowTarget(item.getJSONObject("series")),
    )

    private fun parseWorkflowTarget(item: JSONObject) = WorkflowTargetSettings(
        destinationId = item.getString("destinationId"),
        qMediaSyncTargetPath = item.getString("qMediaSyncTargetPath"),
        embyLibraryId = item.getString("embyLibraryId"),
    )

    private fun providerSettingsBody(input: ProviderSettingsUpdate) = JSONObject()
        .put("qmediaSync", JSONObject().put("baseUrl", input.qmediaSyncBaseUrl).put("apiKey", secretBody(input.qmediaSyncApiKey)))
        .put("emby", JSONObject().put("baseUrl", input.embyBaseUrl).put("apiKey", secretBody(input.embyApiKey)).put("userId", input.embyUserId))
        .put("drive115", JSONObject().put("clientId", input.drive115ClientId))
        .put("tmdb", JSONObject().put("baseUrl", input.tmdbBaseUrl).put("accessToken", secretBody(input.tmdbAccessToken)))
        .put("wecom", JSONObject()
            .put("baseUrl", input.wecom.baseUrl)
            .put("corpId", input.wecom.corpId)
            .put("secret", secretBody(input.wecom.secret))
            .put("sendMode", input.wecom.sendMode)
            .put("agentId", input.wecom.agentId)
            .put("toUser", input.wecom.toUser)
            .put("chatId", input.wecom.chatId))
        .put("workflow", JSONObject().put("qMediaSyncAccountId", input.workflow.qMediaSyncAccountId).put("movie", workflowTargetBody(input.workflow.movie)).put("series", workflowTargetBody(input.workflow.series)))
        .put("sources", JSONArray().apply { input.sources.forEach { source -> put(JSONObject().put("id", source.id).put("baseUrl", source.baseUrl).put("account", source.account).put("authMode", source.authMode).put("token", secretBody(source.token))) } })

    private fun secretBody(value: SecretUpdate) = JSONObject().put("value", value.value).put("clear", value.clear)
    private fun workflowTargetBody(value: WorkflowTargetSettings) = JSONObject()
        .put("destinationId", value.destinationId)
        .put("qMediaSyncTargetPath", value.qMediaSyncTargetPath)
        .put("embyLibraryId", value.embyLibraryId)

    private fun <T> JSONArray.objects(transform: (JSONObject) -> T): List<T> = buildList(length()) {
        for (index in 0 until length()) add(transform(getJSONObject(index)))
    }

    private fun JSONArray.strings(): List<String> = buildList(length()) {
        for (index in 0 until length()) add(getString(index))
    }

    private fun encode(value: String) = URLEncoder.encode(value, Charsets.UTF_8.name())

    private suspend fun request(
        path: String,
        method: String = "GET",
        body: String? = null,
        token: String? = null,
        headers: Map<String, String> = emptyMap(),
    ): String = http.request(path, method, body, token, headers)

    private fun parseError(status: Int, body: String): ApiException {
        val payload = runCatching { JSONObject(body) }.getOrNull()
        return ApiException(
            status = status,
            code = payload?.optionalString("code") ?: "request_failed",
            message = payload?.optionalString("title") ?: "请求失败",
        )
    }

    private fun JSONObject.optionalString(key: String): String? {
        if (!has(key) || isNull(key)) return null
        return getString(key).trim().takeIf(String::isNotEmpty)
    }

    private fun InputStream?.readLimited(limit: Int): String {
        if (this == null) return ""
        return use { input ->
            val output = ByteArrayOutputStream()
            val buffer = ByteArray(8192)
            var total = 0
            while (true) {
                val count = input.read(buffer)
                if (count < 0) break
                total += count
                if (total > limit) throw IOException("Media Hub API response exceeded size limit")
                output.write(buffer, 0, count)
            }
            output.toString(Charsets.UTF_8.name())
        }
    }

    private companion object {
        const val DOWNLOAD_CONNECT_TIMEOUT_MS = 5_000
        const val DOWNLOAD_TIMEOUT_MS = 5 * 60 * 1000
        const val MAX_RESPONSE_BYTES = 4 * 1024 * 1024
    }
}
