(function () {
  if (!window.__mhCaptureInstalled) {
    window.__mhCaptureInstalled = true
    window.__mhCapture = window.__mhCapture || { page: location.href, items: [] }

    function absoluteUrl(input) {
      if (!input) return ''
      try {
        if (typeof URL !== 'undefined' && input instanceof URL) return input.href
        if (typeof Request !== 'undefined' && input instanceof Request) return input.url
        if (typeof input === 'string') return new URL(input, location.href).href
        if (input.url) return new URL(String(input.url), location.href).href
      } catch (e) {}
      return ''
    }

    function kind(url) {
      if (!url || url.indexOf('blob:') === 0 || url.indexOf('data:') === 0) return null
      var lower = String(url).toLowerCase()
      if (lower.indexOf('m3u8') !== -1) return 'hls'
      var path = lower.split('#')[0].split('?')[0]
      if (/\.(mp4|m4v|mkv|webm|mov)$/.test(path)) return 'file'
      return null
    }

    function add(raw) {
      var url = absoluteUrl(raw)
      var nextKind = kind(url)
      if (!nextKind) return
      var items = window.__mhCapture.items
      for (var i = 0; i < items.length; i++) {
        if (items[i].url === url) return
      }
      if (items.length >= 80) return
      items.push({ url: url, kind: nextKind })
    }

    window.__mhCollectCapture = function () {
      window.__mhCapture.page = location.href
      try {
        var nodes = document.querySelectorAll('video, audio, source')
        for (var i = 0; i < nodes.length; i++) {
          var el = nodes[i]
          if (el.src) add(el.src)
          if (el.currentSrc) add(el.currentSrc)
        }
        if (performance && performance.getEntriesByType) {
          var entries = performance.getEntriesByType('resource')
          for (var j = 0; j < entries.length; j++) add(entries[j].name)
        }
      } catch (e) {}
      return window.__mhCapture
    }

    var originalFetch = window.fetch
    if (originalFetch) {
      window.fetch = function () {
        try { add(arguments[0]) } catch (e) {}
        return originalFetch.apply(this, arguments)
      }
    }

    var originalOpen = XMLHttpRequest.prototype.open
    XMLHttpRequest.prototype.open = function (method, url) {
      try { add(url) } catch (e) {}
      return originalOpen.apply(this, arguments)
    }

    try {
      var srcDesc = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'src')
      if (srcDesc && srcDesc.set) {
        Object.defineProperty(HTMLMediaElement.prototype, 'src', {
          configurable: true,
          enumerable: srcDesc.enumerable,
          get: function () { return srcDesc.get.call(this) },
          set: function (value) {
            add(value)
            srcDesc.set.call(this, value)
          }
        })
      }
    } catch (e) {}

    try {
      var observer = new MutationObserver(function () {
        window.__mhCollectCapture()
      })
      observer.observe(document.documentElement || document, {
        subtree: true,
        childList: true,
        attributes: true,
        attributeFilter: ['src']
      })
    } catch (e) {}

    try {
      var po = new PerformanceObserver(function (list) {
        var entries = list.getEntries()
        for (var i = 0; i < entries.length; i++) add(entries[i].name)
      })
      po.observe({ type: 'resource', buffered: true })
    } catch (e) {}
  }

  window.__mhKickCapturePlay = function () {
    try {
      var media = document.querySelectorAll('video, audio')
      for (var i = 0; i < media.length; i++) {
        var el = media[i]
        el.muted = true
        el.defaultMuted = true
        el.autoplay = true
        if ('playsInline' in el) el.playsInline = true
        el.setAttribute('playsinline', '')
        var play = el.play()
        if (play && play.catch) play.catch(function () {})
      }
      var nodes = document.querySelectorAll('button, [role="button"], .vjs-big-play-button, .jw-display-icon-container, [class*="play"]')
      for (var j = 0; j < nodes.length && j < 24; j++) {
        var node = nodes[j]
        var label = [
          node.getAttribute && node.getAttribute('aria-label'),
          node.getAttribute && node.getAttribute('title'),
          node.className,
          node.id,
          node.textContent
        ].join(' ')
        if (/(?:\bplay\b|播放|▶|►|big-play|jw-display-icon)/i.test(label)) {
          try { node.click() } catch (e) {}
        }
      }
      var video = document.querySelector('video')
      if (video) {
        try { video.click() } catch (e) {}
      }
    } catch (e) {}
    if (typeof window.__mhCollectCapture === 'function') {
      window.__mhCollectCapture()
    }
  }

  window.__mhKickCapturePlay()
})()
