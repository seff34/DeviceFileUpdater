import '@testing-library/jest-dom/vitest'
import { afterEach, vi } from 'vitest'

// mockApi spies on fetch; never let it leak into the next test.
afterEach(() => {
  vi.restoreAllMocks()
})

// jsdom logs "Not implemented" for scrollTo; the router calls it on navigate.
window.scrollTo = () => {}
