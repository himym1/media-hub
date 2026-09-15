export function isPlayerView(search = window.location.search) {
  return new URLSearchParams(search).get('view') === 'player'
}

export function playerItemId(search = window.location.search) {
  return new URLSearchParams(search).get('play')
}

export function playerSeriesId(search = window.location.search) {
  return new URLSearchParams(search).get('series')
}
