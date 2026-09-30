import '@testing-library/jest-dom/vitest'

// jsdom logs "Not implemented" for scrollTo; the router calls it on navigate.
window.scrollTo = () => {}
