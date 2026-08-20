import type { Page, Route } from '@playwright/test'

const integrations = [
  { id: 'tmdb', label: 'TMDB', status: 'healthy', detail: '元数据在线' },
  { id: '115', label: '115', status: 'healthy', detail: '授权有效' },
  { id: 'qmediasync', label: 'QMediaSync', status: 'healthy', detail: '同步服务在线' },
  { id: 'emby', label: 'Emby', status: 'healthy', detail: '媒体库在线' },
  { id: 'sources', label: '资源源', status: 'healthy', detail: '4 个资源源可用' },
]

const transfer = {
  id: 'task-1',
  title: '验收影片',
  year: 2026,
  mediaType: 'movie',
  tmdbId: '100',
  source: 'juying',
  state: 'completed',
  retryable: false,
  archived: false,
  createdAt: '2026-08-15T01:00:00Z',
  updatedAt: '2026-08-15T01:05:00Z',
}

const emptySubscriptionPreferences = { resolutions: [], videoCodecs: [], dynamicRanges: [], audioContains: [], preferredSources: [], minSizeBytes: 0, maxSizeBytes: 0, allowUnknownSize: false, preferSmaller: false }
const subscription = {
  id: 'subscription-1', tmdbId: '100', title: '验收影片', originalTitle: '', year: 2026, mediaType: 'movie', season: 0,
  policy: 'upgrade', enabled: true, intervalMinutes: 60, sourceIds: ['juying'], preferences: { ...emptySubscriptionPreferences },
  nextRunAt: '2026-08-16T01:00:00Z', createdAt: '2026-08-15T01:00:00Z', updatedAt: '2026-08-15T01:00:00Z',
}

const sources = ['mikan', 'sidhub', 'framehdr', 'juying', 'dian', 'guanying', 'gimy', 'hdhive'].map((id) => ({
  id,
  label: id,
  baseUrl: `https://${id}.example`,
  account: '',
  authMode: id === 'juying' ? 'web' : '',
  token: { configured: false },
}))

const providerSettings = {
  qmediaSync: { baseUrl: 'http://qmediasync.local', apiKey: { configured: true } },
  emby: { baseUrl: 'http://emby.local', apiKey: { configured: true }, userId: 'user', password: { configured: false } },
  drive115: { clientId: '' },
  tmdb: { baseUrl: 'https://api.themoviedb.org/3', accessToken: { configured: true } },
  wecom: { baseUrl: 'https://qyapi.weixin.qq.com', corpId: '', secret: { configured: false }, sendMode: 'app', agentId: 0, toUser: '@all', chatId: '' },
  workflow: {
    qMediaSyncAccountId: 1,
    movie: { destinationId: '1', qMediaSyncTargetPath: '/movie', embyLibraryId: 'movie' },
    series: { destinationId: '2', qMediaSyncTargetPath: '/series', embyLibraryId: 'series' },
  },
  sources,
}

type FixtureOptions = { authenticated?: boolean; withSubscription?: boolean }
type FixtureState = { transferArchived: boolean; itemDeleted: boolean }

export async function installApiFixtures(page: Page, options: FixtureOptions = {}) {
  const authenticated = options.authenticated ?? true
  const state: FixtureState = { transferArchived: false, itemDeleted: false }
  await page.route('**/api/v1/**', async (route) => respond(route, authenticated, state, options))
}

async function respond(route: Route, authenticated: boolean, state: FixtureState, options: FixtureOptions) {
  const request = route.request()
  const url = new URL(request.url())
  const path = url.pathname

  if (path === '/api/v1/auth/session') {
    if (!authenticated) return json(route, { code: 'authentication_required', title: '需要登录' }, 401)
    return json(route, { authenticated: true, client: 'web', expiresAt: '2026-08-16T00:00:00Z' })
  }
  if (path === '/api/v1/auth/configuration') return json(route, { configured: true })
  if (path === '/api/v1/system/overview') return json(route, { integrations })
  if (path === '/api/v1/transfers' && request.method() === 'GET') {
    const archived = url.searchParams.get('archived') === 'true'
    return json(route, { transfers: archived === state.transferArchived ? [{ ...transfer, archived }] : [] })
  }
  if (path === '/api/v1/transfers/task-1/archived' && request.method() === 'PATCH') {
    state.transferArchived = JSON.parse(request.postData() ?? '{}').archived === true
    return json(route, { ...transfer, archived: state.transferArchived })
  }
  if (path === '/api/v1/transfers/task-1') return json(route, { ...transfer, events: [{ id: 1, state: 'completed', message: '播放验证通过', createdAt: transfer.updatedAt }] })
  if (path === '/api/v1/notifications') return json(route, { notifications: [] })
  if (path === '/api/v1/discovery/trending') return json(route, { items: [{ tmdbId: '100', title: '验收影片', year: 2026, mediaType: 'movie' }] })
  if (path === '/api/v1/search') return json(route, {
    query: url.searchParams.get('query') ?? '',
    partial: false,
    sourceErrors: [],
    results: [{
      id: 'candidate-1',
      title: url.searchParams.get('query') ?? '验收影片',
      year: 2026,
      mediaType: 'movie',
      tmdbId: '100',
      source: 'juying',
      sourceId: 'juying',
      provider: '聚影',
      release: { resolution: '2160p', videoCodec: 'HEVC', dynamicRange: 'HDR', audio: 'DTS', sizeBytes: 10_737_418_240 },
      transferState: 'available',
      transferToken: 'fixture-token',
    }],
  })
  if (path.includes('/recommendations')) return json(route, { items: [] })
  if (path === '/api/v1/statistics/summary') return json(route, { transfersTotal: 1, transfersActive: 0, transfersCompleted: 1, transfersFailed: 0, transfersNeedsAttention: 0, subscriptionsTotal: 0, subscriptionsEnabled: 0, runsTotal: 0, runsFailed: 0, commandsPending: 0, commandsNeedsAttention: 0, notificationsNeedsAttention: 0 })
  if (path === '/api/v1/integrations/qmediasync/status') return json(route, { version: '0.14.23', totalSyncs: 1, recentSyncs: [] })
  if (path === '/api/v1/integrations/115/status') return json(route, { authorized: true, usedBytes: 1_000_000, totalBytes: 2_000_000 })
  if (path === '/api/v1/integrations/emby/libraries') return json(route, { libraries: [{ id: 'movie', name: '电影', collectionType: 'movies' }] })
  if (path === '/api/v1/integrations/emby/libraries/movie/items') return json(route, { items: state.itemDeleted ? [] : [{ id: 'item-1', name: '验收影片', type: 'Movie', year: 2026, providerIds: { Tmdb: '100' } }], total: state.itemDeleted ? 0 : 1 })
  if (path === '/api/v1/integrations/emby/items/item-1' && request.method() === 'GET') {
    if (state.itemDeleted) return json(route, { code: 'not_found', title: '媒体不存在' }, 404)
    return json(route, { id: 'item-1', name: '验收影片', originalTitle: 'Acceptance Movie', overview: '用于验证媒体库详情。', type: 'Movie', year: 2026, providerIds: { Tmdb: '100' }, communityRating: 8.2, runtimeMinutes: 118, genres: ['Drama', 'Science Fiction'], mediaSourceCount: 1, externalUrl: 'https://emby.example/web/index.html#!/item?id=item-1', appUrl: 'emby://items/server-1/item-1' })
  }
  if (path === '/api/v1/integrations/emby/items/item-1/delete-preview' && request.method() === 'GET') return json(route, { id: 'item-1', name: '验收影片', type: 'Movie', fileCount: 1, deletesFiles: true, cloudKept: true })
  if (path === '/api/v1/integrations/emby/items/item-1/delete' && request.method() === 'POST') {
    const confirmation = JSON.parse(request.postData() ?? '{}').confirmation
    if (confirmation !== 'item-1') return json(route, { code: 'invalid_confirmation', title: '删除确认无效' }, 400)
    state.itemDeleted = true
    return json(route, { status: 'deleted' })
  }
  if ((path === '/api/v1/integrations/emby/libraries/movie/refresh' || path === '/api/v1/integrations/emby/items/item-1/refresh') && request.method() === 'POST') return json(route, { status: 'accepted' }, 202)
  if (path === '/api/v1/settings/providers') return json(route, providerSettings)
  if (path === '/api/v1/subscriptions') return json(route, { subscriptions: options.withSubscription ? [subscription] : [] })
  if (path === '/api/v1/subscriptions/subscription-1/runs' && request.method() === 'GET') return json(route, { runs: [] })
  if (path === '/api/v1/subscriptions/subscription-1/runs' && request.method() === 'POST') return json(route, { id: 'run-1', subscriptionId: 'subscription-1', triggerType: 'manual', state: 'queued', retryable: false, startedAt: subscription.updatedAt, updatedAt: subscription.updatedAt }, 202)
  if (path === '/api/v1/integrations/115/files') return json(route, { items: [], total: 0 })
  if (path === '/api/v1/integrations/115/commands') return json(route, { commands: [] })
  if (path === '/api/v1/local-uploads/roots') return json(route, { roots: [] })
  if (path === '/api/v1/local-uploads') return json(route, { uploads: [] })
  if (path === '/api/v1/archive/plans') return json(route, { plans: [] })

  return json(route, {})
}

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}
