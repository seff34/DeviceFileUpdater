import '@testing-library/jest-dom/vitest'
import { afterEach, vi } from 'vitest'

// mockApi spies on fetch and installFakeEventSource stubs EventSource; never let either leak into the next test.
afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

// jsdom logs "Not implemented" for scrollTo; the router calls it on navigate.
window.scrollTo = () => {}
