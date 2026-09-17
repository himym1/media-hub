export const unauthorizedEvent = 'media-hub:unauthorized'

export type IntegrationStatus = 'healthy' | 'degraded' | 'unavailable' | 'unconfigured'

export type Integration = {
  id: string
  label: string
  status: IntegrationStatus
  detail: string
}

export type SecretStatus = { configured: boolean }
export type SecretUpdate = { value: string; clear: boolean }
export type WorkflowTargetSettings = { destinationId: string; qMediaSyncTargetPath: string; embyLibraryId: string }
export type WorkflowSyncMode = 'builtin' | 'qmediasync'
export type CheckInSettings = { enabled: boolean; hour: number; minute: number; sources: Array<'framehdr' | 'juying'> }
export type ProviderSettings = {
  qmediaSync: { baseUrl: string; apiKey: SecretStatus }
  emby: { baseUrl: string; apiKey: SecretStatus; userId: string; password: SecretStatus }
  sharedEmby: { baseUrl: string; username: string; password: SecretStatus; proxyUrl: string }
  drive115: { clientId: string }
  tmdb: { baseUrl: string; accessToken: SecretStatus }
  assrt: { baseUrl: string; token: SecretStatus }
  moviePilot: { baseUrl: string; apiToken: SecretStatus }
  wecom: { baseUrl: string; corpId: string; secret: SecretStatus; sendMode: 'app' | 'appchat'; agentId: number; toUser: string; chatId: string }
  workflow: { syncMode: WorkflowSyncMode; strmBaseUrl: string; strmRootMount: string; qMediaSyncAccountId: number; movie: WorkflowTargetSettings; series: WorkflowTargetSettings; adult: WorkflowTargetSettings }
  checkIn: CheckInSettings
  sources: { id: string; label: string; baseUrl: string; account: string; authMode: string; token: SecretStatus }[]
}
export type ProviderSettingsUpdate = {
  qmediaSync: { baseUrl: string; apiKey: SecretUpdate }
  emby: { baseUrl: string; apiKey: SecretUpdate; userId: string; password: SecretUpdate }
  sharedEmby: { baseUrl: string; username: string; password: SecretUpdate; proxyUrl: string }
  drive115: { clientId: string }
  tmdb: { baseUrl: string; accessToken: SecretUpdate }
  assrt: { baseUrl: string; token: SecretUpdate }
  moviePilot: { baseUrl: string; apiToken: SecretUpdate }
  wecom: { baseUrl: string; corpId: string; secret: SecretUpdate; sendMode: 'app' | 'appchat'; agentId: number; toUser: string; chatId: string }
  workflow: ProviderSettings['workflow']
  checkIn: CheckInSettings
  sources: { id: string; baseUrl: string; account: string; authMode: string; token: SecretUpdate }[]
}
export type OperationalStatistics = {
  transfersTotal: number
  transfersActive: number
  transfersCompleted: number
  transfersFailed: number
  transfersNeedsAttention: number
  subscriptionsTotal: number
  subscriptionsEnabled: number
  runsTotal: number
  runsFailed: number
  commandsPending: number
  commandsNeedsAttention: number
  notificationsNeedsAttention: number
}

export type Release = {
  resolution: string
  videoCodec: string
  dynamicRange?: string
  audio?: string
  sizeBytes: number
}

export type SearchIdentity = {
  tmdbId: string
  title: string
  originalTitle?: string
  year: number
  mediaType: 'movie' | 'series'
  posterUrl?: string
  rating?: number
  overview?: string
}

export type Candidate = {
  id: string
  title: string
  year: number
  season?: number
  episodeStart?: number
  episodeEnd?: number
  mediaType: 'movie' | 'series'
  tmdbId?: string
  source: string
  sourceId: string
  provider?: string
  posterUrl?: string
  rating?: number
  overview?: string
  release: Release
  transferState: 'available' | 'downloadable' | 'identity_required' | 'unavailable' | 'transferring'
  transferToken?: string
}

export type DiscoveryItem = {
  tmdbId: string
  title: string
  year: number
  mediaType: 'movie' | 'series'
  posterUrl?: string
}

export type DiscoveryGenre = {
  id: number
  name: string
}

export type SearchResponse = {
  query: string
  partial: boolean
  identities?: SearchIdentity[]
  results: Candidate[]
  sourceErrors: { source: string; code: string; message: string; retryable: boolean }[]
}

export type TransferState =
  | 'queued'
  | 'transferring'
  | 'downloading'
  | 'retry_wait'
  | 'transferred'
  | 'submitting_sync'
  | 'syncing'
  | 'refreshing_emby'
  | 'indexing_emby'
  | 'verifying_playback'
  | 'completed'
  | 'failed'
  | 'needs_attention'

export type TransferJob = {
  id: string
  title: string
  year: number
  season?: number
  episodeStart?: number
  episodeEnd?: number
  mediaType: 'movie' | 'series' | 'adult'
  tmdbId: string
  source: string
  state: TransferState
  errorCode?: string
  errorMessage?: string
  retryable: boolean
  archived: boolean
  createdAt: string
  updatedAt: string
}

export type TransferEvent = {
  id: number
  state: TransferState
  message: string
  createdAt: string
}

export type TransferJobDetail = TransferJob & { events: TransferEvent[] }

export type TransferNotification = {
  id: string
  jobId: string
  eventType: 'completed' | 'failed' | 'needs_attention'
  jobState: TransferState
  title: string
  state: 'pending' | 'submitting' | 'sent' | 'needs_attention'
  attempts: number
  createdAt: string
  updatedAt: string
}

export type SubscriptionPreferences = {
  resolutions: string[]
  videoCodecs: string[]
  dynamicRanges: string[]
  audioContains: string[]
  preferredSources: string[]
  minSizeBytes: number
  maxSizeBytes: number
  allowUnknownSize: boolean
  preferSmaller: boolean
}

export type SubscriptionInput = {
  tmdbId: string
  title: string
  originalTitle: string
  year: number
  mediaType: 'movie' | 'series'
  season: number
  policy: 'once' | 'upgrade'
  enabled: boolean
  intervalMinutes: number
  sourceIds: string[]
  preferences: SubscriptionPreferences
}

export type Subscription = SubscriptionInput & {
  id: string
  nextRunAt: string
  lastRunAt?: string
  lastEpisode?: number
  createdAt: string
  updatedAt: string
}

export type SubscriptionRunState =
  | 'queued'
  | 'searching'
  | 'retry_wait'
  | 'no_match'
  | 'duplicate'
  | 'enqueued'
  | 'completed'
  | 'failed'
  | 'needs_attention'

export type SubscriptionRun = {
  id: string
  subscriptionId: string
  triggerType: 'scheduled' | 'manual'
  state: SubscriptionRunState
  sourceId?: string
  transferJobId?: string
  errorCode?: string
  message?: string
  retryable: boolean
  startedAt: string
  finishedAt?: string
  updatedAt: string
}

export type DesktopRelease = {
  versionCode: number
  versionName: string
  minimumSupportedVersionCode: number
  sha256: string
  sizeBytes: number
  publishedAt: string
  notes: string
  downloadPath: string
}

export type AuthConfiguration = {
  configured: boolean
}

export type SessionResponse = {
  authenticated: true
  client: 'web' | 'android'
  expiresAt: string
}

export type LoginResponse = SessionResponse & {
  csrfToken?: string
  token?: string
}

export type STRMStatus = {
  mode: WorkflowSyncMode
  running: boolean
  mountPath: string
  mountWritable: boolean
  sessionOk: boolean
  lastError?: string
  lastSummary?: string
  lastSyncAt?: number
  lastMediaType?: string
}

export type EmbyLibrary = {
  id: string
  name: string
  collectionType?: string
  parentId?: string
}

export type EmbyItem = {
  id: string
  name: string
  type: string
  year?: number
  providerIds?: Record<string, string>
  season?: number
  episode?: number
  playbackPositionMs?: number
  played?: boolean
}

export function embyPrimaryImageURL(itemId: string) {
  return `/api/v1/integrations/emby/items/${encodeURIComponent(itemId)}/primary-image`
}

export function embyBackdropImageURL(itemId: string) {
  return `/api/v1/integrations/emby/items/${encodeURIComponent(itemId)}/backdrop-image`
}

export type EmbyMediaTechSpecs = {
  resolution?: string
  videoCodec?: string
  videoRange?: string
  audioCodec?: string
  audioProfile?: string
  audioChannels?: string
  audioChannelCount?: number
  bitDepth?: number
  aspectRatio?: string
  container?: string
}

export type EmbyEpisode = EmbyItem & {
  externalUrl: string
  appUrl?: string
  techSpecs?: EmbyMediaTechSpecs
  overview?: string
  runtimeMinutes?: number
  communityRating?: number
  primaryImageTag?: string
}

export type PlaybackDescriptor = {
  streamUrl: string
  userAgent: string
  title: string
  startPositionMs?: number
  expiresAt?: string
  sessionId?: string
}

export type EmbyItemSearch = {
  items: EmbyItem[]
  total: number
}

export interface EmbyPerson {
  id: string
  name: string
  role?: string
  type?: string
  primaryImageTag?: string
}

export type EmbyItemDetail = EmbyItem & {
  originalTitle?: string
  overview?: string
  communityRating?: number
  runtimeMinutes?: number
  genres?: string[]
  mediaSourceCount: number
  externalUrl: string
  appUrl?: string
  techSpecs?: EmbyMediaTechSpecs
  people?: EmbyPerson[]
}

export type EmbyDeletePreview = {
  id: string
  name: string
  type: string
  fileCount: number
  deletesFiles: boolean
  cloudKept: boolean
  versionCount: number
}

export type EmbyRemoteSubtitle = {
  id: string
  name: string
  language?: string
  format?: string
  providerName?: string
  author?: string
  comment?: string
  communityRating?: number
  downloadCount?: number
  isHashMatch?: boolean
  hearingImpaired?: boolean
  forced?: boolean
}

export type Drive115Status = {
  authorized: boolean
  usedBytes?: number
  totalBytes?: number
  memberLevel?: string
  expiresAt?: number
}

export type Drive115DeviceAuthorization = {
  id: string
  qrImage?: string
  state: 'pending' | 'confirmed' | 'expired' | 'failed'
  expiresAt: string
}

export type Drive115File = {
  id: string
  parentId: string
  name: string
  kind: 'file' | 'folder'
  size: number
  updatedAt: number
}

export type Drive115Command = {
  id: string
  operation: 'create_folder' | 'move' | 'rename' | 'delete'
  state: 'awaiting_confirmation' | 'queued' | 'submitting' | 'completed' | 'failed' | 'needs_attention'
  attempts: number
  errorCode?: string
  errorMessage?: string
  createdAt: string
  updatedAt: string
  events?: { id: number; state: string; message: string; createdAt: number }[]
}

export interface ArchiveSuggestion { fileId: string; currentName: string; suggestedName: string; kind: 'file' | 'directory'; confidence: 'review' }
export interface ArchiveStep { operation: 'rename' | 'move'; fileId: string; name?: string; targetParentId?: string }
export interface ArchivePlan { id: string; state: 'awaiting_confirmation' | 'queued' | 'running' | 'completed' | 'failed' | 'needs_attention'; stepIndex: number; stepTotal: number; steps?: ArchiveStep[]; errorCode?: string; errorMessage?: string; createdAt: string; updatedAt: string }

export type LocalUploadEntry = { name: string; path: string; directory: boolean; size: number }
export type LocalUploadJob = {
  id: string
  path: string
  state: 'queued' | 'hashing' | 'submitting_init' | 'uploading' | 'completed' | 'failed' | 'needs_attention'
  bytesDone: number
  bytesTotal: number
  errorCode?: string
  errorMessage?: string
  createdAt: string
  updatedAt: string
}

type ApiProblem = {
  title?: string
  code?: string
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    credentials: 'same-origin',
    ...init,
    headers: {
      Accept: 'application/json',
      ...init?.headers,
    },
  })
  if (!response.ok) {
    let problem: ApiProblem = {}
    try {
      problem = await response.json() as ApiProblem
    } catch {
      // The status code remains the stable fallback when an upstream proxy returns non-JSON.
    }
    const error = new ApiError(
      response.status,
      problem.code ?? 'request_failed',
      problem.title ?? '请求失败',
    )
    if (response.status === 401 && error.code === 'authentication_required') {
      window.dispatchEvent(new Event(unauthorizedEvent))
    }
    throw error
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export function getLatestDesktopRelease(platform: 'windows' | 'darwin' = 'windows') {
  return requestJSON<DesktopRelease>(`/api/v1/client/desktop/releases/latest?platform=${platform}`, {
    cache: 'no-store',
  })
}

export function getAuthConfiguration() {
  return requestJSON<AuthConfiguration>('/api/v1/auth/configuration')
}

export function getSession() {
  return requestJSON<SessionResponse>('/api/v1/auth/session')
}

export function login(password: string) {
  return requestJSON<LoginResponse>('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password, client: 'web' }),
  })
}

export function logout() {
  return requestJSON<void>('/api/v1/auth/logout', {
    method: 'POST',
    headers: { 'X-CSRF-Token': readCookie('media_hub_csrf') ?? '' },
  })
}

export function changePassword(currentPassword: string, newPassword: string) {
  return requestJSON<void>('/api/v1/auth/password', {
    method: 'PUT',
    headers: writeHeaders(),
    body: JSON.stringify({ currentPassword, newPassword }),
  })
}

export function searchMedia(query: string) {
  return requestJSON<SearchResponse>(`/api/v1/search?query=${encodeURIComponent(query)}`)
}

export function getTrending(mediaType: 'all' | 'movie' | 'series' = 'all', limit = 12) {
  return requestJSON<{ items: DiscoveryItem[] }>(`/api/v1/discovery/trending?mediaType=${mediaType}&limit=${limit}`)
}

export function getDiscoveryCatalog(
  kind: 'popular' | 'top_rated' | 'genre',
  mediaType: 'movie' | 'series',
  options: { genreId?: string | number; limit?: number } = {},
) {
  const params = new URLSearchParams({
    kind,
    mediaType,
    limit: String(options.limit ?? 12),
  })
  if (options.genreId != null && String(options.genreId) !== '') {
    params.set('genreId', String(options.genreId))
  }
  return requestJSON<{ items: DiscoveryItem[] }>(`/api/v1/discovery/catalog?${params}`)
}

export function getDiscoveryGenres(mediaType: 'movie' | 'series' = 'movie') {
  return requestJSON<{ genres: DiscoveryGenre[] }>(
    `/api/v1/discovery/genres?mediaType=${mediaType}`,
  )
}

export function getRecommendations(mediaType: 'movie' | 'series', tmdbId: string, limit = 12) {
  return requestJSON<{ items: DiscoveryItem[] }>(`/api/v1/discovery/${mediaType}/${encodeURIComponent(tmdbId)}/recommendations?limit=${limit}`)
}

export function getOperationalStatistics() {
  return requestJSON<OperationalStatistics>('/api/v1/statistics/summary')
}

export function getSystemOverview() {
  return requestJSON<{ integrations: Integration[] }>('/api/v1/system/overview')
}

export function getProviderSettings() {
	return requestJSON<ProviderSettings>('/api/v1/settings/providers')
}

export function updateProviderSettings(input: ProviderSettingsUpdate) {
	return requestJSON<ProviderSettings>('/api/v1/settings/providers', {
		method: 'PUT',
		headers: writeHeaders(),
		body: JSON.stringify(input),
	})
}

export type SourceCheckIn = {
  sourceId: string
  label: string
  state: 'idle' | 'running' | 'completed' | 'skipped' | 'failed' | 'needs_attention'
  message?: string
  errorCode?: string
  retryable: boolean
  lastSuccessAt?: string
  updatedAt: string
}

export function getSourceCheckIns() {
  return requestJSON<{ items: SourceCheckIn[] }>('/api/v1/integrations/sources/checkins')
}

export function retrySourceCheckIn(sourceId: string) {
  return requestJSON<SourceCheckIn>(`/api/v1/integrations/sources/checkins/${encodeURIComponent(sourceId)}/retry`, {
    method: 'POST',
    headers: writeHeaders(false),
  })
}

export function testWeComNotification() {
  return requestJSON<{ status: 'sent' }>('/api/v1/integrations/wecom/test', {
    method: 'POST',
    headers: writeHeaders(false),
  })
}

export function getSTRMStatus() {
  return requestJSON<STRMStatus>('/api/v1/integrations/strm/status')
}

export function syncSTRMLibrary(input: { mediaType?: 'movie' | 'series' | 'all'; full?: boolean; dryRun?: boolean } = {}) {
  return requestJSON<{ status: 'accepted' }>('/api/v1/integrations/strm/sync', {
    method: 'POST',
    headers: writeHeaders(),
    body: JSON.stringify({ mediaType: input.mediaType ?? 'all', full: Boolean(input.full), dryRun: Boolean(input.dryRun) }),
  })
}

export function getEmbyLibraries() {
  return requestJSON<{ libraries: EmbyLibrary[] }>('/api/v1/integrations/emby/libraries')
}

export function searchEmbyItems(query: string, limit = 20) {
  return requestJSON<EmbyItemSearch>(
    `/api/v1/integrations/emby/items?query=${encodeURIComponent(query)}&limit=${limit}`,
  )
}

export function getEmbyLibraryItems(libraryId: string, offset = 0, limit = 50, sort = '') {
  const sortQuery = sort.trim() ? `&sort=${encodeURIComponent(sort.trim())}` : ''
  return requestJSON<EmbyItemSearch>(
    `/api/v1/integrations/emby/libraries/${encodeURIComponent(libraryId)}/items?offset=${offset}&limit=${limit}${sortQuery}`,
  )
}

export function getEmbyItem(id: string) {
  return requestJSON<EmbyItemDetail>(`/api/v1/integrations/emby/items/${encodeURIComponent(id)}`)
}

export function getEmbyEpisodes(id: string) {
  return requestJSON<{ items: EmbyEpisode[]; total: number }>(
    `/api/v1/integrations/emby/items/${encodeURIComponent(id)}/episodes`,
  )
}

export function createEmbyPlaybackDescriptor(itemId: string, playbackUserAgent: string) {
  const agent = playbackUserAgent.trim()
  return requestJSON<PlaybackDescriptor>('/api/v1/playback/descriptors/emby', {
    method: 'POST',
    headers: writeHeaders(),
    body: JSON.stringify(agent ? { itemId, playbackUserAgent: agent } : { itemId }),
  })
}

export function reportPlaybackSessionEvent(
  sessionId: string,
  event: 'started' | 'progress' | 'stopped',
  positionMs: number,
  paused: boolean,
) {
  return requestJSON<void>(`/api/v1/playback/sessions/${encodeURIComponent(sessionId)}/events`, {
    method: 'POST',
    headers: writeHeaders(),
    body: JSON.stringify({ event, positionMs, paused }),
  })
}

export function refreshEmbyLibrary(id: string) {
  return requestJSON<{ status: 'accepted' }>(`/api/v1/integrations/emby/libraries/${encodeURIComponent(id)}/refresh`, {
    method: 'POST', headers: writeHeaders(false),
  })
}

export function refreshEmbyItem(id: string) {
  return requestJSON<{ status: 'accepted' }>(`/api/v1/integrations/emby/items/${encodeURIComponent(id)}/refresh`, {
    method: 'POST', headers: writeHeaders(false),
  })
}

export function previewEmbyItemDelete(id: string) {
  return requestJSON<EmbyDeletePreview>(`/api/v1/integrations/emby/items/${encodeURIComponent(id)}/delete-preview`)
}

export function deleteEmbyItem(id: string) {
  return requestJSON<{ status: 'deleted' }>(`/api/v1/integrations/emby/items/${encodeURIComponent(id)}/delete`, {
    method: 'POST',
    headers: writeHeaders(),
    body: JSON.stringify({ confirmation: id }),
  })
}

export function searchEmbyRemoteSubtitles(id: string, language = 'chi') {
  const params = new URLSearchParams({ language })
  return requestJSON<{ items: EmbyRemoteSubtitle[] }>(
    `/api/v1/integrations/emby/items/${encodeURIComponent(id)}/remote-subtitles?${params}`,
  )
}

export function downloadEmbyRemoteSubtitle(id: string, subtitleId: string) {
  return requestJSON<{ status: 'accepted' }>(
    `/api/v1/integrations/emby/items/${encodeURIComponent(id)}/remote-subtitles`,
    {
      method: 'POST',
      headers: writeHeaders(),
      body: JSON.stringify({ subtitleId }),
    },
  )
}

export type LocalSubtitle = {
  bytes: ArrayBuffer
  contentType: string
  fileName: string
}

export async function fetchLocalSubtitle(id: string): Promise<LocalSubtitle | null> {
  const response = await fetch(`/api/v1/integrations/emby/items/${encodeURIComponent(id)}/local-subtitle`, {
    credentials: 'same-origin',
    headers: { Accept: 'application/x-subrip, text/x-ssa, application/json' },
  })
  if (response.status === 404) return null
  if (!response.ok) {
    let problem: ApiProblem = {}
    try {
      problem = await response.json() as ApiProblem
    } catch {
      // Status remains the fallback when the body is not a problem document.
    }
    throw new ApiError(response.status, problem.code ?? 'request_failed', problem.title ?? '请求失败')
  }
  const disposition = response.headers.get('content-disposition') ?? ''
  const encoded = disposition.match(/filename\*=UTF-8''([^;]+)/i)
  const quoted = disposition.match(/filename="([^"]+)"/i)
  const plain = disposition.match(/filename=([^;]+)/i)
  const fileName = encoded?.[1]
    ? decodeURIComponent(encoded[1].trim())
    : quoted?.[1] ?? plain?.[1]?.trim() ?? 'subtitle.srt'
  return {
    bytes: await response.arrayBuffer(),
    contentType: response.headers.get('content-type') ?? 'application/x-subrip',
    fileName,
  }
}

export function getDrive115Status() {
  return requestJSON<Drive115Status>('/api/v1/integrations/115/status')
}

export function startDrive115Authorization() {
  return requestJSON<Drive115DeviceAuthorization>('/api/v1/integrations/115/auth/device', {
    method: 'POST',
    headers: writeHeaders(false),
  })
}

export function pollDrive115Authorization(id: string) {
  return requestJSON<Drive115DeviceAuthorization>(`/api/v1/integrations/115/auth/device/${encodeURIComponent(id)}`)
}

export function listDrive115Files(parentId = '0') {
  return requestJSON<{ items: Drive115File[]; total: number }>(`/api/v1/integrations/115/files?parentId=${encodeURIComponent(parentId)}&limit=100`)
}

export function listDrive115Commands() {
  return requestJSON<{ commands: Drive115Command[] }>('/api/v1/integrations/115/commands').then((value) => value.commands)
}

export function createDrive115Command(operation: Drive115Command['operation'], params: Record<string, unknown>) {
  return requestJSON<Drive115Command>('/api/v1/integrations/115/commands', {
    method: 'POST',
    headers: { ...writeHeaders(true), 'Idempotency-Key': crypto.randomUUID() },
    body: JSON.stringify({ operation, params }),
  })
}

export function confirmDrive115Command(command: Drive115Command) {
  return requestJSON<Drive115Command>(`/api/v1/integrations/115/commands/${encodeURIComponent(command.id)}/confirm`, {
    method: 'POST',
    headers: writeHeaders(true),
    body: JSON.stringify({ confirmation: command.id }),
  })
}

export function retryDrive115Command(command: Drive115Command) {
  return requestJSON<Drive115Command>(`/api/v1/integrations/115/commands/${encodeURIComponent(command.id)}/retry`, {
    method: 'POST', headers: writeHeaders(true), body: JSON.stringify({ confirmation: command.id }),
  })
}

export function previewArchive(parentId: string) {
  return requestJSON<{ suggestions: ArchiveSuggestion[] }>('/api/v1/archive/preview', { method: 'POST', headers: writeHeaders(), body: JSON.stringify({ parentId }) })
}
export function createArchivePlan(steps: ArchiveStep[]) {
  return requestJSON<ArchivePlan>('/api/v1/archive/plans', { method: 'POST', headers: writeHeaders(), body: JSON.stringify({ steps }) })
}
export function listArchivePlans() { return requestJSON<{ plans: ArchivePlan[] }>('/api/v1/archive/plans').then((value) => value.plans) }
export function confirmArchivePlan(id: string) {
  return requestJSON<ArchivePlan>(`/api/v1/archive/plans/${encodeURIComponent(id)}/confirm`, { method: 'POST', headers: writeHeaders(), body: JSON.stringify({ confirmation: id }) })
}
export function retryArchivePlan(id: string) {
  return requestJSON<ArchivePlan>(`/api/v1/archive/plans/${encodeURIComponent(id)}/retry`, { method: 'POST', headers: writeHeaders(), body: JSON.stringify({ confirmation: id }) })
}

export function listLocalUploadRoots() {
  return requestJSON<{ roots: { id: string }[] }>('/api/v1/local-uploads/roots').then((value) => value.roots)
}
export function listLocalUploadFiles(rootId: string, path: string) {
  return requestJSON<{ items: LocalUploadEntry[] }>(`/api/v1/local-uploads/files?rootId=${encodeURIComponent(rootId)}&path=${encodeURIComponent(path)}`).then((value) => value.items)
}
export function listLocalUploads() {
  return requestJSON<{ uploads: LocalUploadJob[] }>('/api/v1/local-uploads?limit=100').then((value) => value.uploads)
}
export function createLocalUpload(rootId: string, path: string, destinationId: string) {
  return requestJSON<LocalUploadJob>('/api/v1/local-uploads', { method: 'POST', headers: { ...writeHeaders(true), 'Idempotency-Key': crypto.randomUUID() }, body: JSON.stringify({ rootId, path, destinationId }) })
}
export function retryLocalUpload(upload: LocalUploadJob) {
  return requestJSON<LocalUploadJob>(`/api/v1/local-uploads/${encodeURIComponent(upload.id)}/retry`, { method: 'POST', headers: writeHeaders(true), body: JSON.stringify({ confirmation: upload.id }) })
}

export function createShareImport(input: { url: string; receiveCode?: string; title?: string }, idempotencyKey: string) {
  return requestJSON<TransferJob>('/api/v1/share-imports', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': idempotencyKey,
      'X-CSRF-Token': readCookie('media_hub_csrf') ?? '',
    },
    body: JSON.stringify(input),
  })
}

export type CaptureUploadTicket = {
  destinationId: string
  filename: string
  title: string
  target: string
  host: string
  object: string
  accessid: string
  policy: string
  signature: string
  callback: string
}

export function initCaptureUpload(input: { filename: string; size: number; title?: string }) {
  return requestJSON<CaptureUploadTicket>('/api/v1/capture-uploads/init', {
    method: 'POST',
    headers: writeHeaders(),
    body: JSON.stringify(input),
  })
}

export function completeCaptureUpload(
  input: { destinationId: string; filename: string; title?: string },
  idempotencyKey: string,
) {
  return requestJSON<TransferJob>('/api/v1/capture-uploads', {
    method: 'POST',
    headers: {
      ...writeHeaders(),
      'Idempotency-Key': idempotencyKey,
    },
    body: JSON.stringify(input),
  })
}

export function createTransfer(transferToken: string, idempotencyKey: string) {
  return requestJSON<TransferJob>('/api/v1/transfers', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': idempotencyKey,
      'X-CSRF-Token': readCookie('media_hub_csrf') ?? '',
    },
    body: JSON.stringify({ transferToken }),
  })
}

export function listTransfers(limit = 50, archived = false) {
  return requestJSON<{ transfers: TransferJob[] }>(`/api/v1/transfers?limit=${limit}&archived=${archived}`)
}

export function getTransfer(id: string) {
  return requestJSON<TransferJobDetail>(`/api/v1/transfers/${encodeURIComponent(id)}`)
}

export function retryTransfer(id: string) {
  return requestJSON<TransferJob>(`/api/v1/transfers/${encodeURIComponent(id)}/retry`, {
    method: 'POST',
    headers: { 'X-CSRF-Token': readCookie('media_hub_csrf') ?? '' },
  })
}

export function setTransferArchived(id: string, archived: boolean) {
  return requestJSON<TransferJob>(`/api/v1/transfers/${encodeURIComponent(id)}/archived`, {
    method: 'PATCH',
    headers: writeHeaders(),
    body: JSON.stringify({ archived }),
  })
}

export function deleteTransfer(id: string) {
  return requestJSON<void>(`/api/v1/transfers/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: writeHeaders(),
  })
}

export function listTransferNotifications(limit = 100) {
  return requestJSON<{ notifications: TransferNotification[] }>(`/api/v1/notifications?limit=${limit}`)
}

export function retryTransferNotification(notification: TransferNotification) {
  return requestJSON<TransferNotification>(
    `/api/v1/notifications/${encodeURIComponent(notification.jobId)}/${encodeURIComponent(notification.eventType)}/retry`,
    {
      method: 'POST',
      headers: writeHeaders(),
      body: JSON.stringify({ confirm: notification.id }),
    },
  )
}
export function listSubscriptions() {
  return requestJSON<{ subscriptions: Subscription[] }>('/api/v1/subscriptions')
}

export function createSubscription(input: SubscriptionInput) {
  return requestJSON<Subscription>('/api/v1/subscriptions', {
    method: 'POST',
    headers: writeHeaders(),
    body: JSON.stringify(input),
  })
}

export function updateSubscription(id: string, input: Omit<SubscriptionInput, 'tmdbId' | 'mediaType' | 'season'>) {
  return requestJSON<Subscription>(`/api/v1/subscriptions/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: writeHeaders(),
    body: JSON.stringify(input),
  })
}

export function deleteSubscription(id: string) {
  return requestJSON<void>(`/api/v1/subscriptions/${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: writeHeaders(),
  })
}

export function setSubscriptionEnabled(id: string, enabled: boolean) {
  return requestJSON<Subscription>(`/api/v1/subscriptions/${encodeURIComponent(id)}/enabled`, {
    method: 'PATCH',
    headers: writeHeaders(),
    body: JSON.stringify({ enabled }),
  })
}

export function runSubscription(id: string) {
  return requestJSON<SubscriptionRun>(`/api/v1/subscriptions/${encodeURIComponent(id)}/runs`, {
    method: 'POST',
    headers: writeHeaders(false),
  })
}

export function listSubscriptionRuns(id: string, limit = 50) {
  return requestJSON<{ runs: SubscriptionRun[] }>(
    `/api/v1/subscriptions/${encodeURIComponent(id)}/runs?limit=${limit}`,
  )
}


function writeHeaders(json = true) {
  return {
    ...(json ? { 'Content-Type': 'application/json' } : {}),
    'X-CSRF-Token': readCookie('media_hub_csrf') ?? '',
  }
}

function readCookie(name: string) {
  const prefix = `${encodeURIComponent(name)}=`
  for (const part of document.cookie.split(';')) {
    const cookie = part.trim()
    if (cookie.startsWith(prefix)) return decodeURIComponent(cookie.slice(prefix.length))
  }
  return null
}


export type SubscriptionBackup = {
  version: 1
  exportedAt: string
  subscriptions: SubscriptionInput[]
}

export function exportSubscriptions() {
  return requestJSON<SubscriptionBackup>('/api/v1/subscriptions/backup')
}

export function importSubscriptions(backup: SubscriptionBackup) {
  return requestJSON<{ created: number; skipped: number }>('/api/v1/subscriptions/backup', {
    method: 'POST',
    headers: writeHeaders(),
    body: JSON.stringify(backup),
  })
}

export function setSubscriptionsEnabled(ids: string[], enabled: boolean) {
  return requestJSON<{ subscriptions: Subscription[] }>('/api/v1/subscriptions/batch/enabled', {
    method: 'POST',
    headers: writeHeaders(),
    body: JSON.stringify({ ids, enabled }),
  })
}


