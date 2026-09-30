import { useSyncExternalStore } from 'react'

// Set once any API call returns 401; App then shows the SessionLost screen.
let lost = false
const listeners = new Set<() => void>()

export function markSessionLost() {
  if (lost) return
  lost = true
  listeners.forEach((l) => l())
}

export function useSessionLost() {
  return useSyncExternalStore(
    (l) => {
      listeners.add(l)
      return () => listeners.delete(l)
    },
    () => lost,
  )
}
