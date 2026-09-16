type FullscreenDocument = Document & {
  webkitFullscreenElement?: Element | null
  webkitExitFullscreen?: () => Promise<void> | void
}

type FullscreenElement = Element & {
  webkitRequestFullscreen?: () => Promise<void> | void
}

export function documentFullscreenElement(doc: Document = document): Element | null {
  const webkit = doc as FullscreenDocument
  return doc.fullscreenElement ?? webkit.webkitFullscreenElement ?? null
}

export function requestDocumentFullscreen(element: Element): Promise<void> {
  const target = element as FullscreenElement
  const request = element.requestFullscreen?.bind(element) ?? target.webkitRequestFullscreen?.bind(target)
  if (!request) {
    return Promise.reject(new Error('fullscreen unsupported'))
  }
  return Promise.resolve(request())
}

export function exitDocumentFullscreen(doc: Document = document): Promise<void> {
  const webkit = doc as FullscreenDocument
  const exit = doc.exitFullscreen?.bind(doc) ?? webkit.webkitExitFullscreen?.bind(webkit)
  if (!exit) {
    return Promise.resolve()
  }
  return Promise.resolve(exit())
}

export function toggleDocumentFullscreen(element: Element, doc: Document = document): Promise<void> {
  if (documentFullscreenElement(doc)) {
    return exitDocumentFullscreen(doc)
  }
  return requestDocumentFullscreen(element)
}

export function shouldClosePlayerOnEscape(nativeFullscreen: boolean, documentFullscreen: boolean): boolean {
  return !nativeFullscreen && !documentFullscreen
}
